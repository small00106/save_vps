package scheduler

import (
	"log"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
)

var stopCh chan struct{}

func StartAll() {
	stopCh = make(chan struct{})

	go metricFlusher(stopCh)
	go healthChecker(stopCh)
	go startCompaction(stopCh)
	go startAlertEvaluator(stopCh)

	log.Println("[Scheduler] All background tasks started")
}

func StopAll() {
	close(stopCh)
	log.Println("[Scheduler] All background tasks stopped")
}

// metricFlusher flushes cached metrics to DB every 60 seconds.
func metricFlusher(stop chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			items := cache.MetricsCache.Items()
			for key, item := range items {
				if metric, ok := item.Object.(*models.NodeMetric); ok {
					dbcore.DB().Create(metric)
					cache.MetricsCache.Delete(key)
				}
			}
		case <-stop:
			return
		}
	}
}

// healthChecker marks nodes as offline if no heartbeat for 30s.
func healthChecker(stop chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			threshold := time.Now().Add(-30 * time.Second)
			dbcore.DB().Model(&models.Node{}).
				Where("status = ? AND last_seen < ?", "online", threshold).
				Update("status", "offline")
		case <-stop:
			return
		}
	}
}
