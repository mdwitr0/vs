package worker

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/detector"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/parser/internal/browser"
	"github.com/video-analitics/parser/internal/crawler"
)

type DetectWorker struct {
	natsClient      *nats.Client
	publisher       *nats.Publisher
	cmsDetector     *detector.CMSDetector
	renderDetector  *detector.RenderDetector
	captchaDetector *detector.CaptchaDetector
}

func NewDetectWorker(natsClient *nats.Client) *DetectWorker {
	return &DetectWorker{
		natsClient:      natsClient,
		publisher:       nats.NewPublisher(natsClient),
		cmsDetector:     detector.NewCMSDetector(),
		renderDetector:  detector.NewRenderDetector(),
		captchaDetector: detector.NewCaptchaDetector(),
	}
}

func (w *DetectWorker) Run(ctx context.Context) error {
	log := logger.Log

	consumer, err := nats.NewConsumer(w.natsClient, nats.ConsumerConfig{
		Stream:   nats.StreamDetectTasks,
		Consumer: "detect-worker",
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Msg("detect worker started")

	return consumer.Consume(ctx, func(ctx context.Context, msg *nats.Message) error {
		var task queue.DetectTask
		if err := msg.Unmarshal(&task); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal detect task")
			return err
		}

		w.processTask(ctx, &task)
		return nil
	})
}

func (w *DetectWorker) processTask(ctx context.Context, task *queue.DetectTask) {
	log := logger.Log

	log.Info().Str("site", task.SiteID).Str("domain", task.Domain).Msg("detection started")

	result := queue.DetectResultMsg{
		TaskID:     task.ID,
		SiteID:     task.SiteID,
		FinishedAt: time.Now(),
	}

	baseURL := "https://" + task.Domain

	// Fetch homepage via global browser
	fetchResult, err := browser.Get().FetchPage(ctx, baseURL)
	if err != nil {
		log.Error().Err(err).Str("domain", task.Domain).Msg("detection fetch failed")
		result.Success = false
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		w.sendResult(ctx, &result)
		return
	}

	if fetchResult.Blocked {
		// If blocked but we got cookies (captcha solved), continue with detection
		if len(fetchResult.Cookies) == 0 {
			log.Warn().Str("domain", task.Domain).Str("reason", fetchResult.BlockReason).Msg("detection blocked")
			result.Success = false
			result.Error = "blocked: " + fetchResult.BlockReason
			result.FinishedAt = time.Now()
			w.sendResult(ctx, &result)
			return
		}
	}

	// Check for domain redirect FIRST
	if fetchResult.FinalURL != "" && fetchResult.FinalURL != baseURL {
		isDomainRedirect, targetDomain := checkDomainRedirect(baseURL, fetchResult.FinalURL)
		if isDomainRedirect {
			log.Info().
				Str("site", task.SiteID).
				Str("from", task.Domain).
				Str("to", targetDomain).
				Msg("domain redirect detected")

			result.Success = true
			result.HasDomainRedirect = true
			result.RedirectToDomain = targetDomain
			result.FinishedAt = time.Now()
			w.sendResult(ctx, &result)
			return
		}
	}

	html := fetchResult.HTML

	// Detect CMS
	cmsResult := w.cmsDetector.Detect(html, make(map[string]string))
	result.CMS = string(cmsResult.CMS)
	result.CMSVersion = cmsResult.Version

	// Detect render type (SPA vs SSR)
	renderResult := w.renderDetector.Detect(html, int64(len(html)))
	result.NeedsSPA = renderResult.NeedsBrowser

	// Detect captcha type
	captchaResult := w.captchaDetector.Detect(html, make(map[string]string))
	result.CaptchaType = string(captchaResult.Type)

	// If we got pirate captcha during fetch, mark it
	if fetchResult.IsCaptcha {
		result.CaptchaType = "pirate"
	}

	// Check for sitemap and determine crawl strategy
	sitemapResult := w.detectSitemapsWithValidation(ctx, task.Domain)
	result.HasSitemap = sitemapResult.HasSitemap
	result.SitemapStatus = string(sitemapResult.SitemapStatus)
	result.CrawlStrategy = string(sitemapResult.CrawlStrategy)
	result.SitemapURLs = sitemapResult.SitemapURLs

	// Add cookies from fetch
	for _, c := range fetchResult.Cookies {
		result.Cookies = append(result.Cookies, queue.CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	result.Success = true
	result.FinishedAt = time.Now()

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Str("cms", result.CMS).
		Bool("sitemap", result.HasSitemap).
		Str("sitemap_status", result.SitemapStatus).
		Str("crawl_strategy", result.CrawlStrategy).
		Bool("needs_spa", result.NeedsSPA).
		Str("captcha", result.CaptchaType).
		Msg("detection completed")

	w.sendResult(ctx, &result)
}

type sitemapDetectionResult struct {
	HasSitemap    bool
	SitemapStatus detector.SitemapStatus
	CrawlStrategy detector.CrawlStrategy
	SitemapURLs   []string
}

func (w *DetectWorker) detectSitemapsWithValidation(ctx context.Context, domain string) sitemapDetectionResult {
	log := logger.Log
	result := sitemapDetectionResult{
		SitemapStatus: detector.SitemapNone,
		CrawlStrategy: detector.CrawlStrategySitemap,
	}

	baseURL := "https://" + domain

	// 1. Check robots.txt for sitemap references
	robotsSitemaps := w.checkRobotsTxt(ctx, baseURL)
	log.Debug().Strs("robots_sitemaps", robotsSitemaps).Str("domain", domain).Msg("robots.txt check")

	// 2. Standard sitemap paths (fallback if nothing in robots.txt)
	standardPaths := []string{
		"/sitemap.xml",
		"/sitemap_index.xml",
		"/sitemap-index.xml",
		"/wp-sitemap.xml", // WordPress 5.5+
		"/sitemap.txt",
		"/post-sitemap.xml", // Yoast SEO
	}

	// Build candidates list: robots.txt sitemaps first, then standard paths
	var allCandidates []string
	seen := make(map[string]bool)

	for _, u := range robotsSitemaps {
		if !seen[u] {
			allCandidates = append(allCandidates, u)
			seen[u] = true
		}
	}
	for _, path := range standardPaths {
		u := baseURL + path
		if !seen[u] {
			allCandidates = append(allCandidates, u)
			seen[u] = true
		}
	}

	// 3. Validate each candidate using the real sitemap parser
	for _, sitemapURL := range allCandidates {
		fetchResult, err := browser.Get().FetchSitemap(ctx, sitemapURL)
		if err != nil {
			log.Debug().Err(err).Str("url", sitemapURL).Msg("sitemap fetch failed")
			continue
		}

		if fetchResult.Blocked {
			log.Debug().Str("url", sitemapURL).Str("reason", fetchResult.BlockReason).Msg("sitemap blocked")
			continue
		}

		// Use the real sitemap parser (supports XML, JSON, HTML, plain text)
		parsed, err := crawler.ParseSitemapXML(fetchResult.HTML, sitemapURL)
		if err != nil {
			log.Debug().Err(err).Str("url", sitemapURL).Msg("sitemap parse failed")
			if result.SitemapStatus == detector.SitemapNone {
				result.SitemapStatus = detector.SitemapInvalid
			}
			continue
		}

		urlsCount := len(parsed.PageURLs) + len(parsed.NestedSitemaps)
		log.Debug().
			Str("url", sitemapURL).
			Int("page_urls", len(parsed.PageURLs)).
			Int("nested_sitemaps", len(parsed.NestedSitemaps)).
			Msg("sitemap validated")

		result.HasSitemap = true
		result.SitemapURLs = append(result.SitemapURLs, sitemapURL)

		if urlsCount > 0 {
			result.SitemapStatus = detector.SitemapValid
			result.CrawlStrategy = detector.CrawlStrategySitemap
			log.Info().
				Str("url", sitemapURL).
				Int("page_urls", len(parsed.PageURLs)).
				Int("nested_sitemaps", len(parsed.NestedSitemaps)).
				Msg("valid sitemap found")
			break
		} else {
			result.SitemapStatus = detector.SitemapEmpty
		}
	}

	return result
}

func (w *DetectWorker) checkRobotsTxt(ctx context.Context, baseURL string) []string {
	var sitemapURLs []string

	robotsURL := baseURL + "/robots.txt"
	fetchResult, err := browser.Get().FetchPage(ctx, robotsURL)
	if err != nil || fetchResult.Blocked {
		return sitemapURLs
	}

	// FetchPage returns HTML wrapper, use regex to find Sitemap: directives
	// Works for both raw text and HTML-wrapped content
	// Supports .xml and .txt sitemaps
	sitemapRegex := regexp.MustCompile(`(?i)sitemap:\s*(https?://[^\s<>"\n\r]+\.(xml|txt))`)
	matches := sitemapRegex.FindAllStringSubmatch(fetchResult.HTML, -1)

	for _, match := range matches {
		if len(match) > 1 {
			sitemapURLs = append(sitemapURLs, strings.TrimSpace(match[1]))
		}
	}

	return sitemapURLs
}

func (w *DetectWorker) sendResult(ctx context.Context, result *queue.DetectResultMsg) {
	if err := w.publisher.PublishDetectResult(ctx, result); err != nil {
		logger.Log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to send detect result")
	}
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	return host
}

func checkDomainRedirect(originalURL, finalURL string) (bool, string) {
	originalDomain := extractDomain(originalURL)
	finalDomain := extractDomain(finalURL)

	if originalDomain == "" || finalDomain == "" {
		return false, ""
	}

	if originalDomain == finalDomain {
		return false, ""
	}

	return true, finalDomain
}
