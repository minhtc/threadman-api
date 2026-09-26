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
