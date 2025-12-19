package violations

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MatchType string

const (
	MatchByKinopoisk      MatchType = "kinopoisk"
	MatchByIMDB           MatchType = "imdb"
	MatchByMAL            MatchType = "mal"
	MatchByShikimori      MatchType = "shikimori"
	MatchByMyDramaList    MatchType = "mydramalist"
	MatchByTitleYear      MatchType = "title_year"
	MatchByTitle          MatchType = "title"
	MatchByTitleFuzzyYear MatchType = "title_fuzzy_year"
)

type Violation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ContentID string             `bson:"content_id" json:"content_id"`
	SiteID    string             `bson:"site_id" json:"site_id"`
	PageID    string             `bson:"page_id" json:"page_id"`
	PageURL   string             `bson:"page_url" json:"page_url"`
	PageTitle string             `bson:"page_title" json:"page_title"`
	MatchType MatchType          `bson:"match_type" json:"match_type"`
	FoundAt   time.Time          `bson:"found_at" json:"found_at"`
}

type ContentInfo struct {
	ID            string
	Title         string
	OriginalTitle string
	Year          int
	KinopoiskID   string
	IMDBID        string
	MALID         string
	ShikimoriID   string
	MyDramaListID string
}

type PageMatch struct {
	PageID    string
	SiteID    string
	Domain    string
	URL       string
	Title     string
	MatchType MatchType
	IndexedAt time.Time
}

type ContentStats struct {
	ContentID       string   `json:"content_id"`
	ViolationsCount int64    `json:"violations_count"`
	SitesCount      int64    `json:"sites_count"`
	PageIDs         []string `json:"page_ids,omitempty"`
	SiteIDs         []string `json:"site_ids,omitempty"`
}

type SiteStats struct {
	SiteID          string   `json:"site_id"`
	ViolationsCount int64    `json:"violations_count"`
	ContentsCount   int64    `json:"contents_count"`
	PageIDs         []string `json:"page_ids,omitempty"`
	ContentIDs      []string `json:"content_ids,omitempty"`
}
