package worker

import (
	"context"
	"fmt"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type ProgressProcessor struct {
	natsClient *nats.Client
	taskRepo   *repo.ScanTaskRepo
}

func NewProgressProcessor(natsClient *nats.Client, taskRepo *repo.ScanTaskRepo) *ProgressProcessor {
	return &ProgressProcessor{
		natsClient: natsClient,
		taskRepo:   taskRepo,
	}
}

func (p *ProgressProcessor) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(p.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamCrawlProgress,
		Consumer: "progress-processor",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("progress processor started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var progress queue.CrawlProgress
		if err := msg.Unmarshal(&progress); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal crawl progress")
			return err
		}

		if err := p.taskRepo.UpdateProgress(ctx, &progress); err != nil {
			log.Warn().Err(err).Str("task", progress.TaskID).Msg("failed to update progress")
			return nil
		}

		log.Debug().
			Str("task", progress.TaskID).
			Int("found", progress.PagesFound).
			Int("saved", progress.PagesSaved).
			Msg("progress updated")

		return nil
	})
}
