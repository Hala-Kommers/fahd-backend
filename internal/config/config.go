package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	LogLevel       string
	DatabaseURL    string
	Port           string
	JWTSecret      string
	FrontendOrigin string
	AIProvider     string
	AIAPIKey       string
	SessionTTL     string
	WSQueueSize    int
	WorkerCount    int
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		AppEnv:         getWithDefault("APP_ENV", "development"),
		LogLevel:       getWithDefault("LOG_LEVEL", "info"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           getWithDefault("PORT", "8080"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
		AIProvider:     os.Getenv("AI_PROVIDER"),
		AIAPIKey:       os.Getenv("AI_API_KEY"),
		SessionTTL:     getWithDefault("SESSION_TTL", "1h"),
		WSQueueSize:    100,
		WorkerCount:    2,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.FrontendOrigin == "" {
		return Config{}, fmt.Errorf("FRONTEND_ORIGIN is required")
	}
	if cfg.AIProvider == "" {
		return Config{}, fmt.Errorf("AI_PROVIDER is required")
	}
	if cfg.AIAPIKey == "" {
		return Config{}, fmt.Errorf("AI_API_KEY is required")
	}

	return cfg, nil
}

func getWithDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
