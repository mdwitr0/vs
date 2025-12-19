package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/video-analitics/backend/pkg/logger"
)

type ConsumerConfig struct {
	Stream       string
	Consumer     string
	AckWait      time.Duration
	MaxDeliver   int
	MaxAckPending int
}

type Consumer struct {
	js       jetstream.JetStream
	consumer jetstream.Consumer
	config   ConsumerConfig
}

func NewConsumer(client *Client, cfg ConsumerConfig) (*Consumer, error) {
	if cfg.AckWait == 0 {
		cfg.AckWait = 5 * time.Minute
	}
	if cfg.MaxDeliver == 0 {
		cfg.MaxDeliver = 3
	}
	if cfg.MaxAckPending == 0 {
		cfg.MaxAckPending = 100
	}

	consumerCfg := jetstream.ConsumerConfig{
		Durable:       cfg.Consumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
		MaxAckPending: cfg.MaxAckPending,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	consumer, err := client.js.CreateOrUpdateConsumer(context.Background(), cfg.Stream, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer %s: %w", cfg.Consumer, err)
	}

	logger.Log.Debug().
		Str("stream", cfg.Stream).
		Str("consumer", cfg.Consumer).
		Dur("ack_wait", cfg.AckWait).
		Int("max_deliver", cfg.MaxDeliver).
		Msg("consumer created")

	return &Consumer{
		js:       client.js,
		consumer: consumer,
		config:   cfg,
	}, nil
}

type Message struct {
	msg  jetstream.Msg
	data []byte
}

func (m *Message) Data() []byte {
	return m.data
}

func (m *Message) Unmarshal(v any) error {
	return json.Unmarshal(m.data, v)
}

func (m *Message) Ack() error {
	return m.msg.Ack()
}

func (m *Message) Nak() error {
	return m.msg.Nak()
}

func (m *Message) NakWithDelay(delay time.Duration) error {
	return m.msg.NakWithDelay(delay)
}

func (m *Message) Term() error {
	return m.msg.Term()
}

func (m *Message) InProgress() error {
	return m.msg.InProgress()
}

func (m *Message) Metadata() (*jetstream.MsgMetadata, error) {
	return m.msg.Metadata()
}

func (c *Consumer) Fetch(ctx context.Context, batch int) ([]*Message, error) {
	msgs, err := c.consumer.Fetch(batch, jetstream.FetchMaxWait(5*time.Second))
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("fetch: %w", err)
	}

	var result []*Message
	for msg := range msgs.Messages() {
		result = append(result, &Message{
			msg:  msg,
			data: msg.Data(),
		})
	}

	if err := msgs.Error(); err != nil {
		return result, err
	}

	return result, nil
}

func (c *Consumer) FetchOne(ctx context.Context) (*Message, error) {
	msgs, err := c.Fetch(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return msgs[0], nil
}

type HandlerFunc func(ctx context.Context, msg *Message) error

func (c *Consumer) Consume(ctx context.Context, handler HandlerFunc) error {
	log := logger.Log

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := c.FetchOne(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Error().Err(err).Str("consumer", c.config.Consumer).Msg("fetch error")
			time.Sleep(time.Second)
			continue
		}

		if msg == nil {
			continue
		}

		if err := c.processMessage(ctx, msg, handler); err != nil {
			log.Error().Err(err).Str("consumer", c.config.Consumer).Msg("process error")
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg *Message, handler HandlerFunc) error {
	log := logger.Log

	meta, err := msg.Metadata()
	if err != nil {
		msg.Term()
		return fmt.Errorf("get metadata: %w", err)
	}

	if err := handler(ctx, msg); err != nil {
		if meta.NumDelivered >= uint64(c.config.MaxDeliver) {
			log.Error().
				Err(err).
				Str("consumer", c.config.Consumer).
				Uint64("attempts", meta.NumDelivered).
				Msg("max deliveries reached, terminating")
			msg.Term()
			return nil
		}

		log.Warn().
			Err(err).
			Str("consumer", c.config.Consumer).
			Uint64("attempt", meta.NumDelivered).
			Int("max", c.config.MaxDeliver).
			Msg("processing failed, will retry")
		msg.Nak()
		return nil
	}

	return msg.Ack()
}

func (c *Consumer) ConsumePool(ctx context.Context, workers int, handler HandlerFunc) error {
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			errCh <- c.Consume(ctx, handler)
		}()
	}

	return <-errCh
}
