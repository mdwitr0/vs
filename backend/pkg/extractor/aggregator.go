package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type AggregatorExtractor struct{}

func NewAggregatorExtractor() *AggregatorExtractor {
	return &AggregatorExtractor{}
}

func (e *AggregatorExtractor) Name() string {
	return "aggregator"
}

func (e *AggregatorExtractor) Priority() int {
	return 90
}

func (e *AggregatorExtractor) Extract(doc *goquery.Document, rawHTML string) []ExtractedID {
	var results []ExtractedID

	doc.Find("[data-aggregator]").Each(func(i int, s *goquery.Selection) {
		aggregator := strings.ToLower(s.AttrOr("data-aggregator", ""))
		titleID, hasTitleID := s.Attr("data-title-id")
		if !hasTitleID || titleID == "" {
			return
		}

		var idType IDType
		switch aggregator {
		case "kp", "kinopoisk":
			idType = IDKinopoisk
		case "imdb":
			idType = IDIMDb
		case "tmdb":
			idType = IDTMDB
		case "mal", "myanimelist":
			idType = IDMAL
		case "shikimori":
			idType = IDShikimori
		default:
			return
		}

		results = append(results, ExtractedID{
			Type:     idType,
			Value:    titleID,
			Source:   "aggregator",
			Priority: 90,
		})
	})

	return results
}
