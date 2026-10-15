package httpapi

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"

	"threadman-api/internal/leaderboard"
)

type PoolStats struct {
	Total, Acquired, Idle, Max int32
	EmptyAcquireWait           time.Duration
}

// Metrics are process-lifetime counters (they reset on deploy). Per-day
// numbers come from SQL; see docs/daily-metrics.md.
type Metrics struct {
	requests atomic.Uint64
	// Per game: index 0 = threadman, 1 = threadman_daily.
	sessionsCreated [2]atomic.Uint64
	scoresSubmitted [2]atomic.Uint64
	dailyRejected   atomic.Uint64
	dailyLookups    atomic.Uint64
}

// gameIndex maps a supported game code to its metrics slot.
func gameIndex(code string) int {
	if leaderboard.IsDailyGame(code) {
		return 1
	}
	return 0
}

func perGame(name, help string, v *[2]atomic.Uint64) string {
	return "# HELP " + name + " " + help + "\n" +
		"# TYPE " + name + " counter\n" +
		name + `{game="` + leaderboard.GameThreadman + `"} ` + strconv.FormatUint(v[0].Load(), 10) + "\n" +
		name + `{game="` + leaderboard.GameThreadmanDaily + `"} ` + strconv.FormatUint(v[1].Load(), 10) + "\n"
}

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
		"db_pool_empty_acquire_wait_seconds " + strconv.FormatFloat(stats.EmptyAcquireWait.Seconds(), 'f', 6, 64) + "\n" +
		perGame("game_sessions_created_total", "Game sessions created since start.", &h.metrics.sessionsCreated) +
		perGame("game_scores_submitted_total", "Scores accepted since start.", &h.metrics.scoresSubmitted) +
		"# HELP daily_replay_rejected_total Daily sessions or scores refused because the player already played today.\n" +
		"# TYPE daily_replay_rejected_total counter\n" +
		"daily_replay_rejected_total " + strconv.FormatUint(h.metrics.dailyRejected.Load(), 10) + "\n" +
		"# HELP daily_lookups_total GET /daily requests served.\n" +
		"# TYPE daily_lookups_total counter\n" +
		"daily_lookups_total " + strconv.FormatUint(h.metrics.dailyLookups.Load(), 10) + "\n"
	return c.SendString(body)
}
