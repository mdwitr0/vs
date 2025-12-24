package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/player-monitor/parser/internal/browser"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type FetchResponse struct {
	URL         string `json:"url"`
	FinalURL    string `json:"final_url"`
	HTML        string `json:"html"`
	HTMLLength  int    `json:"html_length"`
	Blocked     bool   `json:"blocked"`
	FetchTimeMs int64  `json:"fetch_time_ms"`
	Error       string `json:"error,omitempty"`
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	port := getEnv("PORT", "8082")
	maxTabs := getEnvInt("MAX_TABS", 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Info().Msg("initializing browser...")
	if err := browser.Init(ctx, maxTabs); err != nil {
		log.Fatal().Err(err).Msg("failed to init browser")
	}
	defer browser.Close()

	app := fiber.New(fiber.Config{
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", handleHealth)
	app.Get("/api/fetch", handleFetch)
	app.Post("/api/fetch", handleFetch)

	go func() {
		log.Info().Str("port", port).Msg("starting parser server")
		if err := app.Listen(":" + port); err != nil {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("shutting down...")
	app.Shutdown()
}

func handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"browser": browser.IsInitialized(),
	})
}

func handleFetch(c *fiber.Ctx) error {
	url := c.Query("url")
	if url == "" {
		var body struct {
			URL string `json:"url"`
		}
		if err := c.BodyParser(&body); err == nil {
			url = body.URL
		}
	}

	if url == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url is required"})
	}

	log.Info().Str("url", url).Msg("fetch request")

	ctx, cancel := context.WithTimeout(c.Context(), 90*time.Second)
	defer cancel()

	start := time.Now()
	result, err := browser.Get().FetchPage(ctx, url)
	elapsed := time.Since(start)

	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("fetch failed")
		return c.JSON(FetchResponse{
			URL:         url,
			Error:       err.Error(),
			FetchTimeMs: elapsed.Milliseconds(),
		})
	}

	log.Info().
		Str("url", url).
		Int("html_len", len(result.HTML)).
		Int64("time_ms", elapsed.Milliseconds()).
		Msg("fetch completed")

	return c.JSON(FetchResponse{
		URL:         url,
		FinalURL:    result.FinalURL,
		HTML:        result.HTML,
		HTMLLength:  len(result.HTML),
		Blocked:     result.Blocked,
		FetchTimeMs: elapsed.Milliseconds(),
	})
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
