package httpapi

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"threadman-api/internal/config"
)

func requestTestConfig() *config.Config {
	return &config.Config{
		AllowedOrigins:       []string{"*"},
		SessionRateLimit:     20,
		ScoreRateLimit:       10,
		LeaderboardRateLimit: 60,
		RateLimitWindow:      time.Minute,
		RequestTimeout:       3 * time.Second,
		ReadinessTimeout:     2 * time.Second,
	}
}

func TestStrictJSONRejectsUnknownFieldsWithRequestID(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	app := fiber.New()
	h.Register(app, requestTestConfig())

	request := httptest.NewRequest("POST", "/v1/game/threadman/session", strings.NewReader(`{"player_id":"9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d","unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
	if response.Header.Get(fiber.HeaderXRequestID) == "" {
		t.Fatal("response did not contain X-Request-ID")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "request_id") {
		t.Fatalf("error body = %s, want request_id", body)
	}
}

func TestLeaderboardRejectsInvalidDate(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	app := fiber.New()
	h.Register(app, requestTestConfig())

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/leaderboard?date=2025-1-15", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestHealthRouteDoesNotRequireDatabase(t *testing.T) {
	h := NewHandler(nil, func(context.Context) error { return context.Canceled }, nil)
	app := fiber.New()
	h.Register(app, requestTestConfig())

	response, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil || response.StatusCode != fiber.StatusOK {
		t.Fatalf("healthz status=%v err=%v", response.StatusCode, err)
	}
}
