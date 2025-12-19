package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/backend/pkg/status"
	indexerQueue "github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type DetectProcessor struct {
	natsClient       *nats.Client
	siteRepo         *repo.SiteRepo
	taskRepo         *repo.ScanTaskRepo
	publisher        *nats.Publisher
	indexerPublisher *indexerQueue.Publisher
}

func NewDetectProcessor(natsClient *nats.Client, siteRepo *repo.SiteRepo, taskRepo *repo.ScanTaskRepo, indexerPublisher *indexerQueue.Publisher) *DetectProcessor {
	return &DetectProcessor{
		natsClient:       natsClient,
		siteRepo:         siteRepo,
		taskRepo:         taskRepo,
		publisher:        nats.NewPublisher(natsClient),
		indexerPublisher: indexerPublisher,
	}
}

func (p *DetectProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamDetectResults,
		Consumer: "detect-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("detect processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var result queue.DetectResultMsg
		if err := msg.Unmarshal(&result); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal detect result")
			return err
		}

		p.processResult(ctx, &result)
		return nil
	})
}

func (p *DetectProcessor) processResult(ctx context.Context, result *queue.DetectResultMsg) {
	log := logger.Log

	// Handle domain redirect FIRST
	if result.Success && result.HasDomainRedirect && result.RedirectToDomain != "" {
		log.Info().
			Str("site", result.SiteID).
			Str("redirect_to", result.RedirectToDomain).
			Msg("processing domain redirect")

		if err := p.handleDomainRedirect(ctx, result.SiteID, result.RedirectToDomain); err != nil {
			log.Error().Err(err).Str("site", result.SiteID).Msg("failed to handle domain redirect")
		}
		return
	}

	if !result.Success {
		// DNS errors should freeze immediately without retries
		if p.isPermanentError(result.Error) {
			log.Warn().
				Str("site", result.SiteID).
				Str("error", result.Error).
				Msg("permanent error detected, freezing site immediately")

			if err := p.siteRepo.MarkFrozen(ctx, result.SiteID, result.Error); err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site as frozen")
			}
			// Cancel all active tasks for this site
			if p.taskRepo != nil {
				if cancelled, err := p.taskRepo.CancelBySiteID(ctx, result.SiteID); err != nil {
					log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to cancel tasks")
				} else if cancelled > 0 {
					log.Info().Int64("cancelled", cancelled).Str("site", result.SiteID).Msg("tasks cancelled for unavailable site")
				}
			}
			return
		}

		site, err := p.siteRepo.FindByID(ctx, result.SiteID)
		if err != nil || site == nil {
			log.Warn().Str("site", result.SiteID).Msg("site not found for detection failure handling")
			return
		}

		newFailureCount := site.FailureCount + 1
		const maxDetectionRetries = 3

		if newFailureCount >= maxDetectionRetries {
			log.Warn().
				Str("site", result.SiteID).
				Str("error", result.Error).
				Int("retries", newFailureCount).
				Msg("detection failed after max retries, freezing site")

			if err := p.siteRepo.MarkFrozen(ctx, result.SiteID, "detection failed after 3 retries: "+result.Error); err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to mark site as frozen")
			}
			// Cancel all active tasks for this site
			if p.taskRepo != nil {
				if cancelled, err := p.taskRepo.CancelBySiteID(ctx, result.SiteID); err != nil {
					log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to cancel tasks")
				} else if cancelled > 0 {
					log.Info().Int64("cancelled", cancelled).Str("site", result.SiteID).Msg("tasks cancelled for frozen site")
				}
			}
		} else {
			log.Warn().
				Str("site", result.SiteID).
				Str("error", result.Error).
				Int("retry", newFailureCount).
				Int("max", maxDetectionRetries).
				Msg("detection failed, will retry")

			if err := p.siteRepo.IncrementFailureCount(ctx, result.SiteID); err != nil {
				log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to increment failure count")
			}
		}
		return
	}

	scannerType := status.ScannerHTTP
	if result.NeedsSPA {
		scannerType = status.ScannerSPA
	}

	var cookies []repo.Cookie
	for _, c := range result.Cookies {
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

	if err := p.siteRepo.UpdateFromDetection(ctx, result.SiteID, repo.DetectionUpdate{
		CMS:           result.CMS,
		HasSitemap:    result.HasSitemap,
		SitemapStatus: status.SitemapStatus(result.SitemapStatus),
		CrawlStrategy: status.CrawlStrategy(result.CrawlStrategy),
		SitemapURLs:   result.SitemapURLs,
		ScannerType:   scannerType,
		CaptchaType:   result.CaptchaType,
		Cookies:       cookies,
	}); err != nil {
		log.Warn().Err(err).Str("site", result.SiteID).Msg("failed to update site from detection")
		return
	}

	log.Info().
		Str("site", result.SiteID).
		Str("cms", result.CMS).
		Bool("sitemap", result.HasSitemap).
		Str("sitemap_status", result.SitemapStatus).
		Str("crawl_strategy", result.CrawlStrategy).
		Str("scanner", string(scannerType)).
		Str("captcha", result.CaptchaType).
		Msg("site updated from detection")

	p.queueImmediateScan(ctx, result.SiteID)
}

// isPermanentError returns true if the error is permanent and should not be retried
func (p *DetectProcessor) isPermanentError(errMsg string) bool {
	permanentErrors := []string{
		"domain not resolvable",
		"no such host",
		"server misbehaving",
	}
	for _, pe := range permanentErrors {
		if strings.Contains(errMsg, pe) {
			return true
		}
	}
	return false
}

func (p *DetectProcessor) handleDomainRedirect(ctx context.Context, siteID, newDomain string) error {
	log := logger.Log

	// 1. Get current site to retrieve original domain
	oldSite, err := p.siteRepo.FindByID(ctx, siteID)
	if err != nil || oldSite == nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	originalDomain := oldSite.Domain

	// 2. Mark old site as moved
	if err := p.siteRepo.MarkAsMoved(ctx, siteID, newDomain); err != nil {
		return fmt.Errorf("failed to mark site as moved: %w", err)
	}
	log.Info().Str("old_site", originalDomain).Str("new_domain", newDomain).Msg("old site marked as moved")

	// 3. Check if new domain already exists
	existingSite, _ := p.siteRepo.FindByDomain(ctx, newDomain)
	if existingSite != nil {
		log.Info().
			Str("domain", newDomain).
			Str("existing_id", existingSite.ID.Hex()).
			Msg("new domain already exists, skipping creation")
		return nil
	}

	// 4. Create new site with reference to original
	newSite := &repo.Site{
		Domain:         newDomain,
		OriginalDomain: originalDomain,
		ScanIntervalH:  oldSite.ScanIntervalH,
	}

	if err := p.siteRepo.Create(ctx, newSite); err != nil {
		return fmt.Errorf("failed to create new site: %w", err)
	}
	log.Info().Str("new_site", newDomain).Str("original", originalDomain).Msg("new site created")

	// 5. Queue detection for new site
	detectTask := queue.DetectTask{
		ID:     uuid.New().String(),
		SiteID: newSite.ID.Hex(),
		Domain: newDomain,
	}
	if err := p.publisher.PublishDetectTask(ctx, detectTask); err != nil {
		log.Warn().Err(err).Str("site", newDomain).Msg("failed to queue detection for new site")
	}

	return nil
}

func (p *DetectProcessor) queueImmediateScan(ctx context.Context, siteID string) {
	log := logger.Log

	site, err := p.siteRepo.FindByID(ctx, siteID)
	if err != nil || site == nil {
		return
	}

	hasActive, _ := p.taskRepo.HasActiveTask(ctx, siteID)
	if hasActive {
		return
	}

	scanTask := &repo.ScanTask{
		SiteID: siteID,
		Domain: site.Domain,
	}
	if err := p.taskRepo.Create(ctx, scanTask); err != nil {
		log.Warn().Err(err).Str("site", site.Domain).Msg("failed to create scan task after detection")
		return
	}

	taskInfo := indexerQueue.TaskInfo{
		TaskID:       scanTask.ID.Hex(),
		Site:         site,
		AutoContinue: true,
	}
	if err := p.indexerPublisher.PublishSitemapCrawlTask(ctx, taskInfo); err != nil {
		log.Warn().Err(err).Str("site", site.Domain).Msg("failed to queue scan after detection")
		return
	}

	log.Info().Str("site", site.Domain).Msg("scan queued immediately after detection")
}
