package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	indexerQueue "github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
	"github.com/video-analitics/indexer/internal/service"
)

type SitemapResultProcessor struct {
	natsClient  *nats.Client
	siteRepo    *repo.SiteRepo
	progressSvc *service.TaskProgressService
	publisher   *indexerQueue.Publisher
}

func NewSitemapResultProcessor(
	natsClient *nats.Client,
	siteRepo *repo.SiteRepo,
	progressSvc *service.TaskProgressService,
	publisher *indexerQueue.Publisher,
) *SitemapResultProcessor {
	return &SitemapResultProcessor{
		natsClient:  natsClient,
		siteRepo:    siteRepo,
		progressSvc: progressSvc,
		publisher:   publisher,
	}
}

func (p *SitemapResultProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamSitemapCrawlResults,
		Consumer: "sitemap-result-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("sitemap result processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var result queue.SitemapCrawlResult
		if err := msg.Unmarshal(&result); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal sitemap crawl result")
			return err
		}

		p.processResult(ctx, &result)
		return nil
	})
}

func (p *SitemapResultProcessor) processResult(ctx context.Context, result *queue.SitemapCrawlResult) {
	log := logger.Log

	// Update sitemap stats on site
	if len(result.SitemapStats) > 0 {
		var stats []repo.SitemapStats
		for _, s := range result.SitemapStats {
			stat := repo.SitemapStats{
				URL:       s.URL,
				URLsFound: s.URLsFound,
			}
			if s.Error != "" {
				stat.Error = fmt.Errorf("%s", s.Error)
			}
			stats = append(stats, stat)
		}
		if err := p.siteRepo.UpdateSitemapStats(ctx, result.SiteID, stats); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to update sitemap stats")
		}
	}

	// Update cookies if received
	if len(result.NewCookies) > 0 {
		var cookies []repo.Cookie
		for _, c := range result.NewCookies {
			cookies = append(cookies, repo.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  c.Expires,
				HTTPOnly: c.HTTPOnly,
				Secure:   c.Secure,
			})
		}
		if err := p.siteRepo.UpdateCookies(ctx, result.SiteID, cookies); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to update cookies")
		}
	}

	// Handle failure
	if !result.Success {
		sitemapResult := &repo.StageResult{Error: result.Error}
		if p.progressSvc != nil {
			if err := p.progressSvc.FailSitemapStage(ctx, result.TaskID, sitemapResult); err != nil {
				log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to mark sitemap stage failed")
			}
		}
		if err := p.siteRepo.MarkFailure(ctx, result.SiteID, false); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site failure")
		}
		log.Info().
			Str("site", result.SiteID).
			Str("task", result.TaskID).
			Str("error", result.Error).
			Msg("sitemap stage failed")
		return
	}

	// No URLs found by parser - fail
	if result.TotalURLs == 0 {
		sitemapResult := &repo.StageResult{Error: "no urls found in sitemap"}
		if p.progressSvc != nil {
			if err := p.progressSvc.FailSitemapStage(ctx, result.TaskID, sitemapResult); err != nil {
				log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to mark sitemap stage failed")
			}
		}
		if err := p.siteRepo.MarkSuccess(ctx, result.SiteID, 0); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site success")
		}
		log.Info().Str("site", result.SiteID).Msg("no urls found in sitemap, task completed")
		return
	}

	// Reset failure count on successful sitemap crawl
	if err := p.siteRepo.MarkSuccess(ctx, result.SiteID, 0); err != nil {
		log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to reset site status")
	}

	// Если AutoContinue=false, завершаем sitemap стадию БЕЗ старта page crawl
	if !result.AutoContinue {
		if p.progressSvc != nil {
			// progressSvc сам получит реальное количество URL из БД
			if err := p.progressSvc.CompleteSitemapStageOnly(ctx, result.TaskID, result.SiteID); err != nil {
				log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to complete sitemap stage")
			}
		}
		log.Info().
			Str("site", result.SiteID).
			Str("task", result.TaskID).
			Msg("sitemap stage completed, waiting for manual page crawl")
		return
	}

	// AutoContinue=true - complete sitemap and start page stage
	if p.progressSvc != nil {
		if err := p.progressSvc.CompleteSitemapStage(ctx, result.TaskID, result.SiteID); err != nil {
			log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to complete sitemap stage")
		}
	}

	log.Info().
		Str("site", result.SiteID).
		Str("task", result.TaskID).
		Bool("auto_continue", result.AutoContinue).
		Msg("sitemap stage completed")

	// Get site info for page crawl
	site, err := p.siteRepo.FindByID(ctx, result.SiteID)
	if err != nil || site == nil {
		log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to find site for page crawl")
		return
	}

	// Publish page crawl task with SAME task ID
	indexerAPIURL := os.Getenv("INDEXER_API_URL")
	if indexerAPIURL == "" {
		indexerAPIURL = "http://localhost:8080"
	}

	taskInfo := indexerQueue.TaskInfo{
		TaskID: result.TaskID, // Same task ID - we're continuing the same task
		Site:   site,
	}
	if err := p.publisher.PublishPageCrawlTask(ctx, taskInfo, indexerAPIURL, 50); err != nil {
		log.Error().Err(err).Str("site", result.SiteID).Msg("failed to publish page crawl task")
		return
	}

	log.Info().
		Str("site", result.SiteID).
		Str("task", result.TaskID).
		Int("pending_urls", result.TotalURLs).
		Msg("page crawl task published")
}
