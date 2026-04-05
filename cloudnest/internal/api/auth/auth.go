package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /api/auth/login
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := dbcore.DB().Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

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
		"token":    token,
		"username": user.Username,
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
	c.JSON(http.StatusOK, gin.H{"username": user.Username})
}

// EnsureDefaultAdmin creates a default admin user if none exists.
func EnsureDefaultAdmin() {
	var count int64
	dbcore.DB().Model(&models.User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	user := models.User{
		Username:     "admin",
		PasswordHash: string(hash),
	}
	dbcore.DB().Create(&user)
}

func isHTTPSRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}
