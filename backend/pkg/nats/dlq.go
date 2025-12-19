package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/video-analitics/backend/pkg/logger"
)

type DLQHandler struct {
	nc *nats.Conn
}

func NewDLQHandler(client *Client) *DLQHandler {
	return &DLQHandler{nc: client.nc}
}

type AdvisoryMaxDeliveries struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Stream     string `json:"stream"`
	Consumer   string `json:"consumer"`
	StreamSeq  uint64 `json:"stream_seq"`
	Deliveries int    `json:"deliveries"`
}

func (h *DLQHandler) Run(ctx context.Context) error {
	log := logger.Log

	sub, err := h.nc.Subscribe("$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.>", func(msg *nats.Msg) {
		var advisory AdvisoryMaxDeliveries
		if err := json.Unmarshal(msg.Data, &advisory); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal DLQ advisory")
			return
		}

		log.Error().
			Str("stream", advisory.Stream).
			Str("consumer", advisory.Consumer).
			Uint64("seq", advisory.StreamSeq).
			Int("deliveries", advisory.Deliveries).
			Time("time", time.Now()).
			Msg("DLQ: message reached max deliveries")
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	log.Info().Msg("DLQ handler started")

	<-ctx.Done()
	return nil
}
