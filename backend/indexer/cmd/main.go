package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/swagger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/config"
	"github.com/video-analitics/indexer/internal/handler"
	"github.com/video-analitics/indexer/internal/middleware"
	indexerQueue "github.com/video-analitics/indexer/internal/queue"
	"github.com/video-analitics/indexer/internal/repo"
	"github.com/video-analitics/indexer/internal/scheduler"
	"github.com/video-analitics/indexer/internal/service"
	"github.com/video-analitics/indexer/internal/worker"

	_ "github.com/video-analitics/indexer/docs"
)

func main() {
	cfg := config.Load()
	logger.Init(logger.IsDev())
	log := logger.Log

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	mongoClient, err := mongo.Connect(connectCtx, options.Client().ApplyURI(cfg.MongoURL))
	connectCancel()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to MongoDB")
	}
	defer mongoClient.Disconnect(context.Background())

	db := mongoClient.Database(cfg.MongoDB)

	// NATS JetStream connection
	natsClient, err := nats.New(cfg.NatsURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer natsClient.Close()

	// Meilisearch клиент (обязателен)
	meiliClient, err := meili.New(cfg.MeiliURL, cfg.MeiliKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Meilisearch")
	}
	log.Info().Str("url", cfg.MeiliURL).Msg("meilisearch connected")

	// Violations service (централизованное управление нарушениями)
	violationsSvc := violations.NewService(db, meiliClient)

	// Repos - чистые, без зависимости от violations
	siteRepo := repo.NewSiteRepo(db)
	pageRepo := repo.NewPageRepo(db)
	taskRepo := repo.NewScanTaskRepo(db)
	contentRepo := repo.NewContentRepo(db)
	sitemapURLRepo := repo.NewSitemapURLRepo(db)
	userRepo := repo.NewUserRepo(db)
	refreshTokenRepo := repo.NewRefreshTokenRepo(db)
	userSiteRepo := repo.NewUserSiteRepo(db)

	// Seed admin user if configured
	if cfg.AdminPassword != "" {
		if err := userRepo.SeedAdmin(context.Background(), cfg.AdminLogin, cfg.AdminPassword); err != nil {
			log.Error().Err(err).Msg("failed to seed admin user")
		} else {
			log.Info().Str("login", cfg.AdminLogin).Msg("admin user ready")
		}
	}

	// Подключаем contentRepo к violations service для обновления кэша счётчиков
	violationsSvc.SetContentUpdater(contentRepo)
	publisher := indexerQueue.NewPublisher(natsClient)

	// TaskProgressService - единая точка управления прогрессом задач
	progressSvc := service.NewTaskProgressService(taskRepo, sitemapURLRepo)

	// Handlers - получают violationsSvc для работы с нарушениями
	siteHandler := handler.NewSiteHandler(siteRepo, pageRepo, taskRepo, sitemapURLRepo, userSiteRepo, publisher, violationsSvc)
	scanHandler := handler.NewScanHandler(siteRepo, taskRepo, sitemapURLRepo, userSiteRepo, publisher)
	pageHandler := handler.NewPageHandler(pageRepo, violationsSvc)
	taskHandler := handler.NewTaskHandler(taskRepo, db)
	contentHandler := handler.NewContentHandler(contentRepo, siteRepo, violationsSvc)
	sitemapURLHandler := handler.NewSitemapURLHandler(sitemapURLRepo)
	authHandler := handler.NewAuthHandler(userRepo, refreshTokenRepo, cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
	userHandler := handler.NewUserHandler(userRepo)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Error().Err(err).Str("path", c.Path()).Msg("request error")
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(cors.New())

	api := app.Group("/api")

	// Public auth routes (no authentication required)
	api.Post("/auth/login", authHandler.Login)
	api.Post("/auth/refresh", authHandler.Refresh)

	// Internal API routes (for parser, protected by internal token)
	internal := api.Group("/internal", middleware.InternalAuth(cfg.InternalAPIToken))
	internal.Get("/sites/:id/pending-urls", sitemapURLHandler.GetPending)

	// Protected auth routes
	authGroup := api.Group("/auth", middleware.AuthMiddleware(cfg.JWTSecret))
	authGroup.Post("/logout", authHandler.Logout)
	authGroup.Get("/me", authHandler.Me)

	// Admin-only user management routes
	usersGroup := api.Group("/users", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly())
	usersGroup.Get("/", userHandler.List)
	usersGroup.Post("/", userHandler.Create)
	usersGroup.Put("/:id", userHandler.Update)
	usersGroup.Delete("/:id", userHandler.Delete)

	// Protected API routes (require authentication)
	protected := api.Group("", middleware.AuthMiddleware(cfg.JWTSecret))
	protected.Post("/sites", siteHandler.Create)
	protected.Post("/sites/batch", siteHandler.CreateBatch)
	protected.Get("/sites", siteHandler.List)
	protected.Get("/sites/:id", siteHandler.Get)
	protected.Get("/sites/:id/violations", siteHandler.GetViolations)
	protected.Post("/sites/:id/unfreeze", siteHandler.Unfreeze)
	protected.Post("/sites/:id/analyze", siteHandler.Analyze)
	protected.Post("/sites/:id/scan-sitemap", siteHandler.ScanSitemap)
	protected.Post("/sites/:id/scan-pages", siteHandler.ScanPages)
	protected.Get("/sites/:id/sitemap-urls", sitemapURLHandler.List)
	protected.Get("/sites/:id/sitemap-urls/stats", sitemapURLHandler.Stats)
	protected.Get("/sites/:id/pending-urls", sitemapURLHandler.GetPending)
	protected.Get("/sites/:id/all-urls", sitemapURLHandler.GetAllURLs)
	protected.Delete("/sites/:id", siteHandler.Delete)
	protected.Post("/sites/scan", scanHandler.StartScan)
	protected.Post("/sites/delete", siteHandler.DeleteBulk)
	protected.Get("/pages", pageHandler.List)
	protected.Get("/pages/stats", pageHandler.Stats)
	protected.Get("/scan-tasks", taskHandler.List)
	protected.Get("/scan-tasks/:id", taskHandler.Get)
	protected.Post("/scan-tasks/cancel", taskHandler.Cancel)
	protected.Post("/content", contentHandler.Create)
	protected.Post("/content/batch", contentHandler.CreateBatch)
	protected.Get("/content", contentHandler.List)
	protected.Post("/content/check-violations", contentHandler.CheckViolations)
	protected.Post("/content/delete", contentHandler.DeleteBulk)
	protected.Get("/content/:id", contentHandler.Get)
	protected.Get("/content/:id/violations", contentHandler.GetViolations)
	protected.Get("/content/:id/violations/export", contentHandler.ExportViolationsCSV)
	protected.Delete("/content/:id", contentHandler.Delete)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/swagger/*", swagger.HandlerDefault)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler (с violationsSvc для периодического обновления нарушений)
	sched, err := scheduler.New(siteRepo, taskRepo, contentRepo, publisher, violationsSvc)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create scheduler")
	}
	if err := sched.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start scheduler")
	}
	defer sched.Stop()

	// Start result processor (NATS consumer)
	resultProcessor := worker.NewResultProcessor(natsClient, siteRepo, taskRepo, contentRepo, sitemapURLRepo, violationsSvc)
	go func() {
		if err := resultProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("result processor error")
		}
	}()

	// Start detect result processor (NATS consumer)
	detectProcessor := worker.NewDetectProcessor(natsClient, siteRepo, taskRepo, publisher)
	go func() {
		if err := detectProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("detect processor error")
		}
	}()

	// Start progress processor (NATS consumer)
	progressProcessor := worker.NewProgressProcessor(natsClient, taskRepo)
	go func() {
		if err := progressProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("progress processor error")
		}
	}()

	// Start DLQ handler (logs failed messages)
	dlqHandler := nats.NewDLQHandler(natsClient)
	go func() {
		if err := dlqHandler.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("DLQ handler error")
		}
	}()

	// Start sitemap batch processor (saves URL batches from sitemap crawl)
	sitemapBatchProcessor := worker.NewSitemapBatchProcessor(natsClient, sitemapURLRepo)
	go func() {
		if err := sitemapBatchProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("sitemap batch processor error")
		}
	}()

	// Start sitemap result processor (creates PageCrawlTask after sitemap crawl)
	sitemapResultProcessor := worker.NewSitemapResultProcessor(natsClient, siteRepo, progressSvc, publisher)
	go func() {
		if err := sitemapResultProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("sitemap result processor error")
		}
	}()

	// Start page single processor (saves parsed pages and updates sitemap_urls status immediately)
	pageSingleProcessor := worker.NewPageSingleProcessor(natsClient, siteRepo, pageRepo, sitemapURLRepo, progressSvc, meiliClient)
	go func() {
		if err := pageSingleProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("page single processor error")
		}
	}()

	// Start page result processor (finalizes page crawl task)
	pageResultProcessor := worker.NewPageResultProcessor(natsClient, siteRepo, progressSvc, contentRepo, violationsSvc)
	go func() {
		if err := pageResultProcessor.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("page result processor error")
		}
	}()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutting down")
		cancel()
		app.Shutdown()
	}()

	log.Info().Str("port", cfg.Port).Str("nats", cfg.NatsURL).Msg("indexer started")
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
