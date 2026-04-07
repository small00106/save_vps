package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/cloudnest/cloudnest-agent/internal/terminal"
	"github.com/gin-gonic/gin"
)

// OnFileStored is called after a file is successfully uploaded.
// Set by agent.go to notify master via WS.
type StoredFileEvent struct {
	FileID       string
	RelativePath string
	StorePath    string
	Size         int64
	ModTime      time.Time
}

var OnFileStored func(event StoredFileEvent)

// Start launches the data plane HTTP server.
func Start(addr string, rateLimit int64) error {
	if err := storage.EnsureDataDirs(); err != nil {
		return err
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if rateLimit > 0 {
		r.Use(RateLimitMiddleware(rateLimit))
	}

	api := r.Group("/api")
	{
		api.PUT("/files/:file_id", validateSignedURL(), handleUpload)
		api.GET("/files/:file_id", validateSignedURL(), handleDownload)
		api.POST("/files/move", validateSignedURL(), handleMove)
		api.GET("/browse", validateSignedURL(), handleBrowseDownload)
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		api.GET("/terminal", validateSignedURL(), terminal.HandleTerminal)
	}

	return r.Run(addr)
}

func validateSignedURL() gin.HandlerFunc {
	return func(c *gin.Context) {
		expires := c.Query("expires")
		sig := c.Query("sig")

		resourceID := c.Param("file_id")
		if resourceID != "" && c.Request.Method == http.MethodPut {
			resourceID = signedUploadResource(resourceID, c.Query("path"), c.Query("name"), c.Query("overwrite"))
		}
		if resourceID == "" {
			resourceID = c.Query("path")
		}
		if resourceID == "" && c.Request.Method == http.MethodPost && c.FullPath() == "/api/files/move" {
			resourceID = signedMoveResource(c.Query("from"), c.Query("to"))
		}
		if resourceID == "" {
			resourceID = c.Query("id")
		}

		if resourceID == "" || expires == "" || sig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing signature params"})
			return
		}

		if !validateSignature(resourceID, c.Request.Method, expires, sig) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid or expired signature"})
			return
		}

		c.Next()
	}
}

func handleUpload(c *gin.Context) {
	fileID := c.Param("file_id")
	if len(fileID) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}

	name := strings.TrimSpace(c.Query("name"))
	overwrite := strings.EqualFold(c.Query("overwrite"), "true")

	filePath := ""
	relativePath := ""
	var err error
	if name == "" {
		_, err = storage.EnsureShardDir(fileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
			return
		}
		filePath, err = storage.FilePath(fileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve storage path"})
			return
		}
	} else {
		filePath, relativePath, err = storage.JoinManagedFilePath(c.DefaultQuery("path", "/"), name)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
			return
		}
		if !overwrite {
			if _, err := os.Stat(filePath); err == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "file already exists"})
				return
			} else if !os.IsNotExist(err) {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat target"})
				return
			}
		}
	}

	info, err := writeUploadedFile(filePath, overwrite || name == "", c.Request.Body)
	if err != nil {
		switch {
		case os.IsExist(err):
			c.JSON(http.StatusConflict, gin.H{"error": "file already exists"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		}
		return
	}

	// Notify master that file is stored
	if OnFileStored != nil {
		go OnFileStored(StoredFileEvent{
			FileID:       fileID,
			RelativePath: relativePath,
			StorePath:    filePath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "stored",
		"file_id":       fileID,
		"relative_path": relativePath,
		"store_path":    filePath,
		"size":          info.Size(),
		"mod_time":      info.ModTime(),
	})
}

func handleMove(c *gin.Context) {
	fromPath := strings.TrimSpace(c.Query("from"))
	toPath := strings.TrimSpace(c.Query("to"))
	if fromPath == "" || toPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}

	sourceAbs, sourceRel, err := storage.ResolveManagedPath(fromPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sourceInfo, err := os.Stat(sourceAbs)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "source file not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat source file"})
		return
	}
	if sourceInfo.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "directory move is not supported"})
		return
	}

	targetPath := storage.NormalizeManagedDirPath(toPath)
	targetDir := path.Dir(targetPath)
	targetName := path.Base(targetPath)
	if targetName == "." || targetName == "/" || targetPath == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target path"})
		return
	}

	targetAbs, targetRel, err := storage.JoinManagedFilePath(targetDir, targetName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if sourceRel == targetRel {
		c.JSON(http.StatusOK, gin.H{
			"relative_path": targetRel,
			"store_path":    sourceAbs,
			"mod_time":      sourceInfo.ModTime(),
		})
		return
	}
	if _, err := os.Stat(targetAbs); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "target file already exists"})
		return
	} else if !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat target file"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create target directory"})
		return
	}

	if err := os.Rename(sourceAbs, targetAbs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to move file"})
		return
	}

	targetInfo, err := os.Stat(targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat moved file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"relative_path": targetRel,
		"store_path":    targetAbs,
		"mod_time":      targetInfo.ModTime(),
	})
}

func handleDownload(c *gin.Context) {
	fileID := c.Param("file_id")
	if len(fileID) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}

	filePath, err := storage.FilePath(fileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	downloadName := sanitizeAttachmentName(c.Query("filename"), fileID)
	c.FileAttachment(filePath, downloadName)
}

func handleBrowseDownload(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	cleanPath, _, err := storage.ResolveManagedPath(path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat path"})
		return
	}

	if info.IsDir() {
		base := sanitizeAttachmentName(info.Name(), "download")
		downloadName := sanitizeAttachmentName(c.Query("filename"), base+".zip")
		streamZipDirectory(c, cleanPath, downloadName)
		return
	}

	defaultName := sanitizeAttachmentName(filepath.Base(cleanPath), "download")
	downloadName := sanitizeAttachmentName(c.Query("filename"), defaultName)
	c.FileAttachment(cleanPath, downloadName)
}

func streamZipDirectory(c *gin.Context, dirPath, downloadName string) {
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
	c.Status(http.StatusOK)

	zw := zip.NewWriter(c.Writer)
	if err := filepath.Walk(dirPath, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, current)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}); err != nil {
		c.Error(err)
	}
	if err := zw.Close(); err != nil {
		c.Error(err)
	}
}

func sanitizeAttachmentName(name, fallback string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = strings.TrimSpace(fallback)
	}
	if n == "" {
		n = "download"
	}
	n = filepath.Base(n)
	if n == "." || n == string(filepath.Separator) || n == "" {
		return "download"
	}
	return n
}

func writeUploadedFile(targetPath string, overwrite bool, body io.Reader) (os.FileInfo, error) {
	parentDir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(parentDir, ".cloudnest-upload-*")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	cleanup := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}

	if err := tempFile.Chmod(0644); err != nil && runtime.GOOS != "windows" {
		cleanup()
		return nil, err
	}

	if _, err := io.Copy(tempFile, body); err != nil {
		cleanup()
		return nil, err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	if !overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			_ = os.Remove(tempPath)
			return nil, os.ErrExist
		} else if !os.IsNotExist(err) {
			_ = os.Remove(tempPath)
			return nil, err
		}
	}

	if err := replaceFile(tempPath, targetPath, overwrite); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	return os.Stat(targetPath)
}

func replaceFile(tempPath, targetPath string, overwrite bool) error {
	if !overwrite {
		return os.Rename(tempPath, targetPath)
	}

	if runtime.GOOS == "windows" {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(tempPath, targetPath)
}
