package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (compatible; PlayerMonitor/1.0)"

type Parser struct {
	client *http.Client
}

func NewParser(client *http.Client) *Parser {
	return &Parser{client: client}
}

func (p *Parser) ParseSitemap(ctx context.Context, domain string) ([]string, error) {
	sitemapURLs := []string{
		fmt.Sprintf("https://%s/sitemap.xml", domain),
		fmt.Sprintf("https://%s/sitemap_index.xml", domain),
		fmt.Sprintf("http://%s/sitemap.xml", domain),
	}

	var allURLs []string
	for _, sitemapURL := range sitemapURLs {
		urls, err := p.fetchSitemap(ctx, sitemapURL)
		if err == nil && len(urls) > 0 {
			allURLs = append(allURLs, urls...)
			break
		}
	}

	if len(allURLs) == 0 {
		return nil, fmt.Errorf("no sitemap found for domain %s", domain)
	}

	return allURLs, nil
}

func (p *Parser) fetchSitemap(ctx context.Context, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(body)

	locRegex := regexp.MustCompile(`<loc>([^<]+)</loc>`)
	matches := locRegex.FindAllStringSubmatch(content, -1)

	var urls []string
	seenURLs := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			if strings.HasSuffix(url, ".xml") {
				subURLs, err := p.fetchSitemap(ctx, url)
				if err == nil {
					for _, subURL := range subURLs {
						if !seenURLs[subURL] {
							seenURLs[subURL] = true
							urls = append(urls, subURL)
						}
					}
				}
			} else {
				if !seenURLs[url] {
					seenURLs[url] = true
					urls = append(urls, url)
				}
			}
		}
	}

	return urls, nil
}

func (p *Parser) FetchPage(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req = req.WithContext(ctx)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (p *Parser) ExtractLinks(html string, baseURL string) ([]string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	hrefRegex := regexp.MustCompile(`<a[^>]+href=["']([^"'#]+)["']`)
	matches := hrefRegex.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	var links []string

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		href := strings.TrimSpace(match[1])
		if href == "" || href == "/" {
			continue
		}

		if strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "mailto:") ||
			strings.HasPrefix(href, "tel:") ||
			strings.HasPrefix(href, "data:") {
			continue
		}

		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}

		resolved := base.ResolveReference(parsed)

		if resolved.Host != "" && resolved.Host != base.Host {
			continue
		}

		resolved.Fragment = ""

		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			resolved.Scheme = base.Scheme
		}

		normalizedURL := resolved.String()

		ext := strings.ToLower(getExtension(resolved.Path))
		if isStaticExtension(ext) {
			continue
		}

		if !seen[normalizedURL] {
			seen[normalizedURL] = true
			links = append(links, normalizedURL)
		}
	}

	return links, nil
}

func getExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx == -1 {
		return ""
	}
	return path[idx:]
}

func isStaticExtension(ext string) bool {
	staticExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true, ".ico": true,
		".css": true, ".js": true, ".json": true, ".xml": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".zip": true, ".rar": true, ".tar": true, ".gz": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".webm": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	}
	return staticExts[ext]
}
