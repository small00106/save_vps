package scheduler

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/cloudnest/cloudnest/internal/audit"
	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/notify"
)

type alertEvaluationContext struct {
	now           time.Time
	rules         []models.AlertRule
	nodes         []models.Node
	nodesByUUID   map[string]models.Node
	channelsByID  map[uint]models.AlertChannel
	metricsByNode map[string][]models.NodeMetric
}

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
	ctx, err := buildAlertEvaluationContext(time.Now())
	if err != nil {
		log.Printf("[Alert] Failed to build evaluation context: %v", err)
		return
	}
	if len(ctx.rules) == 0 {
		return
	}

	for i := range ctx.rules {
		evaluateRule(&ctx.rules[i], ctx)
	}
}

func buildAlertEvaluationContext(now time.Time) (*alertEvaluationContext, error) {
	ctx := &alertEvaluationContext{
		now:           now,
		nodesByUUID:   make(map[string]models.Node),
		channelsByID:  make(map[uint]models.AlertChannel),
		metricsByNode: make(map[string][]models.NodeMetric),
	}

	if err := dbcore.DB().Where("enabled = ?", true).Find(&ctx.rules).Error; err != nil {
		return nil, err
	}
	if len(ctx.rules) == 0 {
		return ctx, nil
	}

	targetNodeSet := make(map[string]struct{})
	channelSet := make(map[uint]struct{})
	hasGlobalRule := false
	maxDuration := 0

	for _, rule := range ctx.rules {
		if rule.NodeUUID == "" {
			hasGlobalRule = true
		} else {
			targetNodeSet[rule.NodeUUID] = struct{}{}
		}
		channelSet[rule.ChannelID] = struct{}{}
		if rule.Metric == "cpu" || rule.Metric == "mem" || rule.Metric == "disk" {
			if rule.Duration > maxDuration {
				maxDuration = rule.Duration
			}
		}
	}

	switch {
	case hasGlobalRule:
		if err := dbcore.DB().Find(&ctx.nodes).Error; err != nil {
			return nil, err
		}
	case len(targetNodeSet) > 0:
		nodeUUIDs := make([]string, 0, len(targetNodeSet))
		for uuid := range targetNodeSet {
			nodeUUIDs = append(nodeUUIDs, uuid)
		}
		if err := dbcore.DB().Where("uuid IN ?", nodeUUIDs).Find(&ctx.nodes).Error; err != nil {
			return nil, err
		}
	}

	for _, node := range ctx.nodes {
		ctx.nodesByUUID[node.UUID] = node
	}

	if len(channelSet) > 0 {
		channelIDs := make([]uint, 0, len(channelSet))
		for id := range channelSet {
			channelIDs = append(channelIDs, id)
		}
		var channels []models.AlertChannel
		if err := dbcore.DB().Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			return nil, err
		}
		for _, ch := range channels {
			ctx.channelsByID[ch.ID] = ch
		}
	}

	if maxDuration <= 0 || len(ctx.nodes) == 0 {
		return ctx, nil
	}

	nodeUUIDs := make([]string, 0, len(ctx.nodes))
	for _, node := range ctx.nodes {
		nodeUUIDs = append(nodeUUIDs, node.UUID)
	}

	since := now.Add(-time.Duration(maxDuration) * time.Second)
	var metrics []models.NodeMetric
	if err := dbcore.DB().
		Where("node_uuid IN ? AND timestamp >= ?", nodeUUIDs, since).
		Order("node_uuid ASC, timestamp ASC").
		Find(&metrics).Error; err != nil {
		return nil, err
	}
	for _, metric := range metrics {
		ctx.metricsByNode[metric.NodeUUID] = append(ctx.metricsByNode[metric.NodeUUID], metric)
	}

	appendLatestCachedMetrics(ctx.metricsByNode, nodeUUIDs, since)
	for nodeUUID, series := range ctx.metricsByNode {
		sort.Slice(series, func(i, j int) bool {
			return series[i].Timestamp.Before(series[j].Timestamp)
		})
		ctx.metricsByNode[nodeUUID] = series
	}

	return ctx, nil
}

func appendLatestCachedMetrics(metricsByNode map[string][]models.NodeMetric, nodeUUIDs []string, since time.Time) {
	for _, nodeUUID := range nodeUUIDs {
		cached, found := cache.FileTreeCache.Get("metric:" + nodeUUID)
		if !found {
			continue
		}
		metric, ok := cached.(*models.NodeMetric)
		if !ok || metric.Timestamp.Before(since) {
			continue
		}

		series := metricsByNode[nodeUUID]
		if len(series) > 0 {
			last := series[len(series)-1]
			if !metric.Timestamp.After(last.Timestamp) {
				alreadyPresent := false
				for _, existing := range series {
					if existing.Timestamp.Equal(metric.Timestamp) {
						alreadyPresent = true
						break
					}
				}
				if alreadyPresent {
					continue
				}
			}
		}
		metricsByNode[nodeUUID] = append(metricsByNode[nodeUUID], *metric)
	}
}

func evaluateRule(rule *models.AlertRule, ctx *alertEvaluationContext) {
	nodes := targetedNodes(rule, ctx)
	if len(nodes) == 0 {
		return
	}

	for _, node := range nodes {
		triggered := false

		switch rule.Metric {
		case "offline":
			triggered = node.Status == "offline"
		case "cpu", "mem", "disk":
			triggered = checkSustainedThreshold(ctx.metricsByNode[node.UUID], rule, ctx.now)
		}

		if !triggered {
			continue
		}

		// Cooldown: don't fire again within 2x duration.
		cooldown := time.Duration(rule.Duration*2) * time.Second
		if cooldown < 5*time.Minute {
			cooldown = 5 * time.Minute
		}
		if !rule.LastFiredAt.IsZero() && time.Since(rule.LastFiredAt) < cooldown {
			continue
		}

		channel, ok := ctx.channelsByID[rule.ChannelID]
		if !ok {
			log.Printf("[Alert] Channel %d not found for rule %d", rule.ChannelID, rule.ID)
			continue
		}
		fireAlert(rule, &node, &channel, ctx.now)
	}
}

func targetedNodes(rule *models.AlertRule, ctx *alertEvaluationContext) []models.Node {
	if rule.NodeUUID == "" {
		return ctx.nodes
	}
	node, ok := ctx.nodesByUUID[rule.NodeUUID]
	if !ok {
		return nil
	}
	return []models.Node{node}
}

// checkSustainedThreshold checks if metric exceeded threshold for the entire duration window.
func checkSustainedThreshold(samples []models.NodeMetric, rule *models.AlertRule, now time.Time) bool {
	if len(samples) < 2 {
		return false
	}

	duration := time.Duration(rule.Duration) * time.Second
	since := now.Add(-duration)

	window := make([]models.NodeMetric, 0, len(samples))
	for _, sample := range samples {
		if !sample.Timestamp.Before(since) {
			window = append(window, sample)
		}
	}

	// Need at least 2 samples to confirm "sustained".
	if len(window) < 2 {
		return false
	}

	// The samples must span at least the full duration to count as "sustained".
	earliest := window[0].Timestamp
	latest := window[len(window)-1].Timestamp
	if latest.Sub(earliest) < duration {
		return false
	}

	for _, sample := range window {
		var value float64
		switch rule.Metric {
		case "cpu":
			value = sample.CPUPercent
		case "mem":
			value = sample.MemPercent
		case "disk":
			value = sample.DiskPercent
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

func fireAlert(rule *models.AlertRule, node *models.Node, channel *models.AlertChannel, now time.Time) {
	// Build message
	title := fmt.Sprintf("[CloudNest] %s", rule.Name)
	message := fmt.Sprintf("Node: %s (%s)\nMetric: %s %s %.1f%%\nTime: %s",
		node.Hostname, node.UUID,
		rule.Metric, rule.Operator, rule.Threshold,
		now.Format("2006-01-02 15:04:05"),
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
	dbcore.DB().Model(rule).Update("last_fired_at", now)

	// Audit log
	audit.Log(audit.Entry{
		Action:     "alert_fired",
		Actor:      audit.ActorSystem,
		Status:     audit.StatusInfo,
		TargetType: "alert_rule",
		TargetID:   audit.TargetIDFromUint(rule.ID),
		NodeUUID:   node.UUID,
		Detail:     fmt.Sprintf("Rule %q fired for node %s", rule.Name, node.Hostname),
	})

	log.Printf("[Alert] Fired: %s for node %s", rule.Name, node.Hostname)
}
