package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

var adminTestDBOnce sync.Once

func initAdminTestDB(t *testing.T) {
	t.Helper()

	adminTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-admin-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	if err := dbcore.DB().Exec("DELETE FROM audit_logs").Error; err != nil {
		t.Fatalf("clear audit_logs: %v", err)
	}
}

func TestGetAuditLogsSupportsFiltersAndLimit(t *testing.T) {
	initAdminTestDB(t)

	logs := []models.AuditLog{
		{
			Action:     "login_success",
			Actor:      "admin",
			Status:     "success",
			TargetType: "auth",
			IP:         "203.0.113.20",
			CreatedAt:  time.Now().Add(-2 * time.Minute),
		},
		{
			Action:     "login_success",
			Actor:      "admin",
			Status:     "success",
			TargetType: "auth",
			IP:         "203.0.113.21",
			CreatedAt:  time.Now().Add(-1 * time.Minute),
		},
		{
			Action:     "login_failed",
			Actor:      "admin",
			Status:     "failed",
			TargetType: "auth",
			IP:         "203.0.113.22",
			CreatedAt:  time.Now(),
		},
	}
	if err := dbcore.DB().Create(&logs).Error; err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/audit?action=login_success&status=success&limit=1", nil)

	GetAuditLogs(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var resp []models.AuditLog
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(resp))
	}
	if resp[0].IP != "203.0.113.21" {
		t.Fatalf("expected latest matching log first, got IP %q", resp[0].IP)
	}
	if resp[0].Action != "login_success" || resp[0].Status != "success" {
		t.Fatalf("expected filtered login_success/success log, got action=%q status=%q", resp[0].Action, resp[0].Status)
	}
}
