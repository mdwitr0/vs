package worker

import (
	"context"
	"fmt"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/repo"
	"github.com/video-analitics/indexer/internal/service"
)

type PageResultProcessor struct {
	natsClient    *nats.Client
	siteRepo      *repo.SiteRepo
	progressSvc   *service.TaskProgressService
	contentRepo   *repo.ContentRepo
	violationsSvc *violations.Service
}

func NewPageResultProcessor(
	natsClient *nats.Client,
	siteRepo *repo.SiteRepo,
	progressSvc *service.TaskProgressService,
	contentRepo *repo.ContentRepo,
	violationsSvc *violations.Service,
) *PageResultProcessor {
	return &PageResultProcessor{
		natsClient:    natsClient,
		siteRepo:      siteRepo,
		progressSvc:   progressSvc,
		contentRepo:   contentRepo,
		violationsSvc: violationsSvc,
	}
}

func (p *PageResultProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamPageCrawlResults,
		Consumer: "page-result-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("page result processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var result queue.PageCrawlResult
		if err := msg.Unmarshal(&result); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal page crawl result")
			return err
		}

		p.processResult(ctx, &result)
		return nil
	})
}

func (p *PageResultProcessor) processResult(ctx context.Context, result *queue.PageCrawlResult) {
	log := logger.Log

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

	// Handle case when no URLs were processed
	if result.NoURLsAvailable {
		if result.AllIndexed {
			log.Info().
				Str("site", result.SiteID).
				Str("task", result.TaskID).
				Msg("all URLs already indexed, task completed")

			if p.progressSvc != nil {
				if err := p.progressSvc.CompletePageStage(ctx, result.TaskID, ""); err != nil {
					log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to complete page stage")
				}
			}
			if err := p.siteRepo.MarkSuccess(ctx, result.SiteID, 0); err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site success")
			}
		} else {
			log.Info().
				Str("site", result.SiteID).
				Str("task", result.TaskID).
				Msg("no URLs available for processing (in retry delay)")

			if p.progressSvc != nil {
				if err := p.progressSvc.FailPageStage(ctx, result.TaskID, "URLs in retry delay"); err != nil {
					log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to complete page stage")
				}
			}
		}
		return
	}

	// Update task based on result
	if p.progressSvc != nil {
		if result.Success {
			if err := p.progressSvc.CompletePageStage(ctx, result.TaskID, ""); err != nil {
				log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to complete page stage")
			}
		} else {
			if err := p.progressSvc.FailPageStage(ctx, result.TaskID, result.Error); err != nil {
				log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to fail page stage")
			}
		}
	}

	// Site status based on crawl result
	// If any pages were successfully parsed, site is active
	// Only mark failure if NO pages succeeded at all
	if result.IPBlocked {
		log.Warn().Str("site", result.SiteID).Str("reason", result.BlockReason).Msg("IP blocked during page crawl (site not frozen)")
	} else if result.PagesSuccess > 0 {
		if err := p.siteRepo.MarkSuccess(ctx, result.SiteID, 0); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site success")
		}
	} else if result.PagesTotal > 0 && result.PagesFailed == result.PagesTotal {
		// All pages failed - mark as failure
		if err := p.siteRepo.MarkFailure(ctx, result.SiteID, false); err != nil {
			log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site failure")
		}
	}

	log.Info().
		Str("site", result.SiteID).
		Str("task", result.TaskID).
		Bool("success", result.Success).
		Int("total", result.PagesTotal).
		Int("success_count", result.PagesSuccess).
		Int("failed_count", result.PagesFailed).
		Msg("page stage completed")

	// Refresh violations if pages were successfully parsed
	if result.PagesSuccess > 0 {
		go p.refreshViolationsForSite(context.Background(), result.SiteID)
	}
}

func (p *PageResultProcessor) refreshViolationsForSite(ctx context.Context, siteID string) {
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
