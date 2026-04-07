package auth

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminUsername                 = "admin"
	defaultAdminPassword                 = "admin"
	defaultPasswordNoticeAcknowledgedKey = "auth.default_admin_notice_acknowledged_at"
	defaultLoginFailureLimit             = 5
	defaultLoginFailureWindow            = 5 * time.Minute
)

type loginFailureState struct {
	count int
	first time.Time
}

type rateLimitDecision struct {
	limited    bool
	retryAfter int
}

type loginFailureLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	state  map[string]loginFailureState
}

var loginRateLimiter = newLoginRateLimiter(defaultLoginFailureLimit, defaultLoginFailureWindow, time.Now)

func newLoginRateLimiter(limit int, window time.Duration, now func() time.Time) *loginFailureLimiter {
	if now == nil {
		now = time.Now
	}
	return &loginFailureLimiter{
		limit:  limit,
		window: window,
		now:    now,
		state:  make(map[string]loginFailureState),
	}
}

func (l *loginFailureLimiter) recordFailure(key string) rateLimitDecision {
	if l == nil || key == "" || l.limit <= 0 || l.window <= 0 {
		return rateLimitDecision{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry, ok := l.state[key]
	if !ok || now.Sub(entry.first) >= l.window {
		entry = loginFailureState{
			count: 1,
			first: now,
		}
		l.state[key] = entry
		return rateLimitDecision{}
	}

	entry.count++
	l.state[key] = entry

	if entry.count <= l.limit {
		return rateLimitDecision{}
	}

	retryAfter := int(math.Ceil(l.window.Seconds() - now.Sub(entry.first).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return rateLimitDecision{limited: true, retryAfter: retryAfter}
}

func (l *loginFailureLimiter) clear(key string) {
	if l == nil || key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, key)
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type authUserResponse struct {
	Username                      string `json:"username"`
	DefaultPasswordNoticeRequired bool   `json:"default_password_notice_required"`
}

// Login handles POST /api/auth/login
func Login(c *gin.Context) {
	clientIP := c.ClientIP()
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := dbcore.DB().Where("username = ?", req.Username).First(&user).Error; err != nil {
		if decision := loginRateLimiter.recordFailure(clientIP); decision.limited {
			c.Header("Retry-After", strconv.Itoa(decision.retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed login attempts"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		if decision := loginRateLimiter.recordFailure(clientIP); decision.limited {
			c.Header("Retry-After", strconv.Itoa(decision.retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed login attempts"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	loginRateLimiter.clear(clientIP)

	defaultPasswordNoticeRequired := shouldRequireDefaultPasswordNotice(&user)

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate session"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	session := models.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	dbcore.DB().Create(&session)

	secureCookie := isHTTPSRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session", token, 30*24*3600, "/", "", secureCookie, true)
	c.JSON(http.StatusOK, gin.H{
		"token":                            token,
		"username":                         user.Username,
		"default_password_notice_required": defaultPasswordNoticeRequired,
	})
}

// Logout handles POST /api/auth/logout
func Logout(c *gin.Context) {
	token, _ := c.Cookie("session")
	if token == "" {
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token != "" {
		dbcore.DB().Where("token = ?", token).Delete(&models.Session{})
	}
	secureCookie := isHTTPSRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session", "", -1, "/", "", secureCookie, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me handles GET /api/auth/me
func Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User
	if err := dbcore.DB().First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, authUserResponse{
		Username:                      user.Username,
		DefaultPasswordNoticeRequired: shouldRequireDefaultPasswordNotice(&user),
	})
}

// AcknowledgeDefaultPasswordNotice handles POST /api/auth/default-password-notice/ack
func AcknowledgeDefaultPasswordNotice(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var user models.User
	if err := dbcore.DB().First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	if user.Username != defaultAdminUsername {
		c.JSON(http.StatusOK, gin.H{"message": "default password notice not applicable"})
		return
	}

	setting := models.Setting{
		Key:   defaultPasswordNoticeAcknowledgedKey,
		Value: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := dbcore.DB().
		Where("key = ?", defaultPasswordNoticeAcknowledgedKey).
		Assign(setting).
		FirstOrCreate(&models.Setting{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acknowledge default password notice"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "default password notice acknowledged"})
}

// ChangePassword handles POST /api/auth/change-password
func ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := dbcore.DB().First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := dbcore.DB().Model(&user).Update("password_hash", string(newHash)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}

// EnsureDefaultAdmin creates a default admin user if none exists.
func EnsureDefaultAdmin() {
	var count int64
	dbcore.DB().Model(&models.User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	user := models.User{
		Username:     defaultAdminUsername,
		PasswordHash: string(hash),
	}
	dbcore.DB().Create(&user)
}

func shouldRequireDefaultPasswordNotice(user *models.User) bool {
	if user == nil || user.Username != defaultAdminUsername {
		return false
	}

	var acknowledgedCount int64
	if err := dbcore.DB().Model(&models.Setting{}).
		Where("key = ?", defaultPasswordNoticeAcknowledgedKey).
		Count(&acknowledgedCount).Error; err != nil {
		return false
	}
	if acknowledgedCount > 0 {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(defaultAdminPassword)) == nil
}

func isHTTPSRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}
