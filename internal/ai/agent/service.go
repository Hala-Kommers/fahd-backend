package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/providers"
	"fahd-backend/internal/config"

	"gorm.io/gorm"
)

type Service struct {
	db            *gorm.DB
	appConfig     config.Config
	promptBuilder *PromptBuilder
	conversations *ConversationService
}

type MessageRequest struct {
	ConversationID int64
	Message        string
}

type MessageResponse struct {
	ConversationID int64          `json:"conversationId"`
	Reply          string         `json:"reply"`
	Actions        []any          `json:"actions"`
	Meta           map[string]any `json:"meta"`
}

func NewService(db *gorm.DB, appConfig config.Config) *Service {
	return &Service{
		db:            db,
		appConfig:     appConfig,
		promptBuilder: NewPromptBuilder(),
		conversations: NewConversationService(db),
	}
}

func (s *Service) HandleMessage(ctx context.Context, req MessageRequest) (MessageResponse, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return MessageResponse{}, fmt.Errorf("message is required")
	}

	conversationID, err := s.conversations.StartOrContinue(ctx, req.ConversationID)
	if err != nil {
		slog.Error("sales assistant conversation setup failed", "requested_conversation_id", req.ConversationID, "error", err)
		return MessageResponse{}, err
	}

	cfg, err := ai.LoadBotConfig(s.db.WithContext(ctx))
	if err != nil {
		slog.Error("sales assistant bot config load failed", "conversation_id", conversationID, "error", err)
		return MessageResponse{}, err
	}
	if !cfg.Enabled {
		reply := "Sales assistant is currently unavailable."
		return MessageResponse{
			ConversationID: conversationID,
			Reply:          reply,
			Actions:        []any{},
			Meta:           defaultMeta(false, nil),
		}, nil
	}
	if cfg.APIKey == "" {
		cfg.APIKey = s.appConfig.AIAPIKey
	}
	if cfg.Provider == "" {
		cfg.Provider = s.appConfig.AIProvider
	}
	slog.Debug("sales assistant provider selected", "conversation_id", conversationID, "provider", cfg.Provider, "model", cfg.Model, "enabled", cfg.Enabled, "has_api_key", cfg.APIKey != "")

	if err := s.conversations.SaveMessage(ctx, conversationID, ai.RoleUser, message); err != nil {
		slog.Error("sales assistant user message save failed", "conversation_id", conversationID, "error", err)
		return MessageResponse{}, err
	}

	messages, err := s.conversations.RecentMessages(ctx, conversationID, 20)
	if err != nil {
		slog.Error("sales assistant message history load failed", "conversation_id", conversationID, "error", err)
		return MessageResponse{}, err
	}

	provider, err := providers.NewProvider(cfg)
	if err != nil {
		slog.Error("sales assistant provider creation failed", "conversation_id", conversationID, "provider", cfg.Provider, "error", err)
		return MessageResponse{}, err
	}

	generated, err := provider.Generate(ctx, ai.GenerateRequest{
		SystemPrompt: s.promptBuilder.Build(cfg),
		Messages:     messages,
		Tools:        nil,
		Temperature:  cfg.Temperature,
		MaxTokens:    cfg.MaxTokens,
	})
	if err != nil {
		attrs := []any{"conversation_id", conversationID, "provider", provider.Name(), "model", cfg.Model, "error", err}
		var providerErr ai.ProviderError
		if errors.As(err, &providerErr) {
			attrs = append(attrs, "provider_error_code", providerErr.Code, "status_code", providerErr.StatusCode)
		}
		slog.Error("sales assistant provider generation failed", attrs...)
		return MessageResponse{}, err
	}

	reply := strings.TrimSpace(generated.Content)
	if reply == "" {
		reply = "I can help with products, orders, and checkout. How can I help you?"
	}

	if err := s.conversations.SaveMessage(ctx, conversationID, ai.RoleAssistant, reply); err != nil {
		slog.Error("sales assistant assistant message save failed", "conversation_id", conversationID, "error", err)
		return MessageResponse{}, err
	}

	return MessageResponse{
		ConversationID: conversationID,
		Reply:          reply,
		Actions:        []any{},
		Meta:           defaultMeta(false, nil),
	}, nil
}

func defaultMeta(orderCreated bool, orderID any) map[string]any {
	return map[string]any{
		"needsHuman":   false,
		"orderCreated": orderCreated,
		"orderId":      orderID,
	}
}
