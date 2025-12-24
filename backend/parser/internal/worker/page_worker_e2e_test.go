//go:build e2e
// +build e2e

package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/video-analitics/backend/pkg/detector"
	"github.com/video-analitics/parser/internal/browser"
)

// TestHTTPFetcher_DirectFetch_E2E tests HTTP fetcher directly without browser
func TestHTTPFetcher_DirectFetch_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fetcher := detector.NewFetcher(detector.WithTimeout(30 * time.Second))

	testCases := []struct {
		name        string
		url         string
		expectBlock bool
		description string
	}{
		{
			name:        "DLE site without captcha",
			url:         "https://lordserialzesty15.top",
			expectBlock: false,
			description: "DLE sites typically work via HTTP",
		},
		{
			name:        "Site with pirate captcha - kinolot",
			url:         "https://kinolot.tv",
			expectBlock: true,
			description: "Site with pirate captcha should be detected as blocked",
		},
		{
			name:        "Site with pirate captcha - narko",
			url:         "https://narko-tv.com",
			expectBlock: true,
			description: "Site with pirate captcha should be detected as blocked",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			result := fetcher.Fetch(ctx, tc.url)
			elapsed := time.Since(start)

			t.Logf("URL: %s", tc.url)
			t.Logf("Description: %s", tc.description)
			t.Logf("Fetch time: %v", elapsed)
			t.Logf("Status code: %d", result.StatusCode)
			t.Logf("Body length: %d", len(result.Body))

			if result.Error != nil {
				t.Logf("Fetch error: %v", result.Error)
			}

			// Check blocking using browser.DetectBlocking
			html := string(result.Body)
			blockResult := browser.DetectBlocking(html, result.StatusCode)

			t.Logf("Blocked: %v", blockResult.Blocked)
			t.Logf("Is Captcha: %v", blockResult.IsCaptcha)
			t.Logf("Block Reason: %s", blockResult.Reason)

			// Show body preview (more for blocked sites)
			preview := html
			maxLen := 500
			if blockResult.Blocked {
				maxLen = 2000
			}
			if len(preview) > maxLen {
				preview = preview[:maxLen]
			}
			t.Logf("Body preview:\n%s", preview)

			if tc.expectBlock {
				if !blockResult.Blocked {
					t.Errorf("Expected page to be blocked, but it wasn't")
				} else {
					t.Logf("OK: Page correctly detected as blocked (reason: %s, is_captcha: %v)", blockResult.Reason, blockResult.IsCaptcha)
				}
			} else {
				if blockResult.Blocked {
					t.Errorf("Expected page NOT to be blocked, but it was: %s", blockResult.Reason)
				}
			}
		})
	}
}

// Note: TestFetchPageHybrid_CaptchaSite_E2E and TestFetchPageHybrid_CookieReuse_E2E
// require browser initialization which only works in Docker environment.
// Run these tests inside Docker container with: go test -tags=e2e -v ./parser/...

func TestHTTPFetcher_BrowserHeaders_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fetcher := detector.NewFetcher(detector.WithTimeout(30 * time.Second))

	// Test against a site that checks headers
	testURL := "https://httpbin.org/headers"

	result := fetcher.Fetch(ctx, testURL)
	if result.Error != nil {
		t.Fatalf("Fetch failed: %v", result.Error)
	}

	body := string(result.Body)
	t.Logf("Response:\n%s", body)

	// Check that browser-like headers are present
	expectedHeaders := []string{
		"sec-ch-ua",
		"Sec-Fetch-Dest",
		"Sec-Fetch-Mode",
		"Upgrade-Insecure-Requests",
	}

	for _, h := range expectedHeaders {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(h)) {
			t.Errorf("Expected header %s not found in request", h)
		}
	}
}
