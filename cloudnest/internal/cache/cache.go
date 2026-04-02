package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

var (
	MetricsCache  *gocache.Cache
	FileTreeCache *gocache.Cache
)

func Init() {
	MetricsCache = gocache.New(1*time.Minute, 2*time.Minute)
	FileTreeCache = gocache.New(5*time.Minute, 10*time.Minute)
}
