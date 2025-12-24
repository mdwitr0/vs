package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port     string
	MongoURL string
	MongoDB  string

	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
	AdminLogin       string
	AdminPassword    string

	// Crawler settings
	CrawlMaxPages     int           // Maximum pages to crawl per site
	CrawlMaxDepth     int           // Maximum recursion depth for link crawling
	CrawlRateLimit    time.Duration // Delay between requests
	ScanIntervalHours int           // Default scan interval in hours for scheduler

	// Parser service
	ParserURL string // URL of browser-based parser service (e.g. http://192.168.2.2:8082)
}

func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8080"),
		MongoURL: getEnv("MONGO_URL", "mongodb://localhost:27017"),
		MongoDB:  getEnv("MONGO_DB", "player_monitor"),

		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  parseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m")),
		JWTRefreshExpiry: parseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h")),
		AdminLogin:       getEnv("ADMIN_LOGIN", "admin"),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),

		CrawlMaxPages:     parseInt(getEnv("CRAWL_MAX_PAGES", "3000000")),
		CrawlMaxDepth:     parseInt(getEnv("CRAWL_MAX_DEPTH", "5")),
		CrawlRateLimit:    parseDuration(getEnv("CRAWL_RATE_LIMIT", "500ms")),
		ScanIntervalHours: parseInt(getEnv("SCAN_INTERVAL_HOURS", "24")),

		ParserURL: getEnv("PARSER_URL", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}
