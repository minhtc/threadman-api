package leaderboard

import (
	"time"

	"github.com/google/uuid"
)

const (
	GameThreadman = "threadman"
	// GameThreadmanDaily is the Daily Challenge: one seeded run per player per
	// leaderboard day, ranked on its own board.
	GameThreadmanDaily = "threadman_daily"
)

func IsSupportedGame(code string) bool {
	return code == GameThreadman || code == GameThreadmanDaily
}

// IsDailyGame reports whether a game code is limited to one score per day.
func IsDailyGame(code string) bool { return code == GameThreadmanDaily }

// DailyInfo describes today's Daily Challenge for one player.
type DailyInfo struct {
	ScoreDate string `json:"score_date"`
	Seed      uint32 `json:"seed"`
	// Played is true when the player already has a score on this day's board.
	Played bool `json:"played"`
	// Streak counts consecutive days with a daily score, ending today when
	// Played, otherwise ending yesterday (a streak survives until a day is
	// missed). Zero without a player ID.
	Streak int `json:"streak"`
}

type Session struct {
	ID          uuid.UUID
	PlayerID    uuid.UUID
	Secret      string
	ExpiresAt   time.Time
	SubmittedAt *time.Time
}

type DecryptedPayload struct {
	PlayerName string `json:"player_name"`
	Score      int64  `json:"score"`
	DurationMs int    `json:"duration_ms"`
	Timestamp  int64  `json:"timestamp"`
}

type LeaderboardEntry struct {
	Rank       int    `json:"rank"`
	PlayerName string `json:"player_name"`
	Score      int64  `json:"score"`
}

type Score struct {
	ID        int64
	PlayerID  uuid.UUID
	Name      string
	Value     int64
	CreatedAt time.Time
}

type SubmitResult struct {
	ScoreDate            string
	Score                Score
	Rank                 int64
	RankAvailable        bool
	Top10                []LeaderboardEntry
	LeaderboardAvailable bool
}
