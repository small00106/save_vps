package scheduler

import (
	"log"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
)

var stopCh chan struct{}

func StartAll() {
	stopCh = make(chan struct{})

	go metricFlusher(stopCh)
	go healthChecker(stopCh)
	go startCompaction(stopCh)
	go startAlertEvaluator(stopCh)
	go startGC(stopCh)

	log.Println("[Scheduler] All background tasks started")
}

func StopAll() {
	close(stopCh)
	log.Println("[Scheduler] All background tasks stopped")
}

// metricFlusher flushes buffered metrics to DB every 60 seconds.
func metricFlusher(stop chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			items := cache.DrainMetrics()
			for _, item := range items {
				if metric, ok := item.(*models.NodeMetric); ok {
					dbcore.DB().Create(metric)
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
			var staleOnline []models.Node
			dbcore.DB().
				Where("status = ? AND last_seen < ?", "online", threshold).
				Find(&staleOnline)
			if len(staleOnline) == 0 {
				continue
			}

			uuids := make([]string, 0, len(staleOnline))
			for _, n := range staleOnline {
				uuids = append(uuids, n.UUID)
			}
			if err := dbcore.DB().Model(&models.Node{}).
				Where("uuid IN ?", uuids).
				Update("status", "offline").Error; err != nil {
				log.Printf("[Scheduler] Failed to mark offline nodes: %v", err)
				continue
			}

			// Push offline status updates so dashboard can refresh without waiting
			// for a reconnect/disconnect event.
			for _, uuid := range uuids {
				ws.GetDashboardHub().Broadcast(map[string]string{
					"type":   "status",
					"node":   uuid,
					"status": "offline",
				})
			}
		case <-stop:
			return
		}
	}
}
