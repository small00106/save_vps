package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

var websocketFileTestDBOnce sync.Once

func initWebsocketFileTestDB(t *testing.T) {
	t.Helper()

	websocketFileTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		cache.Init()

		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-websocket-file-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	db := dbcore.DB()
	if err := db.Exec("DELETE FROM file_replicas").Error; err != nil {
		t.Fatalf("clear file_replicas: %v", err)
	}
	if err := db.Exec("DELETE FROM files").Error; err != nil {
		t.Fatalf("clear files: %v", err)
	}
	cache.Init()
}

func TestHandleFileStoredPersistsStorePath(t *testing.T) {
	initWebsocketFileTestDB(t)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-1",
		Name:   "report.txt",
		Path:   "/docs",
		Status: "uploading",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:   "file-1",
		NodeUUID: "node-1",
		Status:   "pending",
	}).Error; err != nil {
		t.Fatalf("create replica: %v", err)
	}

	params, err := json.Marshal(map[string]string{
		"file_id":       "file-1",
		"store_path":    "/srv/save_vps/files/docs/report.txt",
		"relative_path": "/docs/report.txt",
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	handleFileStored("node-1", params)

	var replica models.FileReplica
	if err := db.Where("file_id = ? AND node_uuid = ?", "file-1", "node-1").First(&replica).Error; err != nil {
		t.Fatalf("load replica: %v", err)
	}
	if replica.StorePath != "/srv/save_vps/files/docs/report.txt" {
		t.Fatalf("expected store_path persisted, got %q", replica.StorePath)
	}
	if replica.Status != "stored" {
		t.Fatalf("expected status stored, got %q", replica.Status)
	}

	var file models.File
	if err := db.Where("file_id = ?", "file-1").First(&file).Error; err != nil {
		t.Fatalf("load file: %v", err)
	}
	if file.Status != "ready" {
		t.Fatalf("expected file status ready, got %q", file.Status)
	}
}

func TestHandleFileStoredMarksReadyWhenOtherReplicasAreLost(t *testing.T) {
	initWebsocketFileTestDB(t)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-2",
		Name:   "report.txt",
		Path:   "/docs",
		Status: "uploading",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:   "file-2",
		NodeUUID: "node-1",
		Status:   "pending",
	}).Error; err != nil {
		t.Fatalf("create pending replica: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:   "file-2",
		NodeUUID: "node-2",
		Status:   "lost",
	}).Error; err != nil {
		t.Fatalf("create lost replica: %v", err)
	}

	params, err := json.Marshal(map[string]string{
		"file_id":    "file-2",
		"store_path": "/srv/save_vps/files/docs/report.txt",
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	handleFileStored("node-1", params)

	var file models.File
	if err := db.Where("file_id = ?", "file-2").First(&file).Error; err != nil {
		t.Fatalf("load file: %v", err)
	}
	if file.Status != "ready" {
		t.Fatalf("expected file status ready, got %q", file.Status)
	}
}
