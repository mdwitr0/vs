package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/status"
	"github.com/video-analitics/backend/pkg/violations"
	indexerQueue "github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type Scheduler struct {
	siteRepo      *repo.SiteRepo
	taskRepo      *repo.ScanTaskRepo
	contentRepo   *repo.ContentRepo
	publisher     *indexerQueue.Publisher
	violationsSvc *violations.Service
	scheduler     gocron.Scheduler
}

func New(siteRepo *repo.SiteRepo, taskRepo *repo.ScanTaskRepo, contentRepo *repo.ContentRepo, publisher *indexerQueue.Publisher, violationsSvc *violations.Service) (*Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		siteRepo:      siteRepo,
		taskRepo:      taskRepo,
		contentRepo:   contentRepo,
		publisher:     publisher,
		violationsSvc: violationsSvc,
		scheduler:     s,
	}, nil
}

const (
	pendingDetectionTimeout    = 5 * time.Minute
	staleTaskPendingTimeout    = 30 * time.Minute
	staleTaskProcessingTimeout = 2 * time.Hour
	maxTaskRetries             = 3
	baseRetryDelay             = 5 * time.Minute
)

func (s *Scheduler) Start(ctx context.Context) error {
	log := logger.Log

	_, err := s.scheduler.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(func() {
			s.queueDueSites(ctx)
		}),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(2*time.Minute),
		gocron.NewTask(func() {
			s.recoverPendingSites(ctx)
		}),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(func() {
			s.recoverStaleTasks(ctx)
		}),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(2*time.Minute),
		gocron.NewTask(func() {
			s.retryFailedTasks(ctx)
		}),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(24*time.Hour),
		gocron.NewTask(func() {
			s.refreshAllViolations(ctx)
		}),
	)
	if err != nil {
		return err
	}

	s.scheduler.Start()
	log.Info().Msg("scheduler started")

	go s.queueDueSites(ctx)
	go s.recoverPendingSites(ctx)
	go s.retryFailedTasks(ctx)

	return nil
}

func (s *Scheduler) Stop() {
	if err := s.scheduler.Shutdown(); err != nil {
		logger.Log.Error().Err(err).Msg("scheduler shutdown error")
	}
}

func (s *Scheduler) queueDueSites(ctx context.Context) {
	log := logger.Log

	sites, err := s.siteRepo.FindDueForScan(ctx, 50)
	if err != nil {
		log.Error().Err(err).Msg("failed to find sites due for scan")
		return
	}

	if len(sites) == 0 {
		return
	}

	queued := 0
	var queuedSiteIDs []string

	for i := range sites {
		site := &sites[i]

		hasActive, err := s.taskRepo.HasActiveTask(ctx, site.ID.Hex())
		if err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to check active task")
			continue
		}
		if hasActive {
			log.Info().Str("site", site.Domain).Msg("site already has active task, skipping")
			continue
		}

		scanTask := &repo.ScanTask{
			SiteID: site.ID.Hex(),
			Domain: site.Domain,
		}
		if err := s.taskRepo.Create(ctx, scanTask); err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to create scan task")
			continue
		}

		taskInfo := indexerQueue.TaskInfo{
			TaskID:       scanTask.ID.Hex(),
			Site:         site,
			AutoContinue: true, // автоматические сканы запускают полный пайплайн
		}
		if err := s.publisher.PublishSitemapCrawlTask(ctx, taskInfo); err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to publish sitemap crawl task")
			continue
		}

		queuedSiteIDs = append(queuedSiteIDs, site.ID.Hex())
		queued++
	}

	if len(queuedSiteIDs) > 0 {
		if err := s.siteRepo.MarkQueued(ctx, queuedSiteIDs); err != nil {
			log.Warn().Err(err).Msg("failed to mark sites as queued")
		}
	}

	if queued > 0 {
		log.Info().Int("sites", queued).Msg("scheduled sitemap crawl tasks queued")
	}
}

func (s *Scheduler) recoverPendingSites(ctx context.Context) {
	log := logger.Log

	pendingSites, err := s.siteRepo.FindPendingSites(ctx, pendingDetectionTimeout, 20)
	if err != nil {
		log.Error().Err(err).Msg("failed to find pending sites")
		return
	}

	if len(pendingSites) == 0 {
		return
	}

	log.Info().Int("count", len(pendingSites)).Msg("found pending sites to recover")

	for _, site := range pendingSites {
		taskID := site.ID.Hex() + "-recovery"
		if err := s.publisher.PublishDetectTask(ctx, taskID, site.ID.Hex(), site.Domain); err != nil {
			log.Warn().Err(err).Str("site", site.Domain).Msg("failed to republish detect task")
			continue
		}
		log.Info().Str("site", site.Domain).Msg("pending site detection re-queued")
	}
}

func (s *Scheduler) recoverStaleTasks(ctx context.Context) {
	log := logger.Log

	staleTasks, err := s.taskRepo.FindStaleTasks(ctx, staleTaskPendingTimeout, staleTaskProcessingTimeout)
	if err != nil {
		log.Error().Err(err).Msg("failed to find stale tasks")
		return
	}

	if len(staleTasks) == 0 {
		return
	}

	log.Info().Int("count", len(staleTasks)).Msg("found stale tasks to recover")

	for _, task := range staleTasks {
		errMsg := "task timed out"
		if task.Status == status.TaskPending {
			errMsg = "task stuck in pending state"
		} else if task.Status == status.TaskProcessing {
			errMsg = "task stuck in processing state (possible worker crash or DLQ)"
		}

		// Calculate next retry time with exponential backoff
		retryDelay := baseRetryDelay * time.Duration(1<<task.RetryCount) // 5m, 10m, 20m, 40m...
		nextRetryAt := time.Now().Add(retryDelay)

		if err := s.taskRepo.MarkFailedWithRetry(ctx, task.ID.Hex(), errMsg, &nextRetryAt); err != nil {
			log.Warn().Err(err).Str("task", task.ID.Hex()).Msg("failed to mark stale task as failed")
			continue
		}

		log.Info().
			Str("task", task.ID.Hex()).
			Str("site", task.Domain).
			Str("status", string(task.Status)).
			Int("retry_count", task.RetryCount).
			Time("next_retry_at", nextRetryAt).
			Msg("stale task marked as failed, scheduled for retry")
	}
}

func (s *Scheduler) refreshAllViolations(ctx context.Context) {
	log := logger.Log

	if s.violationsSvc == nil || s.contentRepo == nil {
		return
	}

	contents, err := s.contentRepo.GetAll(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get contents for violations refresh")
		return
	}

	if len(contents) == 0 {
		return
	}

	contentInfos := make([]violations.ContentInfo, len(contents))
	for i, c := range contents {
		contentInfos[i] = violations.ContentInfo{
			ID:            c.ID.Hex(),
			Title:         c.Title,
			OriginalTitle: c.OriginalTitle,
			Year:          c.Year,
			KinopoiskID:   c.KinopoiskID,
			IMDBID:        c.IMDBID,
			MALID:         c.MALID,
			ShikimoriID:   c.ShikimoriID,
			MyDramaListID: c.MyDramaListID,
		}
	}

	updated, err := s.violationsSvc.RefreshAll(ctx, contentInfos)
	if err != nil {
		log.Error().Err(err).Msg("failed to refresh violations")
		return
	}

	if updated > 0 {
		log.Info().Int64("count", updated).Msg("violations refreshed")
	}
}

func (s *Scheduler) retryFailedTasks(ctx context.Context) {
	log := logger.Log

	failedTasks, err := s.taskRepo.FindFailedTasksForRetry(ctx, maxTaskRetries)
	if err != nil {
		log.Error().Err(err).Msg("failed to find failed tasks for retry")
		return
	}

	if len(failedTasks) == 0 {
		return
	}

	log.Info().Int("count", len(failedTasks)).Msg("found failed tasks to retry")

	for _, task := range failedTasks {
		// Check if max retries exceeded - freeze the site
		if task.RetryCount >= maxTaskRetries {
			reason := "max scan retries exceeded"
			if err := s.siteRepo.MarkFrozen(ctx, task.SiteID, reason); err != nil {
				log.Warn().Err(err).Str("site", task.Domain).Msg("failed to freeze site after max retries")
			} else {
				log.Warn().
					Str("site", task.Domain).
					Int("retries", task.RetryCount).
					Msg("site frozen after max retries")
			}
			continue
		}

		// Reset task to processing and increment retry count
		if err := s.taskRepo.IncrementRetryAndReset(ctx, task.ID.Hex()); err != nil {
			log.Warn().Err(err).Str("task", task.ID.Hex()).Msg("failed to reset task for retry")
			continue
		}

		// Get site for publishing task
		site, err := s.siteRepo.FindByID(ctx, task.SiteID)
		if err != nil {
			log.Warn().Err(err).Str("site", task.SiteID).Msg("failed to get site for retry")
			continue
		}

		taskInfo := indexerQueue.TaskInfo{
			TaskID:       task.ID.Hex(),
			Site:         site,
			AutoContinue: true,
		}

		// Publish to the appropriate queue based on current stage
		var publishErr error
		if task.Stage == status.StageSitemap {
			publishErr = s.publisher.PublishSitemapCrawlTask(ctx, taskInfo)
		} else {
			publishErr = s.publisher.PublishPageCrawlTaskSimple(ctx, taskInfo)
		}

		if publishErr != nil {
			log.Warn().Err(publishErr).Str("task", task.ID.Hex()).Msg("failed to republish task")
			continue
		}

		log.Info().
			Str("task", task.ID.Hex()).
			Str("site", task.Domain).
			Str("stage", string(task.Stage)).
			Int("retry", task.RetryCount+1).
			Msg("failed task retried")
	}
}
