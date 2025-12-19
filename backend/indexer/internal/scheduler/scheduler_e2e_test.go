//go:build e2e
// +build e2e

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	indexerURL = "http://localhost:8081"
	testDomain = "lordfilmfiwy.lat"
)

type SiteResponse struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	Status          string `json:"status"`
	CMS             string `json:"cms"`
	HasSitemap      bool   `json:"has_sitemap"`
	ScannerType     string `json:"scanner_type"`
	TotalURLsCount  int    `json:"total_urls_count"`
	ViolationsCount int    `json:"violations_count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func TestSiteDetectionFlow_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Delete existing site if any
	t.Log("=== Cleaning up existing test site ===")
	cleanupSite(t, client, testDomain)

	// 2. Create a new site
	t.Log("=== Creating new site ===")
	siteID := createSite(t, client, testDomain)
	t.Logf("Created site with ID: %s", siteID)

	// 3. Verify initial status is 'pending'
	site := getSite(t, client, siteID)
	if site.Status != "pending" {
		t.Fatalf("Expected initial status 'pending', got '%s'", site.Status)
	}
	t.Log("Initial status is 'pending' - correct")

	// 4. Wait for detection to complete (polling)
	t.Log("=== Waiting for detection to complete ===")
	deadline := time.Now().Add(3 * time.Minute)
	var finalSite *SiteResponse

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Context cancelled while waiting for detection")
		default:
		}

		if time.Now().After(deadline) {
			t.Fatalf("Detection did not complete within 3 minutes. Last status: %s", finalSite.Status)
		}

		site := getSite(t, client, siteID)
		t.Logf("Current status: %s, CMS: %s, ScannerType: %s", site.Status, site.CMS, site.ScannerType)

		if site.Status != "pending" {
			finalSite = site
			break
		}

		time.Sleep(5 * time.Second)
	}

	// 5. Verify detection results
	t.Log("=== Detection completed ===")
	t.Logf("Final status: %s", finalSite.Status)
	t.Logf("CMS: %s", finalSite.CMS)
	t.Logf("Scanner Type: %s", finalSite.ScannerType)
	t.Logf("Has Sitemap: %v", finalSite.HasSitemap)

	// Detection should move site to 'active' or 'frozen' (if errors)
	validStatuses := map[string]bool{"active": true, "frozen": true, "scanning": true}
	if !validStatuses[finalSite.Status] {
		t.Errorf("Unexpected final status: %s (expected active, frozen, or scanning)", finalSite.Status)
	}

	// Should have determined scanner type
	if finalSite.ScannerType == "" {
		t.Error("Scanner type was not determined")
	}

	t.Log("=== Site detection flow test PASSED ===")
}

func TestPendingSiteRecovery_E2E(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Get all sites
	resp, err := client.Get(indexerURL + "/api/sites")
	if err != nil {
		t.Fatalf("Failed to get sites: %v", err)
	}
	defer resp.Body.Close()

	var listResp struct {
		Items []SiteResponse `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	pendingCount := 0
	for _, site := range listResp.Items {
		if site.Status == "pending" {
			t.Logf("Found pending site: %s (%s)", site.Domain, site.ID)
			pendingCount++

			// Check if site has been pending for too long (more than 10 minutes)
			// This indicates the detection task was lost
		}
	}

	if pendingCount > 0 {
		t.Logf("WARNING: Found %d sites stuck in 'pending' status", pendingCount)
		t.Log("This may indicate detection tasks were lost from Redis queue")
	} else {
		t.Log("No stuck pending sites found")
	}
}

func cleanupSite(t *testing.T, client *http.Client, domain string) {
	t.Helper()

	// Get sites and find by domain
	resp, err := client.Get(indexerURL + "/api/sites")
	if err != nil {
		t.Logf("Failed to get sites for cleanup: %v", err)
		return
	}
	defer resp.Body.Close()

	var listResp struct {
		Items []SiteResponse `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return
	}

	for _, site := range listResp.Items {
		if site.Domain == domain {
			req, _ := http.NewRequest(http.MethodDelete, indexerURL+"/api/sites/"+site.ID, nil)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				t.Logf("Deleted existing site: %s", site.ID)
			}
		}
	}
}

func createSite(t *testing.T, client *http.Client, domain string) string {
	t.Helper()

	body := fmt.Sprintf(`{"domain":"%s"}`, domain)
	resp, err := client.Post(
		indexerURL+"/api/sites",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Failed to create site: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Failed to create site: %s (status %d)", errResp.Error, resp.StatusCode)
	}

	var site SiteResponse
	if err := json.NewDecoder(resp.Body).Decode(&site); err != nil {
		t.Fatalf("Failed to decode site response: %v", err)
	}

	return site.ID
}

func getSite(t *testing.T, client *http.Client, id string) *SiteResponse {
	t.Helper()

	resp, err := http.Get(indexerURL + "/api/sites/" + id)
	if err != nil {
		t.Fatalf("Failed to get site: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Failed to get site: %s", errResp.Error)
	}

	var site SiteResponse
	if err := json.NewDecoder(resp.Body).Decode(&site); err != nil {
		t.Fatalf("Failed to decode site: %v", err)
	}

	return &site
}
