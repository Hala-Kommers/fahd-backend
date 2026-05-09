package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	Port           string
	SessionSecret  string
	FrontendOrigin string
	AIProvider     string
	AIAPIKey       string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           getWithDefault("PORT", "8080"),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
		AIProvider:     os.Getenv("AI_PROVIDER"),
		AIAPIKey:       os.Getenv("AI_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("SESSION_SECRET is required")
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
