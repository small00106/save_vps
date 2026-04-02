package nodes

import (
	"net/http"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

// List handles GET /api/nodes
func List(c *gin.Context) {
	var nodes []models.Node
	dbcore.DB().Order("created_at ASC").Find(&nodes)

	// Enrich with latest cached metrics
	type NodeWithMetrics struct {
		models.Node
		LatestMetric *models.NodeMetric `json:"latest_metric,omitempty"`
	}

	result := make([]NodeWithMetrics, 0, len(nodes))
	for _, node := range nodes {
		nwm := NodeWithMetrics{Node: node}
		if cached, found := cache.MetricsCache.Get("metric:" + node.UUID); found {
			if m, ok := cached.(*models.NodeMetric); ok {
				nwm.LatestMetric = m
			}
		}
		result = append(result, nwm)
	}

	c.JSON(http.StatusOK, result)
}

// Get handles GET /api/nodes/:uuid
func Get(c *gin.Context) {
	uuid := c.Param("uuid")

	var node models.Node
	if err := dbcore.DB().Where("uuid = ?", uuid).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	// Include latest metric
	var latestMetric *models.NodeMetric
	if cached, found := cache.MetricsCache.Get("metric:" + uuid); found {
		if m, ok := cached.(*models.NodeMetric); ok {
			latestMetric = m
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"node":          node,
		"latest_metric": latestMetric,
	})
}

// GetMetrics handles GET /api/nodes/:uuid/metrics?range=1h|4h|24h|7d
func GetMetrics(c *gin.Context) {
	uuid := c.Param("uuid")
	rangeStr := c.DefaultQuery("range", "1h")

	var since time.Time
	switch rangeStr {
	case "4h":
		since = time.Now().Add(-4 * time.Hour)
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	default: // 1h
		since = time.Now().Add(-1 * time.Hour)
	}

	// For ranges <= 4h, use raw metrics; for longer ranges, use compacted
	if rangeStr == "1h" || rangeStr == "4h" {
		var metrics []models.NodeMetric
		dbcore.DB().Where("node_uuid = ? AND timestamp > ?", uuid, since).
			Order("timestamp ASC").Find(&metrics)
		c.JSON(http.StatusOK, metrics)
	} else {
		var metrics []models.NodeMetricCompact
		dbcore.DB().Where("node_uuid = ? AND bucket_time > ?", uuid, since).
			Order("bucket_time ASC").Find(&metrics)
		c.JSON(http.StatusOK, metrics)
	}
}

// GetTraffic handles GET /api/nodes/:uuid/traffic
func GetTraffic(c *gin.Context) {
	uuid := c.Param("uuid")

	// Get latest metric for current totals
	var latest models.NodeMetric
	dbcore.DB().Where("node_uuid = ?", uuid).Order("timestamp DESC").First(&latest)

	c.JSON(http.StatusOK, gin.H{
		"net_in_total":  latest.NetInTotal,
		"net_out_total": latest.NetOutTotal,
		"net_in_speed":  latest.NetInSpeed,
		"net_out_speed": latest.NetOutSpeed,
	})
}

// UpdateTags handles PUT /api/nodes/:uuid/tags
func UpdateTags(c *gin.Context) {
	uuid := c.Param("uuid")

	var req struct {
		Tags string `json:"tags"` // JSON array string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := dbcore.DB().Model(&models.Node{}).Where("uuid = ?", uuid).
		Update("tags", req.Tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tags updated"})
}
