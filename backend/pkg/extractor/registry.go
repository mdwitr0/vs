package extractor

import (
	"sort"

	"github.com/PuerkitoBio/goquery"
)

type ExtractorRegistry struct {
	extractors []SourceExtractor
}

func NewRegistry() *ExtractorRegistry {
	return &ExtractorRegistry{
		extractors: make([]SourceExtractor, 0),
	}
}

func (r *ExtractorRegistry) Register(e SourceExtractor) {
	r.extractors = append(r.extractors, e)
	sort.Slice(r.extractors, func(i, j int) bool {
		return r.extractors[i].Priority() > r.extractors[j].Priority()
	})
}

func (r *ExtractorRegistry) ExtractAll(doc *goquery.Document, rawHTML string) map[IDType]ExtractedID {
	results := make(map[IDType]ExtractedID)

	for _, extractor := range r.extractors {
		ids := extractor.Extract(doc, rawHTML)
		for _, id := range ids {
			if !ValidateID(id.Type, id.Value) {
				continue
			}
			existing, exists := results[id.Type]
			if !exists || id.Priority > existing.Priority {
				results[id.Type] = id
			}
		}
	}

	return results
}
