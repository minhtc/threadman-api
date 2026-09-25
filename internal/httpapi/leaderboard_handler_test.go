package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"threadman-api/internal/leaderboard"
)

type leaderboardRepositoryStub struct {
	dates []string
}

func (r *leaderboardRepositoryStub) CreateSession(context.Context, uuid.UUID, string, uuid.UUID, string, time.Time, int64) error {
	return nil
}

func (r *leaderboardRepositoryStub) FindSession(context.Context, string, uuid.UUID) (leaderboard.Session, error) {
	return leaderboard.Session{}, nil
}

func (r *leaderboardRepositoryStub) ConsumeAndCreateScore(context.Context, string, string, string, uuid.UUID, uuid.UUID, int64, int) (leaderboard.Score, error) {
	return leaderboard.Score{}, nil
}

func (r *leaderboardRepositoryStub) Rank(context.Context, string, string, int64, time.Time, int64) (int64, error) {
	return 1, nil
}

func (r *leaderboardRepositoryStub) Top10(_ context.Context, _, date string) ([]leaderboard.LeaderboardEntry, error) {
	r.dates = append(r.dates, date)
	return []leaderboard.LeaderboardEntry{{Rank: 1, PlayerName: "Ace", Score: 3200}}, nil
}

func (r *leaderboardRepositoryStub) Top10AllTime(context.Context, string) ([]leaderboard.LeaderboardEntry, error) {
	return []leaderboard.LeaderboardEntry{
		{Rank: 1, PlayerName: "Ace", Score: 9900},
		{Rank: 2, PlayerName: "Blitz", Score: 7400},
	}, nil
}

func (r *leaderboardRepositoryStub) PruneExpiredSessions(context.Context) error { return nil }

func newLeaderboardTestApp(repo leaderboard.Repository) *fiber.App {
	handler := NewHandler(leaderboard.NewService(repo, time.UTC, 10*time.Minute, 5), nil, nil)
	app := fiber.New()
	handler.Register(app, requestTestConfig())
	return app
}

func decodeLeaderboardResponse(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func TestLeaderboardRejectsUnknownPeriod(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/leaderboard?period=weekly", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestLeaderboardRejectsDateCombinedWithAllTimePeriod(t *testing.T) {
	app := newLeaderboardTestApp(&leaderboardRepositoryStub{})

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/leaderboard?period=all-time&date=2025-01-15", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestLeaderboardReturnsAllTimePeriod(t *testing.T) {
	repo := &leaderboardRepositoryStub{}
	app := newLeaderboardTestApp(repo)

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/leaderboard?period=all-time", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if len(repo.dates) != 0 {
		t.Fatalf("all-time request queried dates %v", repo.dates)
	}

	payload := decodeLeaderboardResponse(t, response)
	if payload["period"] != periodAllTime {
		t.Fatalf("period = %v, want %q", payload["period"], periodAllTime)
	}
	if _, ok := payload["score_date"]; ok {
		t.Fatal("all-time response must not contain score_date")
	}
	entries, ok := payload["top10"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("top10 = %v, want 2 entries", payload["top10"])
	}
}

func TestLeaderboardDefaultsToDayPeriod(t *testing.T) {
	repo := &leaderboardRepositoryStub{}
	app := newLeaderboardTestApp(repo)

	response, err := app.Test(httptest.NewRequest("GET", "/v1/game/threadman/leaderboard", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}

	payload := decodeLeaderboardResponse(t, response)
	if payload["period"] != "day" {
		t.Fatalf("period = %v, want day", payload["period"])
	}
	scoreDate, ok := payload["score_date"].(string)
	if !ok || scoreDate == "" {
		t.Fatalf("score_date = %v, want a YYYY-MM-DD value", payload["score_date"])
	}
	if len(repo.dates) != 1 || repo.dates[0] != scoreDate {
		t.Fatalf("dates = %v, want [%s]", repo.dates, scoreDate)
	}
}
