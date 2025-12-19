package detector

import (
	"context"
	"regexp"
	"strings"
)

type SitemapDetector struct {
	fetcher *Fetcher
}

func NewSitemapDetector(fetcher *Fetcher) *SitemapDetector {
	return &SitemapDetector{fetcher: fetcher}
}

type SitemapResult struct {
	HasSitemap    bool
	SitemapStatus SitemapStatus
	SitemapURLs   []string
	IsIndex       bool
	URLsCount     int
}

var locTagRegexp = regexp.MustCompile(`<loc>\s*([^<]+)\s*</loc>`)

func (d *SitemapDetector) Detect(ctx context.Context, baseURL string) SitemapResult {
	result := SitemapResult{
		SitemapStatus: SitemapNone,
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	for _, path := range sitemapPaths {
		sitemapURL := baseURL + path

		fetchResult := d.fetcher.Fetch(ctx, sitemapURL)
		if fetchResult.Error != nil || fetchResult.StatusCode != 200 {
			continue
		}

		body := string(fetchResult.Body)
		if !sitemapXMLPattern.MatchString(body) {
			continue
		}

		result.HasSitemap = true
		result.SitemapURLs = append(result.SitemapURLs, sitemapURL)

		if strings.Contains(body, "<sitemapindex") {
			result.IsIndex = true
			nestedSitemaps := d.extractNestedSitemaps(body)
			result.SitemapURLs = append(result.SitemapURLs, nestedSitemaps...)
		}

		result.URLsCount = d.countURLsInSitemap(body)
		if result.URLsCount > 0 {
			result.SitemapStatus = SitemapValid
		} else {
			result.SitemapStatus = SitemapEmpty
		}

		break
	}

	return result
}

func (d *SitemapDetector) countURLsInSitemap(xmlContent string) int {
	matches := locTagRegexp.FindAllStringSubmatch(xmlContent, -1)
	count := 0
	for _, match := range matches {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			if !strings.HasSuffix(url, ".xml") {
				count++
			}
		}
	}
	return count
}

func (d *SitemapDetector) extractNestedSitemaps(xmlContent string) []string {
	var urls []string

	matches := locTagRegexp.FindAllStringSubmatch(xmlContent, -1)
	for _, match := range matches {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			if strings.HasSuffix(url, ".xml") {
				urls = append(urls, url)
			}
		}
	}

	return urls
}

func (d *SitemapDetector) CheckRobotsTxt(ctx context.Context, baseURL string) []string {
	var sitemapURLs []string

	baseURL = strings.TrimSuffix(baseURL, "/")
	robotsURL := baseURL + "/robots.txt"

	fetchResult := d.fetcher.Fetch(ctx, robotsURL)
	if fetchResult.Error != nil || fetchResult.StatusCode != 200 {
		return sitemapURLs
	}

	lines := strings.Split(string(fetchResult.Body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "sitemap:") {
			url := strings.TrimSpace(line[8:])
			if url != "" {
				sitemapURLs = append(sitemapURLs, url)
			}
		}
	}

	return sitemapURLs
}

func (d *SitemapDetector) ValidateRobotsSitemaps(ctx context.Context, baseURL string) SitemapResult {
	result := SitemapResult{
		SitemapStatus: SitemapNone,
	}

	robotsURLs := d.CheckRobotsTxt(ctx, baseURL)
	if len(robotsURLs) == 0 {
		return result
	}

	for _, sitemapURL := range robotsURLs {
		fetchResult := d.fetcher.Fetch(ctx, sitemapURL)
		if fetchResult.Error != nil || fetchResult.StatusCode != 200 {
			if result.SitemapStatus == SitemapNone {
				result.SitemapStatus = SitemapInvalid
			}
			continue
		}

		body := string(fetchResult.Body)
		if !sitemapXMLPattern.MatchString(body) {
			if result.SitemapStatus == SitemapNone {
				result.SitemapStatus = SitemapInvalid
			}
			continue
		}

		result.HasSitemap = true
		result.SitemapURLs = append(result.SitemapURLs, sitemapURL)

		if strings.Contains(body, "<sitemapindex") {
			result.IsIndex = true
			nestedSitemaps := d.extractNestedSitemaps(body)
			result.SitemapURLs = append(result.SitemapURLs, nestedSitemaps...)
		}

		urlsCount := d.countURLsInSitemap(body)
		result.URLsCount += urlsCount

		if urlsCount > 0 {
			result.SitemapStatus = SitemapValid
		} else if result.SitemapStatus != SitemapValid {
			result.SitemapStatus = SitemapEmpty
		}
	}

	return result
}
