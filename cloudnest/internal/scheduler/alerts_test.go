package scheduler

import (
	"testing"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/models"
)

func TestCheckSustainedThresholdReturnsTrueWhenWholeWindowExceeded(t *testing.T) {
	now := time.Now()
	rule := &models.AlertRule{
		Metric:    "cpu",
		Operator:  "gt",
		Threshold: 80,
		Duration:  30,
	}
	samples := []models.NodeMetric{
		{CPUPercent: 85, Timestamp: now.Add(-30 * time.Second)},
		{CPUPercent: 90, Timestamp: now.Add(-15 * time.Second)},
		{CPUPercent: 95, Timestamp: now},
	}

	if !checkSustainedThreshold(samples, rule, now) {
		t.Fatal("expected sustained threshold to be true")
	}
}

func TestCheckSustainedThresholdReturnsFalseWhenAnySampleBreaksRule(t *testing.T) {
	now := time.Now()
	rule := &models.AlertRule{
		Metric:    "mem",
		Operator:  "gt",
		Threshold: 70,
		Duration:  20,
	}
	samples := []models.NodeMetric{
		{MemPercent: 75, Timestamp: now.Add(-25 * time.Second)},
		{MemPercent: 65, Timestamp: now.Add(-10 * time.Second)},
		{MemPercent: 80, Timestamp: now.Add(-1 * time.Second)},
	}

	if checkSustainedThreshold(samples, rule, now) {
		t.Fatal("expected sustained threshold to be false")
	}
}

func TestCheckSustainedThresholdReturnsFalseWhenWindowNotCovered(t *testing.T) {
	now := time.Now()
	rule := &models.AlertRule{
		Metric:    "disk",
		Operator:  "gt",
		Threshold: 90,
		Duration:  60,
	}
	samples := []models.NodeMetric{
		{DiskPercent: 95, Timestamp: now.Add(-20 * time.Second)},
		{DiskPercent: 96, Timestamp: now.Add(-5 * time.Second)},
	}

	if checkSustainedThreshold(samples, rule, now) {
		t.Fatal("expected sustained threshold to be false when sample span is shorter than duration")
	}
}
