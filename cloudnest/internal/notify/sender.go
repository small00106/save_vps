package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
)

// Sender is the interface for notification channels.
type Sender interface {
	Send(title, message string) error
}

// NewSender creates a Sender from channel type and JSON config.
func NewSender(channelType, config string) (Sender, error) {
	switch channelType {
	case "telegram":
		var cfg TelegramConfig
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, err
		}
		return &TelegramSender{cfg}, nil
	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, err
		}
		return &WebhookSender{cfg}, nil
	case "email":
		var cfg EmailConfig
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, err
		}
		return &EmailSender{cfg}, nil
	case "bark":
		var cfg BarkConfig
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, err
		}
		return &BarkSender{cfg}, nil
	default:
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}
}

// === Telegram ===

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type TelegramSender struct {
	cfg TelegramConfig
}

func (s *TelegramSender) Send(title, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.cfg.BotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id": s.cfg.ChatID,
		"text":    fmt.Sprintf("*%s*\n%s", title, message),
		"parse_mode": "Markdown",
	})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// === Webhook ===

type WebhookConfig struct {
	URL string `json:"url"`
}

type WebhookSender struct {
	cfg WebhookConfig
}

func (s *WebhookSender) Send(title, message string) error {
	body, _ := json.Marshal(map[string]string{
		"title":   title,
		"message": message,
	})
	resp, err := http.Post(s.cfg.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// === Email ===

type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort string `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

type EmailSender struct {
	cfg EmailConfig
}

func (s *EmailSender) Send(title, message string) error {
	addr := s.cfg.SMTPHost + ":" + s.cfg.SMTPPort
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.SMTPHost)
	msg := fmt.Sprintf("Subject: %s\r\n\r\n%s", title, message)
	return smtp.SendMail(addr, auth, s.cfg.From, strings.Split(s.cfg.To, ","), []byte(msg))
}

// === Bark ===

type BarkConfig struct {
	ServerURL string `json:"server_url"` // e.g. https://api.day.app/your-key
}

type BarkSender struct {
	cfg BarkConfig
}

func (s *BarkSender) Send(title, message string) error {
	url := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(s.cfg.ServerURL, "/"), title, message)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
