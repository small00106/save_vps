package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

// AuthRequired validates session token from cookie or Authorization header.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var session models.Session
		if err := dbcore.DB().Where("token = ? AND expires_at > ?", token, time.Now()).First(&session).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
			return
		}

		c.Set("user_id", session.UserID)
		c.Next()
	}
}

// AgentAuthRequired validates agent token from Authorization header.
func AgentAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var node models.Node
		if err := dbcore.DB().Where("token = ?", token).First(&node).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}

		c.Set("node_uuid", node.UUID)
		c.Set("node", &node)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	// Try cookie first
	if token, err := c.Cookie("session"); err == nil && token != "" {
		return token
	}
	// Fall back to Bearer token
	return extractBearerToken(c)
}

func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
