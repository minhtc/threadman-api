package leaderboard

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type serviceRepository struct {
	createdSessionID uuid.UUID
	createdSession   bool
	activeSessions   int64
	top10Date        string
	allTimeGames     []string
}

func (r *serviceRepository) CreateSession(_ context.Context, sessionID uuid.UUID, _ string, _ uuid.UUID, _ string, _ time.Time, max int64) error {
	if r.activeSessions >= max {
		return ErrTooManySessions
	}
	r.createdSessionID, r.createdSession = sessionID, true
	return nil
}

func (r *serviceRepository) FindSession(context.Context, string, uuid.UUID) (Session, error) {
	return Session{}, nil
}

func (r *serviceRepository) ConsumeAndCreateScore(context.Context, string, string, string, uuid.UUID, uuid.UUID, int64, int) (Score, error) {
	return Score{}, nil
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
