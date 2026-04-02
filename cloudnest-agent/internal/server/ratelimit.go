package server

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware limits transfer speed if rateLimit > 0 (bytes/sec).
func RateLimitMiddleware(rateLimit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rateLimit <= 0 {
			c.Next()
			return
		}

		// Wrap the response writer with a rate-limited writer
		c.Writer = &rateLimitedWriter{
			ResponseWriter: c.Writer,
			rateLimit:      rateLimit,
		}

		// Wrap the request body with a rate-limited reader
		c.Request.Body = &rateLimitedReader{
			ReadCloser: c.Request.Body,
			rateLimit:  rateLimit,
		}

		c.Next()
	}
}

type rateLimitedWriter struct {
	gin.ResponseWriter
	rateLimit   int64
	written     int64
	lastCheck   time.Time
}

func (w *rateLimitedWriter) Write(data []byte) (int, error) {
	if w.lastCheck.IsZero() {
		w.lastCheck = time.Now()
	}

	n, err := w.ResponseWriter.Write(data)
	w.written += int64(n)

	// Throttle: sleep if we're going too fast
	elapsed := time.Since(w.lastCheck).Seconds()
	if elapsed > 0 {
		currentRate := float64(w.written) / elapsed
		if currentRate > float64(w.rateLimit) {
			sleepTime := time.Duration(float64(w.written)/float64(w.rateLimit)*1e9) - time.Since(w.lastCheck)
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}
		}
	}

	return n, err
}

type rateLimitedReader struct {
	io.ReadCloser
	rateLimit int64
	read      int64
	lastCheck time.Time
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	if r.lastCheck.IsZero() {
		r.lastCheck = time.Now()
	}

	// Limit chunk size to control granularity
	maxChunk := r.rateLimit / 10 // 100ms worth
	if maxChunk < 1024 {
		maxChunk = 1024
	}
	if int64(len(p)) > maxChunk {
		p = p[:maxChunk]
	}

	n, err := r.ReadCloser.Read(p)
	r.read += int64(n)

	elapsed := time.Since(r.lastCheck).Seconds()
	if elapsed > 0 {
		currentRate := float64(r.read) / elapsed
		if currentRate > float64(r.rateLimit) {
			sleepTime := time.Duration(float64(r.read)/float64(r.rateLimit)*1e9) - time.Since(r.lastCheck)
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}
		}
	}

	return n, err
}

// Ensure rateLimitedWriter implements http.ResponseWriter
var _ http.ResponseWriter = (*rateLimitedWriter)(nil)
