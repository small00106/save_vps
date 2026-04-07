package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/gin-gonic/gin"
)

var registerTestDBOnce sync.Once

func initRegisterTestDB(t *testing.T) {
	t.Helper()

	registerTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		if dbcore.DB() != nil {
			return
		}
		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-register-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	if err := dbcore.DB().Exec("DELETE FROM nodes").Error; err != nil {
		t.Fatalf("clear nodes: %v", err)
	}
	SetRegistrationToken("configured-token")
}

func TestRegisterUsesConfiguredRegistrationToken(t *testing.T) {
	initRegisterTestDB(t)

	body, err := json.Marshal(RegisterRequest{Hostname: "node-a"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/agent/register", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer configured-token")
	c.Request.RemoteAddr = "198.51.100.50:8801"

	Register(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterRejectsLegacyDefaultTokenValue(t *testing.T) {
	initRegisterTestDB(t)

	body, err := json.Marshal(RegisterRequest{Hostname: "node-a"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/agent/register", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer cloudnest-register")

	Register(c)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
