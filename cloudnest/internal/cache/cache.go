package cache

import (
	"sync"

	gocache "github.com/patrickmn/go-cache"
)

var (
	FileTreeCache *gocache.Cache

	metricsMu     sync.Mutex
	metricsBuffer []interface{} // stores *models.NodeMetric
)

func Init() {
	FileTreeCache = gocache.New(gocache.NoExpiration, 0)
}

// PushMetric appends a metric to the buffer.
func PushMetric(metric interface{}) {
	metricsMu.Lock()
	metricsBuffer = append(metricsBuffer, metric)
	metricsMu.Unlock()
}

// DrainMetrics returns all buffered metrics and clears the buffer.
func DrainMetrics() []interface{} {
	metricsMu.Lock()
	buf := metricsBuffer
	metricsBuffer = nil
	metricsMu.Unlock()
	return buf
}
