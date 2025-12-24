package crawler

import (
	"regexp"
	"strings"
)

type Detector struct {
	contentIndicators []string
	catalogIndicators []string
}

func NewDetector() *Detector {
	return &Detector{
		contentIndicators: []string{
			"watch", "video", "player", "episode", "movie", "stream",
			"смотреть", "видео", "плеер", "серия", "фильм",
		},
		catalogIndicators: []string{
			"catalog", "list", "category", "genre", "search",
			"каталог", "список", "категория", "жанр", "поиск",
		},
	}
}

func (d *Detector) DetectPageType(html, url string) string {
	htmlLower := strings.ToLower(html)
	urlLower := strings.ToLower(url)

	if strings.Contains(urlLower, "/404") || strings.Contains(htmlLower, "404 not found") || strings.Contains(htmlLower, "страница не найдена") {
		return "error"
	}

	videoTagRegex := regexp.MustCompile(`<video[^>]*>|<iframe[^>]*player[^>]*>`)
	if videoTagRegex.MatchString(htmlLower) {
		return "content"
	}

	contentScore := 0
	catalogScore := 0

	for _, indicator := range d.contentIndicators {
		if strings.Contains(urlLower, indicator) {
			contentScore += 2
		}
		if strings.Contains(htmlLower, indicator) {
			contentScore++
		}
	}

	for _, indicator := range d.catalogIndicators {
		if strings.Contains(urlLower, indicator) {
			catalogScore += 2
		}
		if strings.Contains(htmlLower, indicator) {
			catalogScore++
		}
	}

	if contentScore > catalogScore && contentScore >= 2 {
		return "content"
	}

	if catalogScore > contentScore && catalogScore >= 2 {
		return "catalog"
	}

	return "static"
}
