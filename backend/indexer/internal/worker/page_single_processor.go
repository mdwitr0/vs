package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/models"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/indexer/internal/repo"
	"github.com/video-analitics/indexer/internal/service"
)

type PageSingleProcessor struct {
	natsClient     *nats.Client
	siteRepo       *repo.SiteRepo
	pageRepo       *repo.PageRepo
	sitemapURLRepo *repo.SitemapURLRepo
	progressSvc    *service.TaskProgressService
	meili          *meili.Client
}

func NewPageSingleProcessor(
	natsClient *nats.Client,
	siteRepo *repo.SiteRepo,
	pageRepo *repo.PageRepo,
	sitemapURLRepo *repo.SitemapURLRepo,
	progressSvc *service.TaskProgressService,
	meili *meili.Client,
) *PageSingleProcessor {
	return &PageSingleProcessor{
		natsClient:     natsClient,
		siteRepo:       siteRepo,
		pageRepo:       pageRepo,
		sitemapURLRepo: sitemapURLRepo,
		progressSvc:    progressSvc,
		meili:          meili,
	}
}

func (p *PageSingleProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:        nats.StreamPageSingleResults,
		Consumer:      "page-single-processor",
		MaxAckPending: 50,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("page single result processor started")

	return consumer.ConsumePool(ctx, 5, func(ctx context.Context, msg *nats.Message) error {
		var result queue.PageSingleResult
		if err := msg.Unmarshal(&result); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal page single result")
			return err
		}

		p.processResult(ctx, &result)
		return nil
	})
}

func (p *PageSingleProcessor) processResult(ctx context.Context, result *queue.PageSingleResult) {
	log := logger.Log

	if !result.Success {
		if err := p.sitemapURLRepo.MarkError(ctx, result.SiteID, result.URL, result.Error); err != nil {
			log.Warn().Err(err).Str("url", result.URL).Msg("failed to mark url error")
		}
		p.incrementProgress(ctx, result.TaskID, false)
		return
	}

	if result.Page == nil {
		if err := p.sitemapURLRepo.MarkError(ctx, result.SiteID, result.URL, "no page data"); err != nil {
			log.Warn().Err(err).Str("url", result.URL).Msg("failed to mark url error")
		}
		p.incrementProgress(ctx, result.TaskID, false)
		return
	}

	page := p.convertPageData(result.SiteID, result.Page)

	if err := p.pageRepo.Upsert(ctx, page); err != nil {
		log.Warn().Err(err).Str("url", result.URL).Msg("failed to save page")
		if err := p.sitemapURLRepo.MarkError(ctx, result.SiteID, result.URL, err.Error()); err != nil {
			log.Warn().Err(err).Str("url", result.URL).Msg("failed to mark url error")
		}
		p.incrementProgress(ctx, result.TaskID, false)
		return
	}

	if err := p.sitemapURLRepo.MarkIndexed(ctx, result.SiteID, result.URL); err != nil {
		log.Warn().Err(err).Str("url", result.URL).Msg("failed to mark url indexed")
	}

	if p.meili != nil {
		site, _ := p.siteRepo.FindByID(ctx, result.SiteID)
		domain := ""
		if site != nil {
			domain = site.Domain
		}

		doc := meili.PageDocument{
			ID:            page.ID.Hex(),
			SiteID:        page.SiteID,
			Domain:        domain,
			URL:           page.URL,
			Title:         page.Title,
			Description:   page.Description,
			MainText:      page.MainText,
			Year:          page.Year,
			KinopoiskID:   page.ExternalIDs.KinopoiskID,
			IMDBID:        page.ExternalIDs.IMDBID,
			MALID:         page.ExternalIDs.MALID,
			ShikimoriID:   page.ExternalIDs.ShikimoriID,
			MyDramaListID: page.ExternalIDs.MyDramaListID,
			LinksText:     page.LinksText,
			PlayerURLs:    []string{page.PlayerURL},
			IndexedAt:     page.IndexedAt.Format(time.RFC3339),
		}

		if err := p.meili.IndexPages([]meili.PageDocument{doc}); err != nil {
			log.Warn().Err(err).Str("url", result.URL).Msg("meili indexing failed")
		}
	}

	log.Debug().Str("url", result.URL).Msg("page indexed")
	p.incrementProgress(ctx, result.TaskID, true)
}

func (p *PageSingleProcessor) convertPageData(siteID string, pd *queue.PageData) *models.Page {
	externalIDs := models.ExternalIDs{}
	if pd.ExternalIDs != nil {
		externalIDs.KinopoiskID = pd.ExternalIDs["kinopoisk_id"]
		externalIDs.IMDBID = pd.ExternalIDs["imdb_id"]
		externalIDs.TMDBID = pd.ExternalIDs["tmdb_id"]
		externalIDs.MALID = pd.ExternalIDs["mal_id"]
		externalIDs.ShikimoriID = pd.ExternalIDs["shikimori_id"]
		externalIDs.MyDramaListID = pd.ExternalIDs["mydramalist_id"]
	}

	return &models.Page{
		SiteID:      siteID,
		URL:         pd.URL,
		Title:       pd.Title,
		Description: pd.Description,
		MainText:    pd.MainText,
		Year:        pd.Year,
		PlayerURL:   pd.PlayerURL,
		LinksText:   pd.LinksText,
		ExternalIDs: externalIDs,
		HTTPStatus:  200,
		IndexedAt:   time.Now(),
	}
}

func (p *PageSingleProcessor) incrementProgress(ctx context.Context, taskID string, success bool) {
	if p.progressSvc != nil {
		p.progressSvc.OnPageProcessed(ctx, taskID, success)
	}
}
