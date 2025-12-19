package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	mongoURL := flag.String("mongo", "mongodb://192.168.2.2:27017", "MongoDB URL")
	mongoDB := flag.String("db", "video_analitics", "MongoDB database")
	meiliURL := flag.String("meili", "http://192.168.2.2:7700", "Meilisearch URL")
	meiliKey := flag.String("meili-key", "masterKey", "Meilisearch API key")
	batchSize := flag.Int("batch", 1000, "Batch size for indexing")
	flag.Parse()

	logger.Init(true)
	log := logger.Log

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURL))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to MongoDB")
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal().Err(err).Msg("failed to ping MongoDB")
	}
	log.Info().Str("url", *mongoURL).Msg("connected to MongoDB")

	// Connect to Meilisearch
	meiliClient, err := meili.New(*meiliURL, *meiliKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Meilisearch")
	}
	log.Info().Str("url", *meiliURL).Msg("connected to Meilisearch")

	// Get pages collection
	collection := client.Database(*mongoDB).Collection("pages")

	// Count total pages
	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to count pages")
	}
	log.Info().Int64("total", total).Msg("pages to sync")

	if total == 0 {
		log.Info().Msg("no pages to sync")
		return
	}

	// Fetch and index in batches
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to query pages")
	}
	defer cursor.Close(ctx)

	var batch []meili.PageDocument
	synced := 0

	for cursor.Next(ctx) {
		var page models.Page
		if err := cursor.Decode(&page); err != nil {
			log.Warn().Err(err).Msg("failed to decode page")
			continue
		}

		// Extract domain from URL
		domain := ""
		if u, err := url.Parse(page.URL); err == nil && u.Host != "" {
			domain = u.Host
		}

		doc := meili.PageDocument{
			ID:            page.ID.Hex(),
			SiteID:        page.SiteID,
			Domain:        domain,
			URL:           page.URL,
			Title:         page.Title,
			Description:   page.Description,
			MainText:      page.MainText,
			Year:          page.Year,
			KinopoiskID:   page.ExternalIDs.KinopoiskID,
			IMDBID:        page.ExternalIDs.IMDBID,
			MALID:         page.ExternalIDs.MALID,
			ShikimoriID:   page.ExternalIDs.ShikimoriID,
			MyDramaListID: page.ExternalIDs.MyDramaListID,
			LinksText:     page.LinksText,
			PlayerURLs:    []string{page.PlayerURL},
			IndexedAt:     page.IndexedAt.Format(time.RFC3339),
		}

		batch = append(batch, doc)

		if len(batch) >= *batchSize {
			if err := meiliClient.IndexPages(batch); err != nil {
				log.Error().Err(err).Int("count", len(batch)).Msg("failed to index batch")
			} else {
				synced += len(batch)
				log.Info().Int("synced", synced).Int64("total", total).Msg("batch indexed")
			}
			batch = batch[:0]
		}
	}

	// Index remaining pages
	if len(batch) > 0 {
		if err := meiliClient.IndexPages(batch); err != nil {
			log.Error().Err(err).Int("count", len(batch)).Msg("failed to index final batch")
		} else {
			synced += len(batch)
		}
	}

	fmt.Printf("\nSync completed: %d/%d pages indexed\n", synced, total)
}
