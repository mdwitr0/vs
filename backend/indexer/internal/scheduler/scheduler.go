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
	pendingDetectionTimeout  = 5 * time.Minute
	staleTaskPendingTimeout  = 30 * time.Minute
	staleTaskProcessingTimeout = 2 * time.Hour
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

		if err := s.taskRepo.MarkFailed(ctx, task.ID.Hex(), errMsg); err != nil {
			log.Warn().Err(err).Str("task", task.ID.Hex()).Msg("failed to mark stale task as failed")
			continue
		}

		log.Info().
			Str("task", task.ID.Hex()).
			Str("site", task.Domain).
			Str("status", string(task.Status)).
			Msg("stale task marked as failed")
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
