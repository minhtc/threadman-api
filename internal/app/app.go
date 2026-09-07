package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"threadman-api/internal/config"
	"threadman-api/internal/httpapi"
	"threadman-api/internal/leaderboard"
)

type Server struct {
	*fiber.App
	service *leaderboard.Service
}

func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	repo := &leaderboard.PostgresRepository{DB: pool, SecretKey: cfg.SessionSecretKey}
	service := leaderboard.NewService(repo, cfg.Timezone, cfg.SessionTTL, cfg.MaxActiveSessions)
	fiberApp := fiber.New(fiber.Config{AppName: "Threadman Leaderboard API", BodyLimit: cfg.BodyLimit, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout, TrustProxy: cfg.TrustProxy, ProxyHeader: cfg.ProxyHeader, TrustProxyConfig: fiber.TrustProxyConfig{Proxies: cfg.TrustedProxies}})
	httpapi.NewHandler(service, pool.Ping, func() httpapi.PoolStats {
		stats := pool.Stat()
		return httpapi.PoolStats{Total: stats.TotalConns(), Acquired: stats.AcquiredConns(), Idle: stats.IdleConns(), Max: stats.MaxConns(), EmptyAcquireWait: stats.EmptyAcquireWaitTime()}
	}).Register(fiberApp, cfg)

	// Serve static files after API routes so API endpoints keep precedence.
	registerStaticFiles(fiberApp)

	return &Server{App: fiberApp, service: service}
}

func (s *Server) StartSessionPruner(ctx context.Context, interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneCtx, cancel := context.WithTimeout(context.Background(), timeout)
			if err := s.service.PruneSessions(pruneCtx); err != nil {
				slog.Default().Error("session_pruning_failed", "error", err)
			}
			cancel()
		}
	}
}
