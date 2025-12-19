package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const maxMainTextLength = 10000

var (
	// Паттерн для удаления лишних пробелов и переносов
	whitespaceRegex = regexp.MustCompile(`[\s\n\r\t]+`)
)

// ExtractMainText извлекает основной текстовый контент страницы
// Приоритет: main > article > .content > .post-content > #content > body
func ExtractMainText(doc *goquery.Document) string {
	// Убираем скрипты, стили и другие нетекстовые элементы
	doc.Find("script, style, noscript, iframe, svg, nav, header, footer, aside, .sidebar, .menu, .navigation, .comments, .ad, .advertisement").Remove()

	selectors := []string{
		"main",
		"article",
		".content",
		".post-content",
		".post-body",
		".entry-content",
		".article-content",
		".movie-content",
		".full-story",      // DLE
		".fstory",          // DLE
		"#content",
		".container main",
	}

	for _, sel := range selectors {
		if el := doc.Find(sel).First(); el.Length() > 0 {
			text := cleanText(el.Text())
			if len(text) > 50 { // Минимум 50 символов чтобы считать контент найденным
				return truncate(text, maxMainTextLength)
			}
		}
	}

	// Fallback: берём body
	if body := doc.Find("body").First(); body.Length() > 0 {
		text := cleanText(body.Text())
		return truncate(text, maxMainTextLength)
	}

	return ""
}

// ExtractDescription извлекает description из meta тегов
func ExtractDescription(doc *goquery.Document) string {
	// og:description имеет приоритет
	if desc, exists := doc.Find(`meta[property="og:description"]`).Attr("content"); exists {
		return strings.TrimSpace(desc)
	}

	// Обычный meta description
	if desc, exists := doc.Find(`meta[name="description"]`).Attr("content"); exists {
		return strings.TrimSpace(desc)
	}

	return ""
}

// cleanText очищает текст от лишних пробелов и переносов
func cleanText(s string) string {
	// Заменяем множественные пробелы/переносы на один пробел
	s = whitespaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// truncate обрезает текст до maxLen символов
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Обрезаем по границе слова если возможно
	truncated := s[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen-100 {
		return truncated[:lastSpace]
	}
	return truncated
}
