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

func TestHandleFileStoredCreatesManagedMetadataAndCacheEntry(t *testing.T) {
	initWebsocketFileTestDB(t)

	params, err := json.Marshal(map[string]interface{}{
		"file_id":       "file-3",
		"store_path":    "/srv/save_vps/files/docs/report.txt",
		"relative_path": "/docs/report.txt",
		"size":          int64(42),
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	handleFileStored("node-1", params)

	var file models.File
	if err := dbcore.DB().Where("file_id = ?", "file-3").First(&file).Error; err != nil {
		t.Fatalf("load file: %v", err)
	}
	if file.Path != "/docs" || file.Name != "report.txt" {
		t.Fatalf("expected file metadata from relative path, got path=%q name=%q", file.Path, file.Name)
	}
	if file.Size != 42 {
		t.Fatalf("expected file size 42, got %d", file.Size)
	}
	if file.Status != "ready" {
		t.Fatalf("expected file ready, got %q", file.Status)
	}

	var replica models.FileReplica
	if err := dbcore.DB().Where("file_id = ? AND node_uuid = ?", "file-3", "node-1").First(&replica).Error; err != nil {
		t.Fatalf("load replica: %v", err)
	}
	if replica.Status != "stored" {
		t.Fatalf("expected replica stored, got %q", replica.Status)
	}
	if replica.StorePath != "/srv/save_vps/files/docs/report.txt" {
		t.Fatalf("expected store_path persisted, got %q", replica.StorePath)
	}

	raw, found := cache.FileTreeCache.Get("filetree:node-1")
	if !found {
		t.Fatal("expected file tree cache entry to be created")
	}
	entries, ok := raw.([]FileEntry)
	if !ok {
		t.Fatalf("expected cached file tree entries, got %T", raw)
	}
	if len(entries) != 1 || entries[0].Path != "/docs/report.txt" {
		t.Fatalf("expected cached entry for /docs/report.txt, got %#v", entries)
	}
}

func TestHandleFileStoredMarksOtherReplicasLostAfterOverwrite(t *testing.T) {
	initWebsocketFileTestDB(t)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-4",
		Name:   "report.txt",
		Path:   "/docs",
		Size:   10,
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-4",
		NodeUUID:  "node-1",
		Status:    "stored",
		StorePath: "/srv/save_vps/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create primary replica: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-4",
		NodeUUID:  "node-2",
		Status:    "stored",
		StorePath: "/srv/save_vps/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create secondary replica: %v", err)
	}

	params, err := json.Marshal(map[string]interface{}{
		"file_id":       "file-4",
		"store_path":    "/srv/save_vps/files/docs/report.txt",
		"relative_path": "/docs/report.txt",
		"size":          int64(99),
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	handleFileStored("node-1", params)

	var staleReplica models.FileReplica
	if err := db.Where("file_id = ? AND node_uuid = ?", "file-4", "node-2").First(&staleReplica).Error; err != nil {
		t.Fatalf("load stale replica: %v", err)
	}
	if staleReplica.Status != "lost" {
		t.Fatalf("expected stale replica to be marked lost, got %q", staleReplica.Status)
	}
}
