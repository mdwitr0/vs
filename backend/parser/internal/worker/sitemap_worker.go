package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/video-analitics/backend/pkg/captcha"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/parser/internal/browser"
	"github.com/video-analitics/parser/internal/crawler"
)

const (
	urlBatchSize        = 1000
	inactivityTimeout   = 5 * time.Minute  // таймаут на неактивность (нет прогресса)
	maxSitemapTaskTime  = 30 * time.Minute // максимальное время на задачу (hard limit)
)

// sitemapBlacklistRe matches sitemap URLs we want to skip (character, people, author, tag, category)
var sitemapBlacklistRe = regexp.MustCompile(`(?i)(character|people|author|tag|category).*\.xml`)

// shouldSkipSitemap checks if a nested sitemap URL should be skipped
func shouldSkipSitemap(sitemapURL string) bool {
	return sitemapBlacklistRe.MatchString(sitemapURL)
}

type SitemapWorker struct {
	natsClient *nats.Client
	publisher  *nats.Publisher
}

// inactivityContext creates a context that cancels after inactivity period
// Call keepAlive() to reset the timer on each progress
type inactivityContext struct {
	ctx       context.Context
	cancel    context.CancelFunc
	timer     *time.Timer
	timeout   time.Duration
	cancelled bool
	mu        sync.Mutex
}

func newInactivityContext(parent context.Context, inactivity, hardLimit time.Duration) *inactivityContext {
	ctx, cancel := context.WithTimeout(parent, hardLimit)
	ic := &inactivityContext{
		ctx:     ctx,
		cancel:  cancel,
		timeout: inactivity,
	}
	ic.timer = time.AfterFunc(inactivity, func() {
		ic.mu.Lock()
		ic.cancelled = true
		ic.mu.Unlock()
		cancel()
	})
	return ic
}

func (ic *inactivityContext) keepAlive() {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if !ic.cancelled {
		ic.timer.Reset(ic.timeout)
	}
}

func (ic *inactivityContext) stop() {
	ic.timer.Stop()
}

func (ic *inactivityContext) Done() <-chan struct{} {
	return ic.ctx.Done()
}

func (ic *inactivityContext) Err() error {
	return ic.ctx.Err()
}

func NewSitemapWorker(natsClient *nats.Client) *SitemapWorker {
	return &SitemapWorker{
		natsClient: natsClient,
		publisher:  nats.NewPublisher(natsClient),
	}
}

func (w *SitemapWorker) Run(ctx context.Context) error {
	return w.RunPool(ctx, 1)
}

func (w *SitemapWorker) RunPool(ctx context.Context, workerCount int) error {
	log := logger.Log

	if workerCount < 1 {
		workerCount = 1
	}

	consumer, err := nats.NewConsumer(w.natsClient, nats.ConsumerConfig{
		Stream:        nats.StreamSitemapCrawlTasks,
		Consumer:      "sitemap-worker",
		MaxAckPending: workerCount * 2,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Int("workers", workerCount).Msg("sitemap worker pool started")

	return consumer.ConsumePool(ctx, workerCount, func(ctx context.Context, msg *nats.Message) error {
		var task queue.SitemapCrawlTask
		if err := msg.Unmarshal(&task); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal sitemap crawl task")
			return err
		}

		w.processTask(ctx, &task)
		return nil
	})
}

func (w *SitemapWorker) processTask(ctx context.Context, task *queue.SitemapCrawlTask) {
	log := logger.Log

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Int("sitemaps", len(task.SitemapURLs)).
		Msg("sitemap crawl task received")

	ic := newInactivityContext(ctx, inactivityTimeout, maxSitemapTaskTime)
	defer ic.stop()

	// Если нет sitemap URLs, добавляем главную страницу как seed
	if len(task.SitemapURLs) == 0 {
		log.Info().Str("domain", task.Domain).Msg("no sitemap URLs, adding homepage as seed")
		w.publishHomepageAsURL(ctx, task)
		return
	}

	log.Info().Str("domain", task.Domain).Int("sitemaps", len(task.SitemapURLs)).Msg("starting sitemap crawl")
	w.processSitemapCrawl(ic, task)
}

// publishHomepageAsURL adds the homepage as a seed URL when no sitemap is available
func (w *SitemapWorker) publishHomepageAsURL(ctx context.Context, task *queue.SitemapCrawlTask) {
	log := logger.Log

	homepageURL := "https://" + task.Domain + "/"
	source := "homepage:" + task.Domain

	urlBatch := queue.SitemapURLBatch{
		TaskID:        task.ID,
		SiteID:        task.SiteID,
		URLs:          []queue.ParsedURLData{{URL: homepageURL, Source: source, Depth: 0}},
		BatchNumber:   1,
		SitemapSource: source,
	}

	if err := w.publisher.PublishSitemapURLBatch(ctx, urlBatch); err != nil {
		log.Error().Err(err).Str("domain", task.Domain).Msg("failed to publish homepage URL")
	} else {
		log.Info().Str("domain", task.Domain).Str("url", homepageURL).Msg("homepage URL published as seed")
	}

	// Publish result
	result := queue.SitemapCrawlResult{
		TaskID:       task.ID,
		SiteID:       task.SiteID,
		Success:      true,
		TotalURLs:    1,
		NewURLs:      1,
		AutoContinue: task.AutoContinue,
		SitemapStats: []queue.SitemapStat{{URL: source, URLsFound: 1}},
		FinishedAt:   time.Now(),
	}

	if err := w.publisher.PublishSitemapCrawlResult(ctx, result); err != nil {
		log.Error().Err(err).Str("task", task.ID).Msg("failed to publish sitemap result")
	}
}

func (w *SitemapWorker) processSitemapCrawl(ic *inactivityContext, task *queue.SitemapCrawlTask) {
	log := logger.Log
	ctx := ic.ctx

	result := queue.SitemapCrawlResult{
		TaskID:       task.ID,
		SiteID:       task.SiteID,
		AutoContinue: task.AutoContinue,
		FinishedAt:   time.Now(),
	}

	cookies := w.convertTaskCookies(task.Cookies)
	var newCookies []captcha.Cookie

	var totalURLs int32
	var batchNumber int32
	var sitemapStats []queue.SitemapStat
	var timedOut bool
	var mu sync.Mutex

	// Callback to publish URLs immediately as they're parsed from each sitemap
	onURLs := func(urls []crawler.ParsedURL, source string) {
		if len(urls) == 0 {
			return
		}

		atomic.AddInt32(&totalURLs, int32(len(urls)))

		// Publish in batches
		for i := 0; i < len(urls); i += urlBatchSize {
			end := i + urlBatchSize
			if end > len(urls) {
				end = len(urls)
			}

			batch := urls[i:end]
			num := atomic.AddInt32(&batchNumber, 1)

			urlBatch := queue.SitemapURLBatch{
				TaskID:        task.ID,
				SiteID:        task.SiteID,
				URLs:          w.convertURLsToQueue(batch, source),
				BatchNumber:   int(num),
				SitemapSource: source,
			}

			if err := w.publisher.PublishSitemapURLBatch(context.Background(), urlBatch); err != nil {
				log.Error().Err(err).Int("batch", int(num)).Msg("failed to publish url batch")
			} else {
				log.Debug().Int("batch", int(num)).Int("urls", len(batch)).Str("source", source).Msg("url batch published")
			}
		}
	}

	for _, sitemapURL := range task.SitemapURLs {
		if ctx.Err() != nil {
			timedOut = true
			log.Warn().Str("domain", task.Domain).Msg("sitemap crawl timed out")
			break
		}

		stat := queue.SitemapStat{URL: sitemapURL}
		urlsBeforeParse := atomic.LoadInt32(&totalURLs)

		parsedCookies, err := w.parseSitemapStreaming(ctx, sitemapURL, cookies, ic.keepAlive, onURLs)

		urlsAfterParse := atomic.LoadInt32(&totalURLs)
		stat.URLsFound = int(urlsAfterParse - urlsBeforeParse)

		if err != nil {
			log.Warn().Err(err).Str("sitemap", sitemapURL).Msg("sitemap parse failed")
			stat.Error = err.Error()
			mu.Lock()
			sitemapStats = append(sitemapStats, stat)
			mu.Unlock()
			if ctx.Err() != nil {
				timedOut = true
				break
			}
			continue
		}

		if len(parsedCookies) > 0 {
			cookies = parsedCookies
			newCookies = parsedCookies
		}

		mu.Lock()
		sitemapStats = append(sitemapStats, stat)
		mu.Unlock()
	}

	finalTotalURLs := int(atomic.LoadInt32(&totalURLs))
	result.Success = finalTotalURLs > 0
	result.TotalURLs = finalTotalURLs
	result.NewURLs = finalTotalURLs
	result.SitemapStats = sitemapStats
	result.NewCookies = w.convertCaptchaCookies(newCookies)
	result.FinishedAt = time.Now()

	if timedOut {
		if finalTotalURLs > 0 {
			result.Success = true // partial success - we collected some URLs before timeout
			result.Error = fmt.Sprintf("task timed out (inactivity: %v, max: %v), collected %d urls", inactivityTimeout, maxSitemapTaskTime, finalTotalURLs)
		} else {
			result.Success = false
			result.Error = fmt.Sprintf("task timed out (inactivity: %v, max: %v), no urls collected", inactivityTimeout, maxSitemapTaskTime)
		}
	} else if finalTotalURLs == 0 && len(task.SitemapURLs) > 0 {
		result.Success = false
		result.Error = "no urls found in any sitemap"
	}

	if err := w.publisher.PublishSitemapCrawlResult(context.Background(), result); err != nil {
		log.Error().Err(err).Str("task", task.ID).Msg("failed to publish sitemap crawl result")
	}

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Int("total_urls", finalTotalURLs).
		Bool("success", result.Success).
		Bool("timed_out", timedOut).
		Msg("sitemap crawl completed")
}

const maxSitemapDepth = 5

// progressCallback is called when progress is made (e.g., sitemap parsed successfully)
type progressCallback func()

// urlsCallback is called with URLs immediately after parsing each sitemap (streaming mode)
type urlsCallback func(urls []crawler.ParsedURL, source string)

// parseSitemapStreaming parses sitemap and calls onURLs callback immediately for each leaf sitemap
// This allows publishing URL batches as soon as they're available, not waiting for all sitemaps
func (w *SitemapWorker) parseSitemapStreaming(ctx context.Context, sitemapURL string, cookies []captcha.Cookie, onProgress progressCallback, onURLs urlsCallback) ([]captcha.Cookie, error) {
	visited := make(map[string]bool)
	return w.parseSitemapStreamingRecursive(ctx, sitemapURL, cookies, 0, visited, onProgress, onURLs)
}

func (w *SitemapWorker) parseSitemapStreamingRecursive(ctx context.Context, sitemapURL string, cookies []captcha.Cookie, depth int, visited map[string]bool, onProgress progressCallback, onURLs urlsCallback) ([]captcha.Cookie, error) {
	log := logger.Log

	if depth > maxSitemapDepth {
		log.Warn().Str("url", sitemapURL).Int("depth", depth).Msg("max sitemap depth exceeded")
		return cookies, nil
	}

	if visited[sitemapURL] {
		return cookies, nil
	}
	visited[sitemapURL] = true

	// Set cookies if provided
	if len(cookies) > 0 {
		if err := browser.Get().SetCookies(cookies); err != nil {
			log.Warn().Err(err).Msg("failed to set cookies")
		}
	}

	log.Debug().Str("url", sitemapURL).Int("depth", depth).Msg("fetching sitemap via global browser")

	result, err := browser.Get().FetchSitemap(ctx, sitemapURL)
	if err != nil {
		// If browser timed out, try HTTP stream as fallback (faster for large XML files)
		if ctx.Err() == nil && (strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "TIMED_OUT")) {
			log.Info().Str("url", sitemapURL).Msg("browser timeout, trying HTTP fallback")
			return w.fetchSitemapHTTP(ctx, sitemapURL, cookies, depth, visited, onProgress, onURLs)
		}
		return cookies, err
	}

	// Successfully fetched - signal progress
	if onProgress != nil {
		onProgress()
	}

	if result.Blocked {
		return cookies, fmt.Errorf("blocked: %s", result.BlockReason)
	}

	newCookies := cookies
	if len(result.Cookies) > 0 {
		newCookies = result.Cookies
	}

	// Parse XML from fetched content
	parsed, err := crawler.ParseSitemapXML(result.HTML, sitemapURL)
	if err != nil {
		return newCookies, err
	}

	// Immediately publish page URLs from this sitemap (streaming)
	if len(parsed.PageURLs) > 0 && onURLs != nil {
		urls := make([]crawler.ParsedURL, len(parsed.PageURLs))
		for i, u := range parsed.PageURLs {
			urls[i] = crawler.ParsedURL{Loc: u}
		}
		onURLs(urls, sitemapURL)
		log.Info().
			Str("url", sitemapURL).
			Int("depth", depth).
			Int("urls", len(parsed.PageURLs)).
			Msg("sitemap URLs published immediately")
	}

	// Process nested sitemaps recursively
	if len(parsed.NestedSitemaps) > 0 {
		log.Info().
			Str("url", sitemapURL).
			Int("nested_count", len(parsed.NestedSitemaps)).
			Int("depth", depth).
			Msg("processing nested sitemaps")

		for _, nestedURL := range parsed.NestedSitemaps {
			if ctx.Err() != nil {
				log.Warn().Str("url", sitemapURL).Msg("context cancelled, stopping nested sitemap processing")
				break
			}
			// Skip blacklisted sitemaps (character, people, etc.)
			if shouldSkipSitemap(nestedURL) {
				log.Info().Str("sitemap", nestedURL).Msg("skipping blacklisted sitemap")
				continue
			}
			updatedCookies, err := w.parseSitemapStreamingRecursive(ctx, nestedURL, newCookies, depth+1, visited, onProgress, onURLs)
			if err != nil {
				log.Warn().Err(err).Str("sitemap", nestedURL).Msg("nested sitemap failed")
				if ctx.Err() != nil {
					break
				}
				continue
			}
			if len(updatedCookies) > 0 {
				newCookies = updatedCookies
			}
		}
	}

	log.Debug().
		Str("url", sitemapURL).
		Int("depth", depth).
		Int("nested_sitemaps", len(parsed.NestedSitemaps)).
		Int("page_urls", len(parsed.PageURLs)).
		Msg("sitemap processed")

	return newCookies, nil
}

// fetchSitemapHTTP fetches sitemap via HTTP (fallback for large files that timeout in browser)
func (w *SitemapWorker) fetchSitemapHTTP(ctx context.Context, sitemapURL string, cookies []captcha.Cookie, depth int, visited map[string]bool, onProgress progressCallback, onURLs urlsCallback) ([]captcha.Cookie, error) {
	log := logger.Log

	client := &http.Client{Timeout: 2 * time.Minute}

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return cookies, err
	}

	// Set headers to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")

	// Add cookies if available
	for _, c := range cookies {
		req.AddCookie(&http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		return cookies, fmt.Errorf("http fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return cookies, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return cookies, fmt.Errorf("read body: %w", err)
	}

	log.Info().Str("url", sitemapURL).Int("size", len(body)).Msg("sitemap fetched via HTTP fallback")

	if onProgress != nil {
		onProgress()
	}

	// Parse XML
	parsed, err := crawler.ParseSitemapXML(string(body), sitemapURL)
	if err != nil {
		return cookies, fmt.Errorf("parse xml: %w", err)
	}

	// Publish page URLs immediately
	if len(parsed.PageURLs) > 0 && onURLs != nil {
		urls := make([]crawler.ParsedURL, len(parsed.PageURLs))
		for i, u := range parsed.PageURLs {
			urls[i] = crawler.ParsedURL{Loc: u}
		}
		onURLs(urls, sitemapURL)
		log.Info().Str("url", sitemapURL).Int("urls", len(parsed.PageURLs)).Msg("HTTP sitemap URLs published")
	}

	// Process nested sitemaps
	for _, nestedURL := range parsed.NestedSitemaps {
		if ctx.Err() != nil {
			break
		}
		// For nested sitemaps from HTTP fallback, try browser first again
		_, err := w.parseSitemapStreamingRecursive(ctx, nestedURL, cookies, depth+1, visited, onProgress, onURLs)
		if err != nil {
			log.Warn().Err(err).Str("sitemap", nestedURL).Msg("nested sitemap failed")
		}
	}

	return cookies, nil
}

func (w *SitemapWorker) convertURLsToQueue(urls []crawler.ParsedURL, source string) []queue.ParsedURLData {
	result := make([]queue.ParsedURLData, len(urls))
	for i, u := range urls {
		result[i] = queue.ParsedURLData{
			URL:        u.Loc,
			LastMod:    u.LastMod,
			ChangeFreq: u.ChangeFreq,
			Priority:   u.Priority,
			Source:     source,
			Depth:      0, // URLs из sitemap = depth 0
		}
	}
	return result
}

func (w *SitemapWorker) convertTaskCookies(taskCookies []queue.CookieData) []captcha.Cookie {
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

func (w *SitemapWorker) convertCaptchaCookies(cookies []captcha.Cookie) []queue.CookieData {
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
