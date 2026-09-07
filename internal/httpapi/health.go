package httpapi

import (
	"context"

	"github.com/gofiber/fiber/v3"
)

func (h *Handler) health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *Handler) readyz(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), h.readinessTimeout)
	defer cancel()
	if h.ready == nil || h.ready(ctx) != nil {
		return jsonError(c, fiber.StatusServiceUnavailable, "database is not ready")
	}
	return c.JSON(fiber.Map{"status": "ready"})
}
