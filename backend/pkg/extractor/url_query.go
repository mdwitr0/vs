package extractor

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

var (
	kpQueryRegex   = regexp.MustCompile(`[?&](?:kp|kp_id|kinopoisk_id)=(\d+)`)
	imdbQueryRegex = regexp.MustCompile(`[?&](?:imdb|imdb_id)=(tt\d+)`)
	tmdbQueryRegex = regexp.MustCompile(`[?&](?:tmdb|tmdb_id)=(\d+)`)
)

type URLQueryExtractor struct{}

func NewURLQueryExtractor() *URLQueryExtractor {
	return &URLQueryExtractor{}
}

func (e *URLQueryExtractor) Name() string {
	return "url_query"
}

func (e *URLQueryExtractor) Priority() int {
	return 60
}

func (e *URLQueryExtractor) Extract(doc *goquery.Document, rawHTML string) []ExtractedID {
	var results []ExtractedID

	if matches := kpQueryRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDKinopoisk,
			Value:    matches[1],
			Source:   "url_query",
			Priority: 60,
		})
	}

	if matches := imdbQueryRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDIMDb,
			Value:    matches[1],
			Source:   "url_query",
			Priority: 60,
		})
	}

	if matches := tmdbQueryRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDTMDB,
			Value:    matches[1],
			Source:   "url_query",
			Priority: 60,
		})
	}

	return results
}
