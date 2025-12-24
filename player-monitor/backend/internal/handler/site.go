package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/player-monitor/backend/internal/crawler"
	"github.com/player-monitor/backend/internal/middleware"
	"github.com/player-monitor/backend/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SiteHandler struct {
	siteRepo *repo.SiteRepo
	pageRepo *repo.PageRepo
	crawler  *crawler.Crawler
}

func NewSiteHandler(
	siteRepo *repo.SiteRepo,
	pageRepo *repo.PageRepo,
	crawler *crawler.Crawler,
) *SiteHandler {
	return &SiteHandler{
		siteRepo: siteRepo,
		pageRepo: pageRepo,
		crawler:  crawler,
	}
}

type CreateSiteRequest struct {
	Domain string `json:"domain"`
}

type ImportSitesRequest struct {
	Domains []string `json:"domains"`
}

type SitesListResponse struct {
	Items []repo.Site `json:"items"`
	Total int64       `json:"total"`
}

type PagesListResponse struct {
	Items []repo.Page `json:"items"`
	Total int64       `json:"total"`
}

func (h *SiteHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	domain := c.Query("domain")
	status := c.Query("status")
	sortBy := c.Query("sort_by", "created_at")
	sortOrder := c.Query("sort_order", "desc")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	filter := repo.SiteFilter{
		Domain:    domain,
		Status:    status,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	if !isAdmin {
		filter.UserID = userID
	}

	sites, total, err := h.siteRepo.FindAll(c.Context(), filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch sites"})
	}

	for i := range sites {
		totalPages, withPlayer, withoutPlayer, err := h.pageRepo.CountBySiteID(c.Context(), sites[i].ID)
		if err == nil {
			sites[i].TotalPages = totalPages
			sites[i].PagesWithPlayer = withPlayer
			sites[i].PagesWithoutPlayer = withoutPlayer
		}
	}

	return c.JSON(SitesListResponse{
		Items: sites,
		Total: total,
	})
}

func (h *SiteHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	site, err := h.siteRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	total, withPlayer, withoutPlayer, err := h.pageRepo.CountBySiteID(c.Context(), site.ID)
	if err == nil {
		site.TotalPages = total
		site.PagesWithPlayer = withPlayer
		site.PagesWithoutPlayer = withoutPlayer
	}

	return c.JSON(site)
}

func (h *SiteHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
	}

	var req CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Domain == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "domain is required"})
	}

	req.Domain = strings.TrimSpace(req.Domain)
	req.Domain = strings.TrimPrefix(req.Domain, "http://")
	req.Domain = strings.TrimPrefix(req.Domain, "https://")
	req.Domain = strings.TrimSuffix(req.Domain, "/")

	existing, _ := h.siteRepo.FindByDomain(c.Context(), userOID, req.Domain)
	if existing != nil {
		return c.Status(409).JSON(ErrorResponse{Error: "site with this domain already exists"})
	}

	site := &repo.Site{
		UserID: userOID,
		Domain: req.Domain,
		Status: "active",
	}

	if err := h.siteRepo.Create(c.Context(), site); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create site"})
	}

	return c.Status(201).JSON(site)
}

func (h *SiteHandler) Import(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "file is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to open file"})
	}
	defer f.Close()

	reader := csv.NewReader(f)
	var created, skipped int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(record) == 0 || record[0] == "" {
			continue
		}

		domain := strings.TrimSpace(record[0])
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimSuffix(domain, "/")

		if domain == "" {
			continue
		}

		existing, _ := h.siteRepo.FindByDomain(c.Context(), userOID, domain)
		if existing != nil {
			skipped++
			continue
		}

		site := &repo.Site{
			UserID: userOID,
			Domain: domain,
			Status: "active",
		}

		if err := h.siteRepo.Create(c.Context(), site); err != nil {
			skipped++
			continue
		}

		created++
	}

	return c.JSON(fiber.Map{
		"created": created,
		"skipped": skipped,
	})
}

func (h *SiteHandler) Scan(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	site, err := h.siteRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	if site.Status == "scanning" {
		return c.Status(400).JSON(ErrorResponse{Error: "site is already being scanned"})
	}

	go h.crawler.ScanSite(context.Background(), site)

	return c.JSON(SuccessResponse{Message: "scan started"})
}

func (h *SiteHandler) GetPages(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	site, err := h.siteRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	hasPlayerStr := c.Query("has_player")
	pageType := c.Query("page_type")
	excludeFromReportStr := c.Query("exclude_from_report")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	filter := repo.PageFilter{
		SiteID:   id,
		PageType: pageType,
		Limit:    limit,
		Offset:   offset,
	}

	if !isAdmin {
		filter.UserID = userID
	}

	if hasPlayerStr != "" {
		hasPlayer := hasPlayerStr == "true"
		filter.HasPlayer = &hasPlayer
	}

	if excludeFromReportStr != "" {
		excludeFromReport := excludeFromReportStr == "true"
		filter.ExcludeFromReport = &excludeFromReport
	}

	pages, total, err := h.pageRepo.FindBySiteID(c.Context(), filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch pages"})
	}

	return c.JSON(PagesListResponse{
		Items: pages,
		Total: total,
	})
}

func (h *SiteHandler) ExportPages(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	site, err := h.siteRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	pages, err := h.pageRepo.GetPagesWithoutPlayer(c.Context(), id, false)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch pages"})
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer.Write([]string{"URL", "Page Type", "Last Checked"})

	for _, page := range pages {
		writer.Write([]string{
			page.URL,
			page.PageType,
			page.LastCheckedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()

	filename := fmt.Sprintf("pages_without_player_%s.csv", site.Domain)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return c.Send(buf.Bytes())
}

func (h *SiteHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	filterUserID := userID
	if isAdmin {
		filterUserID = ""
	}

	site, err := h.siteRepo.FindByID(c.Context(), id, filterUserID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	if err := h.pageRepo.DeleteBySiteID(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete pages"})
	}

	if err := h.siteRepo.Delete(c.Context(), id, filterUserID); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete site"})
	}

	return c.JSON(SuccessResponse{Message: "site deleted"})
}
