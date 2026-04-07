package cache

import (
	"testing"

	"github.com/cloudnest/cloudnest/internal/database/models"
)

func TestMetricRingBufferDropsOldestWhenFull(t *testing.T) {
	defer resetMetricsBufferForTest(maxBufferedMetrics)
	resetMetricsBufferForTest(3)

	if dropped := PushMetric(&models.NodeMetric{CPUPercent: 1}); dropped != 0 {
		t.Fatalf("expected no drop on first push, got %d", dropped)
	}
	PushMetric(&models.NodeMetric{CPUPercent: 2})
	PushMetric(&models.NodeMetric{CPUPercent: 3})
	if dropped := PushMetric(&models.NodeMetric{CPUPercent: 4}); dropped != 1 {
		t.Fatalf("expected one dropped sample on overflow, got %d", dropped)
	}

	items, dropped := DrainMetrics()
	if dropped != 1 {
		t.Fatalf("expected dropped count 1, got %d", dropped)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(items))
	}
	if items[0].CPUPercent != 2 || items[1].CPUPercent != 3 || items[2].CPUPercent != 4 {
		t.Fatalf("expected oldest metric dropped, got %+v", []float64{items[0].CPUPercent, items[1].CPUPercent, items[2].CPUPercent})
	}
}

func TestPushMetricsReturnsDroppedCount(t *testing.T) {
	defer resetMetricsBufferForTest(maxBufferedMetrics)
	resetMetricsBufferForTest(2)

	metrics := []*models.NodeMetric{
		{CPUPercent: 1},
		{CPUPercent: 2},
		{CPUPercent: 3},
	}
	if dropped := PushMetrics(metrics); dropped != 1 {
		t.Fatalf("expected one dropped sample, got %d", dropped)
	}

	items, _ := DrainMetrics()
	if len(items) != 2 {
		t.Fatalf("expected 2 metrics after overflow, got %d", len(items))
	}
	if items[0].CPUPercent != 2 || items[1].CPUPercent != 3 {
		t.Fatalf("unexpected metrics order after overflow: %+v", []float64{items[0].CPUPercent, items[1].CPUPercent})
	}
}
