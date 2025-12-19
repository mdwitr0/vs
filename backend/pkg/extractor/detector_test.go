package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestAggregatorExtractor_KP(t *testing.T) {
	html := `<video-player data-title-id="401522" data-aggregator="kp"></video-player>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "401522" {
		t.Errorf("Expected KPID 401522, got %s", ids.Kinopoisk)
	}
}

func TestAggregatorExtractor_KinopoiskAlias(t *testing.T) {
	html := `<div data-title-id="123456" data-aggregator="kinopoisk"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "123456" {
		t.Errorf("Expected KPID 123456, got %s", ids.Kinopoisk)
	}
}

func TestDataAttrExtractor_DataKP(t *testing.T) {
	html := `<div data-kp="789123"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "789123" {
		t.Errorf("Expected KPID 789123, got %s", ids.Kinopoisk)
	}
}

func TestURLPathExtractor_KP(t *testing.T) {
	html := `<iframe src="/embed/kp/555666"></iframe>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "555666" {
		t.Errorf("Expected KPID 555666, got %s", ids.Kinopoisk)
	}
}

func TestURLQueryExtractor_KPID(t *testing.T) {
	html := `<a href="?kp_id=333444">Link</a>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "333444" {
		t.Errorf("Expected KPID 333444, got %s", ids.Kinopoisk)
	}
}

func TestPriorityAggregatorWins(t *testing.T) {
	html := `
		<video-player data-title-id="111111" data-aggregator="kp"></video-player>
		<div data-kp="222222"></div>
		<iframe src="/embed/kp/333333"></iframe>
	`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "111111" {
		t.Errorf("Expected aggregator to win with 111111, got %s", ids.Kinopoisk)
	}
}

func TestIMDbExtraction(t *testing.T) {
	html := `<div data-imdb="tt1234567"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.IMDb != "tt1234567" {
		t.Errorf("Expected IMDb tt1234567, got %s", ids.IMDb)
	}
}

func TestValidation_InvalidKPID(t *testing.T) {
	html := `<div data-kp="abc"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "" {
		t.Errorf("Expected empty KPID for invalid value, got %s", ids.Kinopoisk)
	}
}

func TestValidation_TooLongKPID(t *testing.T) {
	html := `<div data-kp="123456789012345"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	detector := NewIDDetector()
	ids := detector.Detect(doc, html)

	if ids.Kinopoisk != "" {
		t.Errorf("Expected empty KPID for too long value, got %s", ids.Kinopoisk)
	}
}

func TestToExternalIDs(t *testing.T) {
	ids := ContentIDs{
		Kinopoisk: "401522",
		IMDb:      "tt1234567",
	}

	external := ids.ToExternalIDs()

	if external.KinopoiskID != "401522" {
		t.Errorf("Expected KinopoiskID 401522, got %s", external.KinopoiskID)
	}
	if external.IMDBID != "tt1234567" {
		t.Errorf("Expected IMDBID tt1234567, got %s", external.IMDBID)
	}
}
