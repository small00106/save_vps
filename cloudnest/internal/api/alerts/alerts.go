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

	var rule models.AlertRule
	if err := dbcore.DB().First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dbcore.DB().Save(&rule)
	c.JSON(http.StatusOK, rule)
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

	var channel models.AlertChannel
	if err := dbcore.DB().First(&channel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dbcore.DB().Save(&channel)
	c.JSON(http.StatusOK, channel)
}
