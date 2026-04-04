package nodes

import (
	"net/http"
	"sort"
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
		if cached, found := cache.FileTreeCache.Get("metric:" + node.UUID); found {
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
	if cached, found := cache.FileTreeCache.Get("metric:" + uuid); found {
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
		// Long ranges should include compacted history + recent raw data (last 4h)
		// to avoid a visibility gap before compaction catches up.
		recentSince := time.Now().Add(-4 * time.Hour)
		if since.After(recentSince) {
			recentSince = since
		}
		compactEnd := recentSince.Truncate(15 * time.Minute)

		var compacted []models.NodeMetricCompact
		compactQuery := dbcore.DB().Where("node_uuid = ? AND bucket_time > ?", uuid, since)
		if compactEnd.After(since) {
			compactQuery = compactQuery.Where("bucket_time < ?", compactEnd)
		}
		compactQuery.Order("bucket_time ASC").Find(&compacted)

		var recentRaw []models.NodeMetric
		dbcore.DB().Where("node_uuid = ? AND timestamp >= ?", uuid, recentSince).
			Order("timestamp ASC").Find(&recentRaw)
		recentBuckets := aggregateRawToCompact(recentRaw)

		merged := make([]models.NodeMetricCompact, 0, len(compacted)+len(recentBuckets))
		merged = append(merged, compacted...)
		merged = append(merged, recentBuckets...)
		c.JSON(http.StatusOK, merged)
	}
}

func aggregateRawToCompact(raw []models.NodeMetric) []models.NodeMetricCompact {
	if len(raw) == 0 {
		return []models.NodeMetricCompact{}
	}

	type agg struct {
		cpuSum, memSum, diskSum float64
		netInSum, netOutSum     int64
		count                   int64
	}

	nodeUUID := raw[0].NodeUUID
	buckets := make(map[time.Time]*agg)
	for _, m := range raw {
		t := m.Timestamp.Truncate(15 * time.Minute)
		a, ok := buckets[t]
		if !ok {
			a = &agg{}
			buckets[t] = a
		}
		a.cpuSum += m.CPUPercent
		a.memSum += m.MemPercent
		a.diskSum += m.DiskPercent
		a.netInSum += m.NetInSpeed
		a.netOutSum += m.NetOutSpeed
		a.count++
	}

	times := make([]time.Time, 0, len(buckets))
	for t := range buckets {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	out := make([]models.NodeMetricCompact, 0, len(times))
	for _, t := range times {
		a := buckets[t]
		if a.count == 0 {
			continue
		}
		n := float64(a.count)
		out = append(out, models.NodeMetricCompact{
			NodeUUID:    nodeUUID,
			CPUPercent:  a.cpuSum / n,
			MemPercent:  a.memSum / n,
			DiskPercent: a.diskSum / n,
			NetInSpeed:  a.netInSum / a.count,
			NetOutSpeed: a.netOutSum / a.count,
			BucketTime:  t,
		})
	}

	return out
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
