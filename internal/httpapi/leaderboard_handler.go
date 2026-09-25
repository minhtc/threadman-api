package httpapi

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// periodAllTime selects the leaderboard that ignores score_date partitions.
const periodAllTime = "all-time"

func (h *Handler) getLeaderboard(c fiber.Ctx) error {
	period := c.Query("period")
	if period != "" && period != periodAllTime {
		return jsonError(c, fiber.StatusBadRequest, "period must be all-time")
	}

	requestedDate := c.Query("date")
	if requestedDate != "" {
		if period == periodAllTime {
			return jsonError(c, fiber.StatusBadRequest, "date cannot be combined with period=all-time")
		}
		parsed, err := time.Parse(dateTimeLayout, requestedDate)
		if err != nil || parsed.Format(dateTimeLayout) != requestedDate {
			return jsonError(c, fiber.StatusBadRequest, "date must use YYYY-MM-DD format")
		}
	}

	ctx, cancel := h.withRequestTimeout(c)
	defer cancel()

	gameCode := c.Params("gameCode")

	if period == periodAllTime {
		top10, err := h.service.AllTimeLeaderboard(ctx, gameCode)
		if err != nil {
			slog.Error("get_all_time_leaderboard_failed", "request_id", requestID(c), "error", err)
			return jsonError(c, fiber.StatusInternalServerError, "failed to fetch leaderboard")
		}
		return c.JSON(fiber.Map{
			"game_code": gameCode,
			"period":    periodAllTime,
			"top10":     top10,
		})
	}

	date, top10, err := h.service.Leaderboard(ctx, gameCode, requestedDate)
	if err != nil {
		slog.Error("get_leaderboard_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusInternalServerError, "failed to fetch leaderboard")
	}

	return c.JSON(fiber.Map{
		"game_code":  gameCode,
		"period":     "day",
		"score_date": date,
		"top10":      top10,
	})
}
