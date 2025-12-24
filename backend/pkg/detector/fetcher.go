package detector

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type Fetcher struct {
	client    *req.Client
	userAgent string
	cookies   []*http.Cookie
}

type FetcherOption func(*Fetcher)

func WithTimeout(timeout time.Duration) FetcherOption {
	return func(f *Fetcher) {
		f.client.SetTimeout(timeout)
	}
}

func WithUserAgent(ua string) FetcherOption {
	return func(f *Fetcher) {
		f.userAgent = ua
		f.client.SetUserAgent(ua)
	}
}

type CookieData struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  int64
	HTTPOnly bool
	Secure   bool
}

func WithCookies(cookies []CookieData) FetcherOption {
	return func(f *Fetcher) {
		httpCookies := make([]*http.Cookie, len(cookies))
		for i, c := range cookies {
			httpCookies[i] = &http.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  time.Unix(c.Expires, 0),
				HttpOnly: c.HTTPOnly,
				Secure:   c.Secure,
			}
		}
		f.cookies = httpCookies
	}
}

func NewFetcher(opts ...FetcherOption) *Fetcher {
	client := req.C().
		ImpersonateChrome().
		SetTimeout(30 * time.Second).
		SetRedirectPolicy(req.MaxRedirectPolicy(10))

	f := &Fetcher{
		client:    client,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

func (f *Fetcher) Fetch(ctx context.Context, url string) *FetchResult {
	result := &FetchResult{URL: url}

	request := f.client.R().
		SetContext(ctx).
		SetHeaders(map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language":           "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
			"sec-ch-ua":                 `"Chromium";v="131", "Google Chrome";v="131", "Not_A Brand";v="24"`,
			"sec-ch-ua-mobile":          "?0",
			"sec-ch-ua-platform":        `"Windows"`,
			"sec-fetch-dest":            "document",
			"sec-fetch-mode":            "navigate",
			"sec-fetch-site":            "none",
			"sec-fetch-user":            "?1",
			"Upgrade-Insecure-Requests": "1",
			"DNT":                       "1",
		})

	for _, cookie := range f.cookies {
		request.SetCookies(cookie)
	}

	resp, err := request.Get(url)
	if err != nil {
		result.Error = err
		return result
	}

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.GetContentType()

	result.Headers = make(map[string]string)
	for key := range resp.Header {
		result.Headers[strings.ToLower(key)] = resp.Header.Get(key)
	}

	result.Body = resp.Bytes()

	return result
}

func (f *Fetcher) Head(ctx context.Context, url string) *FetchResult {
	result := &FetchResult{URL: url}

	resp, err := f.client.R().
		SetContext(ctx).
		Head(url)
	if err != nil {
		result.Error = err
		return result
	}

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.GetContentType()

	result.Headers = make(map[string]string)
	for key := range resp.Header {
		result.Headers[strings.ToLower(key)] = resp.Header.Get(key)
	}

	return result
}

func (f *Fetcher) CheckURL(ctx context.Context, url string) (exists bool, statusCode int) {
	result := f.Head(ctx, url)
	if result.Error != nil {
		return false, 0
	}
	return result.StatusCode >= 200 && result.StatusCode < 400, result.StatusCode
}
