package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/middleware"
	"github.com/player-monitor/backend/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SettingsHandler struct {
	settingsRepo *repo.SettingsRepo
}

func NewSettingsHandler(settingsRepo *repo.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: settingsRepo}
}

type UpdateSettingsRequest struct {
	PlayerPattern     string `json:"player_pattern"`
	ScanIntervalHours int    `json:"scan_interval_hours"`
}

func (h *SettingsHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
	}

	settings, err := h.settingsRepo.GetByUserID(c.Context(), userOID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch settings"})
	}

	return c.JSON(settings)
}

func (h *SettingsHandler) Update(c *fiber.Ctx) error {
	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.PlayerPattern == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "player_pattern is required"})
	}

	if req.ScanIntervalHours <= 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "scan_interval_hours must be positive"})
	}

	userID := middleware.GetUserID(c)
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
	}

	settings, err := h.settingsRepo.GetByUserID(c.Context(), userOID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch settings"})
	}

	settings.PlayerPattern = req.PlayerPattern
	settings.ScanIntervalHours = req.ScanIntervalHours

	if err := h.settingsRepo.Update(c.Context(), settings); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to update settings"})
	}

	return c.JSON(settings)
}
