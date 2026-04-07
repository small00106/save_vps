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
	"time"

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
	if err := db.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("clear settings: %v", err)
	}

	loginRateLimiter = newLoginRateLimiter(defaultLoginFailureLimit, defaultLoginFailureWindow, time.Now)
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

func TestLoginReturnsDefaultPasswordNoticeForDefaultAdmin(t *testing.T) {
	initAuthTestDB(t)
	createAuthTestUser(t, "admin", "admin")

	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Login(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Token                         string `json:"token"`
		Username                      string `json:"username"`
		DefaultPasswordNoticeRequired bool   `json:"default_password_notice_required"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.Username != "admin" {
		t.Fatalf("expected username=admin, got %q", resp.Username)
	}
	if resp.Token == "" {
		t.Fatalf("expected session token in login response")
	}
	if !resp.DefaultPasswordNoticeRequired {
		t.Fatalf("expected default password notice to be required")
	}
}

func TestMeReturnsDefaultPasswordNoticeUntilAcknowledged(t *testing.T) {
	initAuthTestDB(t)
	user := createAuthTestUser(t, "admin", "admin")

	meRecorder := httptest.NewRecorder()
	meCtx, _ := gin.CreateTestContext(meRecorder)
	meCtx.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meCtx.Set("user_id", user.ID)

	Me(meCtx)

	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from me, got %d body=%s", meRecorder.Code, meRecorder.Body.String())
	}

	var meResp struct {
		Username                      string `json:"username"`
		DefaultPasswordNoticeRequired bool   `json:"default_password_notice_required"`
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if !meResp.DefaultPasswordNoticeRequired {
		t.Fatalf("expected me to require default password notice before acknowledgement")
	}

	ackRecorder := httptest.NewRecorder()
	ackCtx, _ := gin.CreateTestContext(ackRecorder)
	ackCtx.Request = httptest.NewRequest(http.MethodPost, "/api/auth/default-password-notice/ack", nil)
	ackCtx.Set("user_id", user.ID)

	AcknowledgeDefaultPasswordNotice(ackCtx)

	if ackRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from acknowledge, got %d body=%s", ackRecorder.Code, ackRecorder.Body.String())
	}

	var setting models.Setting
	if err := dbcore.DB().First(&setting, "key = ?", defaultPasswordNoticeAcknowledgedKey).Error; err != nil {
		t.Fatalf("expected acknowledgement setting to exist: %v", err)
	}

	meRecorder = httptest.NewRecorder()
	meCtx, _ = gin.CreateTestContext(meRecorder)
	meCtx.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meCtx.Set("user_id", user.ID)

	Me(meCtx)

	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from me after acknowledge, got %d body=%s", meRecorder.Code, meRecorder.Body.String())
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("decode me response after acknowledge: %v", err)
	}
	if meResp.DefaultPasswordNoticeRequired {
		t.Fatalf("expected me to stop requiring default password notice after acknowledgement")
	}
}

func TestMeDoesNotRequireDefaultPasswordNoticeAfterPasswordChange(t *testing.T) {
	initAuthTestDB(t)
	user := createAuthTestUser(t, "admin", "changed-password")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	c.Set("user_id", user.ID)

	Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		DefaultPasswordNoticeRequired bool `json:"default_password_notice_required"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if resp.DefaultPasswordNoticeRequired {
		t.Fatalf("expected changed password to suppress default password notice")
	}
}

func TestLoginRateLimitBlocksSixthFailureWithinWindow(t *testing.T) {
	initAuthTestDB(t)
	createAuthTestUser(t, "admin", "admin")

	now := time.Now()
	loginRateLimiter = newLoginRateLimiter(5, 5*time.Minute, func() time.Time { return now })

	for attempt := 1; attempt <= 5; attempt++ {
		recorder := performLoginRequest(t, "198.51.100.10:1234", "admin", "wrong-password")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d body=%s", attempt, recorder.Code, recorder.Body.String())
		}
	}

	recorder := performLoginRequest(t, "198.51.100.10:1234", "admin", "wrong-password")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected sixth failure to return 429, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if retryAfter := recorder.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatalf("expected Retry-After header on rate limited response")
	}
}

func TestLoginRateLimitSuccessClearsFailures(t *testing.T) {
	initAuthTestDB(t)
	createAuthTestUser(t, "admin", "admin")

	loginRateLimiter = newLoginRateLimiter(1, 5*time.Minute, time.Now)

	recorder := performLoginRequest(t, "198.51.100.20:4321", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected first failed login to return 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = performLoginRequest(t, "198.51.100.20:4321", "admin", "admin")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected successful login to return 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = performLoginRequest(t, "198.51.100.20:4321", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected failures to be cleared after success, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginRateLimitIsScopedPerIP(t *testing.T) {
	initAuthTestDB(t)
	createAuthTestUser(t, "admin", "admin")

	loginRateLimiter = newLoginRateLimiter(1, 5*time.Minute, time.Now)

	recorder := performLoginRequest(t, "198.51.100.30:1111", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected first failed login to return 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = performLoginRequest(t, "198.51.100.30:1111", "admin", "wrong-password")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second failure from same IP to be rate limited, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = performLoginRequest(t, "198.51.100.31:2222", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected different IP to remain unaffected, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginRateLimitExpiresAfterWindow(t *testing.T) {
	initAuthTestDB(t)
	createAuthTestUser(t, "admin", "admin")

	now := time.Now()
	loginRateLimiter = newLoginRateLimiter(1, 2*time.Minute, func() time.Time { return now })

	recorder := performLoginRequest(t, "198.51.100.40:5555", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected first failed login to return 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	now = now.Add(3 * time.Minute)
	recorder = performLoginRequest(t, "198.51.100.40:5555", "admin", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected rate limit window to expire, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func performLoginRequest(t *testing.T, remoteAddr, username, password string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = remoteAddr

	Login(c)
	return recorder
}

func TestMeDoesNotRequireDefaultPasswordNoticeForNonAdminUser(t *testing.T) {
	initAuthTestDB(t)
	user := createAuthTestUser(t, "alice", "admin")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	c.Set("user_id", user.ID)

	Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		DefaultPasswordNoticeRequired bool `json:"default_password_notice_required"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if resp.DefaultPasswordNoticeRequired {
		t.Fatalf("expected non-admin user to skip default password notice")
	}
}
