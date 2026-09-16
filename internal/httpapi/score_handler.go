package httpapi

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"threadman-api/internal/leaderboard"
)

func (h *Handler) submitScore(c fiber.Ctx) error {
	var req submitScoreRequest
	if err := decodeJSON(c, &req); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "request body must be a single JSON object without unknown fields")
	}
	if req.SessionID == "" || req.Payload == "" {
		return jsonError(c, fiber.StatusBadRequest, "session_id and payload are required")
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "invalid session_id format")
	}

	ctx, cancel := h.withRequestTimeout(c)
	defer cancel()

	result, err := h.service.SubmitScore(ctx, c.Params("gameCode"), sessionID, req.Payload)
	if err != nil {
		return h.submitError(c, err)
	}

	return c.JSON(fiber.Map{
		"submitted_score": fiber.Map{
			"id":             result.Score.ID,
			"player_id":      result.Score.PlayerID.String(),
			"name":           result.Score.Name,
			"score":          result.Score.Value,
			"rank":           result.Rank,
			"rank_available": result.RankAvailable,
			"score_date":     result.ScoreDate,
		},
		"top10_today":           result.Top10,
		"leaderboard_available": result.LeaderboardAvailable,
	})
}

func (h *Handler) submitError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, leaderboard.ErrNotFound):
		return jsonError(c, fiber.StatusNotFound, "session not found")
	case errors.Is(err, leaderboard.ErrSessionUnavailable), errors.Is(err, leaderboard.ErrSessionUsed):
		return jsonError(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, leaderboard.ErrSessionExpired), errors.Is(err, leaderboard.ErrInvalidPayload):
		return jsonError(c, fiber.StatusBadRequest, err.Error())
	default:
		slog.Error("submit_score_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusInternalServerError, "score persistence failed")
	}
}
