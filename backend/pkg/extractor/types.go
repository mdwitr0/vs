package extractor

import "github.com/video-analitics/backend/pkg/models"

type IDType string

const (
	IDKinopoisk   IDType = "kinopoisk"
	IDIMDb        IDType = "imdb"
	IDTMDB        IDType = "tmdb"
	IDMAL         IDType = "mal"
	IDShikimori   IDType = "shikimori"
	IDMyDramaList IDType = "mydramalist"
)

type ExtractedID struct {
	Type     IDType
	Value    string
	Source   string
	Priority int
}

type ContentIDs struct {
	Kinopoisk   string
	IMDb        string
	TMDB        string
	MAL         string
	Shikimori   string
	MyDramaList string
}

func (c ContentIDs) ToExternalIDs() models.ExternalIDs {
	return models.ExternalIDs{
		KinopoiskID:   c.Kinopoisk,
		IMDBID:        c.IMDb,
		TMDBID:        c.TMDB,
		MALID:         c.MAL,
		ShikimoriID:   c.Shikimori,
		MyDramaListID: c.MyDramaList,
	}
}

func (c ContentIDs) IsEmpty() bool {
	return c.Kinopoisk == "" && c.IMDb == "" && c.TMDB == "" && c.MAL == "" && c.Shikimori == "" && c.MyDramaList == ""
}
