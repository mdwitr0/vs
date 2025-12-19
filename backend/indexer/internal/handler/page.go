package handler

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/repo"
)

type PageHandler struct {
	pageRepo      *repo.PageRepo
	violationsSvc *violations.Service
}

func NewPageHandler(pageRepo *repo.PageRepo, violationsSvc *violations.Service) *PageHandler {
	return &PageHandler{
		pageRepo:      pageRepo,
		violationsSvc: violationsSvc,
	}
}

type PageExternalIDs struct {
	KinopoiskID   string `json:"kinopoisk_id,omitempty"`
	IMDBID        string `json:"imdb_id,omitempty"`
	TMDBID        string `json:"tmdb_id,omitempty"`
	MALID         string `json:"mal_id,omitempty"`
	ShikimoriID   string `json:"shikimori_id,omitempty"`
	MyDramaListID string `json:"mydramalist_id,omitempty"`
}

type PageResponse struct {
	ID          string          `json:"id"`
	SiteID      string          `json:"site_id"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Year        int             `json:"year,omitempty"`
	ExternalIDs PageExternalIDs `json:"external_ids"`
	PlayerURL   string          `json:"player_url,omitempty"`
	HTTPStatus  int             `json:"http_status"`
	IndexedAt   time.Time       `json:"indexed_at"`
}

type ListPagesResponse struct {
	Items []PageResponse `json:"items"`
	Total int64          `json:"total"`
}

// ListPages godoc
// @Summary List indexed pages
// @Description Get list of indexed pages with filtering
// @Tags pages
// @Produce json
// @Param site_id query string false "Filter by site ID"
// @Param kpid query string false "Filter by Kinopoisk ID"
// @Param imdb_id query string false "Filter by IMDb ID"
// @Param title query string false "Search by title"
// @Param year query int false "Filter by year"
// @Param has_player query bool false "Filter by player presence"
// @Param has_violations query bool false "Filter by violations presence (requires site_id)"
// @Param sort_by query string false "Sort by field" Enums(indexed_at, year) default(indexed_at)
// @Param sort_order query string false "Sort order" Enums(asc, desc) default(desc)
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListPagesResponse
// @Router /api/pages [get]
func (h *PageHandler) List(c *fiber.Ctx) error {
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	siteID := c.Query("site_id")
	year, _ := strconv.Atoi(c.Query("year"))

	query := repo.PageQuery{
		SiteID:      siteID,
		KinopoiskID: c.Query("kinopoisk_id"),
		IMDBID:      c.Query("imdb_id"),
		Title:       c.Query("title"),
		Year:        year,
		SortBy:      c.Query("sort_by", "indexed_at"),
		SortOrder:   c.Query("sort_order", "desc"),
		Limit:       limit,
		Offset:      offset,
	}

	if hp := c.Query("has_player"); hp == "true" || hp == "false" {
		hasPlayer := hp == "true"
		query.HasPlayer = &hasPlayer
	}

	if hv := c.Query("has_violations"); (hv == "true" || hv == "false") && siteID != "" && h.violationsSvc != nil {
		pageIDs, err := h.violationsSvc.GetPageIDsBySiteID(c.Context(), siteID)
		if err != nil {
			pageIDs = []string{}
		}
		if hv == "true" {
			if len(pageIDs) > 0 {
				query.PageIDs = pageIDs
			} else {
				return c.JSON(ListPagesResponse{Items: []PageResponse{}, Total: 0})
			}
		} else {
			query.ExcludePageIDs = pageIDs
		}
	}

	pages, total, err := h.pageRepo.Search(c.Context(), query)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch pages"})
	}

	items := make([]PageResponse, len(pages))
	for i, p := range pages {
		items[i] = PageResponse{
			ID:     p.ID.Hex(),
			SiteID: p.SiteID,
			URL:    p.URL,
			Title:  p.Title,
			Year:   p.Year,
			ExternalIDs: PageExternalIDs{
				KinopoiskID:   p.ExternalIDs.KinopoiskID,
				IMDBID:        p.ExternalIDs.IMDBID,
				TMDBID:        p.ExternalIDs.TMDBID,
				MALID:         p.ExternalIDs.MALID,
				ShikimoriID:   p.ExternalIDs.ShikimoriID,
				MyDramaListID: p.ExternalIDs.MyDramaListID,
			},
			PlayerURL:  p.PlayerURL,
			HTTPStatus: p.HTTPStatus,
			IndexedAt:  p.IndexedAt,
		}
	}

	return c.JSON(ListPagesResponse{
		Items: items,
		Total: total,
	})
}

// GetStats godoc
// @Summary Get page statistics
// @Description Get statistics about indexed pages
// @Tags pages
// @Produce json
// @Param site_id query string false "Filter by site ID"
// @Success 200 {object} repo.PageStats
// @Router /api/pages/stats [get]
func (h *PageHandler) Stats(c *fiber.Ctx) error {
	siteID := c.Query("site_id")

	stats, err := h.pageRepo.GetStats(c.Context(), siteID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to get stats"})
	}

	return c.JSON(stats)
}
