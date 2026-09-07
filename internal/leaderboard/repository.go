package leaderboard

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrSessionUnavailable = errors.New("session unavailable")
)

type Repository interface {
	CountActiveSessions(ctx context.Context, gameCode string, playerID uuid.UUID) (int64, error)
	CreateSession(ctx context.Context, sessionID uuid.UUID, gameCode string, playerID uuid.UUID, secret string, expiresAt time.Time) error
	FindSession(ctx context.Context, gameCode string, sessionID uuid.UUID) (Session, error)
	ConsumeAndCreateScore(ctx context.Context, gameCode, scoreDate, name string, sessionID, playerID uuid.UUID, score int64, durationMS int) (Score, error)
	Rank(ctx context.Context, gameCode, scoreDate string, score int64, createdAt time.Time, scoreID int64) (int64, error)
	Top10(ctx context.Context, gameCode, scoreDate string) ([]LeaderboardEntry, error)
	PruneExpiredSessions(ctx context.Context) error
}

type PostgresRepository struct{ DB *pgxpool.Pool }

func (r *PostgresRepository) CountActiveSessions(ctx context.Context, gameCode string, playerID uuid.UUID) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM game_sessions WHERE game_code = $1 AND player_id = $2 AND submitted_at IS NULL AND expires_at > NOW()`, gameCode, playerID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) CreateSession(ctx context.Context, sessionID uuid.UUID, gameCode string, playerID uuid.UUID, secret string, expiresAt time.Time) error {
	_, err := r.DB.Exec(ctx, `INSERT INTO game_sessions (id, game_code, player_id, session_secret, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, gameCode, playerID, secret, expiresAt)
	return err
}

func (r *PostgresRepository) FindSession(ctx context.Context, gameCode string, sessionID uuid.UUID) (Session, error) {
	var session Session
	err := r.DB.QueryRow(ctx, `SELECT id, player_id, session_secret, expires_at, submitted_at FROM game_sessions WHERE id = $1 AND game_code = $2`, sessionID, gameCode).Scan(&session.ID, &session.PlayerID, &session.Secret, &session.ExpiresAt, &session.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return session, err
}

func (r *PostgresRepository) ConsumeAndCreateScore(ctx context.Context, gameCode, scoreDate, name string, sessionID, playerID uuid.UUID, score int64, durationMS int) (Score, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Score{}, err
	}
	defer tx.Rollback(ctx)

	var consumedID uuid.UUID
	err = tx.QueryRow(ctx, `UPDATE game_sessions SET submitted_at = NOW() WHERE id = $1 AND submitted_at IS NULL AND expires_at > NOW() RETURNING id`, sessionID).Scan(&consumedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Score{}, ErrSessionUnavailable
	}
	if err != nil {
		return Score{}, err
	}

	var result Score
	err = tx.QueryRow(ctx, `INSERT INTO game_scores (session_id, game_code, player_id, player_name, score, duration_ms, score_date) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`, sessionID, gameCode, playerID, name, score, durationMS, scoreDate).Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return Score{}, err
	}
	result.PlayerID, result.Name, result.Value = playerID, name, score

	if err := tx.Commit(ctx); err != nil {
		return Score{}, err
	}
	return result, nil
}

func (r *PostgresRepository) Rank(ctx context.Context, gameCode, scoreDate string, score int64, createdAt time.Time, scoreID int64) (int64, error) {
	var rank int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) + 1 FROM game_scores WHERE game_code = $1 AND score_date = $2 AND (score > $3 OR (score = $3 AND (created_at < $4 OR (created_at = $4 AND id < $5))))`, gameCode, scoreDate, score, createdAt, scoreID).Scan(&rank)
	return rank, err
}

func (r *PostgresRepository) Top10(ctx context.Context, gameCode, scoreDate string) ([]LeaderboardEntry, error) {
	rows, err := r.DB.Query(ctx, `SELECT player_name, score FROM game_scores WHERE game_code = $1 AND score_date = $2 ORDER BY score DESC, created_at ASC, id ASC LIMIT 10`, gameCode, scoreDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LeaderboardEntry, 0, 10)
	for rank := 1; rows.Next(); rank++ {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.PlayerName, &entry.Score); err != nil {
			return nil, err
		}
		entry.Rank = rank
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *PostgresRepository) PruneExpiredSessions(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM game_sessions WHERE expires_at < NOW() - INTERVAL '24 hours' AND submitted_at IS NULL`)
	return err
}
