package httpapi

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

type PoolStats struct {
	Total, Acquired, Idle, Max int32
	EmptyAcquireWait           time.Duration
}

type Metrics struct{ requests atomic.Uint64 }

func (h *Handler) metricsHandler(c fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; version=0.0.4")

	stats := PoolStats{}
	if h.poolStats != nil {
		stats = h.poolStats()
	}

	body := "# HELP http_requests_total Total HTTP requests handled.\n" +
		"# TYPE http_requests_total counter\n" +
		"http_requests_total " + strconv.FormatUint(h.metrics.requests.Load(), 10) + "\n" +
		"# TYPE db_pool_total_connections gauge\n" +
		"db_pool_total_connections " + strconv.FormatInt(int64(stats.Total), 10) + "\n" +
		"# TYPE db_pool_acquired_connections gauge\n" +
		"db_pool_acquired_connections " + strconv.FormatInt(int64(stats.Acquired), 10) + "\n" +
		"# TYPE db_pool_idle_connections gauge\n" +
		"db_pool_idle_connections " + strconv.FormatInt(int64(stats.Idle), 10) + "\n" +
		"# TYPE db_pool_max_connections gauge\n" +
		"db_pool_max_connections " + strconv.FormatInt(int64(stats.Max), 10) + "\n" +
		"# TYPE db_pool_empty_acquire_wait_seconds counter\n" +
		"db_pool_empty_acquire_wait_seconds " + strconv.FormatFloat(stats.EmptyAcquireWait.Seconds(), 'f', 6, 64) + "\n"
	return c.SendString(body)
}
