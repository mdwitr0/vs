package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/middleware"
	"github.com/video-analitics/indexer/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ContentHandler struct {
	contentRepo   *repo.ContentRepo
	siteRepo      *repo.SiteRepo
	violationsSvc *violations.Service
}

func NewContentHandler(contentRepo *repo.ContentRepo, siteRepo *repo.SiteRepo, violationsSvc *violations.Service) *ContentHandler {
	return &ContentHandler{
		contentRepo:   contentRepo,
		siteRepo:      siteRepo,
		violationsSvc: violationsSvc,
	}
}

type CreateContentRequest struct {
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	KinopoiskID   string `json:"kinopoisk_id,omitempty"`
	IMDBID        string `json:"imdb_id,omitempty"`
	MALID         string `json:"mal_id,omitempty"`
	ShikimoriID   string `json:"shikimori_id,omitempty"`
	MyDramaListID string `json:"mydramalist_id,omitempty"`
}

// ContentWithStats - контент со статистикой нарушений
type ContentWithStats struct {
	repo.Content
	ViolationsCount int64 `json:"violations_count"`
	SitesCount      int64 `json:"sites_count"`
}

// Create godoc
// @Summary Create content
// @Description Add content to track for violations
// @Tags content
// @Accept json
// @Produce json
// @Param request body CreateContentRequest true "Content data"
// @Success 201 {object} ContentWithStats
// @Failure 400 {object} ErrorResponse
// @Router /api/content [post]
func (h *ContentHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req CreateContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Title == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "title is required"})
	}

	if req.KinopoiskID == "" && req.IMDBID == "" && req.MALID == "" && req.ShikimoriID == "" && req.MyDramaListID == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "at least one ID is required (kinopoisk_id, imdb_id, mal_id, shikimori_id, mydramalist_id)"})
	}

	var ownerOID primitive.ObjectID
	if !isAdmin && userID != "" {
		var err error
		ownerOID, err = primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
		}
	}

	content := &repo.Content{
		OwnerID:       ownerOID,
		Title:         req.Title,
		OriginalTitle: req.OriginalTitle,
		Year:          req.Year,
		KinopoiskID:   req.KinopoiskID,
		IMDBID:        req.IMDBID,
		MALID:         req.MALID,
		ShikimoriID:   req.ShikimoriID,
		MyDramaListID: req.MyDramaListID,
	}

	if err := h.contentRepo.Create(c.Context(), content); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create content"})
	}

	go h.refreshViolationsForContent(content)

	return c.Status(201).JSON(ContentWithStats{
		Content:         *content,
		ViolationsCount: 0,
		SitesCount:      0,
	})
}

func (h *ContentHandler) refreshViolationsForContent(content *repo.Content) {
	if h.violationsSvc == nil {
		return
	}
	h.violationsSvc.RefreshForContent(context.Background(), violations.ContentInfo{
		ID:            content.ID.Hex(),
		Title:         content.Title,
		OriginalTitle: content.OriginalTitle,
		Year:          content.Year,
		KinopoiskID:   content.KinopoiskID,
		IMDBID:        content.IMDBID,
		MALID:         content.MALID,
		ShikimoriID:   content.ShikimoriID,
		MyDramaListID: content.MyDramaListID,
	})
}

type ListContentResponse struct {
	Items []ContentWithStats `json:"items"`
	Total int64              `json:"total"`
}

// List godoc
// @Summary List content
// @Description Get list of tracked content with violation stats
// @Tags content
// @Produce json
// @Param title query string false "Search by title"
// @Param kinopoisk_id query string false "Filter by Kinopoisk ID"
// @Param imdb_id query string false "Filter by IMDB ID"
// @Param mal_id query string false "Filter by MAL ID"
// @Param shikimori_id query string false "Filter by Shikimori ID"
// @Param mydramalist_id query string false "Filter by MyDramaList ID"
// @Param has_violations query string false "Filter by violations presence (true/false)"
// @Param sort_by query string false "Sort by field" Enums(violations_count, created_at) default(violations_count)
// @Param sort_order query string false "Sort order" Enums(asc, desc) default(desc)
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListContentResponse
// @Router /api/content [get]
func (h *ContentHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	title := c.Query("title")
	kinopoiskID := c.Query("kinopoisk_id")
	imdbID := c.Query("imdb_id")
	malID := c.Query("mal_id")
	shikimoriID := c.Query("shikimori_id")
	mydramalistID := c.Query("mydramalist_id")
	hasViolationsStr := c.Query("has_violations")
	sortBy := c.Query("sort_by", "violations_count")
	sortOrder := c.Query("sort_order", "desc")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	var hasViolations *bool
	if hasViolationsStr == "true" {
		v := true
		hasViolations = &v
	} else if hasViolationsStr == "false" {
		v := false
		hasViolations = &v
	}

	filter := repo.ContentFilter{
		Title:         title,
		KinopoiskID:   kinopoiskID,
		IMDBID:        imdbID,
		MALID:         malID,
		ShikimoriID:   shikimoriID,
		MyDramaListID: mydramalistID,
		HasViolations: hasViolations,
		SortBy:        sortBy,
		SortOrder:     sortOrder,
		Limit:         limit,
		Offset:        offset,
	}

	contents, total, err := h.contentRepo.FindByUserAccess(c.Context(), userID, isAdmin, filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch content"})
	}

	items := make([]ContentWithStats, len(contents))
	for i, content := range contents {
		items[i] = ContentWithStats{
			Content:         content,
			ViolationsCount: content.ViolationsCount,
			SitesCount:      content.SitesCount,
		}
	}

	return c.JSON(ListContentResponse{
		Items: items,
		Total: total,
	})
}

func (h *ContentHandler) checkContentAccess(c *fiber.Ctx, contentID string) (*repo.Content, error) {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	content, err := h.contentRepo.FindByID(c.Context(), contentID)
	if err != nil {
		return nil, c.Status(500).JSON(ErrorResponse{Error: "failed to fetch content"})
	}
	if content == nil {
		return nil, c.Status(404).JSON(ErrorResponse{Error: "content not found"})
	}

	hasAccess, err := h.contentRepo.HasUserAccess(c.Context(), contentID, userID, isAdmin)
	if err != nil {
		return nil, c.Status(500).JSON(ErrorResponse{Error: "failed to check access"})
	}
	if !hasAccess {
		return nil, c.Status(403).JSON(ErrorResponse{Error: "access denied"})
	}

	return content, nil
}

// Get godoc
// @Summary Get content by ID
// @Description Get content details with violation stats
// @Tags content
// @Produce json
// @Param id path string true "Content ID"
// @Success 200 {object} ContentWithStats
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/content/{id} [get]
func (h *ContentHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	content, err := h.checkContentAccess(c, id)
	if err != nil {
		return err
	}

	return c.JSON(ContentWithStats{
		Content:         *content,
		ViolationsCount: content.ViolationsCount,
		SitesCount:      content.SitesCount,
	})
}

// ViolationResponse - нарушение для API
type ViolationResponse struct {
	PageID    string `json:"page_id"`
	SiteID    string `json:"site_id"`
	Domain    string `json:"domain"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	MatchType string `json:"match_type"`
	FoundAt   string `json:"found_at"`
}

type ListViolationsResponse struct {
	Items []ViolationResponse `json:"items"`
	Total int64               `json:"total"`
}

// GetViolations godoc
// @Summary Get violations for content
// @Description Get list of pages where content was found
// @Tags content
// @Produce json
// @Param id path string true "Content ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListViolationsResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/content/{id}/violations [get]
func (h *ContentHandler) GetViolations(c *fiber.Ctx) error {
	id := c.Params("id")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	_, err := h.checkContentAccess(c, id)
	if err != nil {
		return err
	}

	vList, total, err := h.violationsSvc.GetByContentID(c.Context(), id, limit, offset)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch violations"})
	}

	domainMap := h.getSiteDomainsMap(c.Context(), vList)

	items := make([]ViolationResponse, len(vList))
	for i, v := range vList {
		items[i] = ViolationResponse{
			PageID:    v.PageID,
			SiteID:    v.SiteID,
			Domain:    domainMap[v.SiteID],
			URL:       v.PageURL,
			Title:     v.PageTitle,
			MatchType: string(v.MatchType),
			FoundAt:   v.FoundAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(ListViolationsResponse{
		Items: items,
		Total: total,
	})
}

func (h *ContentHandler) getSiteDomainsMap(ctx context.Context, vList []violations.Violation) map[string]string {
	siteIDs := make(map[string]bool)
	for _, v := range vList {
		siteIDs[v.SiteID] = true
	}

	ids := make([]string, 0, len(siteIDs))
	for id := range siteIDs {
		ids = append(ids, id)
	}

	sites, _ := h.siteRepo.FindByIDs(ctx, ids)

	domainMap := make(map[string]string)
	for _, site := range sites {
		domainMap[site.ID.Hex()] = site.Domain
	}
	return domainMap
}

// ExportViolationsCSV godoc
// @Summary Export violations to CSV
// @Description Export all violations for content to CSV file
// @Tags content
// @Produce text/csv
// @Param id path string true "Content ID"
// @Success 200 {file} file
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/content/{id}/violations/export [get]
func (h *ContentHandler) ExportViolationsCSV(c *fiber.Ctx) error {
	id := c.Params("id")

	content, err := h.checkContentAccess(c, id)
	if err != nil {
		return err
	}

	vList, err := h.violationsSvc.GetAllByContentID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch violations"})
	}

	domainMap := h.getSiteDomainsMap(c.Context(), vList)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer.Write([]string{"Домен", "URL", "Название страницы", "Тип совпадения", "Дата обнаружения"})

	for _, v := range vList {
		writer.Write([]string{
			domainMap[v.SiteID],
			v.PageURL,
			v.PageTitle,
			string(v.MatchType),
			v.FoundAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()

	filename := fmt.Sprintf("violations_%s.csv", content.Title)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return c.Send(buf.Bytes())
}

// Delete godoc
// @Summary Delete content
// @Description Remove content from tracking
// @Tags content
// @Param id path string true "Content ID"
// @Success 204
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/content/{id} [delete]
func (h *ContentHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	_, err := h.checkContentAccess(c, id)
	if err != nil {
		return err
	}

	h.violationsSvc.DeleteByContentID(c.Context(), id)

	if err := h.contentRepo.Delete(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete content"})
	}

	return c.SendStatus(204)
}

type CheckViolationsRequest struct {
	ContentIDs []string `json:"content_ids"`
}

type CheckViolationsResponse struct {
	CheckedCount int64 `json:"checked_count"`
}

type CreateContentBatchRequest struct {
	Items []CreateContentRequest `json:"items"`
}

type CreateContentBatchResponse struct {
	Created    int      `json:"created"`
	Failed     int      `json:"failed"`
	ContentIDs []string `json:"content_ids"`
}

// CreateBatch godoc
// @Summary Create multiple content items
// @Description Add multiple content items to track for violations in a single batch request
// @Tags content
// @Accept json
// @Produce json
// @Param request body CreateContentBatchRequest true "Content items data"
// @Success 201 {object} CreateContentBatchResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/content/batch [post]
func (h *ContentHandler) CreateBatch(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req CreateContentBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "items array is required"})
	}

	var ownerOID primitive.ObjectID
	if !isAdmin && userID != "" {
		var err error
		ownerOID, err = primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
		}
	}

	var created, failed int
	var contentIDs []string

	for _, item := range req.Items {
		if item.Title == "" {
			failed++
			continue
		}
		if item.KinopoiskID == "" && item.IMDBID == "" && item.MALID == "" && item.ShikimoriID == "" && item.MyDramaListID == "" {
			failed++
			continue
		}

		content := &repo.Content{
			OwnerID:       ownerOID,
			Title:         item.Title,
			OriginalTitle: item.OriginalTitle,
			Year:          item.Year,
			KinopoiskID:   item.KinopoiskID,
			IMDBID:        item.IMDBID,
			MALID:         item.MALID,
			ShikimoriID:   item.ShikimoriID,
			MyDramaListID: item.MyDramaListID,
		}

		if err := h.contentRepo.Create(c.Context(), content); err != nil {
			failed++
			continue
		}

		go h.refreshViolationsForContent(content)

		created++
		contentIDs = append(contentIDs, content.ID.Hex())
	}

	return c.Status(201).JSON(CreateContentBatchResponse{
		Created:    created,
		Failed:     failed,
		ContentIDs: contentIDs,
	})
}

type DeleteContentRequest struct {
	ContentIDs []string `json:"content_ids"`
}

type DeleteContentResponse struct {
	DeletedCount int64 `json:"deleted_count"`
}

// CheckViolations godoc
// @Summary Check violations for content
// @Description Refresh violation stats for selected content items
// @Tags content
// @Accept json
// @Produce json
// @Param request body CheckViolationsRequest true "Content IDs to check"
// @Success 200 {object} CheckViolationsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/content/check-violations [post]
func (h *ContentHandler) CheckViolations(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req CheckViolationsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.ContentIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "content_ids is required"})
	}

	var checked int64
	for _, id := range req.ContentIDs {
		hasAccess, _ := h.contentRepo.HasUserAccess(c.Context(), id, userID, isAdmin)
		if !hasAccess {
			continue
		}

		content, err := h.contentRepo.FindByID(c.Context(), id)
		if err != nil || content == nil {
			continue
		}

		_, err = h.violationsSvc.RefreshForContent(c.Context(), violations.ContentInfo{
			ID:            id,
			Title:         content.Title,
			OriginalTitle: content.OriginalTitle,
			Year:          content.Year,
			KinopoiskID:   content.KinopoiskID,
			IMDBID:        content.IMDBID,
			MALID:         content.MALID,
			ShikimoriID:   content.ShikimoriID,
			MyDramaListID: content.MyDramaListID,
		})
		if err == nil {
			checked++
		}
	}

	return c.JSON(CheckViolationsResponse{CheckedCount: checked})
}

// DeleteBulk godoc
// @Summary Delete multiple content items
// @Description Remove multiple content items from tracking
// @Tags content
// @Accept json
// @Produce json
// @Param request body DeleteContentRequest true "Content IDs to delete"
// @Success 200 {object} DeleteContentResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/content/delete [post]
func (h *ContentHandler) DeleteBulk(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req DeleteContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.ContentIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "content_ids is required"})
	}

	var deleted int64
	for _, id := range req.ContentIDs {
		hasAccess, _ := h.contentRepo.HasUserAccess(c.Context(), id, userID, isAdmin)
		if !hasAccess {
			continue
		}

		h.violationsSvc.DeleteByContentID(c.Context(), id)
		if err := h.contentRepo.Delete(c.Context(), id); err == nil {
			deleted++
		}
	}

	return c.JSON(DeleteContentResponse{DeletedCount: deleted})
}
