package extractor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	titleYearRegex = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)`)
	yearRegex      = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

type TitleResult struct {
	Title string
	Year  int
}

func ExtractTitle(doc *goquery.Document, html string) TitleResult {
	var result TitleResult

	if ogTitle, exists := doc.Find(`meta[property="og:title"]`).Attr("content"); exists {
		result.Title, result.Year = parseTitle(ogTitle)
		if result.Title != "" {
			return result
		}
	}

	h1 := strings.TrimSpace(doc.Find("h1").First().Text())
	if h1 != "" {
		result.Title, result.Year = parseTitle(h1)
		if result.Title != "" {
			return result
		}
	}

	title := strings.TrimSpace(doc.Find("title").Text())
	if title != "" {
		result.Title, result.Year = parseTitle(title)
		if result.Title != "" {
			return result
		}
	}

	if result.Year == 0 {
		if matches := yearRegex.FindString(html); matches != "" {
			y, _ := strconv.Atoi(matches)
			if isValidYear(y) {
				result.Year = y
			}
		}
	}

	return result
}

func parseTitle(raw string) (title string, year int) {
	raw = strings.TrimSpace(raw)

	// Remove common suffixes
	suffixes := []string{
		" смотреть онлайн",
		" смотреть онлайн бесплатно",
		" бесплатно в хорошем качестве",
		" в хорошем качестве",
	}
	for _, s := range suffixes {
		raw = strings.TrimSuffix(raw, s)
	}

	// Try to match "Title (YYYY)"
	if matches := titleYearRegex.FindStringSubmatch(raw); len(matches) >= 3 {
		title = strings.TrimSpace(matches[1])
		y, _ := strconv.Atoi(matches[2])
		if isValidYear(y) {
			year = y
		}
		return
	}

	// No year found, return cleaned title
	title = raw
	return
}

func isValidYear(year int) bool {
	return year >= 1900 && year <= 2100
}
