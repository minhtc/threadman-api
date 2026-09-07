package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	Timezone                *time.Location
	AllowedOrigins          []string
	SessionSecretKey        []byte
	SessionTTL              time.Duration
	PruneInterval           time.Duration
	MaxActiveSessions       int64
	SessionRateLimit        int
	ScoreRateLimit          int
	LeaderboardRateLimit    int
	RateLimitWindow         time.Duration
	BodyLimit               int
	RequestTimeout          time.Duration
	ReadinessTimeout        time.Duration
	PruneTimeout            time.Duration
	DBConnectTimeout        time.Duration
	DBPingTimeout           time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
	ShutdownTimeout         time.Duration
	DBMaxConns              int32
	DBMinConns              int32
	DBMaxConnIdleTime       time.Duration
	DBMaxConnLifetime       time.Duration
	DBMaxConnLifetimeJitter time.Duration
	TrustProxy              bool
	ProxyHeader             string
	TrustedProxies          []string
	SchemaPath              string
}

func Load() (*Config, error) {
	port := getenv("PORT", "8080")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("PORT must be a number between 1 and 65535")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	parsedURL, err := url.Parse(databaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("DATABASE_URL must be a valid database URL")
	}

	timezone, err := time.LoadLocation(getenv("LEADERBOARD_TIMEZONE", "UTC"))
	if err != nil {
		return nil, fmt.Errorf("invalid LEADERBOARD_TIMEZONE: %w", err)
	}
	origins, err := parseOrigins(os.Getenv("ALLOWED_ORIGINS"))
	if err != nil {
		return nil, err
	}
	trustedProxies, err := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return nil, err
	}
	trustProxy, err := boolEnv("TRUST_PROXY", false)
	if err != nil {
		return nil, err
	}
	secretKey, err := parseSecretKey(os.Getenv("SESSION_SECRET_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port: port, DatabaseURL: databaseURL, Timezone: timezone, AllowedOrigins: origins,
		SessionSecretKey: secretKey, TrustProxy: trustProxy,
		ProxyHeader: getenv("PROXY_HEADER", "CF-Connecting-IP"), TrustedProxies: trustedProxies,
		SchemaPath: getenv("SCHEMA_PATH", "database/schema.sql"),
		SessionTTL: 10 * time.Minute, PruneInterval: 15 * time.Minute, MaxActiveSessions: 5,
		SessionRateLimit: 20, ScoreRateLimit: 10, LeaderboardRateLimit: 60, RateLimitWindow: time.Minute,
		BodyLimit: 4 * 1024, RequestTimeout: 3 * time.Second, ReadinessTimeout: 2 * time.Second, PruneTimeout: 5 * time.Second,
		DBConnectTimeout: 5 * time.Second, DBPingTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		IdleTimeout: 30 * time.Second, ShutdownTimeout: 10 * time.Second,
		DBMaxConns: 15, DBMinConns: 3, DBMaxConnIdleTime: 5 * time.Minute,
		DBMaxConnLifetime: time.Hour, DBMaxConnLifetimeJitter: 5 * time.Minute,
	}
	if err := cfg.loadOverrides(); err != nil {
		return nil, err
	}
	if cfg.ProxyHeader == "" {
		return nil, fmt.Errorf("PROXY_HEADER must not be empty")
	}
	if cfg.TrustProxy && len(cfg.TrustedProxies) == 0 {
		return nil, fmt.Errorf("TRUSTED_PROXIES is required when TRUST_PROXY=true")
	}
	if cfg.DBMinConns < 0 || cfg.DBMaxConns < cfg.DBMinConns || cfg.MaxActiveSessions < 1 || cfg.BodyLimit < 1 || cfg.SessionRateLimit < 1 || cfg.ScoreRateLimit < 1 || cfg.LeaderboardRateLimit < 1 {
		return nil, fmt.Errorf("numeric configuration values are out of range")
	}
	return cfg, nil
}

func (c *Config) loadOverrides() error {
	var err error
	if c.SessionTTL, err = durationEnv("SESSION_TTL", c.SessionTTL); err != nil {
		return err
	}
	if c.PruneInterval, err = durationEnv("PRUNE_INTERVAL", c.PruneInterval); err != nil {
		return err
	}
	if c.RateLimitWindow, err = durationEnv("RATE_LIMIT_WINDOW", c.RateLimitWindow); err != nil {
		return err
	}
	if c.RequestTimeout, err = durationEnv("REQUEST_TIMEOUT", c.RequestTimeout); err != nil {
		return err
	}
	if c.ReadinessTimeout, err = durationEnv("READINESS_TIMEOUT", c.ReadinessTimeout); err != nil {
		return err
	}
	if c.PruneTimeout, err = durationEnv("PRUNE_TIMEOUT", c.PruneTimeout); err != nil {
		return err
	}
	if c.DBConnectTimeout, err = durationEnv("DB_CONNECT_TIMEOUT", c.DBConnectTimeout); err != nil {
		return err
	}
	if c.DBPingTimeout, err = durationEnv("DB_PING_TIMEOUT", c.DBPingTimeout); err != nil {
		return err
	}
	if c.ReadTimeout, err = durationEnv("READ_TIMEOUT", c.ReadTimeout); err != nil {
		return err
	}
	if c.WriteTimeout, err = durationEnv("WRITE_TIMEOUT", c.WriteTimeout); err != nil {
		return err
	}
	if c.IdleTimeout, err = durationEnv("IDLE_TIMEOUT", c.IdleTimeout); err != nil {
		return err
	}
	if c.ShutdownTimeout, err = durationEnv("SHUTDOWN_TIMEOUT", c.ShutdownTimeout); err != nil {
		return err
	}
	if c.DBMaxConnIdleTime, err = durationEnv("DB_MAX_CONN_IDLE_TIME", c.DBMaxConnIdleTime); err != nil {
		return err
	}
	if c.DBMaxConnLifetime, err = durationEnv("DB_MAX_CONN_LIFETIME", c.DBMaxConnLifetime); err != nil {
		return err
	}
	if c.DBMaxConnLifetimeJitter, err = durationEnv("DB_MAX_CONN_LIFETIME_JITTER", c.DBMaxConnLifetimeJitter); err != nil {
		return err
	}
	if c.MaxActiveSessions, err = int64Env("MAX_ACTIVE_SESSIONS", c.MaxActiveSessions); err != nil {
		return err
	}
	if c.SessionRateLimit, err = intEnv("SESSION_RATE_LIMIT", c.SessionRateLimit); err != nil {
		return err
	}
	if c.ScoreRateLimit, err = intEnv("SCORE_RATE_LIMIT", c.ScoreRateLimit); err != nil {
		return err
	}
	if c.LeaderboardRateLimit, err = intEnv("LEADERBOARD_RATE_LIMIT", c.LeaderboardRateLimit); err != nil {
		return err
	}
	if c.BodyLimit, err = intEnv("BODY_LIMIT_BYTES", c.BodyLimit); err != nil {
		return err
	}
	if c.DBMaxConns, err = int32Env("DB_MAX_CONNS", c.DBMaxConns); err != nil {
		return err
	}
	if c.DBMinConns, err = int32Env("DB_MIN_CONNS", c.DBMinConns); err != nil {
		return err
	}
	return nil
}

func parseSecretKey(raw string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("SESSION_SECRET_ENCRYPTION_KEY must be base64-encoded 32-byte key")
	}
	return key, nil
}

func parseOrigins(raw string) ([]string, error) {
	values := parseCSV(raw)
	if len(values) == 0 || (len(values) == 1 && values[0] == "*") {
		return []string{"*"}, nil
	}
	origins := make([]string, 0, len(values))
	for _, origin := range values {
		origin = strings.TrimRight(origin, "/")
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

func parseTrustedProxies(raw string) ([]string, error) {
	proxies := parseCSV(raw)
	for _, proxy := range proxies {
		if net.ParseIP(proxy) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES contains invalid IP or CIDR %q", proxy)
		}
	}
	return proxies, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func parseCSV(raw string) []string {
	values := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}
func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
}
func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}
func int64Env(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}
func int32Env(key string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return int32(parsed), nil
}
func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}
