package config

import (
	"os"
	"strconv"
	"time"
)

type Env struct {
	ListenAddr       string
	NewAPIURL        string
	DataDir          string
	DBPath           string
	WebDir           string
	AdminPassword    string
	NewAPIAdminToken string
	SessionTTL       time.Duration
	TokenCacheTTL    time.Duration
	EnableScheduler  bool
}

func LoadEnv() Env {
	dataDir := getEnv("GUARD_DATA_DIR", "./data")
	return Env{
		ListenAddr:       getEnv("GUARD_LISTEN_ADDR", ":9000"),
		NewAPIURL:        getEnv("GUARD_NEWAPI_URL", ""),
		DataDir:          dataDir,
		DBPath:           getEnv("GUARD_DB_PATH", dataDir+"/guard.db"),
		WebDir:           getEnv("GUARD_WEB_DIR", "./web"),
		AdminPassword:    getEnv("GUARD_ADMIN_PASSWORD", ""),
		NewAPIAdminToken: getEnv("GUARD_NEWAPI_ADMIN_TOKEN", ""),
		SessionTTL:       getDurationEnv("GUARD_SESSION_TTL", 24*time.Hour),
		TokenCacheTTL:    getDurationEnv("GUARD_TOKEN_CACHE_TTL", 5*time.Minute),
		EnableScheduler:  getBoolEnv("GUARD_ENABLE_SCHEDULER", true),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}
