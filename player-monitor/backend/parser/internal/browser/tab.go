package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/rs/zerolog/log"
)

const defaultTabTimeout = 60 * time.Second

type FetchResult struct {
	HTML     string
	FinalURL string
	Blocked  bool
}

func (b *GlobalBrowser) FetchPage(ctx context.Context, url string) (*FetchResult, error) {
	if err := b.AcquireWithContext(ctx); err != nil {
		return nil, fmt.Errorf("acquire browser slot: %w", err)
	}
	defer b.Release()

	tabCtx, tabCancel := chromedp.NewContext(b.browserCtx)
	defer tabCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, defaultTabTimeout)
	defer timeoutCancel()

	var html string
	var finalURL string

	tasks := chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetExtraHTTPHeaders(network.Headers{
				"Accept-Language":           "ru-RU,ru;q=0.9,en;q=0.8",
				"sec-ch-ua":                 `"Chromium";v="120", "Not=A?Brand";v="24"`,
				"sec-ch-ua-mobile":          "?0",
				"sec-ch-ua-platform":        `"Windows"`,
				"Upgrade-Insecure-Requests": "1",
				"DNT":                       "1",
			}).Do(ctx)
		}),
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("html", &html),
	}

	if err := chromedp.Run(timeoutCtx, tasks); err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}

	log.Debug().Str("url", url).Str("final_url", finalURL).Int("html_len", len(html)).Msg("page fetched")

	return &FetchResult{
		HTML:     html,
		FinalURL: finalURL,
	}, nil
}
