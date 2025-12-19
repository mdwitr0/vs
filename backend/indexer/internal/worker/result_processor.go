package worker

import (
	"context"
	"fmt"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/repo"
)

type ResultProcessor struct {
	natsClient     *nats.Client
	siteRepo       *repo.SiteRepo
	taskRepo       *repo.ScanTaskRepo
	contentRepo    *repo.ContentRepo
	sitemapURLRepo *repo.SitemapURLRepo
	violationsSvc  *violations.Service
}

func NewResultProcessor(natsClient *nats.Client, siteRepo *repo.SiteRepo, taskRepo *repo.ScanTaskRepo, contentRepo *repo.ContentRepo, sitemapURLRepo *repo.SitemapURLRepo, violationsSvc *violations.Service) *ResultProcessor {
	return &ResultProcessor{
		natsClient:     natsClient,
		siteRepo:       siteRepo,
		taskRepo:       taskRepo,
		contentRepo:    contentRepo,
		sitemapURLRepo: sitemapURLRepo,
		violationsSvc:  violationsSvc,
	}
}

func (p *ResultProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamCrawlResults,
		Consumer: "result-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("result processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var result queue.CrawlResult
		if err := msg.Unmarshal(&result); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal crawl result")
			return err
		}

		p.processResult(ctx, &result)
		return nil
	})
}

func (p *ResultProcessor) processResult(ctx context.Context, result *queue.CrawlResult) {
	log := logger.Log

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

	if p.sitemapURLRepo != nil && len(result.ParsedURLs) > 0 {
		urlsBySource := make(map[string][]repo.SitemapURLInput)
		for _, pu := range result.ParsedURLs {
			urlsBySource[pu.Source] = append(urlsBySource[pu.Source], repo.SitemapURLInput{
				URL:        pu.URL,
				LastMod:    pu.LastMod,
				Priority:   pu.Priority,
				ChangeFreq: pu.ChangeFreq,
				Depth:      pu.Depth,
			})
		}

		var totalInserted, totalUpdated int
		for source, urls := range urlsBySource {
			inserted, updated, err := p.sitemapURLRepo.UpsertBatch(ctx, result.SiteID, source, urls)
			if err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Str("source", source).Msg("failed to upsert sitemap urls")
				continue
			}
			totalInserted += inserted
			totalUpdated += updated
		}
		if totalInserted > 0 || totalUpdated > 0 {
			log.Info().
				Str("site", result.SiteID).
				Int("inserted", totalInserted).
				Int("updated", totalUpdated).
				Msg("sitemap urls saved")
		}
	}

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
		} else {
			log.Info().Str("site", result.SiteID).Int("cookies", len(cookies)).Msg("cookies updated from crawl result")
		}
	}

	if result.IsBlocked {
		reason := "HTTP 403/429/503 detected"
		if result.BlockedCount > 0 {
			reason = fmt.Sprintf("Blocked %d requests (403/429/503)", result.BlockedCount)
		}
		if err := p.siteRepo.MarkFrozen(ctx, result.SiteID, reason); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to freeze site")
		} else {
			log.Warn().Str("site", result.SiteID).Str("reason", reason).Msg("site frozen due to blocking")
			// Cancel all active tasks for frozen site
			if p.taskRepo != nil {
				if cancelled, err := p.taskRepo.CancelBySiteID(ctx, result.SiteID); err != nil {
					log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to cancel tasks")
				} else if cancelled > 0 {
					log.Info().Int64("cancelled", cancelled).Str("site", result.SiteID).Msg("tasks cancelled for blocked site")
				}
			}
		}
	} else if result.Success {
		if err := p.siteRepo.MarkSuccess(ctx, result.SiteID, result.ScanIntervalH); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site success")
		}
	} else {
		if err := p.siteRepo.MarkFailure(ctx, result.SiteID, result.IsDomainExpired); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site failure")
		}
		// If domain expired, site becomes dead - cancel all tasks
		if result.IsDomainExpired && p.taskRepo != nil {
			if cancelled, err := p.taskRepo.CancelBySiteID(ctx, result.SiteID); err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to cancel tasks")
			} else if cancelled > 0 {
				log.Info().Int64("cancelled", cancelled).Str("site", result.SiteID).Msg("tasks cancelled for dead site")
			}
		}
	}

	if p.taskRepo != nil {
		if err := p.taskRepo.UpdateFromResult(ctx, result); err != nil {
			log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to update task")
		}
	}

	log.Debug().
		Str("site", result.SiteID).
		Bool("success", result.Success).
		Bool("blocked", result.IsBlocked).
		Int("pages", result.PagesSaved).
		Msg("result processed")

	if result.Success && result.PagesSaved > 0 {
		go p.refreshViolationsForSite(context.Background(), result.SiteID)
	}
}

func (p *ResultProcessor) refreshViolationsForSite(ctx context.Context, siteID string) {
	if p.contentRepo == nil || p.violationsSvc == nil {
		return
	}

	log := logger.Log

	contents, err := p.contentRepo.GetAll(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get contents for violations refresh")
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

	updated, err := p.violationsSvc.RefreshForSite(ctx, siteID, contentInfos)
	if err != nil {
		log.Warn().Err(err).Str("site", siteID).Msg("failed to refresh violations for site")
		return
	}

	if updated > 0 {
		log.Info().Int64("count", updated).Str("site", siteID).Msg("violations refreshed for site")
	}
}
