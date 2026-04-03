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
			triggered = checkSustainedThreshold(node.UUID, rule)
		}

		if triggered {
			// Cooldown: don't fire again within 2x duration
			cooldown := time.Duration(rule.Duration*2) * time.Second
			if cooldown < 5*time.Minute {
				cooldown = 5 * time.Minute
			}
			if !rule.LastFiredAt.IsZero() && time.Since(rule.LastFiredAt) < cooldown {
				continue
			}
			fireAlert(rule, &node)
		}
	}
}

// checkSustainedThreshold checks if metric exceeded threshold for the entire duration window.
func checkSustainedThreshold(uuid string, rule *models.AlertRule) bool {
	since := time.Now().Add(-time.Duration(rule.Duration) * time.Second)
	var metrics []models.NodeMetric
	dbcore.DB().Where("node_uuid = ? AND timestamp > ?", uuid, since).
		Order("timestamp ASC").Find(&metrics)

	// Also include the latest real-time cached metric (not yet flushed to DB)
	if cached, found := cache.FileTreeCache.Get("metric:" + uuid); found {
		if m, ok := cached.(*models.NodeMetric); ok && m.Timestamp.After(since) {
			metrics = append(metrics, *m)
		}
	}

	// Need at least 2 samples to confirm "sustained"
	if len(metrics) < 2 {
		return false
	}

	// The samples must span at least half the duration to count as "sustained"
	earliest := metrics[0].Timestamp
	latest := metrics[len(metrics)-1].Timestamp
	if latest.Sub(earliest) < time.Duration(rule.Duration/2)*time.Second {
		return false
	}

	for _, m := range metrics {
		var value float64
		switch rule.Metric {
		case "cpu":
			value = m.CPUPercent
		case "mem":
			value = m.MemPercent
		case "disk":
			value = m.DiskPercent
		}

		exceeded := false
		switch rule.Operator {
		case "gt":
			exceeded = value > rule.Threshold
		case "lt":
			exceeded = value < rule.Threshold
		}

		if !exceeded {
			return false // any sample below threshold = not sustained
		}
	}

	return true
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
