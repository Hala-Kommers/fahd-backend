package providers

import (
	"fmt"

	"fahd-backend/internal/ai"
)

func NewProvider(cfg ai.BotConfig) (ai.Provider, error) {
	switch cfg.Provider {
	case "google", "gemini":
		return NewGoogleProvider(GoogleConfig{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", cfg.Provider)
	}
}
