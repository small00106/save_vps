package auth

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
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var authTestDBOnce sync.Once

func initAuthTestDB(t *testing.T) {
	t.Helper()

	authTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)

		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-auth-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	db := dbcore.DB()
	if err := db.Exec("DELETE FROM sessions").Error; err != nil {
		t.Fatalf("clear sessions: %v", err)
	}
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		t.Fatalf("clear users: %v", err)
	}
}

func createAuthTestUser(t *testing.T, username, password string) models.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Username:     username,
		PasswordHash: string(hash),
	}
	if err := dbcore.DB().Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestChangePasswordUpdatesPasswordHash(t *testing.T) {
	initAuthTestDB(t)
	user := createAuthTestUser(t, "admin", "old-password")

	body, err := json.Marshal(map[string]string{
		"current_password": "old-password",
		"new_password":     "new-password-123",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", user.ID)

	ChangePassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var updated models.User
	if err := dbcore.DB().First(&updated, user.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("new-password-123")); err != nil {
		t.Fatalf("expected new password to match: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("old-password")); err == nil {
		t.Fatalf("expected old password to be rejected")
	}
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	initAuthTestDB(t)
	user := createAuthTestUser(t, "admin", "old-password")

	body, err := json.Marshal(map[string]string{
		"current_password": "wrong-password",
		"new_password":     "new-password-123",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", user.ID)

	ChangePassword(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}

	var unchanged models.User
	if err := dbcore.DB().First(&unchanged, user.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(unchanged.PasswordHash), []byte("old-password")); err != nil {
		t.Fatalf("expected old password to remain valid: %v", err)
	}
}
