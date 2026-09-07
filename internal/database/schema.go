package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitializeSchema(ctx context.Context, pool *pgxpool.Pool, path string) error {
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read database schema %q: %w", path, err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply database schema %q: %w", path, err)
	}
	return nil
}
