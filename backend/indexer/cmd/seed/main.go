package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/video-analitics/backend/pkg/meili"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Content struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Title         string             `bson:"title"`
	OriginalTitle string             `bson:"original_title,omitempty"`
	Year          int                `bson:"year,omitempty"`
	KinopoiskID   string             `bson:"kinopoisk_id,omitempty"`
	IMDBID        string             `bson:"imdb_id,omitempty"`
	MALID         string             `bson:"mal_id,omitempty"`
	ShikimoriID   string             `bson:"shikimori_id,omitempty"`
	MyDramaListID string             `bson:"mydramalist_id,omitempty"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type Site struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Domain        string             `bson:"domain"`
	Status        string             `bson:"status"`
	CMS           string             `bson:"cms,omitempty"`
	HasSitemap    bool               `bson:"has_sitemap"`
	SitemapURLs   []string           `bson:"sitemap_urls,omitempty"`
	LastScanAt    *time.Time         `bson:"last_scan_at,omitempty"`
	NextScanAt    *time.Time         `bson:"next_scan_at,omitempty"`
	FailureCount  int                `bson:"failure_count"`
	ScanIntervalH int                `bson:"scan_interval_h"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type ExternalIDs struct {
	KinopoiskID   string `bson:"kinopoisk_id,omitempty"`
	IMDBID        string `bson:"imdb_id,omitempty"`
	TMDBID        string `bson:"tmdb_id,omitempty"`
	MALID         string `bson:"mal_id,omitempty"`
	ShikimoriID   string `bson:"shikimori_id,omitempty"`
	MyDramaListID string `bson:"mydramalist_id,omitempty"`
}

type Page struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	SiteID      string             `bson:"site_id"`
	Domain      string             `bson:"-"` // не сохраняем в MongoDB, только для Meili
	URL         string             `bson:"url"`
	Title       string             `bson:"title"`
	Description string             `bson:"description,omitempty"`
	MainText    string             `bson:"main_text,omitempty"`
	Year        int                `bson:"year,omitempty"`
	ExternalIDs ExternalIDs        `bson:"external_ids"`
	PlayerURL   string             `bson:"player_url,omitempty"`
	LinksText   string             `bson:"links_text,omitempty"`
	HTTPStatus  int                `bson:"http_status"`
	IndexedAt   time.Time          `bson:"indexed_at"`
}

func main() {
	clearMeili := flag.Bool("clear-meili", false, "Clear all documents from Meilisearch")
	flag.Parse()

	godotenv.Load()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27018"
	}
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "video_analitics"
	}
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = "http://localhost:7700"
	}
	meiliKey := os.Getenv("MEILI_KEY")
	if meiliKey == "" {
		meiliKey = "masterKey"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	// Meilisearch client
	meiliClient, err := meili.New(meiliURL, meiliKey)
	if err != nil {
		log.Printf("Warning: Failed to connect to Meilisearch: %v", err)
		meiliClient = nil
	} else {
		log.Printf("Connected to Meilisearch at %s", meiliURL)
	}

	// Clear Meilisearch if requested
	if *clearMeili && meiliClient != nil {
		log.Println("Clearing Meilisearch index...")
		if err := meiliClient.DeleteAllDocuments(); err != nil {
			log.Printf("Failed to clear Meilisearch: %v", err)
		} else {
			log.Println("Meilisearch index cleared")
		}
		return
	}

	log.Println("Seeding content...")
	seedContent(ctx, db)

	log.Println("Seeding sites...")
	sites := seedSites(ctx, db)

	log.Println("Seeding pages...")
	seedPages(ctx, db, sites, meiliClient)

	log.Println("Seeding completed!")
}

func seedContent(ctx context.Context, db *mongo.Database) {
	coll := db.Collection("content")

	contents := []interface{}{
		Content{
			Title:         "Кибердеревня",
			OriginalTitle: "Cyber Village",
			Year:          2023,
			KinopoiskID:          "5019944",
			IMDBID:        "tt23805348",
			CreatedAt:     time.Now(),
		},
		Content{
			Title:         "Слово пацана. Кровь на асфальте",
			OriginalTitle: "Boy's Word: Blood on the Asphalt",
			Year:          2023,
			KinopoiskID:          "5113274",
			IMDBID:        "tt27765443",
			CreatedAt:     time.Now(),
		},
		Content{
			Title:         "Майор Гром: Чумной Доктор",
			OriginalTitle: "Major Grom: Plague Doctor",
			Year:          2021,
			KinopoiskID:          "1236063",
			IMDBID:        "tt10850932",
			CreatedAt:     time.Now(),
		},
	}

	result, err := coll.InsertMany(ctx, contents)
	if err != nil {
		log.Printf("Warning: Failed to insert content (may already exist): %v", err)
		return
	}
	log.Printf("Inserted %d content items", len(result.InsertedIDs))
}

// SiteInfo содержит ID и домен сайта
type SiteInfo struct {
	ID     string
	Domain string
}

func seedSites(ctx context.Context, db *mongo.Database) []SiteInfo {
	coll := db.Collection("sites")
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)

	sitesData := []Site{
		{
			Domain:        "kinogo.media",
			Status:        "active",
			CMS:           "DLE",
			HasSitemap:    true,
			SitemapURLs:   []string{"https://kinogo.media/sitemap.xml"},
			LastScanAt:    &now,
			NextScanAt:    &now,
			ScanIntervalH: 24,
			CreatedAt:     weekAgo,
		},
		{
			Domain:        "lordfilm.org",
			Status:        "active",
			CMS:           "WordPress",
			HasSitemap:    true,
			SitemapURLs:   []string{"https://lordfilm.org/sitemap.xml"},
			LastScanAt:    &yesterday,
			NextScanAt:    &now,
			ScanIntervalH: 12,
			CreatedAt:     weekAgo,
		},
		{
			Domain:        "hdrezka.ag",
			Status:        "active",
			CMS:           "Custom",
			HasSitemap:    false,
			LastScanAt:    &weekAgo,
			NextScanAt:    &now,
			ScanIntervalH: 24,
			CreatedAt:     weekAgo,
		},
		{
			Domain:        "dead-site.com",
			Status:        "dead",
			HasSitemap:    false,
			FailureCount:  3,
			ScanIntervalH: 24,
			CreatedAt:     weekAgo,
		},
		// Real site for testing - has Кибердеревня
		{
			Domain:        "lordfilmfiwy.lat",
			Status:        "active",
			CMS:           "DLE",
			HasSitemap:    true,
			SitemapURLs:   []string{"https://lordfilmfiwy.lat/uploads/domains/lordfilmfiwy.lat_news_pages.xml"},
			NextScanAt:    &now,
			ScanIntervalH: 24,
			CreatedAt:     now,
		},
	}

	// Convert to interface{} for InsertMany
	sites := make([]interface{}, len(sitesData))
	for i := range sitesData {
		sites[i] = sitesData[i]
	}

	result, err := coll.InsertMany(ctx, sites)
	if err != nil {
		log.Printf("Warning: Failed to insert sites (may already exist): %v", err)
		return nil
	}
	log.Printf("Inserted %d sites", len(result.InsertedIDs))

	var infos []SiteInfo
	for i, id := range result.InsertedIDs {
		infos = append(infos, SiteInfo{
			ID:     id.(primitive.ObjectID).Hex(),
			Domain: sitesData[i].Domain,
		})
	}
	return infos
}

func seedPages(ctx context.Context, db *mongo.Database, sites []SiteInfo, meiliClient *meili.Client) {
	if len(sites) < 3 {
		log.Println("Not enough sites to seed pages")
		return
	}

	coll := db.Collection("pages")
	now := time.Now()

	pagesData := []Page{
		// kinogo.media - has Кибердеревня (violation)
		{
			SiteID:      sites[0].ID,
			Domain:      sites[0].Domain,
			URL:         "https://kinogo.media/serial/kiberderevnya-2023.html",
			Title:       "Кибердеревня (2023) смотреть онлайн",
			Description: "Смотреть сериал Кибердеревня онлайн бесплатно в хорошем качестве",
			MainText:    "Кибердеревня - российский фантастический комедийный сериал о программисте, который переезжает в деревню будущего",
			Year:        2023,
			ExternalIDs: ExternalIDs{KinopoiskID: "5019944", IMDBID: "tt23805348"},
			PlayerURL:   "https://kinogo.media/player/12345",
			HTTPStatus:  200,
			IndexedAt:   now,
		},
		{
			SiteID:      sites[0].ID,
			Domain:      sites[0].Domain,
			URL:         "https://kinogo.media/serial/slovo-pacana-2023.html",
			Title:       "Слово пацана. Кровь на асфальте (2023)",
			Description: "Криминальная драма о подростковых бандах в Казани",
			Year:        2023,
			ExternalIDs: ExternalIDs{KinopoiskID: "5113274"},
			PlayerURL:   "https://kinogo.media/player/12346",
			HTTPStatus:  200,
			IndexedAt:   now,
		},
		{
			SiteID:      sites[0].ID,
			Domain:      sites[0].Domain,
			URL:         "https://kinogo.media/film/random-film.html",
			Title:       "Случайный фильм без отслеживания",
			Year:        2022,
			ExternalIDs: ExternalIDs{KinopoiskID: "9999999"},
			HTTPStatus:  200,
			IndexedAt:   now,
		},

		// lordfilm.org - has Кибердеревня (violation)
		{
			SiteID:      sites[1].ID,
			Domain:      sites[1].Domain,
			URL:         "https://lordfilm.org/serial/kiberderevnya.html",
			Title:       "Кибердеревня 1 сезон",
			Description: "Кибердеревня все серии смотреть онлайн",
			MainText:    "Сериал Кибердеревня рассказывает о приключениях программиста Николая в футуристической деревне",
			Year:        2023,
			ExternalIDs: ExternalIDs{KinopoiskID: "5019944"},
			PlayerURL:   "https://lordfilm.org/player/abc",
			HTTPStatus:  200,
			IndexedAt:   now,
		},
		{
			SiteID:      sites[1].ID,
			Domain:      sites[1].Domain,
			URL:         "https://lordfilm.org/film/major-grom.html",
			Title:       "Майор Гром: Чумной Доктор",
			Description: "Российский супергеройский фильм",
			Year:        2021,
			ExternalIDs: ExternalIDs{KinopoiskID: "1236063", IMDBID: "tt10850932"},
			PlayerURL:   "https://lordfilm.org/player/def",
			HTTPStatus:  200,
			IndexedAt:   now,
		},

		// hdrezka.ag - no violations
		{
			SiteID:      sites[2].ID,
			Domain:      sites[2].Domain,
			URL:         "https://hdrezka.ag/films/comedy/random.html",
			Title:       "Какая-то комедия",
			Year:        2024,
			ExternalIDs: ExternalIDs{KinopoiskID: "8888888"},
			HTTPStatus:  200,
			IndexedAt:   now,
		},
		{
			SiteID:      sites[2].ID,
			Domain:      sites[2].Domain,
			URL:         "https://hdrezka.ag/films/action/another.html",
			Title:       "Какой-то боевик",
			Year:        2023,
			ExternalIDs: ExternalIDs{},
			HTTPStatus:  200,
			IndexedAt:   now,
		},
	}

	// Convert to interface{} for InsertMany
	pages := make([]interface{}, len(pagesData))
	for i := range pagesData {
		pages[i] = pagesData[i]
	}

	result, err := coll.InsertMany(ctx, pages)
	if err != nil {
		log.Printf("Warning: Failed to insert pages (may already exist): %v", err)
		return
	}
	log.Printf("Inserted %d pages into MongoDB", len(result.InsertedIDs))

	// Index in Meilisearch
	if meiliClient != nil {
		var meiliDocs []meili.PageDocument
		for i, id := range result.InsertedIDs {
			p := pagesData[i]
			meiliDocs = append(meiliDocs, meili.PageDocument{
				ID:            id.(primitive.ObjectID).Hex(),
				SiteID:        p.SiteID,
				Domain:        p.Domain,
				URL:           p.URL,
				Title:         p.Title,
				Description:   p.Description,
				MainText:      p.MainText,
				Year:          p.Year,
				KinopoiskID:   p.ExternalIDs.KinopoiskID,
				IMDBID:        p.ExternalIDs.IMDBID,
				MALID:         p.ExternalIDs.MALID,
				ShikimoriID:   p.ExternalIDs.ShikimoriID,
				MyDramaListID: p.ExternalIDs.MyDramaListID,
				LinksText:     p.LinksText,
				PlayerURLs:    []string{p.PlayerURL},
				IndexedAt:     p.IndexedAt.Format(time.RFC3339),
			})
		}

		if err := meiliClient.IndexPages(meiliDocs); err != nil {
			log.Printf("Warning: Failed to index pages in Meilisearch: %v", err)
		} else {
			log.Printf("Indexed %d pages in Meilisearch", len(meiliDocs))
		}
	}
}
