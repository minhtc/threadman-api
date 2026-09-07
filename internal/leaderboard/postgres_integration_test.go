package leaderboard

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run with TEST_DATABASE_URL set. It verifies the database-backed advisory lock,
// not just the in-memory service behavior.
func TestPostgresSessionLimitUnderContention(t *testing.T) {
	connectionURL := os.Getenv("TEST_DATABASE_URL")
	if connectionURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	schema, err := os.ReadFile("../../database/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}

	playerID := uuid.New()
	gameCode := "it-" + uuid.NewString()[:16]
	key := make([]byte, 32)
	key[0] = 1
	repo := &PostgresRepository{DB: pool, SecretKey: key}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM game_scores WHERE player_id = $1`, playerID)
		_, _ = pool.Exec(ctx, `DELETE FROM game_sessions WHERE player_id = $1`, playerID)
	}()

	const attempts = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	tooMany := 0
	errs := make([]error, 0)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.CreateSession(ctx, uuid.New(), gameCode, playerID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", time.Now().Add(time.Hour), 5)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrTooManySessions):
				tooMany++
			default:
				errs = append(errs, err)
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("unexpected database errors: %v", errs)
	}
	if successes != 5 || tooMany != attempts-5 {
		t.Fatalf("successes=%d tooMany=%d, want successes=5 tooMany=%d", successes, tooMany, attempts-5)
	}
}
