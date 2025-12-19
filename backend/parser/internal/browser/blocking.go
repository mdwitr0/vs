package browser

import (
	"fmt"
	"regexp"
	"strings"
)

// BlockResult contains blocking detection result
type BlockResult struct {
	Blocked   bool
	IsCaptcha bool
	Reason    string
}

var ipInTitleRegex = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
var titleRegex = regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)

// DetectBlocking checks if HTML response indicates a blocked page or captcha
// Priority order:
// 1. HTTP status code (403, 429, 503)
// 2. Captcha detection (Pirate-style)
// 3. Exact blocking phrases
// 4. IP address in title
// 5. Error title in short HTML
// 6. noindex + empty title + short HTML
func DetectBlocking(html string, statusCode int) BlockResult {
	// Level 1: HTTP status code
	if statusCode == 403 || statusCode == 429 || statusCode == 503 {
		return BlockResult{Blocked: true, Reason: fmt.Sprintf("HTTP %d", statusCode)}
	}

	// Level 2: Captcha detection (must be handled separately)
	if IsPirateCaptcha(html) {
		return BlockResult{Blocked: true, IsCaptcha: true, Reason: "pirate captcha"}
	}

	// Level 3: Exact blocking phrases
	blockingPhrases := []string{
		"Sorry, your request has been denied",
		"Ваш IP заблокирован",
		"Your IP is blocked",
		"Your IP has been blocked",
		"ERR_NAME_NOT_RESOLVED",
		"ERR_CONNECTION_REFUSED",
		"ERR_CONNECTION_TIMED_OUT",
		"Access Denied",
		"403 Forbidden",
	}
	for _, phrase := range blockingPhrases {
		if containsIgnoreCase(html, phrase) {
			return BlockResult{Blocked: true, Reason: phrase}
		}
	}

	// Level 4: IP address in title
	title := extractTitle(html)
	if ipInTitleRegex.MatchString(title) {
		return BlockResult{Blocked: true, Reason: "IP in title"}
	}

	// Level 5: Error/Ошибка title in short HTML (< 10KB)
	if len(html) < 10000 {
		lowerTitle := strings.ToLower(strings.TrimSpace(title))
		if lowerTitle == "error" || lowerTitle == "ошибка" {
			return BlockResult{Blocked: true, Reason: "error title"}
		}
		if strings.Contains(lowerTitle, "не удается получить доступ") {
			return BlockResult{Blocked: true, Reason: "access denied title"}
		}
	}

	// Level 6: noindex + empty/short title + short HTML
	if len(html) < 3000 {
		hasNoindex := containsIgnoreCase(html, "noindex")
		if hasNoindex && strings.TrimSpace(title) == "" {
			return BlockResult{Blocked: true, Reason: "noindex + empty title"}
		}
	}

	return BlockResult{Blocked: false}
}

// IsPirateCaptcha detects pirate-style captcha in HTML
func IsPirateCaptcha(html string) bool {
	if len(html) == 0 {
		return false
	}

	// Button captcha: "Я не робот" with onclick
	hasButton := containsSubstring(html, "Я не робот") && containsSubstring(html, "onclick=")

	// Confirm text captcha
	hasConfirmText := containsSubstring(html, "Подтвердите") &&
		(containsSubstring(html, "человек") || containsSubstring(html, "робот"))

	// Color captcha
	hasColorCaptcha := containsSubstring(html, "похожий цвет") ||
		containsSubstring(html, "нажмите на похожий цвет")

	// Image captcha
	hasImageCaptcha := containsSubstring(html, "похожую картинку") ||
		containsSubstring(html, "нажмите на похожую картинку")

	return hasButton || hasConfirmText || hasColorCaptcha || hasImageCaptcha
}

// IsSitemapCaptcha detects captcha on sitemap page
func IsSitemapCaptcha(body string) bool {
	hasXML := strings.Contains(body, "<?xml") ||
		strings.Contains(body, "<urlset") ||
		strings.Contains(body, "<sitemapindex")

	if hasXML {
		return false
	}

	return IsPirateCaptcha(body) ||
		containsIgnoreCase(body, "access denied") ||
		containsIgnoreCase(body, "captcha")
}

func extractTitle(html string) string {
	match := titleRegex.FindStringSubmatch(html)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}
