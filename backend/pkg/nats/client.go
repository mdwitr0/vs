package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/video-analitics/backend/pkg/logger"
)

const (
	// Stream names (legacy - single-pass crawl)
	StreamCrawlTasks    = "CRAWL_TASKS"
	StreamCrawlResults  = "CRAWL_RESULTS"
	StreamCrawlProgress = "CRAWL_PROGRESS"
	StreamDetectTasks   = "DETECT_TASKS"
	StreamDetectResults = "DETECT_RESULTS"
	StreamDLQ           = "DLQ"

	// Stream names (two-phase crawl)
	StreamSitemapCrawlTasks   = "SITEMAP_CRAWL_TASKS"
	StreamSitemapURLBatches   = "SITEMAP_URL_BATCHES"
	StreamSitemapCrawlResults = "SITEMAP_CRAWL_RESULTS"
	StreamPageCrawlTasks      = "PAGE_CRAWL_TASKS"
	StreamPageSingleResults   = "PAGE_SINGLE_RESULTS"
	StreamPageCrawlResults    = "PAGE_CRAWL_RESULTS"

	// Subject prefixes (legacy)
	SubjectCrawlTasks    = "crawl.tasks"
	SubjectCrawlResults  = "crawl.results"
	SubjectCrawlProgress = "crawl.progress"
	SubjectDetectTasks   = "detect.tasks"
	SubjectDetectResults = "detect.results"
	SubjectDLQ           = "dlq.>"

	// Subject prefixes (two-phase crawl)
	SubjectSitemapCrawlTasks   = "sitemap.crawl.tasks"
	SubjectSitemapURLBatches   = "sitemap.url.batches"
	SubjectSitemapCrawlResults = "sitemap.crawl.results"
	SubjectPageCrawlTasks      = "page.crawl.tasks"
	SubjectPageSingleResults   = "page.single.results"
	SubjectPageCrawlResults    = "page.crawl.results"
)

type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
}

func New(url string) (*Client, error) {
	log := logger.Log

	opts := []nats.Option{
		nats.Name("video-analitics"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Warn().Str("url", nc.ConnectedUrl()).Msg("nats reconnected")
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Error().Err(err).Msg("nats disconnected")
			}
		}),
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream init: %w", err)
	}

	client := &Client{nc: nc, js: js}

	if err := client.ensureStreams(context.Background()); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure streams: %w", err)
	}

	log.Info().Str("url", url).Msg("nats connected")
	return client, nil
}

func (c *Client) ensureStreams(ctx context.Context) error {
	streams := []jetstream.StreamConfig{
		{
			Name:        StreamCrawlTasks,
			Subjects:    []string{SubjectCrawlTasks},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     100000,
			Description: "Crawl tasks for parser workers",
		},
		{
			Name:        StreamCrawlResults,
			Subjects:    []string{SubjectCrawlResults},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     100000,
			Description: "Crawl results from parser to indexer",
		},
		{
			Name:        StreamCrawlProgress,
			Subjects:    []string{SubjectCrawlProgress},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      1 * time.Hour,
			Storage:     jetstream.MemoryStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     100000,
			Description: "Real-time crawl progress updates",
		},
		{
			Name:        StreamDetectTasks,
			Subjects:    []string{SubjectDetectTasks},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Detection tasks for new sites",
		},
		{
			Name:        StreamDetectResults,
			Subjects:    []string{SubjectDetectResults},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Detection results from parser",
		},
		{
			Name:        StreamDLQ,
			Subjects:    []string{SubjectDLQ},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      7 * 24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Dead letter queue for failed tasks",
		},
		// Two-phase crawl streams
		{
			Name:        StreamSitemapCrawlTasks,
			Subjects:    []string{SubjectSitemapCrawlTasks},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Sitemap crawl tasks",
		},
		{
			Name:        StreamSitemapURLBatches,
			Subjects:    []string{SubjectSitemapURLBatches},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     100000,
			Description: "URL batches from sitemap crawl",
		},
		{
			Name:        StreamSitemapCrawlResults,
			Subjects:    []string{SubjectSitemapCrawlResults},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Sitemap crawl results",
		},
		{
			Name:        StreamPageCrawlTasks,
			Subjects:    []string{SubjectPageCrawlTasks},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Page crawl tasks",
		},
		{
			Name:        StreamPageSingleResults,
			Subjects:    []string{SubjectPageSingleResults},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      6 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     500000,
			Description: "Single page results for immediate status update",
		},
		{
			Name:        StreamPageCrawlResults,
			Subjects:    []string{SubjectPageCrawlResults},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Discard:     jetstream.DiscardOld,
			MaxMsgs:     10000,
			Description: "Page crawl final results",
		},
	}

	for _, cfg := range streams {
		_, err := c.js.CreateOrUpdateStream(ctx, cfg)
		if err != nil {
			return fmt.Errorf("create stream %s: %w", cfg.Name, err)
		}
		logger.Log.Debug().Str("stream", cfg.Name).Msg("stream ensured")
	}

	return nil
}

func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

func (c *Client) Close() {
	c.nc.Close()
}
