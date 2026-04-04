package scheduler

import (
	"encoding/json"
	"log"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
)

func startGC(stop chan struct{}) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gcDeletingFiles()
			gcOfflineNodeCache()
		case <-stop:
			return
		}
	}
}

// gcDeletingFiles retries delete for "deleting" files and cleans up fully-deleted ones.
func gcDeletingFiles() {
	var files []models.File
	dbcore.DB().Where("status = ?", "deleting").Find(&files)

	hub := ws.GetHub()

	for _, file := range files {
		var replicas []models.FileReplica
		dbcore.DB().Where("file_id = ?", file.FileID).Find(&replicas)

		if len(replicas) == 0 {
			// All replicas confirmed deleted, safe to soft delete the file record
			dbcore.DB().Where("file_id = ?", file.FileID).Delete(&models.File{})
			log.Printf("[GC] Cleaned up file %s", file.FileID)
			continue
		}

		// Retry sending delete to agents that still have replicas
		for _, r := range replicas {
			params, _ := json.Marshal(map[string]string{
				"file_id":    file.FileID,
				"store_path": r.StorePath,
			})
			hub.SendToAgent(r.NodeUUID, &ws.RPCMessage{
				JSONRPC: "2.0",
				Method:  "master.deleteFile",
				Params:  params,
			})
		}
	}
}

// gcOfflineNodeCache clears stale cache entries for nodes that have been offline for a while.
func gcOfflineNodeCache() {
	var offlineNodes []models.Node
	threshold := time.Now().Add(-5 * time.Minute)
	dbcore.DB().Where("status = ? AND last_seen < ?", "offline", threshold).Find(&offlineNodes)

	for _, node := range offlineNodes {
		cache.FileTreeCache.Delete("filetree:" + node.UUID)
		cache.FileTreeCache.Delete("metric:" + node.UUID)
	}
}
