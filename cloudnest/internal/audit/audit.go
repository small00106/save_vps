package audit

import (
	"log"
	"strconv"
	"strings"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

const (
	ActorSystem   = "system"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusInfo    = "info"
)

type Entry struct {
	Action     string
	Actor      string
	Status     string
	TargetType string
	TargetID   string
	NodeUUID   string
	Detail     string
	IP         string
}

func Log(entry Entry) {
	if err := Write(entry); err != nil {
		log.Printf("[Audit] failed to write %q: %v", entry.Action, err)
	}
}

func LogRequest(c *gin.Context, entry Entry) {
	if c != nil && strings.TrimSpace(entry.IP) == "" {
		entry.IP = c.ClientIP()
	}
	Log(entry)
}

func Write(entry Entry) error {
	action := strings.TrimSpace(entry.Action)
	if action == "" {
		return nil
	}

	actor := strings.TrimSpace(entry.Actor)
	if actor == "" {
		actor = ActorSystem
	}

	status := strings.TrimSpace(entry.Status)
	if status == "" {
		status = StatusInfo
	}

	record := models.AuditLog{
		Action:     action,
		Actor:      actor,
		Status:     status,
		TargetType: strings.TrimSpace(entry.TargetType),
		TargetID:   strings.TrimSpace(entry.TargetID),
		NodeUUID:   strings.TrimSpace(entry.NodeUUID),
		Detail:     strings.TrimSpace(entry.Detail),
		IP:         strings.TrimSpace(entry.IP),
	}
	return dbcore.DB().Create(&record).Error
}

func UsernameByID(userID uint) string {
	if userID == 0 {
		return ""
	}

	var user models.User
	if err := dbcore.DB().Select("username").First(&user, userID).Error; err != nil {
		return ""
	}
	return user.Username
}

func UsernameFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}

	userIDValue, ok := c.Get("user_id")
	if !ok {
		return ""
	}

	switch value := userIDValue.(type) {
	case uint:
		return UsernameByID(value)
	case int:
		if value > 0 {
			return UsernameByID(uint(value))
		}
	case int64:
		if value > 0 {
			return UsernameByID(uint(value))
		}
	}

	return ""
}

func TargetIDFromUint(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
