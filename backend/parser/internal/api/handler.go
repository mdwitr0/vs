package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/parser/internal/browser"
)

type FetchRequest struct {
	URL string `json:"url" query:"url"`
}

type FetchResponse struct {
	URL         string   `json:"url"`
	FinalURL    string   `json:"final_url"`
	HTML        string   `json:"html"`
	HTMLLength  int      `json:"html_length"`
	Blocked     bool     `json:"blocked"`
	IsCaptcha   bool     `json:"is_captcha,omitempty"`
	BlockReason string   `json:"block_reason,omitempty"`
	Cookies     []Cookie `json:"cookies,omitempty"`
	FetchTimeMs int64    `json:"fetch_time_ms"`
}

type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	HTTPOnly bool   `json:"http_only"`
	Secure   bool   `json:"secure"`
}

func SetupRoutes(app *fiber.App) {
	app.Get("/api/fetch", handleFetch)
	app.Post("/api/fetch", handleFetch)
	app.Get("/health", handleHealth)
}

func handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"browser": browser.IsInitialized(),
	})
}

func handleFetch(c *fiber.Ctx) error {
	log := logger.Log

	var req FetchRequest
	if c.Method() == "POST" {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
	} else {
		req.URL = c.Query("url")
	}

	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url is required"})
	}

	log.Info().Str("url", req.URL).Msg("fetch request received")

	ctx, cancel := context.WithTimeout(c.Context(), 90*time.Second)
	defer cancel()

	start := time.Now()
	result, err := browser.Get().FetchPage(ctx, req.URL)
	elapsed := time.Since(start)

	if err != nil {
		log.Error().Err(err).Str("url", req.URL).Msg("fetch failed")
		return c.Status(500).JSON(fiber.Map{
			"error":        err.Error(),
			"url":          req.URL,
			"fetch_time_ms": elapsed.Milliseconds(),
		})
	}

	var cookies []Cookie
	for _, c := range result.Cookies {
		cookies = append(cookies, Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	resp := FetchResponse{
		URL:         req.URL,
		FinalURL:    result.FinalURL,
		HTML:        result.HTML,
		HTMLLength:  len(result.HTML),
		Blocked:     result.Blocked,
		IsCaptcha:   result.IsCaptcha,
		BlockReason: result.BlockReason,
		Cookies:     cookies,
		FetchTimeMs: elapsed.Milliseconds(),
	}

	log.Info().
		Str("url", req.URL).
		Int("html_len", len(result.HTML)).
		Bool("blocked", result.Blocked).
		Int64("time_ms", elapsed.Milliseconds()).
		Msg("fetch completed")

	return c.JSON(resp)
}
