package httpapi

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestDailyReturnsSeedForDailyGame(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman_daily/daily?player_id=6f1c1c5e-8f7a-4a6e-9a51-6b1a4a0f9d11", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	payload := decodeLeaderboardResponse(t, response)
	if payload["score_date"] == "" || payload["seed"] == nil || payload["played"] != false {
		t.Fatalf("payload = %v", payload)
	}
}

func TestDailyNotFoundForRegularGame(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/daily", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusNotFound)
	}
}

func TestDailyRejectsBadPlayerID(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman_daily/daily?player_id=nope", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestMetricsExposeDailyCounters(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	if _, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman_daily/daily", nil)); err != nil {
		t.Fatal(err)
	}
	response, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	for _, want := range []string{
		`game_sessions_created_total{game="threadman_daily"} 0`,
		`game_scores_submitted_total{game="threadman"} 0`,
		"daily_replay_rejected_total 0",
		"daily_lookups_total 1",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}
