package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Page struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SiteID      string             `bson:"site_id" json:"site_id"`
	URL         string             `bson:"url" json:"url"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	MainText    string             `bson:"main_text,omitempty" json:"main_text,omitempty"`
	Year        int                `bson:"year,omitempty" json:"year,omitempty"`
	ExternalIDs ExternalIDs        `bson:"external_ids" json:"external_ids"`
	PlayerURL   string             `bson:"player_url,omitempty" json:"player_url,omitempty"`
	LinksText   string             `bson:"links_text,omitempty" json:"links_text,omitempty"`
	HTTPStatus  int                `bson:"http_status" json:"http_status"`
	IndexedAt   time.Time          `bson:"indexed_at" json:"indexed_at"`
}

type ExternalIDs struct {
	KinopoiskID   string `bson:"kinopoisk_id,omitempty" json:"kinopoisk_id,omitempty"`
	IMDBID        string `bson:"imdb_id,omitempty" json:"imdb_id,omitempty"`
	TMDBID        string `bson:"tmdb_id,omitempty" json:"tmdb_id,omitempty"`
	MALID         string `bson:"mal_id,omitempty" json:"mal_id,omitempty"`
	ShikimoriID   string `bson:"shikimori_id,omitempty" json:"shikimori_id,omitempty"`
	MyDramaListID string `bson:"mydramalist_id,omitempty" json:"mydramalist_id,omitempty"`
}

func (e ExternalIDs) IsEmpty() bool {
	return e.KinopoiskID == "" && e.IMDBID == "" && e.TMDBID == "" && e.MALID == "" && e.ShikimoriID == "" && e.MyDramaListID == ""
}
