package extractor

import "github.com/PuerkitoBio/goquery"

type IDDetector struct {
	registry *ExtractorRegistry
}

func NewIDDetector() *IDDetector {
	registry := NewRegistry()

	registry.Register(NewAggregatorExtractor())
	registry.Register(NewDataAttrExtractor())
	registry.Register(NewURLPathExtractor())
	registry.Register(NewURLQueryExtractor())

	return &IDDetector{
		registry: registry,
	}
}

func (d *IDDetector) Detect(doc *goquery.Document, rawHTML string) ContentIDs {
	extracted := d.registry.ExtractAll(doc, rawHTML)

	var ids ContentIDs

	if kp, ok := extracted[IDKinopoisk]; ok {
		ids.Kinopoisk = kp.Value
	}
	if imdb, ok := extracted[IDIMDb]; ok {
		ids.IMDb = imdb.Value
	}
	if tmdb, ok := extracted[IDTMDB]; ok {
		ids.TMDB = tmdb.Value
	}
	if mal, ok := extracted[IDMAL]; ok {
		ids.MAL = mal.Value
	}
	if shikimori, ok := extracted[IDShikimori]; ok {
		ids.Shikimori = shikimori.Value
	}

	return ids
}
