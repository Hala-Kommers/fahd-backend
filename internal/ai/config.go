package ai

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type BotConfig struct {
	ID                 int64   `json:"id"`
	Provider           string  `json:"provider"`
	Model              string  `json:"model"`
	APIKey             string  `json:"-"`
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"maxTokens"`
	Enabled            bool    `json:"enabled"`
	Persona            []byte  `json:"persona"`
	CustomInstructions string  `json:"customInstructions"`
}

func LoadBotConfig(db *gorm.DB) (BotConfig, error) {
	var cfg BotConfig
	err := db.Table("bot_config").
		Select("id, provider, model, api_key, temperature, max_tokens, enabled, persona, custom_instructions").
		Order("updated_at DESC").
		Take(&cfg).Error
	if err != nil {
		return BotConfig{}, fmt.Errorf("load bot config: %w", err)
	}

	cfg.Provider = strings.TrimSpace(strings.ToLower(cfg.Provider))
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1000
	}

	return cfg, nil
}
