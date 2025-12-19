package crawler

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/logger"
)

const maxSitemapDepth = 3

type URLSet struct {
	URLs []URL `xml:"url"`
}

type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

type SitemapIndex struct {
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Sitemap struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type ParsedURL struct {
	Loc        string
	LastMod    *time.Time
	ChangeFreq string
	Priority   float64
}

// ParseSitemap fetches and parses a sitemap via HTTP
func ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	visited := make(map[string]bool)
	return parseSitemapWithDepth(ctx, sitemapURL, 0, visited)
}

// ParseSitemapWithMetadata fetches and parses a sitemap with metadata via HTTP
func ParseSitemapWithMetadata(ctx context.Context, sitemapURL string) ([]ParsedURL, error) {
	visited := make(map[string]bool)
	return parseSitemapWithMetadataAndDepth(ctx, sitemapURL, 0, visited)
}

func parseSitemapWithMetadataAndDepth(ctx context.Context, sitemapURL string, depth int, visited map[string]bool) ([]ParsedURL, error) {
	log := logger.Log

	if depth > maxSitemapDepth {
		return nil, fmt.Errorf("max sitemap depth exceeded: %d", depth)
	}

	if visited[sitemapURL] {
		return nil, nil
	}
	visited[sitemapURL] = true

	log.Debug().Str("url", sitemapURL).Int("depth", depth).Msg("parsing sitemap with metadata")

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var index SitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		log.Debug().Int("count", len(index.Sitemaps)).Msg("sitemap index found")
		var allURLs []ParsedURL
		for _, sm := range index.Sitemaps {
			urls, err := parseSitemapWithMetadataAndDepth(ctx, sm.Loc, depth+1, visited)
			if err != nil {
				log.Debug().Err(err).Str("url", sm.Loc).Msg("nested sitemap failed")
				continue
			}
			allURLs = append(allURLs, urls...)
		}
		log.Debug().Int("total_urls", len(allURLs)).Msg("sitemap index parsed with metadata")
		return allURLs, nil
	}

	var urlset URLSet
	if err := xml.Unmarshal(body, &urlset); err != nil {
		plainURLs := parsePlainTextSitemap(string(body))
		if len(plainURLs) > 0 {
			var urls []ParsedURL
			for _, u := range plainURLs {
				urls = append(urls, ParsedURL{Loc: u})
			}
			log.Debug().Int("accepted", len(urls)).Msg("sitemap parsed as plain text with metadata")
			return urls, nil
		}
		return nil, err
	}

	var urls []ParsedURL
	for _, u := range urlset.URLs {
		parsed := ParsedURL{
			Loc:        u.Loc,
			ChangeFreq: u.ChangeFreq,
		}
		if u.LastMod != "" {
			if t, err := parseLastMod(u.LastMod); err == nil {
				parsed.LastMod = &t
			}
		}
		if u.Priority != "" {
			if p, err := parsePriority(u.Priority); err == nil {
				parsed.Priority = p
			}
		}
		urls = append(urls, parsed)
	}

	log.Debug().Int("total", len(urlset.URLs)).Int("accepted", len(urls)).Msg("sitemap urlset parsed with metadata")
	return urls, nil
}

func parseLastMod(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse lastmod: %s", s)
}

func parsePriority(s string) (float64, error) {
	var p float64
	_, err := fmt.Sscanf(s, "%f", &p)
	return p, err
}

func parseSitemapWithDepth(ctx context.Context, sitemapURL string, depth int, visited map[string]bool) ([]string, error) {
	log := logger.Log

	if depth > maxSitemapDepth {
		return nil, fmt.Errorf("max sitemap depth exceeded: %d", depth)
	}

	if visited[sitemapURL] {
		return nil, nil
	}
	visited[sitemapURL] = true

	log.Debug().Str("url", sitemapURL).Int("depth", depth).Msg("parsing sitemap")

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var index SitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		log.Debug().Int("count", len(index.Sitemaps)).Msg("sitemap index found")
		var allURLs []string
		for _, sm := range index.Sitemaps {
			urls, err := parseSitemapWithDepth(ctx, sm.Loc, depth+1, visited)
			if err != nil {
				log.Debug().Err(err).Str("url", sm.Loc).Msg("nested sitemap failed")
				continue
			}
			allURLs = append(allURLs, urls...)
		}
		log.Debug().Int("total_urls", len(allURLs)).Msg("sitemap index parsed")
		return allURLs, nil
	}

	var urlset URLSet
	if err := xml.Unmarshal(body, &urlset); err != nil {
		urls := parsePlainTextSitemap(string(body))
		if len(urls) > 0 {
			log.Debug().Int("accepted", len(urls)).Msg("sitemap parsed as plain text")
			return urls, nil
		}
		return nil, err
	}

	var urls []string
	for _, u := range urlset.URLs {
		urls = append(urls, u.Loc)
	}

	log.Debug().Int("total", len(urlset.URLs)).Msg("sitemap urlset parsed")
	return urls, nil
}

func parsePlainTextSitemap(body string) []string {
	var urls []string
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			continue
		}
		urls = append(urls, line)
	}

	return urls
}

// SitemapParseResult contains parsed sitemap data with separation of nested sitemaps and page URLs
type SitemapParseResult struct {
	NestedSitemaps []string // URLs of nested sitemaps to process recursively
	PageURLs       []string // Actual page URLs
}

// ParseSitemapXML parses sitemap content from string (no HTTP fetch)
// Supports: XML urlset, sitemap index, JSON from browser script, plain text
// Returns separate lists of nested sitemaps and page URLs for recursive processing
func ParseSitemapXML(body string, sitemapURL string) (*SitemapParseResult, error) {
	log := logger.Log

	// Check if it's JSON from browser sitemap script
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		return parseBrowserSitemapJSON(body, sitemapURL)
	}

	// Try as sitemap index
	var index SitemapIndex
	if err := xml.Unmarshal([]byte(body), &index); err == nil && len(index.Sitemaps) > 0 {
		log.Debug().Int("count", len(index.Sitemaps)).Msg("sitemap index found in XML")
		var sitemaps []string
		for _, sm := range index.Sitemaps {
			sitemaps = append(sitemaps, sm.Loc)
		}
		return &SitemapParseResult{NestedSitemaps: sitemaps}, nil
	}

	// Try as URL set
	var urlset URLSet
	if err := xml.Unmarshal([]byte(body), &urlset); err == nil && len(urlset.URLs) > 0 {
		var urls []string
		for _, u := range urlset.URLs {
			urls = append(urls, u.Loc)
		}
		log.Debug().Int("total", len(urlset.URLs)).Msg("sitemap urlset parsed")
		return &SitemapParseResult{PageURLs: urls}, nil
	}

	// Try plain text format
	urls := parsePlainTextSitemap(body)
	if len(urls) > 0 {
		log.Debug().Int("accepted", len(urls)).Msg("sitemap parsed as plain text")
		return &SitemapParseResult{PageURLs: urls}, nil
	}

	// Try HTML format (Chrome XML tree view or HTML page with links)
	result := parseHTMLSitemap(body, sitemapURL)
	if len(result.PageURLs) > 0 || len(result.NestedSitemaps) > 0 {
		log.Debug().
			Int("urls", len(result.PageURLs)).
			Int("sitemaps", len(result.NestedSitemaps)).
			Msg("sitemap parsed from HTML")
		return result, nil
	}

	return nil, fmt.Errorf("cannot parse sitemap content")
}

// parseBrowserSitemapJSON parses JSON output from browser sitemap extraction script
func parseBrowserSitemapJSON(body string, _ string) (*SitemapParseResult, error) {
	type sitemapJSON struct {
		Type     string   `json:"type"`
		Sitemaps []string `json:"sitemaps"`
		URLs     []string `json:"urls"`
	}

	var data sitemapJSON
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, err
	}

	result := &SitemapParseResult{
		NestedSitemaps: data.Sitemaps,
		PageURLs:       data.URLs,
	}

	logger.Log.Debug().
		Str("type", data.Type).
		Int("sitemaps", len(result.NestedSitemaps)).
		Int("urls", len(result.PageURLs)).
		Msg("browser sitemap JSON parsed")

	return result, nil
}

// parseHTMLSitemap extracts URLs from HTML content (Chrome XML tree view or regular HTML)
func parseHTMLSitemap(body string, sitemapURL string) *SitemapParseResult {
	result := &SitemapParseResult{}
	seen := make(map[string]bool)

	baseParsed, err := url.Parse(sitemapURL)
	if err != nil {
		return result
	}
	baseDomain := baseParsed.Host
	baseOrigin := baseParsed.Scheme + "://" + baseParsed.Host

	// Extract <loc> tags content (Chrome renders XML with visible loc content)
	locRegex := regexp.MustCompile(`<loc[^>]*>([^<]+)</loc>|>loc</[^>]+>[^<]*<[^>]+>([^<]+)<`)
	locMatches := locRegex.FindAllStringSubmatch(body, -1)
	for _, m := range locMatches {
		loc := m[1]
		if loc == "" {
			loc = m[2]
		}
		loc = strings.TrimSpace(loc)
		if loc == "" {
			continue
		}
		if !strings.HasPrefix(loc, "http") {
			continue
		}

		// Заменяем хост на наш домен если отличается
		loc = replaceHostIfDifferent(loc, baseDomain, baseOrigin)

		if seen[loc] {
			continue
		}
		seen[loc] = true

		if strings.Contains(loc, "sitemap") && strings.HasSuffix(loc, ".xml") {
			result.NestedSitemaps = append(result.NestedSitemaps, loc)
		} else {
			result.PageURLs = append(result.PageURLs, loc)
		}
	}

	// Also try to find URLs in text content (Chrome XML tree view shows URLs as text)
	urlRegex := regexp.MustCompile(`https?://[^\s<>"'\` + "`" + `]+`)
	urlMatches := urlRegex.FindAllString(body, -1)
	for _, u := range urlMatches {
		u = strings.TrimRight(u, ".,;:)")

		if !strings.HasPrefix(u, "http") {
			continue
		}

		// Заменяем хост на наш домен если отличается
		u = replaceHostIfDifferent(u, baseDomain, baseOrigin)

		if seen[u] {
			continue
		}
		seen[u] = true

		if strings.Contains(u, "sitemap") && strings.HasSuffix(u, ".xml") {
			result.NestedSitemaps = append(result.NestedSitemaps, u)
		} else {
			result.PageURLs = append(result.PageURLs, u)
		}
	}

	return result
}

// replaceHostIfDifferent заменяет хост в URL на базовый домен если они отличаются
func replaceHostIfDifferent(rawURL, baseDomain, baseOrigin string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.Host != baseDomain {
		// Заменяем хост на наш
		return baseOrigin + parsed.Path
	}
	return rawURL
}

// ExtractLinksFromHTML extracts links from <a> tags in HTML content
func ExtractLinksFromHTML(html, baseURL string) []string {
	var links []string
	seen := make(map[string]bool)

	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	baseOrigin := baseParsed.Scheme + "://" + baseParsed.Host

	// Ссылки из тегов <a> — ловим href в любом месте тега
	aTagRegex := regexp.MustCompile(`(?is)<a\s[^>]*href=["']([^"']+)["']`)
	matches := aTagRegex.FindAllStringSubmatch(html, -1)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		link := strings.TrimSpace(m[1])

		// Пропускаем якоря и javascript
		if strings.HasPrefix(link, "#") || strings.HasPrefix(link, "javascript:") {
			continue
		}

		// Абсолютный путь — добавляем origin
		if strings.HasPrefix(link, "/") {
			link = baseOrigin + link
		} else if !strings.HasPrefix(link, "http") {
			// Относительный путь — пропускаем (сложная логика)
			continue
		}

		linkParsed, err := url.Parse(link)
		if err != nil {
			continue
		}

		// Удаляем якоря (#comment, #section и т.д.)
		linkParsed.Fragment = ""
		link = linkParsed.String()

		// Только ссылки на тот же домен
		if linkParsed.Host != baseParsed.Host {
			continue
		}

		if seen[link] {
			continue
		}

		seen[link] = true
		links = append(links, link)
	}

	return links
}
