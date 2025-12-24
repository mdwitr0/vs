package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/crawler"
	"github.com/player-monitor/backend/internal/middleware"
	"github.com/player-monitor/backend/internal/repo"
)

type PageHandler struct {
	pageRepo *repo.PageRepo
	siteRepo *repo.SiteRepo
	crawler  *crawler.Crawler
}

func NewPageHandler(
	pageRepo *repo.PageRepo,
	siteRepo *repo.SiteRepo,
	crawler *crawler.Crawler,
) *PageHandler {
	return &PageHandler{
		pageRepo: pageRepo,
		siteRepo: siteRepo,
		crawler:  crawler,
	}
}

type UpdateExcludeRequest struct {
	Exclude bool `json:"exclude"`
}

func (h *PageHandler) UpdateExclude(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	var req UpdateExcludeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	page, err := h.pageRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch page"})
	}
	if page == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "page not found"})
	}

	if err := h.pageRepo.UpdateExcludeFlag(c.Context(), id, filterUserID, req.Exclude); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to update page"})
	}

	if err := h.crawler.UpdateSiteStats(c.Context(), page.SiteID); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to update site stats"})
	}

	return c.JSON(SuccessResponse{Message: "page updated"})
}
