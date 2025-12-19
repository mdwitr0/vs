package config

import (
	"os"
	"strconv"
)

type Config struct {
	NatsURL          string
	MongoURL         string
	MongoDB          string
	MeiliURL         string
	MeiliKey         string
	WorkerCount      int
	HTTPPort         string
	InternalAPIToken string
}

func Load() *Config {
	return &Config{
		NatsURL:          getEnv("NATS_URL", "nats://192.168.2.2:4222"),
		MongoURL:         getEnv("MONGO_URL", "mongodb://192.168.2.2:27017"),
		MongoDB:          getEnv("MONGO_DB", "video_analitics"),
		MeiliURL:         getEnv("MEILI_URL", "http://192.168.2.2:7700"),
		MeiliKey:         getEnv("MEILI_KEY", "masterKey"),
		WorkerCount:      getEnvInt("WORKER_COUNT", 5),
		HTTPPort:         getEnv("HTTP_PORT", "8082"),
		InternalAPIToken: getEnv("INTERNAL_API_TOKEN", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
