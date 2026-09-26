package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"threadman-api/internal/security"
)

type Repository interface {
	CreateSession(ctx context.Context, sessionID uuid.UUID, gameCode string, playerID uuid.UUID, secret string, expiresAt time.Time, maxActive int64) error
	FindSession(ctx context.Context, gameCode string, sessionID uuid.UUID) (Session, error)
	ConsumeAndCreateScore(ctx context.Context, gameCode, scoreDate, name string, sessionID, playerID uuid.UUID, score int64, durationMS int) (Score, error)
	Rank(ctx context.Context, gameCode, scoreDate string, score int64, createdAt time.Time, scoreID int64) (int64, error)
	Top10(ctx context.Context, gameCode, scoreDate string) ([]LeaderboardEntry, error)
	Top10AllTime(ctx context.Context, gameCode string) ([]LeaderboardEntry, error)
	PruneExpiredSessions(ctx context.Context) error
}

type PostgresRepository struct {
	DB        *pgxpool.Pool
	SecretKey []byte
}

const (
	sessionLockSQL          = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
	countActiveSessionsSQL  = `SELECT COUNT(*) FROM game_sessions WHERE game_code = $1 AND player_id = $2 AND submitted_at IS NULL AND expires_at > NOW()`
	insertSessionSQL        = `INSERT INTO game_sessions (id, game_code, player_id, session_secret, expires_at) VALUES ($1, $2, $3, $4, $5)`
	findSessionSQL          = `SELECT id, player_id, session_secret, expires_at, submitted_at FROM game_sessions WHERE id = $1 AND game_code = $2`
	consumeSessionSQL       = `UPDATE game_sessions SET submitted_at = NOW() WHERE id = $1 AND submitted_at IS NULL AND expires_at > NOW() RETURNING id`
	insertScoreSQL          = `INSERT INTO game_scores (session_id, game_code, player_id, player_name, score, duration_ms, score_date) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`
	rankSQL                 = `SELECT COUNT(*) + 1 FROM game_scores WHERE game_code = $1 AND score_date = $2 AND (score > $3 OR (score = $3 AND (created_at < $4 OR (created_at = $4 AND id < $5))))`
	top10SQL                = `SELECT player_name, score FROM game_scores WHERE game_code = $1 AND score_date = $2 ORDER BY score DESC, created_at ASC, id ASC LIMIT 10`
	top10AllTimeSQL         = `SELECT player_name, score FROM game_scores WHERE game_code = $1 ORDER BY score DESC, created_at ASC, id ASC LIMIT 10`
	pruneExpiredSessionsSQL = `DELETE FROM game_sessions WHERE expires_at < NOW() - INTERVAL '24 hours' AND submitted_at IS NULL`
)

func (r *PostgresRepository) CreateSession(ctx context.Context, sessionID uuid.UUID, gameCode string, playerID uuid.UUID, secret string, expiresAt time.Time, maxActive int64) error {
	encryptedSecret, err := security.EncryptSecret(r.SecretKey, secret)
	if err != nil {
		return fmt.Errorf("encrypt session secret: %w", err)
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Serialize session creation per game/player so the active-session limit cannot be exceeded.
	lockKey := gameCode + ":" + playerID.String()
	if _, err := tx.Exec(ctx, sessionLockSQL, lockKey); err != nil {
		return err
	}

	var active int64
	if err := tx.QueryRow(ctx, countActiveSessionsSQL, gameCode, playerID).Scan(&active); err != nil {
		return err
	}
	if active >= maxActive {
		return ErrTooManySessions
	}

	if _, err := tx.Exec(ctx, insertSessionSQL, sessionID, gameCode, playerID, encryptedSecret, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) FindSession(ctx context.Context, gameCode string, sessionID uuid.UUID) (Session, error) {
	var session Session
	var encryptedSecret string

	err := r.DB.QueryRow(ctx, findSessionSQL, sessionID, gameCode).
		Scan(&session.ID, &session.PlayerID, &encryptedSecret, &session.ExpiresAt, &session.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}

	session.Secret, err = security.DecryptSecret(r.SecretKey, encryptedSecret)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt session secret: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) ConsumeAndCreateScore(ctx context.Context, gameCode, scoreDate, name string, sessionID, playerID uuid.UUID, score int64, durationMS int) (Score, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Score{}, err
	}
	defer tx.Rollback(ctx)

	var consumedID uuid.UUID
	err = tx.QueryRow(ctx, consumeSessionSQL, sessionID).Scan(&consumedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Score{}, ErrSessionUnavailable
	}
	if err != nil {
		return Score{}, err
	}

	var result Score
	err = tx.QueryRow(ctx, insertScoreSQL, sessionID, gameCode, playerID, name, score, durationMS, scoreDate).
		Scan(&result.ID, &result.CreatedAt)
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
	err := r.DB.QueryRow(ctx, rankSQL, gameCode, scoreDate, score, createdAt, scoreID).Scan(&rank)
	return rank, err
}

func (r *PostgresRepository) Top10(ctx context.Context, gameCode, scoreDate string) ([]LeaderboardEntry, error) {
	return r.queryTop10(ctx, top10SQL, gameCode, scoreDate)
}

func (r *PostgresRepository) Top10AllTime(ctx context.Context, gameCode string) ([]LeaderboardEntry, error) {
	return r.queryTop10(ctx, top10AllTimeSQL, gameCode)
}

// queryTop10 reads leaderboard rows and masks offensive words in the player
// names. Names are stored raw, so censoring belongs here on the read path: every
// client-visible name goes through this one function, and a name that is not on
// a leaderboard is never returned to anyone.
func (r *PostgresRepository) queryTop10(ctx context.Context, query string, args ...any) ([]LeaderboardEntry, error) {
	rows, err := r.DB.Query(ctx, query, args...)
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
		entry.PlayerName = security.CensoredName(entry.PlayerName)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *PostgresRepository) PruneExpiredSessions(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, pruneExpiredSessionsSQL)
	return err
}
