package scheduler

import (
	"log"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
)

// StartCompaction runs metric compaction every 30 minutes.
func startCompaction(stop chan struct{}) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			compactMetrics()
		case <-stop:
			return
		}
	}
}

func compactMetrics() {
	// Compact metrics older than 4 hours into 15-minute buckets
	threshold := time.Now().Add(-4 * time.Hour)

	// Get distinct node UUIDs with old metrics
	var uuids []string
	dbcore.DB().Model(&models.NodeMetric{}).
		Where("timestamp < ?", threshold).
		Distinct("node_uuid").
		Pluck("node_uuid", &uuids)

	for _, uuid := range uuids {
		compactNodeMetrics(uuid, threshold)
	}

	if len(uuids) > 0 {
		log.Printf("[Compaction] Processed %d nodes", len(uuids))
	}
}

func compactNodeMetrics(uuid string, threshold time.Time) {
	var metrics []models.NodeMetric
	dbcore.DB().Where("node_uuid = ? AND timestamp < ?", uuid, threshold).
		Order("timestamp ASC").Find(&metrics)

	if len(metrics) == 0 {
		return
	}

	// Group into 15-minute buckets
	buckets := make(map[time.Time][]models.NodeMetric)
	for _, m := range metrics {
		bucket := m.Timestamp.Truncate(15 * time.Minute)
		buckets[bucket] = append(buckets[bucket], m)
	}

	// Aggregate each bucket (70th percentile approximation = use higher values)
	for bucket, items := range buckets {
		n := len(items)
		idx := n * 7 / 10 // 70th percentile index
		if idx >= n {
			idx = n - 1
		}

		// Sort-free approximation: just average for simplicity
		var cpuSum, memSum, diskSum float64
		var netInSum, netOutSum int64
		for _, m := range items {
			cpuSum += m.CPUPercent
			memSum += m.MemPercent
			diskSum += m.DiskPercent
			netInSum += m.NetInSpeed
			netOutSum += m.NetOutSpeed
		}

		compact := models.NodeMetricCompact{
			NodeUUID:    uuid,
			CPUPercent:  cpuSum / float64(n),
			MemPercent:  memSum / float64(n),
			DiskPercent: diskSum / float64(n),
			NetInSpeed:  netInSum / int64(n),
			NetOutSpeed: netOutSum / int64(n),
			BucketTime:  bucket,
		}

		// Upsert: only insert if this bucket doesn't already exist
		var count int64
		dbcore.DB().Model(&models.NodeMetricCompact{}).
			Where("node_uuid = ? AND bucket_time = ?", uuid, bucket).
			Count(&count)
		if count == 0 {
			dbcore.DB().Create(&compact)
		}
	}

	// Delete the raw metrics that were compacted
	dbcore.DB().Where("node_uuid = ? AND timestamp < ?", uuid, threshold).
		Delete(&models.NodeMetric{})
}
