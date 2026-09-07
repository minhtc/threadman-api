package app

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"homielab-api/internal/config"
	"homielab-api/internal/httpapi"
	"homielab-api/internal/leaderboard"
)

type Server struct {
	*fiber.App
	service *leaderboard.Service
}

func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	service := leaderboard.NewService(&leaderboard.PostgresRepository{DB: pool}, cfg.Timezone)
	fiberApp := fiber.New(fiber.Config{AppName: "Threadman Leaderboard API", BodyLimit: 4 * 1024})
	httpapi.NewHandler(service).Register(fiberApp, cfg)
	return &Server{App: fiberApp, service: service}
}

func (s *Server) StartSessionPruner(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.service.PruneSessions(pruneCtx); err != nil {
				log.Printf("session pruning worker error: %v", err)
			}
			cancel()
		}
	}
}
