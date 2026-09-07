package httpapi

import (
	"context"
	"errors"
	"log"
	"mime"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"

	"homielab-api/internal/config"
	"homielab-api/internal/leaderboard"
)

type Handler struct{ service *leaderboard.Service }

func NewHandler(service *leaderboard.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(app *fiber.App, cfg *config.Config) {
	app.Use(recover.New(), requestid.New())
	app.Use(logger.New(logger.Config{Format: "[${time}] ${locals:requestid} ${status} - ${method} ${path} (${ip})\n"}))
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.AllowedOrigins, AllowMethods: []string{"GET", "POST", "OPTIONS"}, AllowHeaders: []string{"Content-Type", "X-Request-ID"}}))

	api := app.Group("/v1/game/:gameCode", validateGame)
	api.Post("/session", enforceJSON, limiter.New(limiter.Config{Max: 20, Expiration: time.Minute}), h.createSession)
	api.Post("/score", enforceJSON, limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), h.submitScore)
	api.Get("/leaderboard", limiter.New(limiter.Config{Max: 60, Expiration: time.Minute}), h.getLeaderboard)
}

func validateGame(c fiber.Ctx) error {
	if !leaderboard.IsSupportedGame(c.Params("gameCode")) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unsupported game"})
	}
	return c.Next()
}

func enforceJSON(c fiber.Ctx) error {
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "Content-Type must be application/json"})
	}
	return c.Next()
}

type createSessionRequest struct {
	PlayerID string `json:"player_id"`
}
type submitScoreRequest struct {
	SessionID string `json:"session_id"`
	Payload   string `json:"payload"`
}

func (h *Handler) createSession(c fiber.Ctx) error {
	var req createSessionRequest
	if err := c.Bind().Body(&req); err != nil || req.PlayerID == "" {
		return jsonError(c, fiber.StatusBadRequest, "player_id is required")
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "player_id must be a valid UUID")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	session, err := h.service.CreateSession(ctx, c.Params("gameCode"), playerID)
	if err != nil {
		if errors.Is(err, leaderboard.ErrTooManySessions) {
			return jsonError(c, fiber.StatusTooManyRequests, err.Error())
		}
		log.Printf("create session: %v", err)
		return jsonError(c, fiber.StatusInternalServerError, "session database error")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"session_id": session.ID.String(), "session_secret": session.Secret, "expires_at": session.ExpiresAt.Format(time.RFC3339)})
}

func (h *Handler) submitScore(c fiber.Ctx) error {
	var req submitScoreRequest
	if err := c.Bind().Body(&req); err != nil || req.SessionID == "" || req.Payload == "" {
		return jsonError(c, fiber.StatusBadRequest, "session_id and payload are required")
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "invalid session_id format")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	result, err := h.service.SubmitScore(ctx, c.Params("gameCode"), sessionID, req.Payload)
	if err != nil {
		return h.submitError(c, err)
	}
	return c.JSON(fiber.Map{"submitted_score": fiber.Map{"id": result.Score.ID, "player_id": result.Score.PlayerID.String(), "name": result.Score.Name, "score": result.Score.Value, "rank": result.Rank, "score_date": result.ScoreDate}, "top10_today": result.Top10})
}

func (h *Handler) submitError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, leaderboard.ErrNotFound):
		return jsonError(c, fiber.StatusNotFound, "session not found")
	case errors.Is(err, leaderboard.ErrSessionUnavailable):
		return jsonError(c, fiber.StatusConflict, "session expired or already submitted")
	case errors.Is(err, leaderboard.ErrSessionUsed):
		return jsonError(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, leaderboard.ErrSessionExpired):
		return jsonError(c, fiber.StatusBadRequest, err.Error())
	case errors.Is(err, leaderboard.ErrInvalidPayload):
		return jsonError(c, fiber.StatusBadRequest, errorMessage(err))
	default:
		log.Printf("submit score: %v", err)
		return jsonError(c, fiber.StatusInternalServerError, "failed to record score")
	}
}

func (h *Handler) getLeaderboard(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	date, top10, err := h.service.Leaderboard(ctx, c.Params("gameCode"))
	if err != nil {
		log.Printf("get leaderboard: %v", err)
		return jsonError(c, fiber.StatusInternalServerError, "failed to fetch leaderboard")
	}
	return c.JSON(fiber.Map{"game_code": c.Params("gameCode"), "score_date": date, "top10": top10})
}

func errorMessage(err error) string {
	const separator = ": "
	message := err.Error()
	for i := 0; i+len(separator) <= len(message); i++ {
		if message[i:i+len(separator)] == separator {
			return message[i+len(separator):]
		}
	}
	return message
}

func jsonError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}
