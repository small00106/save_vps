package cache

import (
	"sync"

	"github.com/cloudnest/cloudnest/internal/database/models"
	gocache "github.com/patrickmn/go-cache"
)

var (
	FileTreeCache *gocache.Cache

	metricsMu sync.Mutex

	metricsRingBuffer = newMetricRingBuffer(maxBufferedMetrics)
)

const maxBufferedMetrics = 50000

type metricDrainResult struct {
	Metrics []*models.NodeMetric
	Dropped int64
}

type metricRingBuffer struct {
	items   []*models.NodeMetric
	head    int
	size    int
	dropped int64
}

func newMetricRingBuffer(capacity int) *metricRingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &metricRingBuffer{
		items: make([]*models.NodeMetric, capacity),
	}
}

func (b *metricRingBuffer) push(metric *models.NodeMetric) int64 {
	if metric == nil {
		return 0
	}
	if b.size < len(b.items) {
		idx := (b.head + b.size) % len(b.items)
		b.items[idx] = metric
		b.size++
		return 0
	}

	drop := int64(1)
	b.items[b.head] = metric
	b.head = (b.head + 1) % len(b.items)
	b.dropped += drop
	return drop
}

func (b *metricRingBuffer) drain() metricDrainResult {
	result := metricDrainResult{
		Metrics: make([]*models.NodeMetric, 0, b.size),
		Dropped: b.dropped,
	}
	for i := 0; i < b.size; i++ {
		idx := (b.head + i) % len(b.items)
		result.Metrics = append(result.Metrics, b.items[idx])
		b.items[idx] = nil
	}

	b.head = 0
	b.size = 0
	b.dropped = 0
	return result
}

func Init() {
	FileTreeCache = gocache.New(gocache.NoExpiration, 0)
}

// PushMetric appends a metric to the buffer.
func PushMetric(metric *models.NodeMetric) int64 {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	return metricsRingBuffer.push(metric)
}

// PushMetrics appends a batch of metrics and returns how many items were dropped.
func PushMetrics(metrics []*models.NodeMetric) int64 {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	var dropped int64
	for _, metric := range metrics {
		dropped += metricsRingBuffer.push(metric)
	}
	return dropped
}

// DrainMetrics returns all buffered metrics, how many old samples were dropped, and clears the buffer.
func DrainMetrics() ([]*models.NodeMetric, int64) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	result := metricsRingBuffer.drain()
	return result.Metrics, result.Dropped
}

func resetMetricsBufferForTest(capacity int) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	metricsRingBuffer = newMetricRingBuffer(capacity)
}

func metricsBufferSizeForTest() int {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	return metricsRingBuffer.size
}
