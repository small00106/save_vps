package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/notify"
)

func startAlertEvaluator(stop chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			evaluateAlerts()
		case <-stop:
			return
		}
	}
}

func evaluateAlerts() {
	var rules []models.AlertRule
	dbcore.DB().Where("enabled = ?", true).Find(&rules)

	for _, rule := range rules {
		evaluateRule(&rule)
	}
}

func evaluateRule(rule *models.AlertRule) {
	// Get nodes to check
	var nodes []models.Node
	if rule.NodeUUID != "" {
		dbcore.DB().Where("uuid = ?", rule.NodeUUID).Find(&nodes)
	} else {
		dbcore.DB().Find(&nodes)
	}

	for _, node := range nodes {
		triggered := false

		switch rule.Metric {
		case "offline":
			triggered = node.Status == "offline"
		case "cpu", "mem", "disk":
			triggered = checkMetricThreshold(node.UUID, rule)
		}

		if triggered {
			// Check cooldown (don't fire more than once per Duration)
			if !rule.LastFiredAt.IsZero() && time.Since(rule.LastFiredAt) < time.Duration(rule.Duration)*time.Second {
				continue
			}
			fireAlert(rule, &node)
		}
	}
}

func checkMetricThreshold(uuid string, rule *models.AlertRule) bool {
	cached, found := cache.MetricsCache.Get("metric:" + uuid)
	if !found {
		return false
	}
	metric, ok := cached.(*models.NodeMetric)
	if !ok {
		return false
	}

	var value float64
	switch rule.Metric {
	case "cpu":
		value = metric.CPUPercent
	case "mem":
		value = metric.MemPercent
	case "disk":
		value = metric.DiskPercent
	}

	switch rule.Operator {
	case "gt":
		return value > rule.Threshold
	case "lt":
		return value < rule.Threshold
	}
	return false
}

func fireAlert(rule *models.AlertRule, node *models.Node) {
	// Get channel
	var channel models.AlertChannel
	if err := dbcore.DB().First(&channel, rule.ChannelID).Error; err != nil {
		log.Printf("[Alert] Channel %d not found for rule %d", rule.ChannelID, rule.ID)
		return
	}

	// Build message
	title := fmt.Sprintf("[CloudNest] %s", rule.Name)
	message := fmt.Sprintf("Node: %s (%s)\nMetric: %s %s %.1f%%\nTime: %s",
		node.Hostname, node.UUID,
		rule.Metric, rule.Operator, rule.Threshold,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// Send notification
	sender, err := notify.NewSender(channel.Type, channel.Config)
	if err != nil {
		log.Printf("[Alert] Failed to create sender: %v", err)
		return
	}

	if err := sender.Send(title, message); err != nil {
		log.Printf("[Alert] Failed to send notification: %v", err)
		return
	}

	// Update last fired time
	dbcore.DB().Model(rule).Update("last_fired_at", time.Now())

	// Audit log
	dbcore.DB().Create(&models.AuditLog{
		Action: "alert_fired",
		Detail: fmt.Sprintf("Rule '%s' fired for node %s", rule.Name, node.Hostname),
	})

	log.Printf("[Alert] Fired: %s for node %s", rule.Name, node.Hostname)
}
