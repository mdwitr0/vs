package handler

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/status"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/middleware"
	"github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func normalizeDomain(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if !strings.Contains(input, "://") {
		input = "https://" + input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return strings.TrimPrefix(strings.TrimPrefix(input, "https://"), "http://")
	}

	host := parsed.Hostname()
	if host == "" {
		return input
	}

	return strings.ToLower(host)
}

type SiteHandler struct {
	siteRepo       *repo.SiteRepo
	pageRepo       *repo.PageRepo
	taskRepo       *repo.ScanTaskRepo
	sitemapURLRepo *repo.SitemapURLRepo
	userSiteRepo   *repo.UserSiteRepo
	publisher      *queue.Publisher
	violationsSvc  *violations.Service
	meili          *meili.Client
}

func NewSiteHandler(siteRepo *repo.SiteRepo, pageRepo *repo.PageRepo, taskRepo *repo.ScanTaskRepo, sitemapURLRepo *repo.SitemapURLRepo, userSiteRepo *repo.UserSiteRepo, publisher *queue.Publisher, violationsSvc *violations.Service, meiliClient *meili.Client) *SiteHandler {
	return &SiteHandler{
		siteRepo:       siteRepo,
		pageRepo:       pageRepo,
		taskRepo:       taskRepo,
		sitemapURLRepo: sitemapURLRepo,
		userSiteRepo:   userSiteRepo,
		publisher:      publisher,
		meili:          meiliClient,
		violationsSvc:  violationsSvc,
	}
}

type CreateSiteRequest struct {
	Domain        string   `json:"domain"`
	CMS           string   `json:"cms,omitempty"`
	HasSitemap    bool     `json:"has_sitemap"`
	SitemapURLs   []string `json:"sitemap_urls,omitempty"`
	ScanIntervalH int      `json:"scan_interval_h,omitempty"`
}

// ActiveTaskProgress - прогресс активной задачи
type ActiveTaskProgress struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// LastScanResult - результат последнего завершённого сканирования
type LastScanResult struct {
	Success int    `json:"success"`
	Total   int    `json:"total"`
	Status  string `json:"status"`
}

// SiteWithStats - сайт со статистикой нарушений
type SiteWithStats struct {
	repo.Site
	ViolationsCount    int64               `json:"violations_count"`
	ActiveStage        string              `json:"active_stage,omitempty"`
	ActiveTaskProgress *ActiveTaskProgress `json:"active_task_progress,omitempty"`
	PendingURLsCount   int64               `json:"pending_urls_count,omitempty"`
	LastScan           *LastScanResult     `json:"last_scan,omitempty"`
}

// CreateSite godoc
// @Summary Create a new site
// @Description Add a new site to monitor
// @Tags sites
// @Accept json
// @Produce json
// @Param request body CreateSiteRequest true "Site data"
// @Success 201 {object} repo.Site
// @Success 200 {object} repo.Site "Site already exists, user linked"
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Site already in user's list"
// @Router /api/sites [post]
func (h *SiteHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.Domain == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "domain is required"})
	}

	domain := normalizeDomain(req.Domain)
	if domain == "" {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid domain"})
	}

	existing, _ := h.siteRepo.FindByDomain(c.Context(), domain)
	if existing != nil {
		if !isAdmin {
			link, _ := h.userSiteRepo.FindByUserAndSite(c.Context(), userID, existing.ID.Hex())
			if link != nil {
				return c.Status(409).JSON(ErrorResponse{Error: "site already in your list"})
			}

			userOID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
			}

			if existing.OwnerID == userOID {
				return c.Status(409).JSON(ErrorResponse{Error: "site already in your list"})
			}

			err = h.userSiteRepo.Create(c.Context(), &repo.UserSite{
				UserID: userOID,
				SiteID: existing.ID,
			})
			if err != nil {
				return c.Status(500).JSON(ErrorResponse{Error: "failed to link site"})
			}
		}
		return c.Status(200).JSON(existing)
	}

	var ownerOID primitive.ObjectID
	if !isAdmin && userID != "" {
		var err error
		ownerOID, err = primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
		}
	}

	site := &repo.Site{
		OwnerID:       ownerOID,
		Domain:        domain,
		CMS:           req.CMS,
		HasSitemap:    req.HasSitemap,
		SitemapURLs:   req.SitemapURLs,
		ScanIntervalH: req.ScanIntervalH,
	}

	if err := h.siteRepo.Create(c.Context(), site); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create site"})
	}

	taskID := uuid.New().String()
	if err := h.publisher.PublishDetectTask(c.Context(), taskID, site.ID.Hex(), site.Domain); err != nil {
		// Log but don't fail - site was created successfully
	}

	return c.Status(201).JSON(site)
}

type ListSitesResponse struct {
	Items []SiteWithStats `json:"items"`
	Total int64           `json:"total"`
}

// ListSites godoc
// @Summary List all sites
// @Description Get list of monitored sites with pagination
// @Tags sites
// @Produce json
// @Param status query string false "Filter by status (active, down, dead)"
// @Param scanned_since query string false "Filter by last scan date (today, week, month)"
// @Param has_violations query string false "Filter by violations (true, false)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListSitesResponse
// @Router /api/sites [get]
func (h *SiteHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	statusFilter := c.Query("status")
	scannedSince := c.Query("scanned_since")
	hasViolations := c.Query("has_violations")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	filter := repo.SiteFilter{
		Status: statusFilter,
		Limit:  limit,
		Offset: offset,
	}

	now := time.Now()
	switch scannedSince {
	case "today":
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		filter.ScannedSince = &t
	case "week":
		t := now.AddDate(0, 0, -7)
		filter.ScannedSince = &t
	case "month":
		t := now.AddDate(0, -1, 0)
		filter.ScannedSince = &t
	}

	allStats, err := h.violationsSvc.GetAllSiteStats(c.Context())
	if err != nil {
		logger.Log.Error().Err(err).Msg("failed to get all site stats")
	}

	if hasViolations == "true" {
		var siteIDsWithViolations []string
		for siteID, stats := range allStats {
			if stats.ViolationsCount > 0 {
				siteIDsWithViolations = append(siteIDsWithViolations, siteID)
			}
		}
		logger.Log.Debug().Int("sites_with_violations", len(siteIDsWithViolations)).Msg("filtering sites with violations")
		if len(siteIDsWithViolations) == 0 {
			return c.JSON(ListSitesResponse{Items: []SiteWithStats{}, Total: 0})
		}
		filter.SiteIDs = siteIDsWithViolations
	} else if hasViolations == "false" {
		var siteIDsWithViolations []string
		for siteID, stats := range allStats {
			if stats.ViolationsCount > 0 {
				siteIDsWithViolations = append(siteIDsWithViolations, siteID)
			}
		}
		filter.ExcludeIDs = siteIDsWithViolations
	}

	sites, total, err := h.siteRepo.FindByUserAccess(c.Context(), userID, isAdmin, filter)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch sites"})
	}

	// Собираем ID сайтов для batch запросов
	siteIDs := make([]string, len(sites))
	for i, site := range sites {
		siteIDs[i] = site.ID.Hex()
	}

	// Получаем информацию об активных задачах (этап + прогресс)
	activeTasksInfo, _ := h.taskRepo.GetActiveTasksInfo(c.Context(), siteIDs)

	// Получаем количество pending URL для всех сайтов
	pendingCounts, _ := h.sitemapURLRepo.GetPendingCounts(c.Context(), siteIDs)

	// Получаем информацию о последних завершённых задачах
	lastScanInfo, _ := h.taskRepo.GetLastCompletedTasksInfo(c.Context(), siteIDs)

	// Add violations count, active stage, progress, and pending URLs for each site
	result := make([]SiteWithStats, 0, len(sites))
	for _, site := range sites {
		siteID := site.ID.Hex()
		var violationsCount int64
		if stats, ok := allStats[siteID]; ok {
			violationsCount = stats.ViolationsCount
		}

		siteWithStats := SiteWithStats{
			Site:            site,
			ViolationsCount: violationsCount,
		}

		if taskInfo, ok := activeTasksInfo[siteID]; ok {
			siteWithStats.ActiveStage = string(taskInfo.Stage)
			siteWithStats.ActiveTaskProgress = &ActiveTaskProgress{
				Total:   taskInfo.Total,
				Success: taskInfo.Success,
				Failed:  taskInfo.Failed,
			}
		}

		if count, ok := pendingCounts[siteID]; ok {
			siteWithStats.PendingURLsCount = count
		}

		if scanInfo, ok := lastScanInfo[siteID]; ok {
			siteWithStats.LastScan = &LastScanResult{
				Success: scanInfo.Success,
				Total:   scanInfo.Total,
				Status:  string(scanInfo.Status),
			}
		}

		result = append(result, siteWithStats)
	}

	return c.JSON(ListSitesResponse{
		Items: result,
		Total: total,
	})
}

func (h *SiteHandler) checkSiteAccess(c *fiber.Ctx, siteID string) (*repo.Site, error) {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	site, err := h.siteRepo.FindByID(c.Context(), siteID)
	if err != nil {
		return nil, c.Status(500).JSON(ErrorResponse{Error: "failed to fetch site"})
	}
	if site == nil {
		return nil, c.Status(404).JSON(ErrorResponse{Error: "site not found"})
	}

	hasAccess, err := h.siteRepo.HasUserAccess(c.Context(), siteID, userID, isAdmin, h.userSiteRepo)
	if err != nil {
		return nil, c.Status(500).JSON(ErrorResponse{Error: "failed to check access"})
	}
	if !hasAccess {
		return nil, c.Status(403).JSON(ErrorResponse{Error: "access denied"})
	}

	return site, nil
}

// GetSite godoc
// @Summary Get site by ID
// @Description Get site details
// @Tags sites
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} SiteWithStats
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id} [get]
func (h *SiteHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	site, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	var violationsCount int64
	if stats, err := h.violationsSvc.GetSiteStats(c.Context(), id); err == nil && stats != nil {
		violationsCount = stats.ViolationsCount
	}

	result := SiteWithStats{
		Site:            *site,
		ViolationsCount: violationsCount,
	}

	if activeStages, err := h.taskRepo.GetActiveStages(c.Context(), []string{id}); err == nil {
		if stage, ok := activeStages[id]; ok {
			result.ActiveStage = string(stage)
		}
	}

	if pendingCounts, err := h.sitemapURLRepo.GetPendingCounts(c.Context(), []string{id}); err == nil {
		if count, ok := pendingCounts[id]; ok {
			result.PendingURLsCount = count
		}
	}

	return c.JSON(result)
}

// SiteViolationResponse - нарушение для API сайта
type SiteViolationResponse struct {
	PageID    string `json:"page_id"`
	ContentID string `json:"content_id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	MatchType string `json:"match_type"`
	FoundAt   string `json:"found_at"`
}

type ListSiteViolationsResponse struct {
	Items []SiteViolationResponse `json:"items"`
	Total int64                   `json:"total"`
}

// GetViolations godoc
// @Summary Get violations for site
// @Description Get list of content violations found on this site
// @Tags sites
// @Produce json
// @Param id path string true "Site ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ListSiteViolationsResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id}/violations [get]
func (h *SiteHandler) GetViolations(c *fiber.Ctx) error {
	id := c.Params("id")
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)

	if limit > 100 {
		limit = 100
	}

	_, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	vList, total, err := h.violationsSvc.GetBySiteID(c.Context(), id, limit, offset)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch violations"})
	}

	items := make([]SiteViolationResponse, len(vList))
	for i, v := range vList {
		items[i] = SiteViolationResponse{
			PageID:    v.PageID,
			ContentID: v.ContentID,
			URL:       v.PageURL,
			Title:     v.PageTitle,
			MatchType: string(v.MatchType),
			FoundAt:   v.FoundAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(ListSiteViolationsResponse{
		Items: items,
		Total: total,
	})
}

// AnalyzeSite godoc
// @Summary Re-analyze a site
// @Description Re-run detection for a frozen or pending site
// @Tags sites
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id}/analyze [post]
func (h *SiteHandler) Analyze(c *fiber.Ctx) error {
	id := c.Params("id")

	site, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	if err := h.siteRepo.ResetToPending(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to reset site"})
	}

	taskID := uuid.New().String()
	if err := h.publisher.PublishDetectTask(c.Context(), taskID, id, site.Domain); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to queue detection"})
	}

	return c.JSON(fiber.Map{"status": "analyzing", "task_id": taskID})
}

type UnfreezeRequest struct {
	ScannerType string `json:"scanner_type"` // "http" или "spa"
}

// UnfreezeSite godoc
// @Summary Unfreeze a frozen site
// @Description Unfreeze a site and optionally change scanner type
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Param request body UnfreezeRequest false "Scanner type"
// @Success 200 {object} repo.Site
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id}/unfreeze [post]
func (h *SiteHandler) Unfreeze(c *fiber.Ctx) error {
	id := c.Params("id")

	site, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	if site.Status != status.SiteFrozen {
		return c.Status(400).JSON(ErrorResponse{Error: "site is not frozen"})
	}

	var req UnfreezeRequest
	c.BodyParser(&req)

	scannerType := status.ScannerType(req.ScannerType)
	if scannerType != "" && scannerType != status.ScannerHTTP && scannerType != status.ScannerSPA {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid scanner_type, must be 'http' or 'spa'"})
	}

	if err := h.siteRepo.Unfreeze(c.Context(), id, scannerType); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to unfreeze site"})
	}

	site, _ = h.siteRepo.FindByID(c.Context(), id)
	return c.JSON(site)
}

type ScanStageResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// ScanSitemap godoc
// @Summary Scan sitemap only
// @Description Start sitemap crawl to collect URLs without parsing pages
// @Tags sites
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} ScanStageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id}/scan-sitemap [post]
func (h *SiteHandler) ScanSitemap(c *fiber.Ctx) error {
	log := logger.Log
	id := c.Params("id")

	site, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	if site.Status == status.SitePending {
		return c.Status(400).JSON(ErrorResponse{Error: "site is pending detection"})
	}

	// Check for active task
	hasActive, _ := h.taskRepo.HasActiveTask(c.Context(), id)
	if hasActive {
		return c.Status(400).JSON(ErrorResponse{Error: "site already has active task"})
	}

	// Create task for sitemap stage only
	task := &repo.ScanTask{
		SiteID: id,
		Domain: site.Domain,
	}
	if err := h.taskRepo.Create(c.Context(), task); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create task"})
	}

	// Publish sitemap crawl task (auto_continue=false - manual page crawl start)
	taskInfo := queue.TaskInfo{
		TaskID:       task.ID.Hex(),
		Site:         site,
		AutoContinue: false,
	}
	if err := h.publisher.PublishSitemapCrawlTask(c.Context(), taskInfo); err != nil {
		log.Error().Err(err).Str("site", site.Domain).Msg("failed to publish sitemap crawl task")
		return c.Status(500).JSON(ErrorResponse{Error: "failed to queue sitemap crawl"})
	}

	log.Info().Str("site", site.Domain).Str("task", task.ID.Hex()).Msg("sitemap crawl queued")

	return c.JSON(ScanStageResponse{
		TaskID:  task.ID.Hex(),
		Message: "sitemap crawl queued",
	})
}

// ScanPages godoc
// @Summary Scan pages only
// @Description Start page crawl for pending URLs without re-scanning sitemap
// @Tags sites
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} ScanStageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id}/scan-pages [post]
func (h *SiteHandler) ScanPages(c *fiber.Ctx) error {
	log := logger.Log
	id := c.Params("id")

	site, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	if site.Status == status.SitePending {
		return c.Status(400).JSON(ErrorResponse{Error: "site is pending detection"})
	}

	// Check for active task
	hasActive, _ := h.taskRepo.HasActiveTask(c.Context(), id)
	if hasActive {
		return c.Status(400).JSON(ErrorResponse{Error: "site already has active task"})
	}

	// Check if there are pending URLs
	pendingCounts, _ := h.sitemapURLRepo.GetPendingCounts(c.Context(), []string{id})
	pendingCount := pendingCounts[id]
	if pendingCount == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "no pending URLs to parse"})
	}

	// Reset retry delays for pending URLs
	if _, err := h.sitemapURLRepo.ResetPendingRetryDelay(c.Context(), id); err != nil {
		log.Warn().Err(err).Str("site", site.Domain).Msg("failed to reset retry delays")
	}

	// Create task starting at page stage
	task := &repo.ScanTask{
		SiteID: id,
		Domain: site.Domain,
	}
	if err := h.taskRepo.CreateForPageStage(c.Context(), task, int(pendingCount)); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create task"})
	}

	// Publish page crawl task
	taskInfo := queue.TaskInfo{
		TaskID: task.ID.Hex(),
		Site:   site,
	}
	if err := h.publisher.PublishPageCrawlTaskSimple(c.Context(), taskInfo); err != nil {
		log.Error().Err(err).Str("site", site.Domain).Msg("failed to publish page crawl task")
		return c.Status(500).JSON(ErrorResponse{Error: "failed to queue page crawl"})
	}

	log.Info().Str("site", site.Domain).Str("task", task.ID.Hex()).Int64("pending", pendingCount).Msg("page crawl queued")

	return c.JSON(ScanStageResponse{
		TaskID:  task.ID.Hex(),
		Message: "page crawl queued",
	})
}

// Delete godoc
// @Summary Delete site
// @Description Delete a site by ID along with all related pages and tasks
// @Tags sites
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/sites/{id} [delete]
func (h *SiteHandler) Delete(c *fiber.Ctx) error {
	log := logger.Log
	id := c.Params("id")

	_, err := h.checkSiteAccess(c, id)
	if err != nil {
		return err
	}

	pagesDeleted, _ := h.pageRepo.DeleteBySiteID(c.Context(), id)
	tasksDeleted, _ := h.taskRepo.DeleteBySiteID(c.Context(), id)
	h.userSiteRepo.DeleteBySiteID(c.Context(), id)

	// Удаляем страницы из Meilisearch
	if h.meili != nil {
		if err := h.meili.DeleteBySiteID(id); err != nil {
			log.Warn().Err(err).Str("site_id", id).Msg("failed to delete pages from meilisearch")
		}
	}

	// Удаляем violations
	if h.violationsSvc != nil {
		if deleted, err := h.violationsSvc.DeleteBySiteID(c.Context(), id); err != nil {
			log.Warn().Err(err).Str("site_id", id).Msg("failed to delete violations")
		} else if deleted > 0 {
			log.Info().Int64("count", deleted).Str("site_id", id).Msg("violations deleted")
		}
	}

	if err := h.siteRepo.Delete(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to delete site"})
	}

	return c.JSON(fiber.Map{
		"message":       "site deleted",
		"pages_deleted": pagesDeleted,
		"tasks_deleted": tasksDeleted,
	})
}

type CreateSitesBatchRequest struct {
	Sites []CreateSiteRequest `json:"sites"`
}

type CreateSitesBatchResponse struct {
	Created int      `json:"created"`
	Failed  int      `json:"failed"`
	SiteIDs []string `json:"site_ids"`
}

// CreateBatch godoc
// @Summary Create multiple sites
// @Description Add multiple sites to monitor in a single batch request
// @Tags sites
// @Accept json
// @Produce json
// @Param request body CreateSitesBatchRequest true "Sites data"
// @Success 201 {object} CreateSitesBatchResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/batch [post]
func (h *SiteHandler) CreateBatch(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req CreateSitesBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.Sites) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "sites array is required"})
	}

	var ownerOID primitive.ObjectID
	if !isAdmin && userID != "" {
		var err error
		ownerOID, err = primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{Error: "invalid user id"})
		}
	}

	var created, failed, linked int
	var siteIDs []string

	for _, siteReq := range req.Sites {
		if siteReq.Domain == "" {
			failed++
			continue
		}

		domain := normalizeDomain(siteReq.Domain)
		if domain == "" {
			failed++
			continue
		}

		existing, _ := h.siteRepo.FindByDomain(c.Context(), domain)
		if existing != nil {
			if !isAdmin && !ownerOID.IsZero() {
				if existing.OwnerID != ownerOID {
					linkExists, _ := h.userSiteRepo.ExistsByUserAndSite(c.Context(), userID, existing.ID.Hex())
					if !linkExists {
						h.userSiteRepo.Create(c.Context(), &repo.UserSite{
							UserID: ownerOID,
							SiteID: existing.ID,
						})
						linked++
						siteIDs = append(siteIDs, existing.ID.Hex())
						continue
					}
				}
			}
			failed++
			continue
		}

		site := &repo.Site{
			OwnerID:       ownerOID,
			Domain:        domain,
			CMS:           siteReq.CMS,
			HasSitemap:    siteReq.HasSitemap,
			SitemapURLs:   siteReq.SitemapURLs,
			ScanIntervalH: siteReq.ScanIntervalH,
		}

		if err := h.siteRepo.Create(c.Context(), site); err != nil {
			failed++
			continue
		}

		taskID := uuid.New().String()
		h.publisher.PublishDetectTask(c.Context(), taskID, site.ID.Hex(), site.Domain)

		created++
		siteIDs = append(siteIDs, site.ID.Hex())
	}

	return c.Status(201).JSON(CreateSitesBatchResponse{
		Created: created + linked,
		Failed:  failed,
		SiteIDs: siteIDs,
	})
}

type DeleteSitesRequest struct {
	SiteIDs []string `json:"site_ids"`
}

type DeleteSitesResponse struct {
	DeletedCount int64 `json:"deleted_count"`
	PagesDeleted int64 `json:"pages_deleted"`
	TasksDeleted int64 `json:"tasks_deleted"`
}

// DeleteBulk godoc
// @Summary Delete multiple sites
// @Description Delete multiple sites by ID along with all related pages and tasks
// @Tags sites
// @Accept json
// @Produce json
// @Param request body DeleteSitesRequest true "Site IDs to delete"
// @Success 200 {object} DeleteSitesResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/delete [post]
func (h *SiteHandler) DeleteBulk(c *fiber.Ctx) error {
	log := logger.Log
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req DeleteSitesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.SiteIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "site_ids is required"})
	}

	var deleted, pagesDeleted, tasksDeleted int64
	for _, id := range req.SiteIDs {
		hasAccess, _ := h.siteRepo.HasUserAccess(c.Context(), id, userID, isAdmin, h.userSiteRepo)
		if !hasAccess {
			continue
		}

		pages, _ := h.pageRepo.DeleteBySiteID(c.Context(), id)
		tasks, _ := h.taskRepo.DeleteBySiteID(c.Context(), id)
		h.userSiteRepo.DeleteBySiteID(c.Context(), id)

		// Удаляем страницы из Meilisearch
		if h.meili != nil {
			if err := h.meili.DeleteBySiteID(id); err != nil {
				log.Warn().Err(err).Str("site_id", id).Msg("failed to delete pages from meilisearch")
			}
		}

		// Удаляем violations
		if h.violationsSvc != nil {
			h.violationsSvc.DeleteBySiteID(c.Context(), id)
		}

		if err := h.siteRepo.Delete(c.Context(), id); err == nil {
			deleted++
			pagesDeleted += pages
			tasksDeleted += tasks
		}
	}

	return c.JSON(DeleteSitesResponse{
		DeletedCount: deleted,
		PagesDeleted: pagesDeleted,
		TasksDeleted: tasksDeleted,
	})
}
