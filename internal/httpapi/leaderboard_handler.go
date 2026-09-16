package httpapi

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (h *Handler) getLeaderboard(c fiber.Ctx) error {
	requestedDate := c.Query("date")
	if requestedDate != "" {
		parsed, err := time.Parse(dateTimeLayout, requestedDate)
		if err != nil || parsed.Format(dateTimeLayout) != requestedDate {
			return jsonError(c, fiber.StatusBadRequest, "date must use YYYY-MM-DD format")
		}
	}

	ctx, cancel := h.withRequestTimeout(c)
	defer cancel()

	date, top10, err := h.service.Leaderboard(ctx, c.Params("gameCode"), requestedDate)
	if err != nil {
		slog.Error("get_leaderboard_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusInternalServerError, "failed to fetch leaderboard")
	}

	return c.JSON(fiber.Map{
		"game_code":  c.Params("gameCode"),
		"score_date": date,
		"top10":      top10,
	})
}
