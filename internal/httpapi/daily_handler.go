package httpapi

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"threadman-api/internal/leaderboard"
)

// getDaily serves GET /v1/game/:gameCode/daily[?player_id=<uuid>].
// Only the daily game code has a daily challenge.
func (h *Handler) getDaily(c fiber.Ctx) error {
	if !leaderboard.IsDailyGame(c.Params("gameCode")) {
		return jsonError(c, fiber.StatusNotFound, "no daily challenge for this game")
	}

	var playerID *uuid.UUID
	if raw := c.Query("player_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return jsonError(c, fiber.StatusBadRequest, "player_id must be a valid UUID")
		}
		playerID = &id
	}

	ctx, cancel := h.withRequestTimeout(c)
	defer cancel()

	info, err := h.service.Daily(ctx, playerID)
	if err != nil {
		slog.Error("daily_lookup_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusServiceUnavailable, "daily challenge unavailable")
	}
	h.metrics.dailyLookups.Add(1)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(info)
}
