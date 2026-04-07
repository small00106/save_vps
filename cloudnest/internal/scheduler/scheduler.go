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

const metricsInsertBatchSize = 500

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
			items, dropped := cache.DrainMetrics()
			if dropped > 0 {
				log.Printf("[Scheduler] Metric buffer overflow dropped %d samples before flush", dropped)
			}
			if len(items) == 0 {
				continue
			}

			if err := dbcore.DB().CreateInBatches(items, metricsInsertBatchSize).Error; err != nil {
				requeueDropped := cache.PushMetrics(items)
				log.Printf("[Scheduler] Failed to flush %d metrics (requeued): %v", len(items), err)
				if requeueDropped > 0 {
					log.Printf("[Scheduler] Metric buffer overflow dropped %d samples while requeueing failed flush", requeueDropped)
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
			stillConnected := make([]string, 0, len(staleOnline))
			for _, n := range staleOnline {
				// If WS is still connected, treat node as online and refresh last_seen.
				if ws.GetHub().Get(n.UUID) != nil {
					stillConnected = append(stillConnected, n.UUID)
					continue
				}
				uuids = append(uuids, n.UUID)
			}

			if len(stillConnected) > 0 {
				if err := dbcore.DB().Model(&models.Node{}).
					Where("uuid IN ?", stillConnected).
					Update("last_seen", time.Now()).Error; err != nil {
					log.Printf("[Scheduler] Failed to refresh last_seen for connected nodes: %v", err)
				}
			}

			if len(uuids) == 0 {
				continue
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
