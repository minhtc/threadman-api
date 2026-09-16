package leaderboard

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"threadman-api/internal/security"
)

// dateTimeLayout is the canonical calendar-day format for leaderboard partitions.
const dateTimeLayout = "2006-01-02"

type Service struct {
	repo              Repository
	timezone          *time.Location
	now               func() time.Time
	sessionTTL        time.Duration
	maxActiveSessions int64
}

func NewService(repo Repository, timezone *time.Location, sessionTTL time.Duration, maxActiveSessions int64) *Service {
	return &Service{
		repo:              repo,
		timezone:          timezone,
		now:               time.Now,
		sessionTTL:        sessionTTL,
		maxActiveSessions: maxActiveSessions,
	}
}

func (s *Service) CreateSession(ctx context.Context, gameCode string, playerID uuid.UUID) (Session, error) {
	secret, err := security.NewSessionSecret()
	if err != nil {
		return Session{}, err
	}

	session := Session{
		ID:        uuid.New(),
		PlayerID:  playerID,
		Secret:    secret,
		ExpiresAt: s.now().UTC().Add(s.sessionTTL),
	}
	if err := s.repo.CreateSession(ctx, session.ID, gameCode, playerID, secret, session.ExpiresAt, s.maxActiveSessions); err != nil {
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
		return SubmitResult{}, &InvalidPayloadError{Message: "payload decryption failed"}
	}

	var payload DecryptedPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return SubmitResult{}, &InvalidPayloadError{Message: "malformed JSON in payload"}
	}

	name, err := security.SanitizePlayerName(payload.PlayerName)
	if err != nil {
		return SubmitResult{}, &InvalidPayloadError{Message: err.Error()}
	}
	if err := security.ValidateScore(payload.Score, payload.DurationMs, payload.Timestamp, s.now()); err != nil {
		return SubmitResult{}, &InvalidPayloadError{Message: err.Error()}
	}

	scoreDate := s.scoreDate()
	score, err := s.repo.ConsumeAndCreateScore(ctx, gameCode, scoreDate, name, sessionID, session.PlayerID, payload.Score, payload.DurationMs)
	if err != nil {
		return SubmitResult{}, err
	}

	result := SubmitResult{
		ScoreDate: scoreDate,
		Score:     score,
		Top10:     make([]LeaderboardEntry, 0),
	}

	// Rank and top-10 are enrichment only; a committed score must not fail because of them.
	if rank, rankErr := s.repo.Rank(ctx, gameCode, scoreDate, score.Value, score.CreatedAt, score.ID); rankErr == nil {
		result.Rank, result.RankAvailable = rank, true
	}
	if top10, topErr := s.repo.Top10(ctx, gameCode, scoreDate); topErr == nil {
		result.Top10, result.LeaderboardAvailable = top10, true
	}
	return result, nil
}

func (s *Service) Leaderboard(ctx context.Context, gameCode, requestedDate string) (string, []LeaderboardEntry, error) {
	date := requestedDate
	if date == "" {
		date = s.scoreDate()
	}
	entries, err := s.repo.Top10(ctx, gameCode, date)
	return date, entries, err
}

func (s *Service) PruneSessions(ctx context.Context) error {
	return s.repo.PruneExpiredSessions(ctx)
}

func (s *Service) scoreDate() string {
	return s.now().In(s.timezone).Format(dateTimeLayout)
}
