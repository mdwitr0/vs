package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/backend/pkg/captcha"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/parser/internal/api"
	"github.com/video-analitics/parser/internal/browser"
	"github.com/video-analitics/parser/internal/config"
	"github.com/video-analitics/parser/internal/worker"
)

func main() {
	cfg := config.Load()
	logger.Init(logger.IsDev())
	log := logger.Log

	natsClient, err := nats.New(cfg.NatsURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer natsClient.Close()

	// Initialize global browser
	solver := captcha.NewPirateSolver()
	if err := browser.Init(context.Background(), solver, cfg.PageLoadDelay, cfg.MaxBrowserTabs); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize browser")
	}
	defer browser.Close()
	log.Info().Int("max_tabs", cfg.MaxBrowserTabs).Msg("global browser initialized")

	// Start HTTP API server
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             10 * 1024 * 1024,
	})
	api.SetupRoutes(app)
	go func() {
		addr := ":" + cfg.HTTPPort
		log.Info().Str("addr", addr).Msg("HTTP API server starting")
		if err := app.Listen(addr); err != nil {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	crawlWorker := worker.New(natsClient)
	detectWorker := worker.NewDetectWorker(natsClient)
	sitemapWorker := worker.NewSitemapWorker(natsClient)
	pageWorker := worker.NewPageWorker(natsClient, cfg.InternalAPIToken)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutting down")
		cancel()
	}()

	log.Info().
		Str("nats", cfg.NatsURL).
		Int("workers", cfg.WorkerCount).
		Msg("parser started")

	go func() {
		if err := detectWorker.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("detect worker error")
		}
	}()

	go func() {
		if err := sitemapWorker.RunPool(ctx, 2); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("sitemap worker error")
		}
	}()

	go func() {
		if err := pageWorker.RunPool(ctx, cfg.WorkerCount); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("page worker error")
		}
	}()

	if err := crawlWorker.RunPool(ctx, cfg.WorkerCount); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("crawl worker error")
	}
}
