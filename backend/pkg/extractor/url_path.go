package extractor

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

var (
	kpPathRegex   = regexp.MustCompile(`/(?:kp|kinopoisk)[/_]?(\d+)`)
	imdbPathRegex = regexp.MustCompile(`/(?:imdb/?)?(tt\d+)`)
	tmdbPathRegex = regexp.MustCompile(`/tmdb[/_]?(\d+)`)
)

type URLPathExtractor struct{}

func NewURLPathExtractor() *URLPathExtractor {
	return &URLPathExtractor{}
}

func (e *URLPathExtractor) Name() string {
	return "url_path"
}

func (e *URLPathExtractor) Priority() int {
	return 70
}

func (e *URLPathExtractor) Extract(doc *goquery.Document, rawHTML string) []ExtractedID {
	var results []ExtractedID

	if matches := kpPathRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDKinopoisk,
			Value:    matches[1],
			Source:   "url_path",
			Priority: 70,
		})
	}

	if matches := imdbPathRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDIMDb,
			Value:    matches[1],
			Source:   "url_path",
			Priority: 70,
		})
	}

	if matches := tmdbPathRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		results = append(results, ExtractedID{
			Type:     IDTMDB,
			Value:    matches[1],
			Source:   "url_path",
			Priority: 70,
		})
	}

	return results
}
