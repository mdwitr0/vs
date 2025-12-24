package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/status"
	"github.com/video-analitics/indexer/internal/middleware"
	"github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type ScanHandler struct {
	siteRepo       *repo.SiteRepo
	taskRepo       *repo.ScanTaskRepo
	sitemapURLRepo *repo.SitemapURLRepo
	userSiteRepo   *repo.UserSiteRepo
	publisher      *queue.Publisher
}

func NewScanHandler(siteRepo *repo.SiteRepo, taskRepo *repo.ScanTaskRepo, sitemapURLRepo *repo.SitemapURLRepo, userSiteRepo *repo.UserSiteRepo, publisher *queue.Publisher) *ScanHandler {
	return &ScanHandler{
		siteRepo:       siteRepo,
		taskRepo:       taskRepo,
		sitemapURLRepo: sitemapURLRepo,
		userSiteRepo:   userSiteRepo,
		publisher:      publisher,
	}
}

type ScanRequest struct {
	SiteIDs []string `json:"site_ids"`
	Force   bool     `json:"force"`
}

type ScanResponse struct {
	Message   string   `json:"message"`
	SiteCount int      `json:"site_count"`
	TaskIDs   []string `json:"task_ids"`
}

// StartScan godoc
// @Summary Start scanning sites
// @Description Queues sites for crawling and indexing
// @Tags scan
// @Accept json
// @Produce json
// @Param request body ScanRequest true "Site IDs to scan"
// @Success 200 {object} ScanResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/sites/scan [post]
func (h *ScanHandler) StartScan(c *fiber.Ctx) error {
	log := logger.Log
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if len(req.SiteIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{Error: "site_ids is required"})
	}

	sites, err := h.siteRepo.FindByIDs(c.Context(), req.SiteIDs)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to fetch sites"})
	}

	if len(sites) == 0 {
		return c.Status(404).JSON(ErrorResponse{Error: "no sites found"})
	}

	// Filter sites by user access
	if !isAdmin {
		var accessibleSites []repo.Site
		for _, site := range sites {
			hasAccess, _ := h.siteRepo.HasUserAccess(c.Context(), site.ID.Hex(), userID, isAdmin, h.userSiteRepo)
			if hasAccess {
				accessibleSites = append(accessibleSites, site)
			}
		}
		sites = accessibleSites
		if len(sites) == 0 {
			return c.Status(403).JSON(ErrorResponse{Error: "no accessible sites found"})
		}
	}

	// Создаём ScanTask и собираем только успешные для публикации
	var taskIDs []string
	var tasksToPublish []queue.TaskInfo
	var skippedSites []string

	for i := range sites {
		site := &sites[i]

		// Нельзя сканировать сайт в статусе pending - ещё не завершена детекция
		if site.Status == status.SitePending {
			log.Info().Str("site", site.Domain).Msg("site pending detection, skipping scan")
			skippedSites = append(skippedSites, site.Domain)
			continue
		}

		// Проверяем нет ли уже активной задачи для этого сайта
		hasActive, err := h.taskRepo.HasActiveTask(c.Context(), site.ID.Hex())
		if err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to check active task")
		}
		if hasActive {
			log.Info().Str("site", site.Domain).Msg("site already has active task, skipping")
			skippedSites = append(skippedSites, site.Domain)
			continue
		}

		task := &repo.ScanTask{
			SiteID: site.ID.Hex(),
			Domain: site.Domain,
		}
		if err := h.taskRepo.Create(c.Context(), task); err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to create task record")
			continue
		}
		taskIDs = append(taskIDs, task.ID.Hex())
		tasksToPublish = append(tasksToPublish, queue.TaskInfo{
			TaskID:       task.ID.Hex(),
			Site:         site,
			AutoContinue: true, // полный скан через UI запускает оба этапа
		})
	}

	if len(tasksToPublish) == 0 {
		return c.Status(500).JSON(ErrorResponse{Error: "failed to create any tasks"})
	}

	// Обновляем next_scan_at чтобы избежать дублирования scheduler'ом
	var siteIDs []string
	for _, info := range tasksToPublish {
		siteIDs = append(siteIDs, info.Site.ID.Hex())
	}
	if err := h.siteRepo.MarkQueued(c.Context(), siteIDs); err != nil {
		log.Warn().Err(err).Msg("failed to update next_scan_at")
	}

	for _, info := range tasksToPublish {
		siteID := info.Site.ID.Hex()

		if req.Force {
			// Force rescan: reset all indexed/error pages to pending
			forceReset, err := h.sitemapURLRepo.ResetAllToPending(c.Context(), siteID)
			if err != nil {
				log.Warn().Err(err).Str("site", info.Site.Domain).Msg("failed to force reset URLs")
			} else if forceReset > 0 {
				log.Info().
					Str("site", info.Site.Domain).
					Int64("urls_reset", forceReset).
					Msg("force rescan: all URLs reset to pending")
			}
		} else {
			// Normal scan: only reset errors and retry delays
			errorsReset, err := h.sitemapURLRepo.ResetErrorsToPending(c.Context(), siteID)
			if err != nil {
				log.Warn().Err(err).Str("site", info.Site.Domain).Msg("failed to reset error URLs")
			}

			pendingReset, err := h.sitemapURLRepo.ResetPendingRetryDelay(c.Context(), siteID)
			if err != nil {
				log.Warn().Err(err).Str("site", info.Site.Domain).Msg("failed to reset pending retry delay")
			}

			if errorsReset > 0 || pendingReset > 0 {
				log.Info().
					Str("site", info.Site.Domain).
					Int64("errors_reset", errorsReset).
					Int64("pending_reset", pendingReset).
					Msg("URL states reset for manual scan")
			}
		}

		if err := h.publisher.PublishSitemapCrawlTask(c.Context(), info); err != nil {
			log.Error().Err(err).Str("site", info.Site.Domain).Msg("failed to publish sitemap crawl task")
		}
	}

	log.Info().Int("sites", len(tasksToPublish)).Msg("scan tasks queued")

	return c.JSON(ScanResponse{
		Message:   "scan tasks queued",
		SiteCount: len(tasksToPublish),
		TaskIDs:   taskIDs,
	})
}

type ErrorResponse struct {
	Error string `json:"error"`
}
