package leaderboard

import (
	"time"

	"github.com/google/uuid"
)

const GameThreadman = "threadman"

func IsSupportedGame(code string) bool { return code == GameThreadman }

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
