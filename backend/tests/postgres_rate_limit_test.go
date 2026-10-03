package tests

import (
	"crypto/sha256"
	"fmt"
	"net/http/httptest"
	"profhit-backend/config"
	"profhit-backend/middleware"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPostgresRateLimitAcrossInstances(t *testing.T) {
	getPostgresDB(t)
	window := time.Duration(time.Now().UnixNano()) // unique bucket; no other test is affected
	routers := []*gin.Engine{gin.New(), gin.New()}
	for _, router := range routers {
		require.NoError(t, router.SetTrustedProxies(nil))
		router.GET("/protected", middleware.RateLimit(5, window), func(c *gin.Context) { c.Status(204) })
	}
	var allowed, rejected, unexpected atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/protected", nil)
			r.RemoteAddr = "192.0.2.21:1234"
			w := httptest.NewRecorder()
			routers[i%2].ServeHTTP(w, r)
			switch w.Code {
			case 204:
				allowed.Add(1)
			case 429:
				rejected.Add(1)
			default:
				unexpected.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.EqualValues(t, 5, allowed.Load())
	require.EqualValues(t, 15, rejected.Load())
	require.Zero(t, unexpected.Load())
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", 5, window, "192.0.2.21"))))
	require.NoError(t, config.DB.Exec("UPDATE rate_limit_buckets SET expires_at = ? WHERE key = ?", time.Now().Add(-time.Minute), key).Error)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/protected", nil)
	r.RemoteAddr = "192.0.2.21:1234"
	routers[1].ServeHTTP(w, r)
	require.Equal(t, 204, w.Code)
}
