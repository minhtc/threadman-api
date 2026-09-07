package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"homielab-api/internal/app"
	"homielab-api/internal/config"
	"homielab-api/internal/database"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := database.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	schemaCtx, schemaCancel := context.WithTimeout(ctx, cfg.DBConnectTimeout)
	if err := database.InitializeSchema(schemaCtx, pool, cfg.SchemaPath); err != nil {
		schemaCancel()
		log.Fatal(err)
	}
	schemaCancel()

	server := app.New(cfg, pool)
	go server.StartSessionPruner(ctx, cfg.PruneInterval, cfg.PruneTimeout)

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("starting server on :%s (timezone: %s, origins: %v)", cfg.Port, cfg.Timezone, cfg.AllowedOrigins)
	if err := server.Listen(":" + cfg.Port); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
