package admin

import (
	"net/http"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

// GetAuditLogs handles GET /api/admin/audit
func GetAuditLogs(c *gin.Context) {
	var logs []models.AuditLog
	dbcore.DB().Order("created_at DESC").Limit(200).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

// GetSettings handles GET /api/admin/settings
func GetSettings(c *gin.Context) {
	// Return basic system stats
	var nodeCount, fileCount int64
	dbcore.DB().Model(&models.Node{}).Count(&nodeCount)
	dbcore.DB().Model(&models.File{}).Count(&fileCount)

	var onlineCount int64
	dbcore.DB().Model(&models.Node{}).Where("status = ?", "online").Count(&onlineCount)

	c.JSON(http.StatusOK, gin.H{
		"node_count":   nodeCount,
		"online_count": onlineCount,
		"file_count":   fileCount,
	})
}
