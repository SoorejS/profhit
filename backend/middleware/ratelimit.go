package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// tokenBucket is a simple in-memory per-IP rate limiter.
type tokenBucket struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	rate      int           // tokens added per window
	window    time.Duration // size of the window
	lastSweep time.Time
	capacity  int // max burst
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(rate int, window time.Duration) *tokenBucket {
	return &tokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		window:   window,
		capacity: rate,
	}
}

func (tb *tokenBucket) allow(ip string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	if now.Sub(tb.lastSweep) >= tb.window {
		for key, b := range tb.buckets {
			if now.Sub(b.lastReset) >= tb.window {
				delete(tb.buckets, key)
			}
		}
		tb.lastSweep = now
	}
	b, ok := tb.buckets[ip]
	if !ok || time.Since(b.lastReset) >= tb.window {
		tb.buckets[ip] = &bucket{tokens: tb.capacity - 1, lastReset: time.Now()}
		return true
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit returns a gin middleware that limits requests to `rate` per `window` per IP.
func RateLimit(rate int, window time.Duration) gin.HandlerFunc {
	limiter := newRateLimiter(rate, window)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
