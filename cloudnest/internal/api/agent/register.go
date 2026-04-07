package agent

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var registrationToken string

func SetRegistrationToken(token string) {
	registrationToken = strings.TrimSpace(token)
}

// RegisterRequest is the body for agent registration.
type RegisterRequest struct {
	Hostname  string `json:"hostname"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Region    string `json:"region"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUModel  string `json:"cpu_model"`
	CPUCores  int    `json:"cpu_cores"`
	DiskTotal int64  `json:"disk_total"`
	RAMTotal  int64  `json:"ram_total"`
	Version   string `json:"version"`
}

// Register handles POST /api/agent/register
func Register(c *gin.Context) {
	// Validate registration token
	if registrationToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registration token is not configured"})
		return
	}

	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != registrationToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid registration token"})
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate UUID and token
	nodeUUID := uuid.New().String()
	agentToken, err := generateToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	node := models.Node{
		UUID:      nodeUUID,
		Token:     agentToken,
		Hostname:  req.Hostname,
		IP:        req.IP,
		Port:      req.Port,
		Region:    req.Region,
		OS:        req.OS,
		Arch:      req.Arch,
		CPUModel:  req.CPUModel,
		CPUCores:  req.CPUCores,
		DiskTotal: req.DiskTotal,
		RAMTotal:  req.RAMTotal,
		Version:   req.Version,
		Status:    "online",
		Tags:      "[]",
		LastSeen:  time.Now(),
	}

	// Auto-detect IP from request if not provided
	if node.IP == "" {
		node.IP = c.ClientIP()
	}

	if node.Port == 0 {
		node.Port = 8801
	}

	if err := dbcore.DB().Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uuid":  nodeUUID,
		"token": agentToken,
	})
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
