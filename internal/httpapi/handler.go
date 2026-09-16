package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"

	"threadman-api/internal/config"
	"threadman-api/internal/leaderboard"
)

// dateTimeLayout is the canonical calendar-day format used by leaderboard dates.
const dateTimeLayout = "2006-01-02"

type Handler struct {
	service          *leaderboard.Service
	ready            func(context.Context) error
	poolStats        func() PoolStats
	requestTimeout   time.Duration
	readinessTimeout time.Duration
	metrics          *Metrics
}

type createSessionRequest struct {
	PlayerID string `json:"player_id"`
}

type submitScoreRequest struct {
	SessionID string `json:"session_id"`
	Payload   string `json:"payload"`
}

// NewHandler builds the HTTP layer. Timeouts are applied by Register from config.
func NewHandler(service *leaderboard.Service, ready func(context.Context) error, poolStats func() PoolStats) *Handler {
	return &Handler{
		service:   service,
		ready:     ready,
		poolStats: poolStats,
		metrics:   &Metrics{},
	}
}

func (h *Handler) Register(app *fiber.App, cfg *config.Config) {
	h.requestTimeout = cfg.RequestTimeout
	h.readinessTimeout = cfg.ReadinessTimeout

	app.Use(
		recover.New(),
		requestid.New(),
		h.requestLogger,
		cors.New(cors.Config{
			AllowOrigins: cfg.AllowedOrigins,
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "X-Request-ID"},
		}),
	)

	app.Get("/healthz", h.health)
	app.Get("/readyz", h.readyz)
	app.Get("/metrics", h.metricsHandler)

	api := app.Group("/v1/game/:gameCode", validateGame)
	api.Post("/session",
		enforceJSON,
		newRateLimiter(cfg.SessionRateLimit, cfg.RateLimitWindow),
		h.createSession,
	)
	api.Post("/score",
		enforceJSON,
		newRateLimiter(cfg.ScoreRateLimit, cfg.RateLimitWindow),
		h.submitScore,
	)
	api.Get("/leaderboard",
		newRateLimiter(cfg.LeaderboardRateLimit, cfg.RateLimitWindow),
		h.getLeaderboard,
	)
}

func newRateLimiter(max int, window time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{Max: max, Expiration: window})
}

func (h *Handler) withRequestTimeout(c fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Context(), h.requestTimeout)
}

func validateGame(c fiber.Ctx) error {
	if !leaderboard.IsSupportedGame(c.Params("gameCode")) {
		return jsonError(c, fiber.StatusNotFound, "unsupported game")
	}
	return c.Next()
}

func enforceJSON(c fiber.Ctx) error {
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return jsonError(c, fiber.StatusUnsupportedMediaType, "Content-Type must be application/json")
	}
	return c.Next()
}

func decodeJSON(c fiber.Ctx, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func (h *Handler) requestLogger(c fiber.Ctx) error {
	started := time.Now()
	err := c.Next()
	h.metrics.requests.Add(1)
	slog.Info("http_request",
		"request_id", requestID(c),
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"duration_ms", time.Since(started).Milliseconds(),
		"ip", c.IP(),
	)
	return err
}

func (h *Handler) createSession(c fiber.Ctx) error {
	var req createSessionRequest
	if err := decodeJSON(c, &req); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "request body must be a single JSON object without unknown fields")
	}
	if req.PlayerID == "" {
		return jsonError(c, fiber.StatusBadRequest, "player_id is required")
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "player_id must be a valid UUID")
	}

	ctx, cancel := h.withRequestTimeout(c)
	defer cancel()

	session, err := h.service.CreateSession(ctx, c.Params("gameCode"), playerID)
	if err != nil {
		if errors.Is(err, leaderboard.ErrTooManySessions) {
			return jsonError(c, fiber.StatusTooManyRequests, err.Error())
		}
		slog.Error("create_session_failed", "request_id", requestID(c), "error", err)
		return jsonError(c, fiber.StatusInternalServerError, "session database error")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"session_id":     session.ID.String(),
		"session_secret": session.Secret,
		"expires_at":     session.ExpiresAt.Format(time.RFC3339),
	})
}

func requestID(c fiber.Ctx) string {
	if id := requestid.FromContext(c); id != "" {
		return id
	}
	return c.Get(fiber.HeaderXRequestID)
}

func jsonError(c fiber.Ctx, status int, message string) error {
	response := fiber.Map{"error": message}
	if id := requestID(c); id != "" {
		response["request_id"] = id
	}
	return c.Status(status).JSON(response)
}
