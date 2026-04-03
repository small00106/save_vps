package alerts

import (
	"net/http"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

// ListRules handles GET /api/alerts/rules
func ListRules(c *gin.Context) {
	var rules []models.AlertRule
	dbcore.DB().Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// CreateRule handles POST /api/alerts/rules
func CreateRule(c *gin.Context) {
	var rule models.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dbcore.DB().Create(&rule)
	c.JSON(http.StatusOK, rule)
}

// UpdateRule handles PUT /api/alerts/rules/:id
func UpdateRule(c *gin.Context) {
	id := c.Param("id")

	var existing models.AlertRule
	if err := dbcore.DB().First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// Bind to a map so we only update fields that were actually sent
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Never allow overwriting ID or CreatedAt
	delete(updates, "id")
	delete(updates, "created_at")

	if err := dbcore.DB().Model(&existing).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}

	// Reload and return
	dbcore.DB().First(&existing, id)
	c.JSON(http.StatusOK, existing)
}

// DeleteRule handles DELETE /api/alerts/rules/:id
func DeleteRule(c *gin.Context) {
	id := c.Param("id")
	dbcore.DB().Delete(&models.AlertRule{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListChannels handles GET /api/alerts/channels
func ListChannels(c *gin.Context) {
	var channels []models.AlertChannel
	dbcore.DB().Find(&channels)
	c.JSON(http.StatusOK, channels)
}

// CreateChannel handles POST /api/alerts/channels
func CreateChannel(c *gin.Context) {
	var channel models.AlertChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dbcore.DB().Create(&channel)
	c.JSON(http.StatusOK, channel)
}

// UpdateChannel handles PUT /api/alerts/channels/:id
func UpdateChannel(c *gin.Context) {
	id := c.Param("id")

	var existing models.AlertChannel
	if err := dbcore.DB().First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	delete(updates, "id")

	dbcore.DB().Model(&existing).Updates(updates)
	dbcore.DB().First(&existing, id)
	c.JSON(http.StatusOK, existing)
}
