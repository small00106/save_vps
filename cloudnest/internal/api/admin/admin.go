package admin

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/cloudnest/cloudnest/internal/audit"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

// GetAuditLogs handles GET /api/admin/audit
func GetAuditLogs(c *gin.Context) {
	limit := 200
	if rawLimit := c.Query("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 500 {
		limit = 500
	}

	query := dbcore.DB().Model(&models.AuditLog{})
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var logs []models.AuditLog
	query.Order("created_at DESC").Limit(limit).Find(&logs)
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

	keys := make([]string, 0, len(req))
	for key := range req {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	audit.LogRequest(c, audit.Entry{
		Action:     "settings_updated",
		Actor:      audit.UsernameFromContext(c),
		Status:     audit.StatusSuccess,
		TargetType: "settings",
		Detail:     fmt.Sprintf("Updated settings: %v", keys),
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
