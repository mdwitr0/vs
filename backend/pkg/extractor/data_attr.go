package extractor

import "github.com/PuerkitoBio/goquery"

type DataAttrExtractor struct{}

func NewDataAttrExtractor() *DataAttrExtractor {
	return &DataAttrExtractor{}
}

func (e *DataAttrExtractor) Name() string {
	return "data_attr"
}

func (e *DataAttrExtractor) Priority() int {
	return 80
}

func (e *DataAttrExtractor) Extract(doc *goquery.Document, rawHTML string) []ExtractedID {
	var results []ExtractedID

	kpAttrs := []string{"data-kp", "data-kinopoisk-id", "data-kpid", "data-kinopoisk"}
	doc.Find("[data-kp], [data-kinopoisk-id], [data-kpid], [data-kinopoisk]").Each(func(i int, s *goquery.Selection) {
		for _, attr := range kpAttrs {
			if val, exists := s.Attr(attr); exists && val != "" {
				results = append(results, ExtractedID{
					Type:     IDKinopoisk,
					Value:    val,
					Source:   "data_attr",
					Priority: 80,
				})
				return
			}
		}
	})

	imdbAttrs := []string{"data-imdb", "data-imdb-id"}
	doc.Find("[data-imdb], [data-imdb-id]").Each(func(i int, s *goquery.Selection) {
		for _, attr := range imdbAttrs {
			if val, exists := s.Attr(attr); exists && val != "" {
				results = append(results, ExtractedID{
					Type:     IDIMDb,
					Value:    val,
					Source:   "data_attr",
					Priority: 80,
				})
				return
			}
		}
	})

	tmdbAttrs := []string{"data-tmdb", "data-tmdb-id"}
	doc.Find("[data-tmdb], [data-tmdb-id]").Each(func(i int, s *goquery.Selection) {
		for _, attr := range tmdbAttrs {
			if val, exists := s.Attr(attr); exists && val != "" {
				results = append(results, ExtractedID{
					Type:     IDTMDB,
					Value:    val,
					Source:   "data_attr",
					Priority: 80,
				})
				return
			}
		}
	})

	return results
}
