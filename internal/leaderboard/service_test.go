package leaderboard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"threadman-api/internal/security"
)

type serviceRepository struct {
	createdSessionID uuid.UUID
	createdSession   bool
	activeSessions   int64
	top10Date        string
	allTimeGames     []string
	storedName       string
	dailyPlayed      bool
	dailyCheckedDate string
	recentDates      []string
}

func (r *serviceRepository) CreateSession(_ context.Context, sessionID uuid.UUID, _ string, _ uuid.UUID, _ string, _ time.Time, max int64) error {
	if r.activeSessions >= max {
		return ErrTooManySessions
	}
	r.createdSessionID, r.createdSession = sessionID, true
	return nil
}

func (r *serviceRepository) FindSession(context.Context, string, uuid.UUID) (Session, error) {
	return Session{
		ID:        uuid.New(),
		PlayerID:  uuid.New(),
		Secret:    strings.Repeat("00", 32),
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (r *serviceRepository) ConsumeAndCreateScore(_ context.Context, _ string, _ string, name string, _ uuid.UUID, _ uuid.UUID, _ int64, _ int) (Score, error) {
	r.storedName = name
	return Score{Name: name}, nil
}

func (r *serviceRepository) Rank(context.Context, string, string, int64, time.Time, int64) (int64, error) {
	return 1, nil
}

func (r *serviceRepository) Top10(_ context.Context, _, date string) ([]LeaderboardEntry, error) {
	r.top10Date = date
	return nil, nil
}

func (r *serviceRepository) Top10AllTime(_ context.Context, gameCode string) ([]LeaderboardEntry, error) {
	r.allTimeGames = append(r.allTimeGames, gameCode)
	return []LeaderboardEntry{{Rank: 1, PlayerName: "Ace", Score: 3200}}, nil
}

func (r *serviceRepository) PruneExpiredSessions(context.Context) error { return nil }

func (r *serviceRepository) RecentScoreDates(_ context.Context, _ string, _ uuid.UUID, through string, _ int) ([]string, error) {
	r.dailyCheckedDate = through
	if r.recentDates != nil {
		return r.recentDates, nil
	}
	if r.dailyPlayed {
		return []string{through}, nil
	}
	return nil, nil
}

func (r *serviceRepository) HasScoreOnDate(_ context.Context, _ string, _ uuid.UUID, date string) (bool, error) {
	r.dailyCheckedDate = date
	return r.dailyPlayed, nil
}

func TestLeaderboardUsesRequestedDate(t *testing.T) {
	repo := &serviceRepository{}
	service := NewService(repo, time.UTC, 10*time.Minute, 5)

	date, _, err := service.Leaderboard(context.Background(), GameThreadman, "2025-01-15")
	if err != nil {
		t.Fatalf("Leaderboard() error = %v", err)
	}
	if date != "2025-01-15" || repo.top10Date != "2025-01-15" {
		t.Fatalf("Leaderboard() date = %q, repository date = %q", date, repo.top10Date)
	}
}

func TestAllTimeLeaderboardIgnoresDate(t *testing.T) {
	repo := &serviceRepository{}
	service := NewService(repo, time.UTC, 10*time.Minute, 5)

	entries, err := service.AllTimeLeaderboard(context.Background(), GameThreadman)
	if err != nil {
		t.Fatalf("AllTimeLeaderboard() error = %v", err)
	}
	if repo.top10Date != "" {
		t.Fatalf("all-time lookup passed date %q to the repository", repo.top10Date)
	}
	if len(repo.allTimeGames) != 1 || repo.allTimeGames[0] != GameThreadman {
		t.Fatalf("allTimeGames = %v, want [%s]", repo.allTimeGames, GameThreadman)
	}
	if len(entries) != 1 || entries[0].Rank != 1 || entries[0].PlayerName != "Ace" {
		t.Fatalf("AllTimeLeaderboard() entries = %+v", entries)
	}
}

func TestCreateSessionPersistsReturnedID(t *testing.T) {
	repo := &serviceRepository{}
	service := NewService(repo, time.UTC, 10*time.Minute, 5)

	session, err := service.CreateSession(context.Background(), GameThreadman, uuid.New())
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if !repo.createdSession || session.ID != repo.createdSessionID {
		t.Fatalf("returned session ID %s differs from persisted ID %s", session.ID, repo.createdSessionID)
	}
}

func TestCreateSessionRejectsTooManyActiveSessions(t *testing.T) {
	repo := &serviceRepository{activeSessions: 5}
	service := NewService(repo, time.UTC, 10*time.Minute, 5)

	_, err := service.CreateSession(context.Background(), GameThreadman, uuid.New())
	if err != ErrTooManySessions {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrTooManySessions)
	}
}

// TestSubmitScoreStoresRawNameAndReturnsCensoredName pins the storage contract:
// the raw player-supplied name reaches the database untouched, while the copy
// echoed back in the response is masked for display.
func TestSubmitScoreStoresRawNameAndReturnsCensoredName(t *testing.T) {
	const raw = "Ace fucker"
	repo := &serviceRepository{}
	service := NewService(repo, time.UTC, 10*time.Minute, 5)
	service.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	payload, err := json.Marshal(DecryptedPayload{
		PlayerName: raw,
		Score:      600,
		DurationMs: 5000,
		Timestamp:  1_700_000_000,
	})
	if err != nil {
		t.Fatalf("marshal payload error = %v", err)
	}
	encoded, err := security.EncryptAESGCM(strings.Repeat("00", 32), payload)
	if err != nil {
		t.Fatalf("encrypt payload error = %v", err)
	}

	result, err := service.SubmitScore(context.Background(), GameThreadman, uuid.New(), encoded)
	if err != nil {
		t.Fatalf("SubmitScore() error = %v", err)
	}
	if repo.storedName != raw {
		t.Errorf("stored name = %q, want the raw name %q", repo.storedName, raw)
	}
	if result.Score.Name != "Ace **cker" {
		t.Errorf("returned name = %q, want the censored name %q", result.Score.Name, "Ace **cker")
	}
}

func TestDailySeedIsStablePerDate(t *testing.T) {
	if DailySeed("2026-10-14") != DailySeed("2026-10-14") {
		t.Fatal("seed must be deterministic")
	}
	if DailySeed("2026-10-14") == DailySeed("2026-10-15") {
		t.Fatal("consecutive days should not share a seed")
	}
}

func TestDailyCreateSessionRejectsSecondAttempt(t *testing.T) {
	repo := &serviceRepository{dailyPlayed: true}
	service := NewService(repo, time.UTC, time.Hour, 5)
	if _, err := service.CreateSession(context.Background(), GameThreadmanDaily, uuid.New()); err != ErrDailyPlayed {
		t.Fatalf("err = %v, want ErrDailyPlayed", err)
	}
	if repo.createdSession {
		t.Fatal("no session should be created after the daily run was played")
	}
}

func TestRegularCreateSessionSkipsDailyCheck(t *testing.T) {
	repo := &serviceRepository{dailyPlayed: true}
	service := NewService(repo, time.UTC, time.Hour, 5)
	if _, err := service.CreateSession(context.Background(), GameThreadman, uuid.New()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if repo.dailyCheckedDate != "" {
		t.Fatal("regular runs must not consult the daily table")
	}
}

func TestDailyInfoReportsPlayed(t *testing.T) {
	repo := &serviceRepository{dailyPlayed: true}
	service := NewService(repo, time.UTC, time.Hour, 5)
	service.now = func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) }
	id := uuid.New()
	info, err := service.Daily(context.Background(), &id)
	if err != nil {
		t.Fatal(err)
	}
	if info.ScoreDate != "2026-10-14" || !info.Played || info.Seed != DailySeed("2026-10-14") {
		t.Fatalf("info = %+v", info)
	}
}

func TestDailyStreak(t *testing.T) {
	cases := []struct {
		name   string
		dates  []string
		played bool
		streak int
	}{
		{"none", nil, false, 0},
		{"today only", []string{"2026-10-14"}, true, 1},
		{"through today", []string{"2026-10-14", "2026-10-13", "2026-10-12"}, true, 3},
		{"alive from yesterday", []string{"2026-10-13", "2026-10-12"}, false, 2},
		{"broken", []string{"2026-10-12", "2026-10-11"}, false, 0},
		{"gap stops count", []string{"2026-10-14", "2026-10-13", "2026-10-10"}, true, 2},
		{"month boundary", []string{"2026-03-01", "2026-02-28", "2026-02-27"}, true, 3},
	}
	for _, tc := range cases {
		today := "2026-10-14"
		if tc.name == "month boundary" {
			today = "2026-03-01"
		}
		played, streak := dailyStreak(today, tc.dates)
		if played != tc.played || streak != tc.streak {
			t.Errorf("%s: got played=%v streak=%d, want %v %d", tc.name, played, streak, tc.played, tc.streak)
		}
	}
}

func TestDailyInfoStreak(t *testing.T) {
	repo := &serviceRepository{recentDates: []string{"2026-10-13", "2026-10-12", "2026-10-11"}}
	service := NewService(repo, time.UTC, time.Hour, 5)
	service.now = func() time.Time { return time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC) }
	id := uuid.New()
	info, err := service.Daily(context.Background(), &id)
	if err != nil {
		t.Fatal(err)
	}
	if info.Played || info.Streak != 3 {
		t.Fatalf("info = %+v, want played=false streak=3", info)
	}
}
