package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	playerJsRegex = regexp.MustCompile(`file:\s*["']([^"']+)["']`)
	embedRegex    = regexp.MustCompile(`(https?://[^\s"'<>]+(?:embed|player|video)[^\s"'<>]*)`)
)

// TODO: Требует сбора паттернов
var playerSelectors = []string{
	"iframe[src*='player']",
	"iframe[src*='embed']",
	"iframe[data-src*='player']",
	"iframe[data-src*='embed']",
	".player iframe",
	".video-player iframe",
	"#player iframe",
}

func ExtractPlayerURL(doc *goquery.Document, html string) string {
	for _, sel := range playerSelectors {
		if iframe := doc.Find(sel).First(); iframe.Length() > 0 {
			if src := getIframeSrc(iframe); isValidPlayerURL(src) {
				return src
			}
		}
	}

	var playerURL string
	doc.Find("iframe").Each(func(i int, s *goquery.Selection) {
		if playerURL != "" {
			return
		}
		src := getIframeSrc(s)
		if isPlayerURL(src) && isValidPlayerURL(src) {
			playerURL = src
		}
	})
	if playerURL != "" {
		return playerURL
	}

	if matches := playerJsRegex.FindStringSubmatch(html); len(matches) > 1 {
		if isValidPlayerURL(matches[1]) {
			return matches[1]
		}
	}

	if matches := embedRegex.FindStringSubmatch(html); len(matches) > 1 {
		if isValidPlayerURL(matches[1]) {
			return matches[1]
		}
	}

	return ""
}

func isValidPlayerURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func getIframeSrc(s *goquery.Selection) string {
	if src, exists := s.Attr("src"); exists && src != "" && src != "about:blank" {
		return src
	}
	if src, exists := s.Attr("data-src"); exists && src != "" {
		return src
	}
	return ""
}

func isPlayerURL(url string) bool {
	if url == "" {
		return false
	}
	keywords := []string{"player", "embed", "video", "stream", "watch", "cdn", "api"}
	lower := strings.ToLower(url)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
