package detector

import (
	"context"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	cdpopts "github.com/video-analitics/backend/pkg/chromedp"
	"github.com/video-analitics/backend/pkg/logger"
)

type BrowserFetcher struct {
	timeout time.Duration
}

func NewBrowserFetcher() *BrowserFetcher {
	return &BrowserFetcher{
		timeout: 30 * time.Second,
	}
}

func (f *BrowserFetcher) Fetch(ctx context.Context, targetURL string) *FetchResult {
	opts := cdpopts.GetExecAllocatorOptions()

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, f.timeout)
	defer timeoutCancel()

	var html string
	var finalURL string
	var statusCode int

	err := chromedp.Run(timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, func(ev interface{}) {
				if resp, ok := ev.(*network.EventResponseReceived); ok {
					if resp.Type == network.ResourceTypeDocument {
						statusCode = int(resp.Response.Status)
						finalURL = resp.Response.URL
					}
				}
			})
			return nil
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
		chromedp.Navigate(targetURL),
		chromedp.Sleep(5*time.Second),
		chromedp.OuterHTML("html", &html),
	)

	log := logger.Log
	preview := html
	if len(preview) > 500 {
		preview = preview[:500]
	}
	log.Debug().
		Int("html_len", len(html)).
		Int("status", statusCode).
		Str("final_url", finalURL).
		Str("preview", preview).
		Msg("browser fetch result")

	if err != nil {
		return &FetchResult{
			Error: err,
		}
	}

	if finalURL == "" {
		finalURL = targetURL
	}
	if statusCode == 0 {
		statusCode = 200
	}

	return &FetchResult{
		Body:        []byte(html),
		StatusCode:  statusCode,
		FinalURL:    finalURL,
		ContentType: "text/html",
		Headers:     make(map[string]string),
	}
}

func IsBlockedResponse(html string, statusCode int) bool {
	if statusCode == 403 || statusCode == 429 || statusCode == 503 {
		return true
	}

	if len(html) < 2000 {
		if blockRequestDeniedPattern.MatchString(html) {
			return true
		}
		if blockAccessDeniedPattern.MatchString(html) {
			return true
		}
		if blockErrorTitlePattern.MatchString(html) {
			return true
		}
	}

	return false
}
