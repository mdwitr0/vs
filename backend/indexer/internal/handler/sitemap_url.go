package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/indexer/internal/repo"
)

type SitemapURLHandler struct {
	sitemapURLRepo *repo.SitemapURLRepo
}

func NewSitemapURLHandler(sitemapURLRepo *repo.SitemapURLRepo) *SitemapURLHandler {
	return &SitemapURLHandler{
		sitemapURLRepo: sitemapURLRepo,
	}
}

type SitemapURLsResponse struct {
	URLs  []repo.SitemapURL `json:"urls"`
	Total int64             `json:"total"`
	Limit int               `json:"limit"`
	Page  int               `json:"page"`
}

// ListSitemapURLs godoc
// @Summary List sitemap URLs for a site
// @Description Get paginated list of URLs from sitemap
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Param status query string false "Filter by status (pending, indexed, error, skipped)"
// @Param limit query int false "Items per page" default(50)
// @Param page query int false "Page number" default(1)
// @Success 200 {object} SitemapURLsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/{id}/sitemap-urls [get]
func (h *SitemapURLHandler) List(c *fiber.Ctx) error {
	siteID := c.Params("id")
	if siteID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "site_id is required"})
	}

	status := c.Query("status", "")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	page, _ := strconv.Atoi(c.Query("page", "1"))

	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	urls, total, err := h.sitemapURLRepo.FindByFilter(c.Context(), siteID, status, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(SitemapURLsResponse{
		URLs:  urls,
		Total: total,
		Limit: limit,
		Page:  page,
	})
}

// GetSitemapURLsStats godoc
// @Summary Get sitemap URLs statistics
// @Description Get count of URLs by status
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} repo.SitemapURLStats
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/{id}/sitemap-urls/stats [get]
func (h *SitemapURLHandler) Stats(c *fiber.Ctx) error {
	siteID := c.Params("id")
	if siteID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "site_id is required"})
	}

	stats, err := h.sitemapURLRepo.GetStats(c.Context(), siteID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(stats)
}

type PendingURLWithDepth struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

type PendingURLsResponse struct {
	URLs        []PendingURLWithDepth `json:"urls"`
	AllIndexed  bool                  `json:"all_indexed,omitempty"`
	InRetry     bool                  `json:"in_retry,omitempty"`
	TotalURLs   int64                 `json:"total_urls,omitempty"`
	IndexedURLs int64                 `json:"indexed_urls,omitempty"`
}

type AllURLsResponse struct {
	URLs  []string `json:"urls"`
	Count int      `json:"count"`
}

// GetAllURLs godoc
// @Summary Get all discovered URLs for a site
// @Description Returns all URL strings for a site (for crawler deduplication)
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} AllURLsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/{id}/all-urls [get]
func (h *SitemapURLHandler) GetAllURLs(c *fiber.Ctx) error {
	siteID := c.Params("id")
	if siteID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "site_id is required"})
	}

	urls, err := h.sitemapURLRepo.GetAllURLStrings(c.Context(), siteID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(AllURLsResponse{
		URLs:  urls,
		Count: len(urls),
	})
}

// GetPendingURLs godoc
// @Summary Get pending URLs for crawling
// @Description Get URLs that need to be parsed (status=pending, respects retry logic)
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Param limit query int false "Max URLs to return" default(50)
// @Success 200 {object} PendingURLsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/{id}/pending-urls [get]
func (h *SitemapURLHandler) GetPending(c *fiber.Ctx) error {
	siteID := c.Params("id")
	if siteID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "site_id is required"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	sitemapURLs, err := h.sitemapURLRepo.FindPendingAndLock(c.Context(), siteID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	urls := make([]PendingURLWithDepth, len(sitemapURLs))
	for i, u := range sitemapURLs {
		urls[i] = PendingURLWithDepth{
			URL:   u.URL,
			Depth: u.Depth,
		}
	}

	resp := PendingURLsResponse{URLs: urls}

	if len(urls) == 0 {
		stats, err := h.sitemapURLRepo.GetStats(c.Context(), siteID)
		if err == nil {
			resp.TotalURLs = stats.Total
			resp.IndexedURLs = stats.Indexed
			if stats.Total > 0 && stats.Pending == 0 {
				resp.AllIndexed = true
			} else if stats.Pending > 0 {
				resp.InRetry = true
			}
		}
	}

	return c.JSON(resp)
}
