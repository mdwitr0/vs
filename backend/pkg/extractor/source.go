package extractor

import "github.com/PuerkitoBio/goquery"

type SourceExtractor interface {
	Name() string
	Priority() int
	Extract(doc *goquery.Document, rawHTML string) []ExtractedID
}
