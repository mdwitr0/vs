package detector

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/captcha"
	"github.com/video-analitics/backend/pkg/logger"
)

type Detector interface {
	Detect(ctx context.Context, targetURL string) (*Result, error)
}

type detector struct {
	fetcher         *Fetcher
	browserFetcher  *BrowserFetcher
	cmsDetector     *CMSDetector
	renderDetector  *RenderDetector
	captchaDetector *CaptchaDetector
	sitemapDetector *SitemapDetector
	captchaSolver   *captcha.PirateSolver
	dnsChecker      *DNSChecker

	checkSitemap   bool
	checkRobots    bool
	checkDNS       bool
	useBrowserOnly bool
}

type Option func(*detector)

func WithCheckSitemap(check bool) Option {
	return func(d *detector) {
		d.checkSitemap = check
	}
}

func WithCheckRobots(check bool) Option {
	return func(d *detector) {
		d.checkRobots = check
	}
}

func WithCustomFetcher(f *Fetcher) Option {
	return func(d *detector) {
		d.fetcher = f
		d.sitemapDetector = NewSitemapDetector(f)
	}
}

func WithCheckDNS(check bool) Option {
	return func(d *detector) {
		d.checkDNS = check
	}
}

func New(opts ...Option) Detector {
	fetcher := NewFetcher()

	d := &detector{
		fetcher:         fetcher,
		browserFetcher:  NewBrowserFetcher(),
		cmsDetector:     NewCMSDetector(),
		renderDetector:  NewRenderDetector(),
		captchaDetector: NewCaptchaDetector(),
		sitemapDetector: NewSitemapDetector(fetcher),
		captchaSolver:   captcha.NewPirateSolver(),
		dnsChecker:      NewDNSChecker(),
		checkSitemap:    true,
		checkRobots:     true,
		checkDNS:        true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

func (d *detector) Detect(ctx context.Context, targetURL string) (*Result, error) {
	result := &Result{
		DetectedAt: time.Now(),
		DetectedBy: make([]string, 0),
	}

	normalizedURL, err := d.normalizeURL(targetURL)
	if err != nil {
		return nil, err
	}

	// DNS pre-check: быстро отсекаем несуществующие домены
	if d.checkDNS {
		u, _ := url.Parse(normalizedURL)
		if u != nil {
			dnsResult := d.dnsChecker.Check(ctx, u.Host)
			if !dnsResult.Resolvable {
				logger.Log.Info().
					Str("domain", u.Host).
					Err(dnsResult.Error).
					Msg("DNS check failed, domain does not exist")
				return nil, fmt.Errorf("domain not resolvable: %s", u.Host)
			}
			logger.Log.Debug().
				Str("domain", u.Host).
				Strs("ips", dnsResult.IPs).
				Msg("DNS check passed")
		}
	}

	var fetchResult *FetchResult
	usedBrowser := false

	// Сначала пробуем HTTP
	fetchResult = d.fetcher.Fetch(ctx, normalizedURL)
	if fetchResult.Error != nil {
		altURL := d.switchProtocol(normalizedURL)
		fetchResult = d.fetcher.Fetch(ctx, altURL)
	}

	// Если HTTP заблокирован или вернул ошибку - пробуем browser
	htmlStr := string(fetchResult.Body)
	isBlocked := IsBlockedResponse(htmlStr, fetchResult.StatusCode)
	logger.Log.Debug().
		Int("status", fetchResult.StatusCode).
		Int("body_len", len(htmlStr)).
		Bool("is_blocked", isBlocked).
		Bool("has_error", fetchResult.Error != nil).
		Str("url", normalizedURL).
		Msg("HTTP fetch result")

	if fetchResult.Error != nil || isBlocked {
		logger.Log.Info().Str("url", normalizedURL).Msg("trying browser fallback")
		browserResult := d.browserFetcher.Fetch(ctx, normalizedURL)
		if browserResult.Error == nil {
			browserHTML := string(browserResult.Body)
			browserBlocked := IsBlockedResponse(browserHTML, browserResult.StatusCode)

			if browserBlocked {
				logger.Log.Info().Str("url", normalizedURL).Msg("browser also blocked, trying captcha solver")
				solveResult, err := d.captchaSolver.Solve(ctx, normalizedURL)
				if err == nil && solveResult.Success {
					logger.Log.Info().Str("url", normalizedURL).Int("cookies", len(solveResult.Cookies)).Msg("captcha solved successfully")
					fetchResult = &FetchResult{
						Body:       []byte(solveResult.HTML),
						StatusCode: 200,
						FinalURL:   normalizedURL,
					}
					usedBrowser = true
					result.DetectedBy = append(result.DetectedBy, "fetch:captcha_solved")
					result.CaptchaType = CaptchaPirate
					for _, c := range solveResult.Cookies {
						result.Cookies = append(result.Cookies, CookieData{
							Name:     c.Name,
							Value:    c.Value,
							Domain:   c.Domain,
							Path:     c.Path,
							HTTPOnly: c.HTTPOnly,
							Secure:   c.Secure,
						})
					}
				} else {
					logger.Log.Warn().Err(err).Str("url", normalizedURL).Msg("captcha solve failed, using blocked response")
					fetchResult = browserResult
					usedBrowser = true
					result.DetectedBy = append(result.DetectedBy, "fetch:browser_blocked")
				}
			} else {
				fetchResult = browserResult
				usedBrowser = true
				result.DetectedBy = append(result.DetectedBy, "fetch:browser")
				logger.Log.Info().Str("url", normalizedURL).Msg("browser fallback successful")
			}
		} else {
			logger.Log.Warn().Err(browserResult.Error).Str("url", normalizedURL).Msg("browser fallback failed")
			if fetchResult.Error != nil {
				return nil, fetchResult.Error
			}
		}
	}

	result.StatusCode = fetchResult.StatusCode
	result.ContentType = fetchResult.ContentType
	result.Headers = fetchResult.Headers

	if fetchResult.FinalURL != normalizedURL {
		result.HasRedirects = true
		result.FinalURL = fetchResult.FinalURL
	}

	html := string(fetchResult.Body)

	cmsResult := d.cmsDetector.Detect(html, fetchResult.Headers)
	result.CMS = cmsResult.CMS
	result.CMSVersion = cmsResult.Version
	for _, m := range cmsResult.Markers {
		result.DetectedBy = append(result.DetectedBy, "cms:"+m.Name)
	}

	renderResult := d.renderDetector.Detect(html, int64(len(fetchResult.Body)))
	result.RenderType = renderResult.RenderType
	result.Framework = renderResult.Framework
	// NeedsBrowser true если: HTTP был заблокирован ИЛИ render detector определил SPA
	result.NeedsBrowser = usedBrowser || renderResult.NeedsBrowser
	for _, m := range renderResult.Markers {
		result.DetectedBy = append(result.DetectedBy, "render:"+m.Name)
	}

	captchaResult := d.captchaDetector.Detect(html, fetchResult.Headers)
	// Не перезаписываем если уже установлен (например, из captcha solver)
	if result.CaptchaType == "" || result.CaptchaType == CaptchaNone {
		result.CaptchaType = captchaResult.Type
	}
	for _, m := range captchaResult.Markers {
		result.DetectedBy = append(result.DetectedBy, "captcha:"+m.Name)
	}

	if d.checkSitemap {
		baseURL := d.getBaseURL(fetchResult.FinalURL)

		sitemapResult := d.sitemapDetector.Detect(ctx, baseURL)
		result.HasSitemap = sitemapResult.HasSitemap
		result.SitemapStatus = sitemapResult.SitemapStatus
		result.SitemapURLs = sitemapResult.SitemapURLs

		if d.checkRobots && sitemapResult.SitemapStatus != SitemapValid {
			robotsResult := d.sitemapDetector.ValidateRobotsSitemaps(ctx, baseURL)
			if robotsResult.SitemapStatus == SitemapValid {
				result.HasSitemap = true
				result.SitemapStatus = SitemapValid
				result.SitemapURLs = append(result.SitemapURLs, robotsResult.SitemapURLs...)
			} else if robotsResult.SitemapStatus == SitemapInvalid && result.SitemapStatus == SitemapNone {
				result.SitemapStatus = SitemapInvalid
				result.DetectedBy = append(result.DetectedBy, "sitemap:invalid_in_robots")
			}
		}

		// Единая стратегия: sitemap если есть, иначе с главной + рекурсивный сбор ссылок
		result.CrawlStrategy = CrawlStrategySitemap
		switch result.SitemapStatus {
		case SitemapValid:
			result.DetectedBy = append(result.DetectedBy, "sitemap:valid")
		case SitemapEmpty:
			result.DetectedBy = append(result.DetectedBy, "sitemap:empty")
		case SitemapInvalid:
			result.DetectedBy = append(result.DetectedBy, "sitemap:invalid")
		default:
			result.DetectedBy = append(result.DetectedBy, "sitemap:none")
		}
	}

	result.SubdomainPerMovie = d.detectSubdomainPerMovie(targetURL)
	if result.SubdomainPerMovie {
		result.DetectedBy = append(result.DetectedBy, "pattern:subdomain_per_movie")
	}

	result.Confidence = d.calculateOverallConfidence(cmsResult, renderResult, captchaResult)

	return result, nil
}

func (d *detector) normalizeURL(rawURL string) (string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	u.Host = strings.TrimPrefix(u.Host, "www.")

	if u.Path == "" {
		u.Path = "/"
	}

	return u.String(), nil
}

func (d *detector) switchProtocol(rawURL string) string {
	if strings.HasPrefix(rawURL, "https://") {
		return strings.Replace(rawURL, "https://", "http://", 1)
	}
	return strings.Replace(rawURL, "http://", "https://", 1)
}

func (d *detector) getBaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Scheme + "://" + u.Host
}

func (d *detector) detectSubdomainPerMovie(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := u.Hostname()
	parts := strings.Split(host, ".")

	if len(parts) < 3 {
		return false
	}

	subdomain := parts[0]

	if !strings.Contains(subdomain, "-") {
		return false
	}

	standardSubdomains := []string{"www", "m", "mobile", "api", "cdn", "static", "img", "images", "video", "stream"}
	for _, std := range standardSubdomains {
		if subdomain == std {
			return false
		}
	}

	if len(subdomain) < 10 {
		return false
	}

	return true
}

func (d *detector) calculateOverallConfidence(cms CMSResult, render RenderResult, captcha CaptchaResult) float64 {
	total := cms.Confidence*0.5 + render.Confidence*0.3 + captcha.Confidence*0.2

	if cms.CMS == CMSUnknown || cms.CMS == CMSCustom {
		total *= 0.8
	}

	if total > 1.0 {
		total = 1.0
	}

	return total
}
