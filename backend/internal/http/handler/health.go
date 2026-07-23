package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler expone /health/live y /health/ready.
type HealthHandler struct {
	pool *pgxpool.Pool
}

// NewHealthHandler construye el handler.
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler { return &HealthHandler{pool: pool} }

// Live responde 200 si el proceso está vivo.
func (h *HealthHandler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Ready verifica conectividad con la BD.
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 1_500_000_000) // 1.5s
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_unreachable"})
	}
	return c.JSON(fiber.Map{"status": "ready"})
}
