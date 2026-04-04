package files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloudnest/cloudnest/internal/api/agent"
	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BrowseNodeFiles handles GET /api/nodes/:uuid/files?path=
func BrowseNodeFiles(c *gin.Context) {
	nodeUUID := c.Param("uuid")
	pathQuery := c.DefaultQuery("path", "/")

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

	// Filter: return direct children of pathQuery
	pathQuery = strings.TrimSuffix(pathQuery, "/")
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
	filePath := c.Query("path")
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
	allowed := false
	for _, e := range entries {
		if e.Path == filePath {
			if e.IsDir {
				c.JSON(http.StatusBadRequest, gin.H{"error": "path is a directory"})
				return
			}
			allowed = true
			break
		}
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in node file tree"})
		return
	}

	// Verify the node exists and is online
	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", nodeUUID, "online").First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	proxyURL := fmt.Sprintf("/api/proxy/browse?node=%s&path=%s", nodeUUID, url.QueryEscape(filePath))
	c.JSON(http.StatusOK, gin.H{"url": proxyURL})
}

// Search handles GET /api/files/search?q=keyword
func Search(c *gin.Context) {
	query := strings.ToLower(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}

	type SearchResult struct {
		NodeUUID string          `json:"node_uuid"`
		Entry    agent.FileEntry `json:"entry"`
	}

	var results []SearchResult

	// Iterate cache items by key to extract UUID directly
	for key, item := range cache.FileTreeCache.Items() {
		if !strings.HasPrefix(key, "filetree:") {
			continue
		}
		nodeUUID := strings.TrimPrefix(key, "filetree:")

		entries, ok := item.Object.([]agent.FileEntry)
		if !ok {
			continue
		}

		for _, e := range entries {
			if strings.Contains(strings.ToLower(e.Name), query) {
				results = append(results, SearchResult{
					NodeUUID: nodeUUID,
					Entry:    e,
				})
			}
		}
	}

	if results == nil {
		results = []SearchResult{}
	}
	c.JSON(http.StatusOK, results)
}

// InitUpload handles POST /api/files/upload
func InitUpload(c *gin.Context) {
	var req struct {
		Name      string   `json:"name" binding:"required"`
		Size      int64    `json:"size"`
		Path      string   `json:"path"`
		NodeUUIDs []string `json:"node_uuids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve online target nodes first. If none are available, fail fast and avoid
	// creating dangling file records stuck in uploading state.
	seen := make(map[string]struct{}, len(req.NodeUUIDs))
	onlineNodes := make([]models.Node, 0, len(req.NodeUUIDs))
	for _, nodeUUID := range req.NodeUUIDs {
		if _, exists := seen[nodeUUID]; exists {
			continue
		}
		seen[nodeUUID] = struct{}{}
		var node models.Node
		if err := dbcore.DB().Where("uuid = ? AND status = ?", nodeUUID, "online").First(&node).Error; err == nil {
			onlineNodes = append(onlineNodes, node)
		}
	}
	if len(onlineNodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no online target nodes"})
		return
	}

	fileID := uuid.New().String()

	// Create file record
	file := models.File{
		FileID: fileID,
		Name:   req.Name,
		Path:   req.Path,
		Size:   req.Size,
		Status: "uploading",
	}
	dbcore.DB().Create(&file)

	// Create replicas and generate signed URLs
	type Target struct {
		NodeUUID string `json:"node_uuid"`
		URL      string `json:"url"`
	}
	var targets []Target

	for _, node := range onlineNodes {
		replica := models.FileReplica{
			FileID:   fileID,
			NodeUUID: node.UUID,
			Status:   "pending",
		}
		dbcore.DB().Create(&replica)

		proxyURL := fmt.Sprintf("/api/proxy/upload/%s?node=%s", fileID, node.UUID)

		targets = append(targets, Target{
			NodeUUID: node.UUID,
			URL:      proxyURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id": fileID,
		"targets": targets,
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

	proxyURL := fmt.Sprintf("/api/proxy/download/%s?node=%s", fileID, replica.NodeUUID)

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
		params, _ := json.Marshal(map[string]string{"file_id": fileID})
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

	updates := map[string]interface{}{}
	if req.NewPath != "" {
		updates["path"] = req.NewPath
	}
	if req.NewName != "" {
		updates["name"] = req.NewName
	}

	dbcore.DB().Model(&models.File{}).Where("file_id = ?", fileID).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "file moved"})
}

func parentPath(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}
