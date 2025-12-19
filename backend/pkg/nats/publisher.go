package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

type Publisher struct {
	js jetstream.JetStream
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{js: client.JetStream()}
}

func (p *Publisher) Publish(ctx context.Context, subject string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = p.js.Publish(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}

	return nil
}

func (p *Publisher) PublishCrawlTask(ctx context.Context, task any) error {
	return p.Publish(ctx, SubjectCrawlTasks, task)
}

func (p *Publisher) PublishCrawlResult(ctx context.Context, result any) error {
	return p.Publish(ctx, SubjectCrawlResults, result)
}

func (p *Publisher) PublishCrawlProgress(ctx context.Context, progress any) error {
	return p.Publish(ctx, SubjectCrawlProgress, progress)
}

func (p *Publisher) PublishDetectTask(ctx context.Context, task any) error {
	return p.Publish(ctx, SubjectDetectTasks, task)
}

func (p *Publisher) PublishDetectResult(ctx context.Context, result any) error {
	return p.Publish(ctx, SubjectDetectResults, result)
}

func (p *Publisher) PublishToDLQ(ctx context.Context, originalSubject string, data any, reason string) error {
	dlqMsg := map[string]any{
		"original_subject": originalSubject,
		"data":             data,
		"reason":           reason,
	}
	return p.Publish(ctx, "dlq."+originalSubject, dlqMsg)
}

// Two-phase crawl publishers

func (p *Publisher) PublishSitemapCrawlTask(ctx context.Context, task any) error {
	return p.Publish(ctx, SubjectSitemapCrawlTasks, task)
}

func (p *Publisher) PublishSitemapURLBatch(ctx context.Context, batch any) error {
	return p.Publish(ctx, SubjectSitemapURLBatches, batch)
}

func (p *Publisher) PublishSitemapCrawlResult(ctx context.Context, result any) error {
	return p.Publish(ctx, SubjectSitemapCrawlResults, result)
}

func (p *Publisher) PublishPageCrawlTask(ctx context.Context, task any) error {
	return p.Publish(ctx, SubjectPageCrawlTasks, task)
}

func (p *Publisher) PublishPageSingleResult(ctx context.Context, result any) error {
	return p.Publish(ctx, SubjectPageSingleResults, result)
}

func (p *Publisher) PublishPageCrawlResult(ctx context.Context, result any) error {
	return p.Publish(ctx, SubjectPageCrawlResults, result)
}
