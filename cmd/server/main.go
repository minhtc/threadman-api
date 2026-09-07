package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"homielab-api/internal/app"
	"homielab-api/internal/config"
	"homielab-api/internal/database"
)

func main() {
	_ = godotenv.Load()

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

	server := app.New(cfg, pool)
	go server.StartSessionPruner(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
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
