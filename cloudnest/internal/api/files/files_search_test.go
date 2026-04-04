package files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

var searchTestDBOnce sync.Once

func initSearchTestDB(t *testing.T) {
	t.Helper()

	searchTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		cache.Init()

		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-files-search-%d.db", os.Getpid()))
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
}

func TestSearchReturnsManagedFilesByName(t *testing.T) {
	initSearchTestDB(t)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-1",
		Name:   "monthly-report.pdf",
		Path:   "/docs",
		Size:   128,
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/files/search?q=report", nil)
	c.Request = req

	Search(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var results []models.File
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].FileID != "file-1" {
		t.Fatalf("expected file-1, got %q", results[0].FileID)
	}
	if results[0].Name != "monthly-report.pdf" {
		t.Fatalf("expected monthly-report.pdf, got %q", results[0].Name)
	}
}

func TestSearchMatchesPathAndExcludesNonReadyFiles(t *testing.T) {
	initSearchTestDB(t)

	db := dbcore.DB()
	files := []models.File{
		{
			FileID: "file-2",
			Name:   "notes.txt",
			Path:   "/projects/alpha",
			Size:   64,
			Status: "ready",
		},
		{
			FileID: "file-3",
			Name:   "alpha-draft.txt",
			Path:   "/projects/alpha",
			Size:   64,
			Status: "uploading",
		},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create files: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/files/search?q=alpha", nil)
	c.Request = req

	Search(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var results []models.File
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 ready result, got %d", len(results))
	}
	if results[0].FileID != "file-2" {
		t.Fatalf("expected file-2, got %q", results[0].FileID)
	}
}
