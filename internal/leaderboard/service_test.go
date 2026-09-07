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
}

func (r *serviceRepository) CountActiveSessions(context.Context, string, uuid.UUID) (int64, error) {
	return r.activeSessions, nil
}
func (r *serviceRepository) CreateSession(_ context.Context, sessionID uuid.UUID, _ string, _ uuid.UUID, _ string, _ time.Time) error {
	r.createdSessionID = sessionID
	r.createdSession = true
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
func (r *serviceRepository) Top10(context.Context, string, string) ([]LeaderboardEntry, error) {
	return nil, nil
}
func (r *serviceRepository) PruneExpiredSessions(context.Context) error { return nil }

func TestCreateSessionPersistsReturnedID(t *testing.T) {
	repo := &serviceRepository{}
	service := NewService(repo, time.UTC)

	session, err := service.CreateSession(context.Background(), GameThreadman, uuid.New())
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if !repo.createdSession {
		t.Fatal("CreateSession() did not call repository")
	}
	if session.ID != repo.createdSessionID {
		t.Fatalf("returned session ID %s differs from persisted ID %s", session.ID, repo.createdSessionID)
	}
}

func TestCreateSessionRejectsTooManyActiveSessions(t *testing.T) {
	repo := &serviceRepository{activeSessions: 5}
	service := NewService(repo, time.UTC)

	_, err := service.CreateSession(context.Background(), GameThreadman, uuid.New())
	if err != ErrTooManySessions {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrTooManySessions)
	}
}
