package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/video-analitics/backend/pkg/logger"
)

type HTMLCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewHTMLCache(redisURL string, ttl time.Duration) (*HTMLCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &HTMLCache{
		client: client,
		ttl:    ttl,
	}, nil
}

func (c *HTMLCache) Get(ctx context.Context, url string) (string, bool) {
	key := c.makeKey(url)
	html, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false
	}
	if err != nil {
		logger.Log.Debug().Err(err).Str("url", url).Msg("html cache get error")
		return "", false
	}
	return html, true
}

func (c *HTMLCache) Set(ctx context.Context, url string, html string) error {
	key := c.makeKey(url)
	return c.client.Set(ctx, key, html, c.ttl).Err()
}

func (c *HTMLCache) Close() error {
	return c.client.Close()
}

func (c *HTMLCache) makeKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return "html:" + hex.EncodeToString(hash[:])
}
