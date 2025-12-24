package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/video-analitics/backend/pkg/captcha"
	cdpopts "github.com/video-analitics/backend/pkg/chromedp"
	"github.com/video-analitics/backend/pkg/logger"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 YaBrowser/25.10.0.0 Safari/537.36"

const defaultTabTimeout = 60 * time.Second

// FetchResult represents the result of fetching a URL
type FetchResult struct {
	HTML        string
	FinalURL    string
	Blocked     bool
	IsCaptcha   bool
	BlockReason string
	Cookies     []captcha.Cookie
}

// FetchPage loads a page in a new tab, handles blocking/captcha, returns clean HTML
func (b *GlobalBrowser) FetchPage(ctx context.Context, url string) (*FetchResult, error) {
	log := logger.Log

	if err := b.AcquireWithContext(ctx); err != nil {
		return nil, fmt.Errorf("acquire browser slot: %w", err)
	}
	defer b.Release()

	var html string
	var finalURL string

	// Create new tab context
	tabCtx, tabCancel := chromedp.NewContext(b.browserCtx)
	defer tabCancel()

	tabTimeoutCtx, tabTimeoutCancel := context.WithTimeout(tabCtx, defaultTabTimeout)
	defer tabTimeoutCancel()

	tasks := chromedp.Tasks{
		// Set User-Agent via emulation API (more reliable than command-line flag)
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(userAgent).
				WithAcceptLanguage("ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7").
				WithPlatform("macOS").
				Do(ctx)
		}),
		// Block unnecessary resources (images, videos, fonts, stylesheets)
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetBlockedURLs([]string{
				"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg", "*.ico",
				"*.mp4", "*.webm", "*.avi", "*.mov",
				"*.woff", "*.woff2", "*.ttf", "*.eot", "*.otf",
				"*.css",
				"*google-analytics*", "*googletagmanager*", "*facebook*", "*yandex*metrika*",
				"*ads*", "*analytics*", "*tracking*",
			}).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetExtraHTTPHeaders(network.Headers{
				"Accept-Language":           "ru-RU,ru;q=0.9,en;q=0.8",
				"sec-ch-ua":                 `"Chromium";v="140", "Not=A?Brand";v="24", "YaBrowser";v="25.10", "Yowser";v="2.5"`,
				"sec-ch-ua-mobile":          "?0",
				"sec-ch-ua-platform":        `"macOS"`,
				"Upgrade-Insecure-Requests": "1",
				"DNT":                       "1",
			}).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(cdpopts.GetStealthScripts()).Do(ctx)
			return err
		}),
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(b.pageLoadDelay),
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("html", &html),
	}

	if err := chromedp.Run(tabTimeoutCtx, tasks); err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}

	log.Debug().Str("url", url).Str("final_url", finalURL).Int("html_len", len(html)).Msg("page fetched")

	// Check for blocking/captcha
	blockResult := DetectBlocking(html, 200)
	if blockResult.Blocked {
		if blockResult.IsCaptcha && b.solver != nil {
			log.Info().Str("url", url).Msg("captcha detected, solving...")

			result, err := b.solveCaptchaInTab(tabTimeoutCtx, url)
			if err != nil {
				return &FetchResult{
					FinalURL:    finalURL,
					Blocked:     true,
					IsCaptcha:   true,
					BlockReason: fmt.Sprintf("captcha solve failed: %v", err),
				}, nil
			}

			if !result.Success {
				return &FetchResult{
					FinalURL:    finalURL,
					Blocked:     true,
					IsCaptcha:   true,
					BlockReason: fmt.Sprintf("captcha solve unsuccessful: %v", result.Error),
				}, nil
			}

			html = result.HTML
			cookies := result.Cookies

			// Re-check after captcha solve
			blockResult = DetectBlocking(html, 200)
			if blockResult.Blocked {
				return &FetchResult{
					HTML:        html,
					FinalURL:    finalURL,
					Blocked:     true,
					IsCaptcha:   blockResult.IsCaptcha,
					BlockReason: blockResult.Reason,
					Cookies:     cookies,
				}, nil
			}

			log.Info().Str("url", url).Int("cookies", len(cookies)).Msg("captcha solved successfully")
			return &FetchResult{
				HTML:     html,
				FinalURL: finalURL,
				Cookies:  cookies,
			}, nil
		}

		return &FetchResult{
			FinalURL:    finalURL,
			Blocked:     true,
			IsCaptcha:   blockResult.IsCaptcha,
			BlockReason: blockResult.Reason,
		}, nil
	}

	// Get cookies
	cookies, _ := b.getCookies(tabTimeoutCtx)

	return &FetchResult{
		HTML:     html,
		FinalURL: finalURL,
		Cookies:  cookies,
	}, nil
}

// FetchSitemap loads a sitemap URL in a new tab, handles captcha, returns content
func (b *GlobalBrowser) FetchSitemap(ctx context.Context, sitemapURL string) (*FetchResult, error) {
	if err := b.AcquireWithContext(ctx); err != nil {
		return nil, fmt.Errorf("acquire browser slot: %w", err)
	}
	defer b.Release()

	log := logger.Log

	tabCtx, tabCancel := chromedp.NewContext(b.browserCtx)
	defer tabCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, defaultTabTimeout)
	defer timeoutCancel()

	var body string
	tasks := chromedp.Tasks{
		// Set User-Agent via emulation API
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(userAgent).
				WithAcceptLanguage("ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7").
				WithPlatform("macOS").
				Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetExtraHTTPHeaders(network.Headers{
				"Accept-Language":           "ru-RU,ru;q=0.9,en;q=0.8",
				"sec-ch-ua":                 `"Chromium";v="140", "Not=A?Brand";v="24", "YaBrowser";v="25.10", "Yowser";v="2.5"`,
				"sec-ch-ua-mobile":          "?0",
				"sec-ch-ua-platform":        `"macOS"`,
				"Upgrade-Insecure-Requests": "1",
				"DNT":                       "1",
			}).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(cdpopts.GetStealthScripts()).Do(ctx)
			return err
		}),
		chromedp.Navigate(sitemapURL),
		chromedp.Sleep(b.pageLoadDelay),
		chromedp.Evaluate(getSitemapExtractScript(), &body),
	}

	if err := chromedp.Run(timeoutCtx, tasks); err != nil {
		return nil, fmt.Errorf("fetch sitemap: %w", err)
	}

	log.Debug().Str("url", sitemapURL).Int("body_len", len(body)).Msg("sitemap fetched")

	// Check for captcha on sitemap
	if IsSitemapCaptcha(body) && b.solver != nil {
		log.Info().Str("url", sitemapURL).Msg("sitemap captcha detected, solving...")

		result, err := b.solveCaptchaInTab(timeoutCtx, sitemapURL)
		if err != nil {
			return &FetchResult{
				Blocked:     true,
				IsCaptcha:   true,
				BlockReason: fmt.Sprintf("sitemap captcha solve failed: %v", err),
			}, nil
		}

		if !result.Success {
			return &FetchResult{
				Blocked:     true,
				IsCaptcha:   true,
				BlockReason: fmt.Sprintf("sitemap captcha solve unsuccessful: %v", result.Error),
			}, nil
		}

		// Re-fetch sitemap after captcha
		var newBody string
		refetchTasks := chromedp.Tasks{
			chromedp.Navigate(sitemapURL),
			chromedp.Sleep(b.pageLoadDelay),
			chromedp.Evaluate(getSitemapExtractScript(), &newBody),
		}

		if err := chromedp.Run(timeoutCtx, refetchTasks); err != nil {
			return nil, fmt.Errorf("re-fetch sitemap after captcha: %w", err)
		}

		if IsSitemapCaptcha(newBody) {
			return &FetchResult{
				Blocked:     true,
				IsCaptcha:   true,
				BlockReason: "sitemap captcha still present after solving",
			}, nil
		}

		body = newBody
		log.Info().Str("url", sitemapURL).Int("cookies", len(result.Cookies)).Msg("sitemap captcha solved")

		return &FetchResult{
			HTML:    body,
			Cookies: result.Cookies,
		}, nil
	}

	cookies, _ := b.getCookies(timeoutCtx)

	return &FetchResult{
		HTML:    body,
		Cookies: cookies,
	}, nil
}

// solveCaptchaInTab solves captcha in the current browser context
func (b *GlobalBrowser) solveCaptchaInTab(ctx context.Context, url string) (*captcha.SolveResult, error) {
	if b.solver == nil {
		return nil, fmt.Errorf("no captcha solver configured")
	}
	return b.solver.SolveInContext(b.browserCtx, url)
}

// getCookies retrieves current browser cookies
func (b *GlobalBrowser) getCookies(ctx context.Context) ([]captcha.Cookie, error) {
	var cdpCookies []*network.Cookie
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cdpCookies, err = network.GetCookies().Do(ctx)
		return err
	}))
	if err != nil {
		return nil, err
	}

	cookies := make([]captcha.Cookie, len(cdpCookies))
	for i, c := range cdpCookies {
		cookies[i] = captcha.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}
	return cookies, nil
}

// SetCookies sets cookies in the browser
func (b *GlobalBrowser) SetCookies(cookies []captcha.Cookie) error {
	var cdpCookies []*network.CookieParam
	for _, c := range cookies {
		cdpCookies = append(cdpCookies, &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}
	return chromedp.Run(b.browserCtx, network.SetCookies(cdpCookies))
}

// getSitemapExtractScript returns JavaScript code to extract sitemap content
func getSitemapExtractScript() string {
	return `
		(function() {
			var text = document.body.innerText || '';
			var sitemaps = [];
			var urls = [];

			// Yoast SEO: таблица #sitemap с ссылками
			var yoastLinks = document.querySelectorAll('#sitemap a[href]');
			if (yoastLinks.length > 0) {
				yoastLinks.forEach(function(el) {
					var href = el.href;
					if (href.includes('yoa.st') || href.includes('sitemaps.org')) {
						return;
					}
					if (href.includes('category-sitemap') ||
					    href.includes('post_tag-sitemap') ||
					    href.includes('author-sitemap') ||
					    href.includes('tag-sitemap')) {
						return;
					}
					if (href.includes('-sitemap') && href.endsWith('.xml')) {
						sitemaps.push(href);
					} else if (href.startsWith('http')) {
						urls.push(href);
					}
				});
				if (sitemaps.length > 0 || urls.length > 0) {
					return JSON.stringify({type: 'yoast', sitemaps: sitemaps, urls: urls});
				}
			}

			// Chrome XML tree view: ссылки внутри .folder или .tag
			var xmlLinks = document.querySelectorAll('.collapsible-content a, .tag a, span.webkit-html-attribute-value');
			if (xmlLinks.length > 0) {
				xmlLinks.forEach(function(el) {
					var href = el.href || el.textContent || '';
					href = href.replace(/^["']|["']$/g, '');
					if (href.startsWith('http')) {
						if (href.endsWith('.xml') || href.includes('sitemap')) {
							sitemaps.push(href);
						} else {
							urls.push(href);
						}
					}
				});
				if (sitemaps.length > 0 || urls.length > 0) {
					return JSON.stringify({type: 'xml_tree', sitemaps: sitemaps, urls: urls});
				}
			}

			// Fallback: raw XML/text content
			return document.documentElement.outerHTML || text;
		})()
	`
}
