package worker

import (
	"context"
	"fmt"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type SitemapBatchProcessor struct {
	natsClient     *nats.Client
	sitemapURLRepo *repo.SitemapURLRepo
}

func NewSitemapBatchProcessor(natsClient *nats.Client, sitemapURLRepo *repo.SitemapURLRepo) *SitemapBatchProcessor {
	return &SitemapBatchProcessor{
		natsClient:     natsClient,
		sitemapURLRepo: sitemapURLRepo,
	}
}

func (p *SitemapBatchProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamSitemapURLBatches,
		Consumer: "sitemap-batch-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("sitemap batch processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var batch queue.SitemapURLBatch
		if err := msg.Unmarshal(&batch); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal sitemap url batch")
			return err
		}

		p.processBatch(ctx, &batch)
		return nil
	})
}

func (p *SitemapBatchProcessor) processBatch(ctx context.Context, batch *queue.SitemapURLBatch) {
	log := logger.Log

	if len(batch.URLs) == 0 {
		return
	}

	urls := make([]repo.SitemapURLInput, len(batch.URLs))
	for i, u := range batch.URLs {
		urls[i] = repo.SitemapURLInput{
			URL:        u.URL,
			LastMod:    u.LastMod,
			Priority:   u.Priority,
			ChangeFreq: u.ChangeFreq,
			Depth:      u.Depth,
		}
	}

	inserted, updated, err := p.sitemapURLRepo.UpsertBatch(ctx, batch.SiteID, batch.SitemapSource, urls)
	if err != nil {
		log.Error().
			Err(err).
			Str("site", batch.SiteID).
			Str("task", batch.TaskID).
			Int("batch", batch.BatchNumber).
			Msg("failed to upsert sitemap urls batch")
		return
	}

	log.Info().
		Str("site", batch.SiteID).
		Int("batch", batch.BatchNumber).
		Int("inserted", inserted).
		Int("updated", updated).
		Msg("sitemap urls batch saved")
}
