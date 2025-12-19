package service

import (
	"context"
	"time"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/status"
	"github.com/video-analitics/indexer/internal/repo"
)

// TaskProgressService единая точка управления прогрессом задач сканирования
type TaskProgressService struct {
	taskRepo       *repo.ScanTaskRepo
	sitemapURLRepo *repo.SitemapURLRepo
}

func NewTaskProgressService(taskRepo *repo.ScanTaskRepo, sitemapURLRepo *repo.SitemapURLRepo) *TaskProgressService {
	return &TaskProgressService{
		taskRepo:       taskRepo,
		sitemapURLRepo: sitemapURLRepo,
	}
}

// OnPageProcessed вызывается после обработки каждой страницы
func (s *TaskProgressService) OnPageProcessed(ctx context.Context, taskID string, success bool) {
	if taskID == "" {
		return
	}
	if err := s.taskRepo.IncrementPageProgress(ctx, taskID, success); err != nil {
		logger.Log.Warn().Err(err).Str("task", taskID).Bool("success", success).Msg("failed to increment page progress")
	}
}

// getSitemapURLCount получает количество URL из БД с retry (для race condition с batch processing)
// Ждёт пока count стабилизируется (одинаковое значение в 2 последовательных запросах)
func (s *TaskProgressService) getSitemapURLCount(ctx context.Context, siteID string) int64 {
	var prevCount int64 = -1
	stableCount := 0

	for i := 0; i < 10; i++ {
		stats, err := s.sitemapURLRepo.GetStats(ctx, siteID)
		if err != nil {
			logger.Log.Warn().Err(err).Str("site", siteID).Msg("failed to get sitemap url stats")
			return 0
		}

		if stats.Total == prevCount && stats.Total > 0 {
			stableCount++
			if stableCount >= 2 {
				logger.Log.Debug().Str("site", siteID).Int64("count", stats.Total).Msg("sitemap url count stable")
				return stats.Total
			}
		} else {
			stableCount = 0
		}

		prevCount = stats.Total
		time.Sleep(300 * time.Millisecond)
	}

	// Если за 3 секунды не стабилизировался, возвращаем последний результат
	if prevCount > 0 {
		logger.Log.Warn().Str("site", siteID).Int64("count", prevCount).Msg("sitemap url count not stable, using last value")
		return prevCount
	}
	return 0
}

// CompleteSitemapStage завершает этап sitemap и начинает этап page
// Сам получает реальное количество URL из sitemap_urls
func (s *TaskProgressService) CompleteSitemapStage(ctx context.Context, taskID, siteID string) error {
	total := s.getSitemapURLCount(ctx, siteID)
	return s.taskRepo.CompleteSitemapStage(ctx, taskID, total)
}

// CompleteSitemapStageOnly завершает этап sitemap без запуска page (AutoContinue=false)
// Сам получает реальное количество URL из sitemap_urls
func (s *TaskProgressService) CompleteSitemapStageOnly(ctx context.Context, taskID, siteID string) error {
	total := s.getSitemapURLCount(ctx, siteID)
	return s.taskRepo.CompleteSitemapStageOnly(ctx, taskID, total)
}

// FailSitemapStage помечает этап sitemap как failed
func (s *TaskProgressService) FailSitemapStage(ctx context.Context, taskID string, sitemapResult *repo.StageResult) error {
	return s.taskRepo.FailSitemapStage(ctx, taskID, sitemapResult)
}

// CompletePageStage завершает этап page и всю задачу
func (s *TaskProgressService) CompletePageStage(ctx context.Context, taskID string, errorMsg string) error {
	pageResult := &repo.StageResult{
		Status: status.TaskCompleted,
		Error:  errorMsg,
	}
	return s.taskRepo.CompletePageStage(ctx, taskID, pageResult)
}

// FailPageStage помечает этап page как failed
func (s *TaskProgressService) FailPageStage(ctx context.Context, taskID string, errorMsg string) error {
	pageResult := &repo.StageResult{
		Status: status.TaskFailed,
		Error:  errorMsg,
	}
	return s.taskRepo.FailPageStage(ctx, taskID, pageResult)
}

