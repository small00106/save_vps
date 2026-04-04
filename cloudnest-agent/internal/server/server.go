package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudnest/cloudnest-agent/internal/terminal"
	"github.com/gin-gonic/gin"
)

// OnFileStored is called after a file is successfully uploaded.
// Set by agent.go to notify master via WS.
var OnFileStored func(fileID string)

// Start launches the data plane HTTP server.
func Start(addr string, rateLimit int64) error {
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

		// Determine the signed identifier: file_id param, path query, or id query
		fileID := c.Param("file_id")
		if fileID == "" {
			fileID = c.Query("path")
		}
		if fileID == "" {
			fileID = c.Query("id")
		}

		if fileID == "" || expires == "" || sig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing signature params"})
			return
		}

		if !validateSignature(fileID, c.Request.Method, expires, sig) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid or expired signature"})
			return
		}

		c.Next()
	}
}

func handleUpload(c *gin.Context) {
	fileID := c.Param("file_id")

	dir := filepath.Join("/data/files", fileID[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	filePath := filepath.Join(dir, fileID)
	f, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create file"})
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}

	// Notify master that file is stored
	if OnFileStored != nil {
		go OnFileStored(fileID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "stored", "file_id": fileID})
}

func handleDownload(c *gin.Context) {
	fileID := c.Param("file_id")

	filePath := filepath.Join("/data/files", fileID[:2], fileID)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.File(filePath)
}

func handleBrowseDownload(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		c.JSON(http.StatusForbidden, gin.H{"error": "path traversal not allowed"})
		return
	}

	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.File(cleanPath)
}
