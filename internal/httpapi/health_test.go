package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"threadman-api/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		AllowedOrigins: []string{"*"}, SessionRateLimit: 20, ScoreRateLimit: 10,
		LeaderboardRateLimit: 60, RateLimitWindow: time.Minute,
		RequestTimeout: 3 * time.Second, ReadinessTimeout: 2 * time.Second}
}

func TestHealthAndReadiness(t *testing.T) {
	h := NewHandler(nil, func(context.Context) error { return nil }, nil)
	app := fiber.New()
	h.Register(app, testConfig())

	response, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil || response.StatusCode != fiber.StatusOK {
		t.Fatalf("healthz status=%v err=%v", response.StatusCode, err)
	}
	if response.Header.Get(fiber.HeaderXRequestID) == "" {
		t.Fatal("healthz did not return a request ID")
	}

	response, err = app.Test(httptest.NewRequest("GET", "/readyz", nil))
	if err != nil || response.StatusCode != fiber.StatusOK {
		t.Fatalf("readyz status=%v err=%v", response.StatusCode, err)
	}
}

func TestReadinessFailureIncludesRequestID(t *testing.T) {
	h := NewHandler(nil, func(context.Context) error { return context.Canceled }, nil)
	app := fiber.New()
	h.Register(app, testConfig())
	response, err := app.Test(httptest.NewRequest("GET", "/readyz", nil))
	if err != nil || response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("readyz status=%v err=%v", response.StatusCode, err)
	}
	if response.Header.Get(fiber.HeaderXRequestID) == "" {
		t.Fatal("readiness failure did not return a request ID")
	}
}
