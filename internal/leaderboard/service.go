package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"homielab-api/internal/security"
)

var (
	ErrTooManySessions = errors.New("too many concurrent active sessions; complete existing games first")
	ErrSessionUsed     = errors.New("session already used")
	ErrSessionExpired  = errors.New("session expired")
	ErrInvalidPayload  = errors.New("invalid score payload")
)

type Service struct {
	repo     Repository
	timezone *time.Location
	now      func() time.Time
}

func NewService(repo Repository, timezone *time.Location) *Service {
	return &Service{repo: repo, timezone: timezone, now: time.Now}
}

func (s *Service) CreateSession(ctx context.Context, gameCode string, playerID uuid.UUID) (Session, error) {
	active, err := s.repo.CountActiveSessions(ctx, gameCode, playerID)
	if err != nil {
		return Session{}, err
	}
	if active >= 5 {
		return Session{}, ErrTooManySessions
	}

	secret, err := security.NewSessionSecret()
	if err != nil {
		return Session{}, err
	}
	session := Session{ID: uuid.New(), PlayerID: playerID, Secret: secret, ExpiresAt: s.now().UTC().Add(10 * time.Minute)}
	if err := s.repo.CreateSession(ctx, session.ID, gameCode, playerID, secret, session.ExpiresAt); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) SubmitScore(ctx context.Context, gameCode string, sessionID uuid.UUID, encodedPayload string) (SubmitResult, error) {
	session, err := s.repo.FindSession(ctx, gameCode, sessionID)
	if err != nil {
		return SubmitResult{}, err
	}
	if session.SubmittedAt != nil {
		return SubmitResult{}, ErrSessionUsed
	}
	if s.now().UTC().After(session.ExpiresAt) {
		return SubmitResult{}, ErrSessionExpired
	}

	plain, err := security.DecryptAESGCM(session.Secret, encodedPayload)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: payload decryption failed", ErrInvalidPayload)
	}
	var payload DecryptedPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return SubmitResult{}, fmt.Errorf("%w: malformed JSON in payload", ErrInvalidPayload)
	}

	name, err := security.SanitizePlayerName(payload.PlayerName)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	if err := security.ValidateScore(payload.Score, payload.DurationMs, payload.Timestamp, s.now()); err != nil {
		return SubmitResult{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	scoreDate := s.now().In(s.timezone).Format("2006-01-02")
	score, err := s.repo.ConsumeAndCreateScore(ctx, gameCode, scoreDate, name, sessionID, session.PlayerID, payload.Score, payload.DurationMs)
	if err != nil {
		return SubmitResult{}, err
	}

	rank, err := s.repo.Rank(ctx, gameCode, scoreDate, score.Value, score.CreatedAt, score.ID)
	if err != nil {
		return SubmitResult{}, err
	}
	top10, err := s.repo.Top10(ctx, gameCode, scoreDate)
	if err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{ScoreDate: scoreDate, Score: score, Rank: rank, Top10: top10}, nil
}

func (s *Service) Leaderboard(ctx context.Context, gameCode string) (string, []LeaderboardEntry, error) {
	date := s.now().In(s.timezone).Format("2006-01-02")
	entries, err := s.repo.Top10(ctx, gameCode, date)
	return date, entries, err
}

func (s *Service) PruneSessions(ctx context.Context) error {
	return s.repo.PruneExpiredSessions(ctx)
}
