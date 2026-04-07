package files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	stdpath "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/api/agent"
	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/transfer"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BrowseNodeFiles handles GET /api/nodes/:uuid/files?path=
func BrowseNodeFiles(c *gin.Context) {
	nodeUUID := c.Param("uuid")
	pathQuery := normalizeNodePath(c.DefaultQuery("path", "/"))

	raw, found := cache.FileTreeCache.Get("filetree:" + nodeUUID)
	if !found {
		c.JSON(http.StatusOK, []agent.FileEntry{})
		return
	}

	entries, ok := raw.([]agent.FileEntry)
	if !ok {
		c.JSON(http.StatusOK, []agent.FileEntry{})
		return
	}

	var result []agent.FileEntry
	for _, e := range entries {
		parent := parentPath(e.Path)
		if parent == pathQuery {
			result = append(result, e)
		}
	}

	if result == nil {
		result = []agent.FileEntry{}
	}
	c.JSON(http.StatusOK, result)
}

// NodeDownloadURL handles GET /api/nodes/:uuid/download?path=
func NodeDownloadURL(c *gin.Context) {
	nodeUUID := c.Param("uuid")
	filePath := normalizeNodePath(c.Query("path"))
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	// Only allow downloading files that are present in the node's reported file tree.
	// This prevents signing arbitrary paths outside scanned directories.
	raw, found := cache.FileTreeCache.Get("filetree:" + nodeUUID)
	if !found {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node file tree unavailable"})
		return
	}
	entries, ok := raw.([]agent.FileEntry)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node file tree unavailable"})
		return
	}
	var matched *agent.FileEntry
	for _, e := range entries {
		if e.Path == filePath {
			entry := e
			matched = &entry
			break
		}
	}
	if matched == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in node file tree"})
		return
	}

	// Verify the node exists and is online
	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", nodeUUID, "online").First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	filename := baseNameFromPath(filePath, "download")
	proxyURL := fmt.Sprintf("/api/proxy/browse?node=%s&path=%s&filename=%s", nodeUUID, url.QueryEscape(filePath), url.QueryEscape(filename))
	if matched.IsDir {
		filename += ".zip"
		proxyURL += "&archive=zip"
	}
	c.JSON(http.StatusOK, gin.H{
		"url":      proxyURL,
		"filename": filename,
	})
}

// Search handles GET /api/files/search?q=keyword
func Search(c *gin.Context) {
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}

	like := "%" + query + "%"
	var files []models.File
	if err := dbcore.DB().
		Where("status = ? AND is_dir = ? AND (LOWER(name) LIKE ? OR LOWER(path) LIKE ?)", "ready", false, like, like).
		Order("path ASC, name ASC").
		Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	if files == nil {
		files = []models.File{}
	}
	c.JSON(http.StatusOK, files)
}

// InitUpload handles POST /api/files/upload
func InitUpload(c *gin.Context) {
	var req struct {
		NodeUUID  string   `json:"node_uuid"`
		NodeUUIDs []string `json:"node_uuids"`
		Name      string   `json:"name" binding:"required"`
		Size      int64    `json:"size"`
		Path      string   `json:"path"`
		Overwrite bool     `json:"overwrite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodeUUID := strings.TrimSpace(req.NodeUUID)
	if nodeUUID == "" && len(req.NodeUUIDs) > 0 {
		nodeUUID = strings.TrimSpace(req.NodeUUIDs[0])
	}
	if nodeUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_uuid required"})
		return
	}

	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", nodeUUID, "online").First(&node).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no online target nodes"})
		return
	}

	req.Path = normalizeNodePath(req.Path)
	logicalPath := logicalManagedFilePath(req.Path, req.Name)
	cacheConflict := nodeTreeContainsPath(nodeUUID, logicalPath)
	existingFileID, exists := findNodeFileConflict(nodeUUID, req.Path, req.Name)
	if (exists || cacheConflict) && !req.Overwrite {
		c.JSON(http.StatusConflict, gin.H{
			"error": "file already exists",
			"conflict": gin.H{
				"node_uuid": nodeUUID,
				"path":      req.Path,
				"name":      req.Name,
			},
		})
		return
	}

	fileID := existingFileID
	if fileID == "" {
		fileID = uuid.New().String()
	}

	proxyURL := fmt.Sprintf(
		"/api/proxy/upload/%s?node=%s&path=%s&name=%s&overwrite=%t",
		fileID,
		node.UUID,
		url.QueryEscape(req.Path),
		url.QueryEscape(req.Name),
		req.Overwrite,
	)

	c.JSON(http.StatusOK, gin.H{
		"file_id": fileID,
		"url":     proxyURL,
		"targets": []gin.H{{
			"node_uuid": node.UUID,
			"url":       proxyURL,
		}},
	})
}

// GetDownloadURL handles GET /api/files/download/:id
func GetDownloadURL(c *gin.Context) {
	fileID := c.Param("id")

	var file models.File
	if err := dbcore.DB().Where("file_id = ? AND status != ?", fileID, "deleting").First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// Find a stored replica on an online node.
	var replica models.FileReplica
	if err := dbcore.DB().
		Table("file_replicas").
		Select("file_replicas.*").
		Joins("JOIN nodes ON nodes.uuid = file_replicas.node_uuid").
		Where("file_replicas.file_id = ? AND file_replicas.status = ? AND nodes.status = ?", fileID, "stored", "online").
		Order("file_replicas.id ASC").
		First(&replica).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no available replica"})
		return
	}

	// Just need to verify the replica node is online; the proxy handler will do the actual signing.
	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", replica.NodeUUID, "online").First(&node).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "replica node is offline"})
		return
	}

	managedPath := logicalManagedFilePath(file.Path, file.Name)
	filename := url.QueryEscape(baseNameFromPath(file.Name, fileID))
	proxyURL := fmt.Sprintf("/api/proxy/browse?node=%s&path=%s&filename=%s", replica.NodeUUID, url.QueryEscape(managedPath), filename)

	c.JSON(http.StatusOK, gin.H{
		"url":      proxyURL,
		"filename": file.Name,
		"size":     file.Size,
	})
}

// ListFiles handles GET /api/files?path=
func ListFiles(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	var files []models.File
	dbcore.DB().Where("path = ? AND status != ?", path, "deleting").Find(&files)
	c.JSON(http.StatusOK, files)
}

// CreateDir handles POST /api/files/mkdir
func CreateDir(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dir := models.File{
		FileID: uuid.New().String(),
		Name:   req.Name,
		Path:   req.Path,
		IsDir:  true,
		Status: "ready",
	}
	dbcore.DB().Create(&dir)
	c.JSON(http.StatusOK, dir)
}

// DeleteFile handles DELETE /api/files/:id
func DeleteFile(c *gin.Context) {
	fileID := c.Param("id")

	// Mark file as deleting (don't soft delete yet — wait for agent confirmations)
	dbcore.DB().Model(&models.File{}).Where("file_id = ?", fileID).Update("status", "deleting")

	// Find replicas and send delete commands to online agents
	var replicas []models.FileReplica
	dbcore.DB().Where("file_id = ?", fileID).Find(&replicas)

	hub := ws.GetHub()
	for _, r := range replicas {
		params, _ := json.Marshal(map[string]string{
			"file_id":    fileID,
			"store_path": r.StorePath,
		})
		hub.SendToAgent(r.NodeUUID, &ws.RPCMessage{
			JSONRPC: "2.0",
			Method:  "master.deleteFile",
			Params:  params,
		})
	}

	// Don't soft delete here. handleFileDeleted will remove replicas one by one.
	// Once all replicas are gone, the GC task will soft delete the file record.

	c.JSON(http.StatusOK, gin.H{"message": "file deletion initiated"})
}

// MoveFile handles PUT /api/files/:id/move
func MoveFile(c *gin.Context) {
	fileID := c.Param("id")

	var req struct {
		NewPath string `json:"new_path"`
		NewName string `json:"new_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var file models.File
	if err := dbcore.DB().Where("file_id = ? AND status != ?", fileID, "deleting").First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	newPath := normalizeNodePath(file.Path)
	if strings.TrimSpace(req.NewPath) != "" {
		newPath = normalizeNodePath(req.NewPath)
	}
	newName := strings.TrimSpace(file.Name)
	if strings.TrimSpace(req.NewName) != "" {
		newName = strings.TrimSpace(req.NewName)
	}

	fromPath := logicalManagedFilePath(file.Path, file.Name)
	toPath := logicalManagedFilePath(newPath, newName)
	if fromPath == toPath {
		c.JSON(http.StatusOK, gin.H{"message": "file moved"})
		return
	}

	var replicas []models.FileReplica
	if err := dbcore.DB().
		Where("file_id = ? AND status = ?", fileID, "stored").
		Find(&replicas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query replicas"})
		return
	}
	if len(replicas) != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "move only supported for a single stored replica"})
		return
	}

	replica := replicas[0]
	node, err := resolveNode(replica.NodeUUID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "replica node is offline"})
		return
	}

	moveResult, err := moveManagedFileOnNode(c, node, fromPath, toPath)
	if err != nil {
		status := http.StatusBadGateway
		if moveErr, ok := err.(*agentMoveError); ok {
			status = moveErr.StatusCode
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if err := dbcore.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.File{}).
			Where("file_id = ?", fileID).
			Updates(map[string]interface{}{
				"path": newPath,
				"name": newName,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.FileReplica{}).
			Where("file_id = ? AND node_uuid = ?", fileID, replica.NodeUUID).
			Update("store_path", moveResult.StorePath).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update moved file metadata"})
		return
	}

	agent.RemoveFileTreeEntry(replica.NodeUUID, fromPath)
	agent.UpsertFileTreeEntry(replica.NodeUUID, agent.FileEntry{
		Path:    moveResult.RelativePath,
		Name:    newName,
		Size:    file.Size,
		IsDir:   false,
		ModTime: moveResult.ModTime,
	})

	c.JSON(http.StatusOK, gin.H{"message": "file moved"})
}

func parentPath(path string) string {
	path = normalizeNodePath(path)
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func normalizeNodePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := stdpath.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	return cleaned
}

func findNodeFileConflict(nodeUUID, path, name string) (string, bool) {
	var existing struct {
		FileID string
	}
	err := dbcore.DB().
		Table("files").
		Select("files.file_id").
		Joins("JOIN file_replicas ON file_replicas.file_id = files.file_id").
		Where("file_replicas.node_uuid = ? AND files.path = ? AND files.name = ? AND files.status != ?", nodeUUID, path, name, "deleting").
		Order("files.created_at DESC").
		First(&existing).Error
	if err != nil {
		return "", false
	}
	return existing.FileID, true
}

func nodeTreeContainsPath(nodeUUID, fullPath string) bool {
	raw, found := cache.FileTreeCache.Get("filetree:" + nodeUUID)
	if !found {
		return false
	}
	entries, ok := raw.([]agent.FileEntry)
	if !ok {
		return false
	}
	for _, entry := range entries {
		if entry.Path == fullPath {
			return true
		}
	}
	return false
}

func logicalManagedFilePath(dirPath, name string) string {
	cleanName := strings.TrimSpace(name)
	cleanName = strings.ReplaceAll(cleanName, "\\", "/")
	cleanName = stdpath.Base("/" + strings.TrimPrefix(cleanName, "/"))
	cleanName = strings.TrimPrefix(cleanName, "/")
	if cleanName == "" || cleanName == "." {
		return normalizeNodePath(dirPath)
	}
	dir := normalizeNodePath(dirPath)
	if dir == "/" {
		return "/" + cleanName
	}
	return dir + "/" + cleanName
}

func baseNameFromPath(p string, fallback string) string {
	name := strings.TrimSpace(p)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "download"
	}
	name = strings.TrimRight(name, `/\`)
	if name == "" {
		return "download"
	}
	if idx := strings.LastIndexAny(name, `/\`); idx >= 0 && idx < len(name)-1 {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "download"
	}
	return name
}

type agentMoveResult struct {
	RelativePath string    `json:"relative_path"`
	StorePath    string    `json:"store_path"`
	ModTime      time.Time `json:"mod_time"`
}

type agentMoveError struct {
	StatusCode int
	Message    string
}

func (e *agentMoveError) Error() string {
	return e.Message
}

func moveManagedFileOnNode(c *gin.Context, node *models.Node, fromPath, toPath string) (*agentMoveResult, error) {
	agentBase := fmt.Sprintf("http://%s:%d/api/files/move", node.IP, node.Port)
	agentURL := transfer.GenerateSignedURL(agentBase, signedMoveResource(fromPath, toPath), http.MethodPost, 5*time.Minute)
	agentURL += "&from=" + url.QueryEscape(fromPath)
	agentURL += "&to=" + url.QueryEscape(toPath)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, agentURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		message := "agent move failed"
		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && strings.TrimSpace(payload["error"]) != "" {
			message = payload["error"]
		}
		return nil, &agentMoveError{StatusCode: resp.StatusCode, Message: message}
	}

	var result agentMoveResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.StorePath) != "" {
		result.StorePath = filepath.ToSlash(strings.TrimSpace(result.StorePath))
	}
	return &result, nil
}
