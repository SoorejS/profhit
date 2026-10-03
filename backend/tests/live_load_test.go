package tests

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/models"
	"profhit-backend/routes"
	"profhit-backend/services"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Opt-in LOCAL fixture workload. This does not certify PostgreSQL or public hosting capacity.
func TestLiveLocalLoad(t *testing.T) {
	if os.Getenv("RUN_LIVE_LOAD_TESTS") != "true" {
		t.Skip("opt-in local workload; not a production capacity certification")
	}
	for _, clients := range []int{100, 500, 1000} {
		t.Run(fmt.Sprint(clients), func(t *testing.T) {
			setupTestDB()
			config.DB.Logger = logger.Default.LogMode(logger.Silent)
			t.Setenv("JWT_SECRET", "isolated-load-test-only-secret-with-no-live-access")
			t.Setenv("GIN_MODE", "release")
			now := time.Now().UTC()
			market := compliantTestMarket("Weather", "Easy", now.Add(time.Hour))
			market.Title = "LOCAL LOAD FIXTURE ONLY"
			require.NoError(t, config.DB.Create(&market).Error)
			users := make([]models.User, clients)
			for i := range users {
				users[i] = models.User{Username: fmt.Sprintf("local_load_%d", i), Email: fmt.Sprintf("load_%d@example.test", i), ReferralCode: fmt.Sprintf("LOAD%d", i)}
			}
			require.NoError(t, config.DB.CreateInBatches(&users, 100).Error)
			tokens := make([]string, clients)
			for i, u := range users {
				var err error
				tokens[i], err = controllers.GenerateToken(u)
				require.NoError(t, err)
			}
			var queries atomic.Int64
			require.NoError(t, config.DB.Callback().Query().Before("gorm:query").Register("load:count", func(*gorm.DB) { queries.Add(1) }))
			router := routes.SetupRouter()
			server := httptest.NewServer(router)
			defer server.Close()
			httpClient := &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{MaxConnsPerHost: 100, MaxIdleConnsPerHost: 100}}
			defer httpClient.CloseIdleConnections()
			sql, err := config.DB.DB()
			require.NoError(t, err)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			connections := make([]*websocket.Conn, clients)
			errorsOut := make(chan error, clients)
			var wg sync.WaitGroup
			started := time.Now()
			handshakes := make(chan struct{}, 32)
			for i := range users {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					handshakes <- struct{}{}
					defer func() { <-handshakes }()
					conn, _, err := (&websocket.Dialer{HandshakeTimeout: 45 * time.Second}).Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/ws?token="+tokens[index], nil)
					if err == nil {
						conn.SetReadDeadline(time.Now().Add(45 * time.Second))
						_, _, err = conn.ReadMessage()
					}
					if err != nil {
						if conn != nil {
							conn.Close()
						}
						errorsOut <- err
						return
					}
					connections[index] = conn
				}(i)
			}
			wg.Wait()
			close(errorsOut)
			for err := range errorsOut {
				require.NoError(t, err)
			}
			defer func() {
				for _, conn := range connections {
					if conn != nil {
						conn.Close()
					}
				}
			}()
			services.BroadcastToAll("news_event_updated", map[string]interface{}{"test_fixture": true})
			for _, conn := range connections {
				_, body, err := conn.ReadMessage()
				require.NoError(t, err)
				var message services.WSMessage
				require.NoError(t, json.Unmarshal(body, &message))
				require.Equal(t, "news_event_updated", message.Event)
			}
			// Browsers consume continuously; inactive readers would correctly be evicted.
			for _, conn := range connections {
				go func(connection *websocket.Conn) {
					for {
						if _, _, err := connection.ReadMessage(); err != nil {
							return
						}
					}
				}(conn)
			}
			latencies := make([]time.Duration, clients)
			var accepted, limited, failed atomic.Int64
			for i := range users {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					begin := time.Now()
					response, err := httpClient.Get(server.URL + "/api/live-feed")
					if err != nil {
						failed.Add(1)
						return
					}
					io.Copy(io.Discard, response.Body)
					response.Body.Close()
					if response.StatusCode != 200 {
						failed.Add(1)
					}
					latencies[index] = time.Since(begin)
					body := fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, market.ID)
					req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/predictions", strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+tokens[index])
					response, err = httpClient.Do(req)
					if err != nil {
						failed.Add(1)
						return
					}
					io.Copy(io.Discard, response.Body)
					response.Body.Close()
					switch response.StatusCode {
					case 200:
						accepted.Add(1)
					case 429:
						limited.Add(1)
					default:
						failed.Add(1)
					}
				}(i)
			}
			wg.Wait()
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			runtime.ReadMemStats(&after)
			stats := sql.Stats()
			require.Zero(t, failed.Load())
			var count int64
			require.NoError(t, config.DB.Model(&models.PredictionSubmission{}).Where("market_id = ?", market.ID).Count(&count).Error)
			require.Equal(t, accepted.Load(), count)
			require.NoError(t, config.DB.First(&market, market.ID).Error)
			require.EqualValues(t, count, market.Volume)
			result := map[string]interface{}{"local_fixture_only": true, "clients": clients, "websockets": len(connections), "feed_p50_ms": float64(latencies[clients/2].Microseconds()) / 1000, "feed_p95_ms": float64(latencies[(clients*95)/100].Microseconds()) / 1000, "elapsed_seconds": time.Since(started).Seconds(), "accepted_predictions": accepted.Load(), "rate_limited_shared_ip": limited.Load(), "other_failures": failed.Load(), "query_callbacks": queries.Load(), "db_max_connections": stats.MaxOpenConnections, "db_wait_seconds": stats.WaitDuration.Seconds(), "heap_before_bytes": before.HeapAlloc, "heap_after_bytes": after.HeapAlloc, "cpu": "not measured", "news_ai_throughput": "requires real provider credentials"}
			encoded, _ := json.Marshal(result)
			t.Log(string(encoded))
		})
	}
}
