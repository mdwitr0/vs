package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/video-analitics/backend/pkg/logger"
	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/violations"
	"github.com/video-analitics/indexer/internal/repo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	mongoURL := flag.String("mongo", "mongodb://localhost:27017", "MongoDB URL")
	mongoDB := flag.String("db", "video_analitics", "MongoDB database")
	meiliURL := flag.String("meili", "http://localhost:7700", "Meilisearch URL")
	meiliKey := flag.String("meili-key", "masterKey", "Meilisearch API key")
	contentID := flag.String("content", "", "Content ID to recalculate (empty = all)")
	flag.Parse()

	logger.Init(true)
	log := logger.Log

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	meiliClient, err := meili.New(*meiliURL, *meiliKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Meilisearch")
	}
	log.Info().Str("url", *meiliURL).Msg("connected to Meilisearch")

	db := client.Database(*mongoDB)
	contentRepo := repo.NewContentRepo(db)
	violationsSvc := violations.NewService(db, meiliClient)
	violationsSvc.SetContentUpdater(contentRepo)

	if *contentID != "" {
		content, err := contentRepo.FindByID(ctx, *contentID)
		if err != nil {
			log.Fatal().Err(err).Str("id", *contentID).Msg("failed to find content")
		}
		if content == nil {
			log.Fatal().Str("id", *contentID).Msg("content not found")
		}

		log.Info().
			Str("id", *contentID).
			Str("title", content.Title).
			Str("mal_id", content.MALID).
			Int64("violations_before", content.ViolationsCount).
			Msg("recalculating violations for content")

		stats, err := violationsSvc.RefreshForContent(ctx, violations.ContentInfo{
			ID:            content.ID.Hex(),
			Title:         content.Title,
			OriginalTitle: content.OriginalTitle,
			Year:          content.Year,
			KinopoiskID:   content.KinopoiskID,
			IMDBID:        content.IMDBID,
			MALID:         content.MALID,
			ShikimoriID:   content.ShikimoriID,
			MyDramaListID: content.MyDramaListID,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("failed to refresh violations")
		}

		fmt.Printf("\nRecalculation complete for '%s':\n", content.Title)
		fmt.Printf("  Violations: %d -> %d\n", content.ViolationsCount, stats.ViolationsCount)
		fmt.Printf("  Sites: %d -> %d\n", content.SitesCount, stats.SitesCount)
		return
	}

	contents, err := contentRepo.GetAll(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get all contents")
	}

	log.Info().Int("count", len(contents)).Msg("recalculating violations for all contents")

	contentInfos := make([]violations.ContentInfo, len(contents))
	for i, c := range contents {
		contentInfos[i] = violations.ContentInfo{
			ID:            c.ID.Hex(),
			Title:         c.Title,
			OriginalTitle: c.OriginalTitle,
			Year:          c.Year,
			KinopoiskID:   c.KinopoiskID,
			IMDBID:        c.IMDBID,
			MALID:         c.MALID,
			ShikimoriID:   c.ShikimoriID,
			MyDramaListID: c.MyDramaListID,
		}
	}

	updated, err := violationsSvc.RefreshAll(ctx, contentInfos)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to refresh all violations")
	}

	fmt.Printf("\nRecalculation complete: %d contents updated\n", updated)
}
