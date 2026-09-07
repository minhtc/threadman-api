package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	Timezone       *time.Location
	AllowedOrigins []string
}

func Load() (*Config, error) {
	port := getenv("PORT", "8080")
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	timezoneName := getenv("LEADERBOARD_TIMEZONE", "UTC")
	timezone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return nil, fmt.Errorf("invalid LEADERBOARD_TIMEZONE: %w", err)
	}

	return &Config{
		Port:           port,
		DatabaseURL:    databaseURL,
		Timezone:       timezone,
		AllowedOrigins: parseOrigins(os.Getenv("ALLOWED_ORIGINS")),
	}, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseOrigins(raw string) []string {
	origins := make([]string, 0)
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}
