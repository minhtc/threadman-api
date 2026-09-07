package httpapi

import (
	"context"
	"log/slog"

	"github.com/gofiber/fiber/v3"
)

func (h *Handler) getLeaderboard(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), h.requestTimeout)
	defer cancel()
	date, top10, err := h.service.Leaderboard(ctx, c.Params("gameCode"))
	if err != nil {
		slog.Default().Error("get_leaderboard_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusInternalServerError, "failed to fetch leaderboard")
	}
	return c.JSON(fiber.Map{"game_code": c.Params("gameCode"), "score_date": date, "top10": top10})
}
