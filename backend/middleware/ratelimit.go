package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"profhit-backend/config"
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
		allowed := false
		if config.DB != nil && config.DB.Dialector.Name() == "postgres" {
			now := time.Now().UTC()
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", rate, window, ip))))
			claim := config.DB.Exec(`INSERT INTO rate_limit_buckets (key, requests, expires_at) VALUES (?,1,?)
			 ON CONFLICT (key) DO UPDATE SET
			 requests = CASE WHEN rate_limit_buckets.expires_at <= ? THEN 1 ELSE rate_limit_buckets.requests + 1 END,
			 expires_at = CASE WHEN rate_limit_buckets.expires_at <= ? THEN EXCLUDED.expires_at ELSE rate_limit_buckets.expires_at END
			 WHERE rate_limit_buckets.expires_at <= ? OR rate_limit_buckets.requests < ?`, key, now.Add(window), now, now, now, rate)
			if claim.Error != nil {
				c.AbortWithStatusJSON(503, gin.H{"error": "Request protection temporarily unavailable"})
				return
			}
			allowed = claim.RowsAffected == 1
		} else {
			allowed = limiter.allow(ip)
		}
		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
