//go:build e2e
// +build e2e

package detector

import (
	"context"
	"strings"
	"testing"
	"time"
)

const testDomain = "kinolot.tv"

func TestDNSChecker_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	checker := NewDNSChecker()

	testCases := []struct {
		domain       string
		shouldResolve bool
		description  string
	}{
		{
			domain:       "fsdafsfasdfsafsadfs.ddddddddd",
			shouldResolve: false,
			description:  "Invalid TLD - should fail immediately",
		},
		{
			domain:       "kinolot.tv",
			shouldResolve: true,
			description:  "Real site with captcha - should resolve",
		},
		{
			domain:       "lordfilmfiwy.lat",
			shouldResolve: true,
			description:  "Real site with DDoS-Guard - should resolve",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.domain, func(t *testing.T) {
			result := checker.Check(ctx, tc.domain)

			t.Logf("Domain: %s", tc.domain)
			t.Logf("Description: %s", tc.description)
			t.Logf("Resolvable: %v", result.Resolvable)
			if result.Error != nil {
				t.Logf("Error: %v", result.Error)
			}
			if len(result.IPs) > 0 {
				t.Logf("IPs: %v", result.IPs)
			}

			if result.Resolvable != tc.shouldResolve {
				t.Errorf("Expected resolvable=%v, got %v", tc.shouldResolve, result.Resolvable)
			}
		})
	}
}

func TestDetector_DNSPreCheck_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	det := New()

	t.Run("Invalid domain fails fast", func(t *testing.T) {
		_, err := det.Detect(ctx, "https://fsdafsfasdfsafsadfs.ddddddddd")
		if err == nil {
			t.Error("Expected error for invalid domain, got nil")
		} else {
			t.Logf("Got expected error: %v", err)
			if !strings.Contains(err.Error(), "not resolvable") {
				t.Errorf("Expected 'not resolvable' error, got: %v", err)
			}
		}
	})

	t.Run("Valid domain with captcha passes DNS", func(t *testing.T) {
		// Только проверяем что DNS проходит, полный Detect может упасть из-за капчи
		checker := NewDNSChecker()
		result := checker.Check(ctx, "kinolot.tv")
		if !result.Resolvable {
			t.Errorf("kinolot.tv should be resolvable, error: %v", result.Error)
		} else {
			t.Logf("kinolot.tv resolved to: %v", result.IPs)
		}
	})

	t.Run("Valid domain with DDoS-Guard passes DNS", func(t *testing.T) {
		checker := NewDNSChecker()
		result := checker.Check(ctx, "lordfilmfiwy.lat")
		if !result.Resolvable {
			t.Errorf("lordfilmfiwy.lat should be resolvable, error: %v", result.Error)
		} else {
			t.Logf("lordfilmfiwy.lat resolved to: %v", result.IPs)
		}
	})
}

func TestDetector_LordFilm_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	det := New()
	domain := "lordfilmfiwy.lat"

	t.Log("=== Starting detection on", domain, "===")

	result, err := det.Detect(ctx, "https://"+domain)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	t.Log("=== Detection Results ===")
	t.Logf("Status Code: %d", result.StatusCode)
	t.Logf("CMS: %s (version: %s)", result.CMS, result.CMSVersion)
	t.Logf("Render Type: %s", result.RenderType)
	t.Logf("Needs Browser: %v", result.NeedsBrowser)
	t.Logf("Captcha Type: %s", result.CaptchaType)
	t.Logf("Has Sitemap: %v", result.HasSitemap)
	t.Logf("Sitemap URLs: %v", result.SitemapURLs)
	t.Logf("Detected By: %v", result.DetectedBy)

	// Проверяем что сайт доступен
	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}

	// Lordfilm должен определяться как DLE
	if result.CMS != CMSDLE {
		t.Errorf("Expected CMS to be DLE, got %s", result.CMS)
	}

	// Должен иметь sitemap
	if !result.HasSitemap {
		t.Error("Expected site to have sitemap")
	}

	// Должен работать через HTTP (не SPA)
	if result.NeedsBrowser {
		t.Error("Expected NeedsBrowser=false for DLE site, but got true")
	}

	t.Logf("Scanner type should be: %s", getScannerType(result))
}

func TestDetector_PirateCaptcha_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	det := New()

	t.Log("=== Starting detection on", testDomain, "===")

	result, err := det.Detect(ctx, "https://"+testDomain)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	t.Log("=== Detection Results ===")
	t.Logf("Status Code: %d", result.StatusCode)
	t.Logf("CMS: %s (version: %s)", result.CMS, result.CMSVersion)
	t.Logf("Render Type: %s", result.RenderType)
	t.Logf("Framework: %s", result.Framework)
	t.Logf("Needs Browser: %v", result.NeedsBrowser)
	t.Logf("Captcha Type: %s", result.CaptchaType)
	t.Logf("Has Sitemap: %v", result.HasSitemap)
	t.Logf("Sitemap URLs: %v", result.SitemapURLs)
	t.Logf("Subdomain Per Movie: %v", result.SubdomainPerMovie)
	t.Logf("Has Redirects: %v", result.HasRedirects)
	t.Logf("Final URL: %s", result.FinalURL)
	t.Logf("Confidence: %.2f", result.Confidence)
	t.Logf("Detected By: %v", result.DetectedBy)

	// Проверяем что получили реальный ответ (не заблокированы)
	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}

	// Проверяем что капча была решена (если была)
	if result.CaptchaType == CaptchaPirate {
		t.Log("Pirate captcha was detected")
		// Проверяем что несмотря на капчу мы получили контент
		hasContentMarkers := false
		for _, marker := range result.DetectedBy {
			if marker == "fetch:captcha_solved" {
				hasContentMarkers = true
				t.Log("Captcha was solved successfully!")
				break
			}
		}
		if !hasContentMarkers {
			t.Log("WARNING: Captcha detected but no 'captcha_solved' marker found")
		}
	}

	// Проверяем что CMS определена
	if result.CMS == CMSUnknown {
		t.Log("WARNING: CMS not detected")
	} else {
		t.Logf("CMS detected: %s", result.CMS)
	}

	// Проверяем sitemap
	if result.HasSitemap {
		t.Logf("Sitemap found! URLs count: %d", len(result.SitemapURLs))
		for i, url := range result.SitemapURLs {
			if i < 3 {
				t.Logf("  - %s", url)
			}
		}
		if len(result.SitemapURLs) > 3 {
			t.Logf("  ... and %d more", len(result.SitemapURLs)-3)
		}
	} else {
		t.Log("No sitemap found")
	}
}

func TestDetector_CompareWithCrawler_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	det := New()

	t.Log("=== Testing detector flow matches crawler expectations ===")

	result, err := det.Detect(ctx, "https://"+testDomain)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Эти значения должны совпадать с тем, что ожидает crawler
	t.Logf("Scanner Type should be: %s", getScannerType(result))
	t.Logf("Captcha Type: %s", result.CaptchaType)
	t.Logf("Has Sitemap: %v", result.HasSitemap)
	t.Logf("Needs Browser: %v", result.NeedsBrowser)

	// Проверяем что есть достаточно информации для краулера
	if result.CaptchaType == CaptchaPirate && !result.NeedsBrowser {
		t.Error("BUG: Pirate captcha detected but NeedsBrowser is false - crawler won't use browser!")
	}

	// Если есть pirate капча, должен быть установлен NeedsBrowser
	if result.CaptchaType == CaptchaPirate {
		if !result.NeedsBrowser {
			t.Error("CRITICAL: Pirate captcha site should have NeedsBrowser=true")
		} else {
			t.Log("OK: NeedsBrowser correctly set for pirate captcha site")
		}
	}
}

func getScannerType(result *Result) string {
	if result.NeedsBrowser {
		return "spa"
	}
	return "http"
}

func TestDetector_BlockedResponse_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Тестируем что IsBlockedResponse правильно определяет блокировку
	testCases := []struct {
		name    string
		html    string
		status  int
		blocked bool
	}{
		{
			name:    "Normal page",
			html:    "<html><head><title>Movie</title></head><body>Content here</body></html>",
			status:  200,
			blocked: false,
		},
		{
			name:    "Access denied",
			html:    "<html><body>Access Denied</body></html>",
			status:  403,
			blocked: true,
		},
		{
			name:    "Cloudflare challenge",
			html:    "<html><body>Just a moment...<div id='cf-browser-verification'></div></body></html>",
			status:  503,
			blocked: true,
		},
		{
			name:    "DDoS protection",
			html:    "<html><body>DDoS protection by Cloudflare</body></html>",
			status:  200,
			blocked: true,
		},
		{
			name:    "Empty denied",
			html:    "denied",
			status:  200,
			blocked: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsBlockedResponse(tc.html, tc.status)
			if result != tc.blocked {
				t.Errorf("IsBlockedResponse() = %v, want %v", result, tc.blocked)
			}
		})
	}

	// Теперь проверяем на реальном сайте
	t.Log("=== Testing on real site ===")
	det := New()
	result, err := det.Detect(ctx, "https://"+testDomain)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Проверяем что мы не получили заблокированный ответ в финальном результате
	for _, marker := range result.DetectedBy {
		if marker == "fetch:browser_blocked" {
			t.Log("WARNING: Site returned blocked response even after browser fallback")
		}
		if marker == "fetch:captcha_solved" {
			t.Log("OK: Captcha was solved, site accessible")
		}
	}
}
