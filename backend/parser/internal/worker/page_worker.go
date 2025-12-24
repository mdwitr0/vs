package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/captcha"
	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/models"
	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/parser/internal/browser"
	"github.com/video-analitics/parser/internal/cache"
	"github.com/video-analitics/parser/internal/crawler"
	"github.com/video-analitics/parser/internal/extractor"
)

type PageWorker struct {
	natsClient    *nats.Client
	publisher     *nats.Publisher
	extractor     *extractor.Extractor
	httpClient    *http.Client
	internalToken string
}

func NewPageWorker(natsClient *nats.Client, internalToken string) *PageWorker {
	return &PageWorker{
		natsClient:    natsClient,
		publisher:     nats.NewPublisher(natsClient),
		extractor:     extractor.New(),
		internalToken: internalToken,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (w *PageWorker) Run(ctx context.Context) error {
	return w.RunPool(ctx, 1)
}

func (w *PageWorker) RunPool(ctx context.Context, workerCount int) error {
	log := logger.Log

	if workerCount < 1 {
		workerCount = 1
	}

	consumer, err := nats.NewConsumer(w.natsClient, nats.ConsumerConfig{
		Stream:        nats.StreamPageCrawlTasks,
		Consumer:      "page-worker",
		MaxAckPending: workerCount,
		AckWait:       30 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	log.Info().Int("workers", workerCount).Msg("page worker pool started")

	return consumer.ConsumePool(ctx, workerCount, func(ctx context.Context, msg *nats.Message) error {
		var task queue.PageCrawlTask
		if err := msg.Unmarshal(&task); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal page crawl task")
			return err
		}

		w.processTask(&task)
		return nil
	})
}

func (w *PageWorker) processTask(task *queue.PageCrawlTask) {
	log := logger.Log

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Int("batch_size", task.BatchSize).
		Msg("page crawl started")

	batchSize := task.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	result := queue.PageCrawlResult{
		TaskID:     task.ID,
		SiteID:     task.SiteID,
		Success:    true,
		FinishedAt: time.Now(),
	}

	cookies := w.convertTaskCookies(task.Cookies)
	var newCookies []captcha.Cookie
	totalProcessed := 0
	totalSuccess := 0
	totalFailed := 0

	w.processPages(task, &result, &totalProcessed, &totalSuccess, &totalFailed, batchSize, cookies, &newCookies)

	result.PagesTotal = totalProcessed
	result.PagesSuccess = totalSuccess
	result.PagesFailed = totalFailed
	result.NewCookies = w.convertCaptchaCookies(newCookies)
	result.FinishedAt = time.Now()

	if totalProcessed == 0 && result.Success {
		result.NoURLsAvailable = true
	}

	bgCtx := context.Background()
	if err := w.publisher.PublishPageCrawlResult(bgCtx, result); err != nil {
		log.Error().Err(err).Str("task", task.ID).Msg("failed to publish page crawl result")
	}

	log.Info().
		Str("site", task.SiteID).
		Str("domain", task.Domain).
		Int("total", totalProcessed).
		Int("success", totalSuccess).
		Int("failed", totalFailed).
		Bool("result_success", result.Success).
		Msg("page crawl completed")
}

func (w *PageWorker) processPages(task *queue.PageCrawlTask, result *queue.PageCrawlResult, totalProcessed, totalSuccess, totalFailed *int, batchSize int, cookies []captcha.Cookie, newCookies *[]captcha.Cookie) {
	log := logger.Log
	bgCtx := context.Background()

	if len(cookies) > 0 {
		if err := browser.Get().SetCookies(cookies); err != nil {
			log.Warn().Err(err).Msg("failed to set cookies")
		}
	}

	// Initialize Bloom filter with existing URLs to avoid duplicates
	bloomFilter := cache.NewURLBloomFilter(1_000_000, 0.001)
	existingURLs, err := w.fetchAllURLs(bgCtx, task.IndexerAPIURL, task.SiteID)
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch existing URLs for bloom filter")
	} else {
		bloomFilter.LoadBatch(existingURLs)
		log.Info().Int("urls_loaded", len(existingURLs)).Msg("bloom filter initialized")
	}

	log.Info().Str("domain", task.Domain).Msg("starting page processing")

	for {
		fetchResult, err := w.fetchPendingURLs(bgCtx, task.IndexerAPIURL, task.SiteID, batchSize)
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch pending urls")
			result.Success = false
			result.Error = fmt.Sprintf("failed to fetch pending urls: %v", err)
			break
		}

		if len(fetchResult.URLs) == 0 {
			if fetchResult.AllIndexed {
				log.Info().Str("site", task.SiteID).Int64("indexed", fetchResult.IndexedCount).Msg("all URLs already indexed")
				result.AllIndexed = true
				result.IndexedCount = int(fetchResult.IndexedCount)
			} else {
				log.Info().Str("site", task.SiteID).Msg("no more pending urls")
			}
			break
		}
		urls := fetchResult.URLs

		for _, urlData := range urls {
			pageResult, html := w.parsePageSPAWithHTML(urlData.URL, task.SiteID, newCookies)

			// Публикуем результат сразу после парсинга
			singleResult := queue.PageSingleResult{
				TaskID:    task.ID,
				SiteID:    task.SiteID,
				URL:       urlData.URL,
				Success:   pageResult.Success,
				Error:     pageResult.Error,
				Page:      pageResult.Page,
				IPBlocked: pageResult.IPBlocked,
				Timestamp: time.Now(),
			}
			if err := w.publisher.PublishPageSingleResult(bgCtx, singleResult); err != nil {
				log.Warn().Err(err).Str("url", urlData.URL).Msg("failed to publish single result")
			}

			if pageResult.IPBlocked {
				log.Error().Str("site", task.SiteID).Str("url", urlData.URL).Str("reason", pageResult.Error).Msg("IP blocked, stopping crawl")
				result.Success = false
				result.Error = pageResult.Error
				result.IPBlocked = true
				result.BlockReason = pageResult.Error
				return
			}

			// Извлекаем ссылки из успешно спарсенных страниц (depth < 3)
			if pageResult.Success && html != "" && urlData.Depth < 3 {
				// Передаём оба домена:
				// - pageDomain для фильтрации ссылок (из URL страницы)
				// - task.Domain для нормализации (куда заменять)
				pageDomain := extractDomainFromURL(urlData.URL)
				if pageDomain == "" {
					pageDomain = task.Domain
				}
				w.extractAndPublishLinks(bgCtx, task.ID, task.SiteID, pageDomain, task.Domain, urlData.URL, html, urlData.Depth, bloomFilter)
			}

			if pageResult.Success {
				*totalSuccess++
			} else {
				log.Debug().Str("url", urlData.URL).Str("error", pageResult.Error).Msg("page failed, continuing")
				*totalFailed++
			}
			*totalProcessed++
		}

		log.Info().
			Str("site", task.SiteID).
			Int("batch_urls", len(urls)).
			Int("total_processed", *totalProcessed).
			Int("success", *totalSuccess).
			Int("failed", *totalFailed).
			Msg("batch URLs processed")
	}
}

type pendingURLWithDepth struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

type pendingURLsResponse struct {
	URLs        []pendingURLWithDepth `json:"urls"`
	AllIndexed  bool                  `json:"all_indexed,omitempty"`
	InRetry     bool                  `json:"in_retry,omitempty"`
	TotalURLs   int64                 `json:"total_urls,omitempty"`
	IndexedURLs int64                 `json:"indexed_urls,omitempty"`
}

type fetchPendingResult struct {
	URLs         []pendingURLWithDepth
	AllIndexed   bool
	InRetry      bool
	IndexedCount int64
}

func (w *PageWorker) fetchPendingURLs(ctx context.Context, apiURL, siteID string, limit int) (*fetchPendingResult, error) {
	url := fmt.Sprintf("%s/api/internal/sites/%s/pending-urls?limit=%d", apiURL, siteID, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if w.internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+w.internalToken)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result pendingURLsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &fetchPendingResult{
		URLs:         result.URLs,
		AllIndexed:   result.AllIndexed,
		InRetry:      result.InRetry,
		IndexedCount: result.IndexedURLs,
	}, nil
}

type allURLsResponse struct {
	URLs  []string `json:"urls"`
	Count int      `json:"count"`
}

func (w *PageWorker) fetchAllURLs(ctx context.Context, apiURL, siteID string) ([]string, error) {
	url := fmt.Sprintf("%s/api/internal/sites/%s/all-urls", apiURL, siteID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if w.internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+w.internalToken)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result allURLsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.URLs, nil
}

func (w *PageWorker) parsePageSPA(pageURL, siteID string, newCookies *[]captcha.Cookie) queue.PageResult {
	result, _ := w.parsePageSPAWithHTML(pageURL, siteID, newCookies)
	return result
}

func (w *PageWorker) parsePageSPAWithHTML(pageURL, siteID string, newCookies *[]captcha.Cookie) (queue.PageResult, string) {
	log := logger.Log

	result := queue.PageResult{
		URL:     pageURL,
		Success: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fetchResult, err := browser.Get().FetchPage(ctx, pageURL)
	if err != nil {
		log.Warn().Err(err).Str("url", pageURL).Msg("page fetch failed")
		result.Error = err.Error()
		return result, ""
	}

	if fetchResult.Blocked {
		result.Error = "blocked: " + fetchResult.BlockReason
		result.IPBlocked = true
		return result, ""
	}

	if len(fetchResult.Cookies) > 0 {
		*newCookies = fetchResult.Cookies
		log.Info().Str("url", pageURL).Int("cookies", len(fetchResult.Cookies)).Msg("new cookies from page")
	}

	page, err := w.extractor.Extract(fetchResult.HTML, pageURL, siteID, 200)
	if err != nil {
		result.Error = err.Error()
		return result, fetchResult.HTML
	}
	page.IndexedAt = time.Now()

	if page.Title == "" {
		result.Error = "empty title"
		return result, fetchResult.HTML
	}

	if isBadTitle(page.Title) {
		result.Error = fmt.Sprintf("error page title: %s", page.Title)
		return result, fetchResult.HTML
	}

	result.Success = true
	result.Page = w.convertPageToQueueData(page)

	return result, fetchResult.HTML
}

func (w *PageWorker) convertPageToQueueData(page *models.Page) *queue.PageData {
	externalIDs := make(map[string]string)
	if page.ExternalIDs.KinopoiskID != "" {
		externalIDs["kinopoisk_id"] = page.ExternalIDs.KinopoiskID
	}
	if page.ExternalIDs.IMDBID != "" {
		externalIDs["imdb_id"] = page.ExternalIDs.IMDBID
	}
	if page.ExternalIDs.TMDBID != "" {
		externalIDs["tmdb_id"] = page.ExternalIDs.TMDBID
	}
	if page.ExternalIDs.MALID != "" {
		externalIDs["mal_id"] = page.ExternalIDs.MALID
	}
	if page.ExternalIDs.ShikimoriID != "" {
		externalIDs["shikimori_id"] = page.ExternalIDs.ShikimoriID
	}
	if page.ExternalIDs.MyDramaListID != "" {
		externalIDs["mydramalist_id"] = page.ExternalIDs.MyDramaListID
	}

	return &queue.PageData{
		URL:         page.URL,
		Title:       page.Title,
		Description: page.Description,
		MainText:    page.MainText,
		Year:        page.Year,
		PlayerURL:   page.PlayerURL,
		LinksText:   page.LinksText,
		ExternalIDs: externalIDs,
	}
}

func (w *PageWorker) convertTaskCookies(taskCookies []queue.CookieData) []captcha.Cookie {
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

func (w *PageWorker) convertCaptchaCookies(cookies []captcha.Cookie) []queue.CookieData {
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

var badTitlePatterns = []string{
	"page not found",
	"404",
	"not found",
	"ошибка",
	"страница не найдена",
	"403 forbidden",
	"access denied",
	"доступ запрещен",
}

func isBadTitle(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	for _, pattern := range badTitlePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

const maxLinkExtractionDepth = 3

func (w *PageWorker) extractAndPublishLinks(ctx context.Context, taskID, siteID, filterDomain, targetDomain, pageURL, html string, currentDepth int, bloomFilter *cache.URLBloomFilter) {
	log := logger.Log

	if currentDepth >= maxLinkExtractionDepth {
		return
	}

	links := crawler.ExtractLinksFromHTML(html, pageURL)
	if len(links) == 0 {
		return
	}

	// Фильтруем только внутренние ссылки и нормализуем домен
	var validLinks []queue.ParsedURLData
	nextDepth := currentDepth + 1
	skippedByBloom := 0

	for _, link := range links {
		if !w.isInternalLink(link, filterDomain) {
			continue
		}
		// Нормализуем URL: заменяем домен на targetDomain
		normalizedURL := normalizeURLDomain(link, targetDomain)
		if normalizedURL == "" {
			continue
		}
		// Check Bloom filter for duplicates
		if bloomFilter != nil && bloomFilter.MayContain(normalizedURL) {
			skippedByBloom++
			continue
		}
		// Add to Bloom filter for future deduplication
		if bloomFilter != nil {
			bloomFilter.Add(normalizedURL)
		}
		validLinks = append(validLinks, queue.ParsedURLData{
			URL:    normalizedURL,
			Source: pageURL,
			Depth:  nextDepth,
		})
	}

	if len(validLinks) == 0 {
		if skippedByBloom > 0 {
			log.Debug().Str("url", pageURL).Int("skipped_bloom", skippedByBloom).Msg("all links filtered by bloom")
		}
		return
	}

	// Ограничиваем размер батча
	const maxBatchSize = 100
	if len(validLinks) > maxBatchSize {
		validLinks = validLinks[:maxBatchSize]
	}

	batch := queue.SitemapURLBatch{
		TaskID:        taskID,
		SiteID:        siteID,
		URLs:          validLinks,
		BatchNumber:   0,
		SitemapSource: pageURL,
	}

	if err := w.publisher.PublishSitemapURLBatch(ctx, batch); err != nil {
		log.Warn().Err(err).Str("url", pageURL).Int("links", len(validLinks)).Msg("failed to publish extracted links")
	} else {
		log.Debug().
			Str("url", pageURL).
			Int("links", len(validLinks)).
			Int("skipped_bloom", skippedByBloom).
			Int("depth", nextDepth).
			Msg("extracted links published")
	}
}

func extractDomainFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// normalizeURLDomain заменяет домен в URL на targetDomain
// kinogo1.biz/page → go.kinogo1.biz/page
func normalizeURLDomain(rawURL, targetDomain string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsed.Host = targetDomain
	return parsed.String()
}

func (w *PageWorker) isInternalLink(link, domain string) bool {
	parsed, err := url.Parse(link)
	if err != nil {
		return false
	}

	// Отсекаем протокол-relative URLs которые неправильно склеились (//www.external.com)
	if strings.HasPrefix(parsed.Path, "//") {
		return false
	}

	linkHost := strings.TrimPrefix(parsed.Hostname(), "www.")
	ourDomain := strings.TrimPrefix(domain, "www.")

	// Точное совпадение или субдомен (go.kinogo1.biz → kinogo1.biz)
	return linkHost == ourDomain || strings.HasSuffix(linkHost, "."+ourDomain)
}
