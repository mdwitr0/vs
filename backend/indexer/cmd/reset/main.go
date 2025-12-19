package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/video-analitics/backend/pkg/meili"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collections = []string{
	"content",
	"sites",
	"pages",
	"scan_tasks",
}

var redisQueues = []string{
	"crawl_tasks",
	"crawl_results",
	"crawl_progress",
	"detect_tasks",
	"detect_results",
	"cancelled_tasks",
}

func main() {
	skipMeili := flag.Bool("skip-meili", false, "Skip clearing Meilisearch")
	skipMongo := flag.Bool("skip-mongo", false, "Skip clearing MongoDB")
	skipRedis := flag.Bool("skip-redis", false, "Skip clearing Redis queues")
	skipNats := flag.Bool("skip-nats", false, "Skip clearing NATS JetStream")
	flag.Parse()

	godotenv.Load()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://192.168.2.2:27018"
	}
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "video_analitics"
	}
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = "http://192.168.2.2:7701"
	}
	meiliKey := os.Getenv("MEILI_KEY")
	if meiliKey == "" {
		meiliKey = "masterKey"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "192.168.2.2:6389"
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://192.168.2.2:4223"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clear MongoDB
	if !*skipMongo {
		log.Println("Connecting to MongoDB...")
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
		if err != nil {
			log.Fatalf("Failed to connect to MongoDB: %v", err)
		}
		defer client.Disconnect(ctx)

		db := client.Database(dbName)
		collections, _ := db.ListCollectionNames(ctx, bson.M{})

		for _, coll := range collections {
			result, err := db.Collection(coll).DeleteMany(ctx, bson.M{})
			if err != nil {
				log.Printf("Failed to clear %s: %v", coll, err)
				continue
			}
			log.Printf("Cleared %s: %d documents deleted", coll, result.DeletedCount)
		}
	}

	// Clear Meilisearch
	if !*skipMeili {
		log.Println("Connecting to Meilisearch...")
		meiliClient, err := meili.New(meiliURL, meiliKey)
		if err != nil {
			log.Printf("Warning: Failed to connect to Meilisearch: %v", err)
		} else {
			if err := meiliClient.DeleteAllDocuments(); err != nil {
				log.Printf("Failed to clear Meilisearch: %v", err)
			} else {
				log.Println("Cleared Meilisearch index")
			}
		}
	}

	// Clear Redis queues
	if !*skipRedis {
		log.Println("Connecting to Redis...")
		rdb := redis.NewClient(&redis.Options{
			Addr: redisURL,
		})
		defer rdb.Close()

		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v", err)
		} else {
			for _, queue := range redisQueues {
				deleted, err := rdb.Del(ctx, queue).Result()
				if err != nil {
					log.Printf("Failed to clear queue %s: %v", queue, err)
					continue
				}
				if deleted > 0 {
					log.Printf("Cleared queue: %s", queue)
				}
			}
			log.Println("Redis queues cleared")
		}
	}

	// Clear NATS JetStream
	if !*skipNats {
		log.Println("Connecting to NATS...")
		nc, err := nats.Connect(natsURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to NATS: %v", err)
		} else {
			defer nc.Close()

			js, err := jetstream.New(nc)
			if err != nil {
				log.Printf("Warning: Failed to create JetStream context: %v", err)
			} else {
				streams := js.ListStreams(ctx)
				deletedCount := 0
				for info := range streams.Info() {
					if err := js.DeleteStream(ctx, info.Config.Name); err != nil {
						log.Printf("Failed to delete stream %s: %v", info.Config.Name, err)
					} else {
						log.Printf("Deleted stream: %s", info.Config.Name)
						deletedCount++
					}
				}
				if streams.Err() != nil {
					log.Printf("Warning: Error listing streams: %v", streams.Err())
				}
				if deletedCount > 0 {
					log.Printf("NATS JetStream cleared: %d streams deleted", deletedCount)
				} else {
					log.Println("NATS JetStream: no streams to delete")
				}
			}
		}
	}

	log.Println("Reset completed!")
}
