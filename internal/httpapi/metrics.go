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
	body := "# HELP http_requests_total Total HTTP requests handled.\n# TYPE http_requests_total counter\nhttp_requests_total " + formatUint(h.metrics.requests.Load()) + "\n" +
		"# TYPE db_pool_total_connections gauge\ndb_pool_total_connections " + formatInt(stats.Total) + "\n" +
		"# TYPE db_pool_acquired_connections gauge\ndb_pool_acquired_connections " + formatInt(stats.Acquired) + "\n" +
		"# TYPE db_pool_idle_connections gauge\ndb_pool_idle_connections " + formatInt(stats.Idle) + "\n" +
		"# TYPE db_pool_max_connections gauge\ndb_pool_max_connections " + formatInt(stats.Max) + "\n" +
		"# TYPE db_pool_empty_acquire_wait_seconds counter\ndb_pool_empty_acquire_wait_seconds " + formatDurationSeconds(stats.EmptyAcquireWait) + "\n"
	return c.SendString(body)
}

func formatUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}
func formatDurationSeconds(value time.Duration) string {
	return strconv.FormatFloat(value.Seconds(), 'f', 6, 64)
}

func formatInt(value int32) string {
	if value < 0 {
		return "-" + formatUint(uint64(-value))
	}
	return formatUint(uint64(value))
}
