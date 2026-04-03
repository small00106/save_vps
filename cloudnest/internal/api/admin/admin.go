package admin

import (
	"fmt"
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
	var nodeCount, fileCount int64
	dbcore.DB().Model(&models.Node{}).Count(&nodeCount)
	dbcore.DB().Model(&models.File{}).Count(&fileCount)

	var onlineCount int64
	dbcore.DB().Model(&models.Node{}).Where("status = ?", "online").Count(&onlineCount)

	// Load persisted settings
	settings := loadSettings()
	settings["node_count"] = nodeCount
	settings["online_count"] = onlineCount
	settings["file_count"] = fileCount

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings handles PUT /api/admin/settings
func UpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for key, value := range req {
		dbcore.DB().Where("key = ?", key).Assign(models.Setting{Key: key, Value: toString(value)}).FirstOrCreate(&models.Setting{})
	}

	dbcore.DB().Create(&models.AuditLog{
		Action: "settings_updated",
		Detail: "System settings updated",
		IP:     c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

func loadSettings() gin.H {
	var settings []models.Setting
	dbcore.DB().Find(&settings)
	result := gin.H{}
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
