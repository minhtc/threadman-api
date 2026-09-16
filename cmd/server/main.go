package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"threadman-api/internal/app"
	"threadman-api/internal/config"
	"threadman-api/internal/database"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		fatal("config_load_failed", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := database.Open(ctx, cfg)
	if err != nil {
		fatal("database_open_failed", err)
	}
	defer pool.Close()

	schemaCtx, schemaCancel := context.WithTimeout(ctx, cfg.DBConnectTimeout)
	if err := database.InitializeSchema(schemaCtx, pool, cfg.SchemaPath); err != nil {
		schemaCancel()
		fatal("schema_init_failed", err)
	}
	schemaCancel()

	server := app.New(cfg, pool)
	go server.StartSessionPruner(ctx, cfg.PruneInterval, cfg.PruneTimeout)

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.ShutdownWithContext(shutdownCtx); err != nil {
			slog.Error("server_shutdown_failed", "error", err)
		}
	}()

	slog.Info("server_starting", "port", cfg.Port, "timezone", cfg.Timezone.String(), "origins", cfg.AllowedOrigins)
	if err := server.Listen(":" + cfg.Port); err != nil && ctx.Err() == nil {
		fatal("server_listen_failed", err)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}
