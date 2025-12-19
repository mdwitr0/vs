package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const maxLinksTextLen = 50000

var urlRegex = regexp.MustCompile(`https?://[^\s"'<>]+`)

func ExtractLinksText(doc *goquery.Document, html string) string {
	var sb strings.Builder
	seen := make(map[string]bool)

	addURL := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "//") {
			return
		}
		seen[url] = true
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(url)
	}

	// 1. href from <a>
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			addURL(href)
		}
	})

	// 2. src from <iframe>, <script>, <img>, <source>, <video>
	doc.Find("iframe[src], script[src], img[src], source[src], video[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			addURL(src)
		}
	})

	// 3. data-src and data-* attributes with URLs or numeric IDs
	doc.Find("[data-src], [data-url], [data-player], [data-file], [data-kp], [data-kinopoisk-id], [data-imdb], [data-mal], [data-shikimori]").Each(func(i int, s *goquery.Selection) {
		for _, attr := range s.Nodes[0].Attr {
			if strings.HasPrefix(attr.Key, "data-") {
				addURL(attr.Val)
			}
		}
	})

	// 4. URLs in JavaScript (simple regex extraction)
	matches := urlRegex.FindAllString(html, -1)
	for _, m := range matches {
		addURL(m)
	}

	result := sb.String()
	if len(result) > maxLinksTextLen {
		result = result[:maxLinksTextLen]
	}

	return result
}
