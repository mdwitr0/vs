package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/captcha"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/parser/internal/browser"
	"github.com/video-analitics/parser/internal/crawler"
)

type Worker struct {
	natsClient *nats.Client
	publisher  *nats.Publisher
}

func New(natsClient *nats.Client) *Worker {
	return &Worker{
		natsClient: natsClient,
		publisher:  nats.NewPublisher(natsClient),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	return w.RunPool(ctx, 1)
}

func (w *Worker) RunPool(ctx context.Context, workerCount int) error {
	log := logger.Log

	if workerCount < 1 {
		workerCount = 1
	}

	consumer, err := nats.NewConsumer(w.natsClient, nats.ConsumerConfig{
		Stream:        nats.StreamCrawlTasks,
		Consumer:      "crawl-worker",
		MaxAckPending: workerCount * 2,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Int("workers", workerCount).Msg("starting worker pool")

	return consumer.ConsumePool(ctx, workerCount, func(ctx context.Context, msg *nats.Message) error {
		var task queue.CrawlTask
		if err := msg.Unmarshal(&task); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal crawl task")
			return err
		}

		w.processTask(ctx, &task)
		return nil
	})
}

func (w *Worker) processTask(ctx context.Context, task *queue.CrawlTask) {
	log := logger.Log

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Msg("crawl started")

	startProgress := queue.CrawlProgress{
		TaskID:     task.ID,
		PagesFound: 0,
		PagesSaved: 0,
	}
	if err := w.publisher.PublishCrawlProgress(ctx, &startProgress); err != nil {
		log.Debug().Err(err).Msg("failed to send start progress")
	}

	result := queue.CrawlResult{
		TaskID:        task.ID,
		SiteID:        task.SiteID,
		ScanIntervalH: task.ScanIntervalH,
		FinishedAt:    time.Now(),
	}

	cookies := w.convertTaskCookies(task.Cookies)
	if len(cookies) > 0 {
		if err := browser.Get().SetCookies(cookies); err != nil {
			log.Warn().Err(err).Msg("failed to set cookies")
		}
	}

	// Collect URLs from sitemaps
	urlSet := make(map[string]bool)
	var sitemapStats []queue.SitemapStat
	var allParsedURLs []queue.ParsedURLData
	var newCookies []captcha.Cookie

	if task.HasSitemap && len(task.SitemapURLs) > 0 {
		visited := make(map[string]bool)
		for _, sitemapURL := range task.SitemapURLs {
			pageURLs, stats, cookies := w.parseSitemapRecursive(ctx, sitemapURL, 0, visited, sitemapURL)
			sitemapStats = append(sitemapStats, stats...)
			if len(cookies) > 0 {
				newCookies = cookies
			}
			for _, u := range pageURLs {
				if !urlSet[u] {
					urlSet[u] = true
					allParsedURLs = append(allParsedURLs, queue.ParsedURLData{
						URL:    u,
						Source: sitemapURL,
					})
				}
			}
		}
	}

	urls := make([]string, 0, len(urlSet))
	for u := range urlSet {
		urls = append(urls, u)
	}

	// Fallback to homepage if no sitemap URLs
	if len(urls) == 0 {
		homepageURLs := w.crawlFromHomepage(ctx, task.Domain)
		for _, u := range homepageURLs {
			if !urlSet[u] {
				urlSet[u] = true
				allParsedURLs = append(allParsedURLs, queue.ParsedURLData{
					URL:    u,
					Source: "https://" + task.Domain,
					Depth:  1,
				})
			}
		}
		urls = homepageURLs
	}

	totalFound := len(urls)

	log.Info().
		Str("domain", task.Domain).
		Int("urls_found", totalFound).
		Msg("sitemap crawl completed, URLs sent to indexer")

	result.PagesFound = totalFound
	result.PagesSaved = 0
	result.NewCookies = w.convertCaptchaCookies(newCookies)
	result.ParsedURLs = allParsedURLs
	result.SitemapStats = sitemapStats
	result.Success = totalFound > 0 || len(allParsedURLs) == 0
	result.FinishedAt = time.Now()

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Int("urls", totalFound).
		Msg("crawl completed")

	w.sendResult(ctx, &result)
}

func (w *Worker) crawlFromHomepage(ctx context.Context, domain string) []string {
	log := logger.Log
	baseURL := "https://" + domain

	result, err := browser.Get().FetchPage(ctx, baseURL)
	if err != nil {
		log.Warn().Err(err).Str("domain", domain).Msg("homepage fetch failed")
		return nil
	}

	if result.Blocked {
		log.Warn().Str("domain", domain).Str("reason", result.BlockReason).Msg("homepage blocked")
		return nil
	}

	return crawler.ExtractLinksFromHTML(result.HTML, baseURL)
}

func (w *Worker) convertTaskCookies(taskCookies []queue.CookieData) []captcha.Cookie {
	cookies := make([]captcha.Cookie, len(taskCookies))
	for i, tc := range taskCookies {
		cookies[i] = captcha.Cookie{
			Name:     tc.Name,
			Value:    tc.Value,
			Domain:   tc.Domain,
			Path:     tc.Path,
			HTTPOnly: tc.HTTPOnly,
			Secure:   tc.Secure,
		}
	}
	return cookies
}

func (w *Worker) convertCaptchaCookies(cookies []captcha.Cookie) []queue.CookieData {
	result := make([]queue.CookieData, len(cookies))
	for i, c := range cookies {
		result[i] = queue.CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}
	return result
}

func isDomainExpiredError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "nxdomain") ||
		strings.Contains(errStr, "dns") ||
		strings.Contains(errStr, "domain expired") ||
		strings.Contains(errStr, "domain for sale")
}

const workerMaxSitemapDepth = 5

func (w *Worker) parseSitemapRecursive(ctx context.Context, sitemapURL string, depth int, visited map[string]bool, rootSource string) ([]string, []queue.SitemapStat, []captcha.Cookie) {
	log := logger.Log
	var stats []queue.SitemapStat
	var pageURLs []string
	var cookies []captcha.Cookie

	if depth > workerMaxSitemapDepth {
		log.Warn().Str("url", sitemapURL).Int("depth", depth).Msg("max sitemap depth exceeded")
		return nil, nil, nil
	}

	if visited[sitemapURL] {
		return nil, nil, nil
	}
	visited[sitemapURL] = true

	stat := queue.SitemapStat{URL: sitemapURL}

	fetchResult, err := browser.Get().FetchSitemap(ctx, sitemapURL)
	if err != nil {
		log.Warn().Err(err).Str("sitemap", sitemapURL).Msg("sitemap fetch failed")
		stat.Error = err.Error()
		return nil, []queue.SitemapStat{stat}, nil
	}

	if fetchResult.Blocked {
		log.Warn().Str("sitemap", sitemapURL).Str("reason", fetchResult.BlockReason).Msg("sitemap blocked")
		stat.Error = fetchResult.BlockReason
		return nil, []queue.SitemapStat{stat}, nil
	}

	if len(fetchResult.Cookies) > 0 {
		cookies = fetchResult.Cookies
	}

	parsed, err := crawler.ParseSitemapXML(fetchResult.HTML, sitemapURL)
	if err != nil {
		log.Warn().Err(err).Str("sitemap", sitemapURL).Msg("sitemap parse failed")
		stat.Error = err.Error()
		return nil, []queue.SitemapStat{stat}, cookies
	}

	// Process nested sitemaps recursively
	if len(parsed.NestedSitemaps) > 0 {
		log.Info().
			Str("url", sitemapURL).
			Int("nested_count", len(parsed.NestedSitemaps)).
			Int("depth", depth).
			Msg("processing nested sitemaps")

		for _, nestedURL := range parsed.NestedSitemaps {
			nestedPages, nestedStats, nestedCookies := w.parseSitemapRecursive(ctx, nestedURL, depth+1, visited, rootSource)
			pageURLs = append(pageURLs, nestedPages...)
			stats = append(stats, nestedStats...)
			if len(nestedCookies) > 0 {
				cookies = nestedCookies
			}
		}
	}

	pageURLs = append(pageURLs, parsed.PageURLs...)
	stat.URLsFound = len(parsed.PageURLs)
	stats = append(stats, stat)

	log.Debug().
		Str("url", sitemapURL).
		Int("depth", depth).
		Int("nested_sitemaps", len(parsed.NestedSitemaps)).
		Int("page_urls", len(parsed.PageURLs)).
		Int("total_collected", len(pageURLs)).
		Msg("sitemap parsed")

	return pageURLs, stats, cookies
}

func (w *Worker) sendResult(ctx context.Context, result *queue.CrawlResult) {
	if err := w.publisher.PublishCrawlResult(ctx, result); err != nil {
		logger.Log.Warn().Err(err).Str("task", result.TaskID).Msg("failed to send result")
	}
}
