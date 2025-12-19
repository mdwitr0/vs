package extractor

import (
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	idextractor "github.com/video-analitics/backend/pkg/extractor"
	"github.com/video-analitics/backend/pkg/models"
)

type Extractor struct{}

func New() *Extractor {
	return &Extractor{}
}

func (e *Extractor) Extract(html string, url string, siteID string, httpStatus int) (*models.Page, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	titleResult := ExtractTitle(doc, html)
	detector := idextractor.NewIDDetector()
	externalIDs := detector.Detect(doc, html).ToExternalIDs()
	playerURL := ExtractPlayerURL(doc, html)
	description := ExtractDescription(doc)
	linksText := ExtractLinksText(doc, html)

	// Создаём копию doc для ExtractMainText (он модифицирует DOM)
	docCopy, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	mainText := ExtractMainText(docCopy)

	page := &models.Page{
		SiteID:      siteID,
		URL:         url,
		Title:       titleResult.Title,
		Description: description,
		MainText:    mainText,
		Year:        titleResult.Year,
		ExternalIDs: externalIDs,
		PlayerURL:   playerURL,
		LinksText:   linksText,
		HTTPStatus:  httpStatus,
		IndexedAt:   time.Now(),
	}

	return page, nil
}
