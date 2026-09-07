package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"threadman-api/internal/config"
)

func Open(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MinConns = cfg.DBMinConns
	poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolConfig.MaxConnLifetimeJitter = cfg.DBMaxConnLifetimeJitter
	poolConfig.PingTimeout = cfg.DBPingTimeout

	initCtx, cancel := context.WithTimeout(ctx, cfg.DBConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(initCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	if err := pool.Ping(initCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	return pool, nil
}
