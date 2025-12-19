package config

import (
	"os"
	"time"
)

type Config struct {
	Port     string
	NatsURL  string
	MongoURL string
	MongoDB  string
	MeiliURL string
	MeiliKey string

	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
	AdminLogin       string
	AdminPassword    string

	InternalAPIToken string
}

func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8080"),
		NatsURL:  getEnv("NATS_URL", "nats://192.168.2.2:4222"),
		MongoURL: getEnv("MONGO_URL", "mongodb://192.168.2.2:27017"),
		MongoDB:  getEnv("MONGO_DB", "video_analitics"),
		MeiliURL: getEnv("MEILI_URL", "http://192.168.2.2:7700"),
		MeiliKey: getEnv("MEILI_KEY", "masterKey"),

		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  parseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m")),
		JWTRefreshExpiry: parseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h")),
		AdminLogin:       getEnv("ADMIN_LOGIN", "admin"),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),

		InternalAPIToken: getEnv("INTERNAL_API_TOKEN", ""),
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
