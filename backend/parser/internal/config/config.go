package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	NatsURL          string
	WorkerCount      int
	MaxBrowserTabs   int
	HTTPPort         string
	InternalAPIToken string
	PageLoadDelay    time.Duration
}

func Load() *Config {
	return &Config{
		NatsURL:          getEnv("NATS_URL", "nats://192.168.2.2:4222"),
		WorkerCount:      getEnvInt("WORKER_COUNT", 5),
		MaxBrowserTabs:   getEnvInt("MAX_BROWSER_TABS", 10),
		HTTPPort:         getEnv("HTTP_PORT", "8082"),
		InternalAPIToken: getEnv("INTERNAL_API_TOKEN", ""),
		PageLoadDelay:    getEnvDuration("PAGE_LOAD_DELAY", 2*time.Second),
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

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
