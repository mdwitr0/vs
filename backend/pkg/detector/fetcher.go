package detector

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	client    *http.Client
	userAgent string
	cookies   []*http.Cookie
}

type FetcherOption func(*Fetcher)

func WithTimeout(timeout time.Duration) FetcherOption {
	return func(f *Fetcher) {
		f.client.Timeout = timeout
	}
}

func WithUserAgent(ua string) FetcherOption {
	return func(f *Fetcher) {
		f.userAgent = ua
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
	f := &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

func (f *Fetcher) Fetch(ctx context.Context, url string) *FetchResult {
	result := &FetchResult{URL: url}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.Error = err
		return result
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")

	for _, cookie := range f.cookies {
		req.AddCookie(cookie)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.Header.Get("Content-Type")

	result.Headers = make(map[string]string)
	for key := range resp.Header {
		result.Headers[strings.ToLower(key)] = resp.Header.Get(key)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		result.Error = err
		return result
	}
	result.Body = body

	return result
}

func (f *Fetcher) Head(ctx context.Context, url string) *FetchResult {
	result := &FetchResult{URL: url}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		result.Error = err
		return result
	}

	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.Header.Get("Content-Type")

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
