package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/providers"
	aitools "fahd-backend/internal/ai/tools"
	"fahd-backend/internal/config"

	"gorm.io/gorm"
)

type AgentAction struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

var actionRe = regexp.MustCompile(`\[ACTION:(\w+)(?::([^\]]+))?\]`)

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

func parseActions(text string) (string, []AgentAction) {
	actions := []AgentAction{}
	cleaned := actionRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := actionRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		actionType := parts[1]
		switch actionType {
		case "address_form":
			actions = append(actions, AgentAction{Type: "address_form"})
		case "order_confirmation":
			actions = append(actions, AgentAction{Type: "order_confirmation"})
		case "show_product":
			if len(parts) >= 3 && parts[2] != "" {
				if id, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
					actions = append(actions, AgentAction{Type: "show_product", Payload: map[string]any{"productId": id}})
				}
			}
		}
		return ""
	})
	cleaned = strings.TrimSpace(cleaned)
	return cleaned, actions
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
	toolRegistry := aitools.NewRegistry(s.db.WithContext(ctx))
	toolDefinitions := toolRegistry.Definitions()
	slog.Debug("sales assistant tools registered", "conversation_id", conversationID, "tool_count", len(toolDefinitions))

	systemPrompt := s.promptBuilder.Build(cfg)
	providerName := provider.Name()
	var generated ai.GenerateResponse
	var allToolCalls []ai.ToolCall
	var allToolResults []ai.ToolResult
	var reply string
	for step := 0; step < 8; step++ {
		generated, err = provider.Generate(ctx, ai.GenerateRequest{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        toolDefinitions,
			Temperature:  cfg.Temperature,
			MaxTokens:    cfg.MaxTokens,
		})
		if err != nil {
			attrs := []any{"conversation_id", conversationID, "provider", providerName, "model", cfg.Model, "error", err}
			var providerErr ai.ProviderError
			if errors.As(err, &providerErr) {
				attrs = append(attrs, "provider_error_code", providerErr.Code, "status_code", providerErr.StatusCode)
			}
			slog.Error("sales assistant provider generation failed", attrs...)
			return MessageResponse{}, err
		}

		if len(generated.ToolCalls) == 0 {
			reply = strings.TrimSpace(generated.Content)
			break
		}

		allToolCalls = append(allToolCalls, generated.ToolCalls...)
		messages = append(messages, ai.Message{
			Role:      ai.RoleAssistant,
			Content:   generated.Content,
			ToolCalls: generated.ToolCalls,
		})

		slog.Debug("sales assistant tool calls requested", "conversation_id", conversationID, "count", len(generated.ToolCalls), "step", step+1)
		for _, call := range generated.ToolCalls {
			slog.Debug("sales assistant executing tool", "conversation_id", conversationID, "tool", call.Name)
			tool, ok := toolRegistry.Find(call.Name)
			if !ok {
				errContent := `{"error":"unknown tool"}`
				messages = append(messages, ai.Message{Role: ai.RoleTool, Content: errContent, ToolCallID: call.ID, Metadata: map[string]any{"name": call.Name}})
				allToolResults = append(allToolResults, ai.ToolResult{Name: call.Name, Content: errContent})
				continue
			}

			result, execErr := tool.Execute(ctx, call.Arguments)
			if execErr != nil {
				slog.Error("sales assistant tool execution failed", "conversation_id", conversationID, "tool", call.Name, "error", execErr)
				errContent := `{"error":"tool execution failed"}`
				messages = append(messages, ai.Message{Role: ai.RoleTool, Content: errContent, ToolCallID: call.ID, Metadata: map[string]any{"name": call.Name}})
				allToolResults = append(allToolResults, ai.ToolResult{Name: call.Name, Content: errContent})
				continue
			}

			if count, ok := result.Data["count"]; ok {
				slog.Debug("sales assistant tool result", "conversation_id", conversationID, "tool", call.Name, "count", count)
			}
			messages = append(messages, ai.Message{Role: ai.RoleTool, Content: result.Content, ToolCallID: call.ID, Metadata: map[string]any{"name": call.Name}})
			allToolResults = append(allToolResults, ai.ToolResult{Name: call.Name, Content: result.Content})
		}
	}

	if reply == "" {
		reply = "I can help with products, orders, and checkout. How can I help you?"
	}

	cleanReply, actions := parseActions(reply)
	reply = cleanReply

	if err := s.conversations.SaveAssistantMessage(ctx, conversationID, reply, allToolCalls, allToolResults, generated.Usage, providerName, cfg.Model); err != nil {
		slog.Error("sales assistant assistant message save failed", "conversation_id", conversationID, "error", err)
	}

	actionList := make([]any, len(actions))
	for i, a := range actions {
		actionList[i] = a
	}

	return MessageResponse{
		ConversationID: conversationID,
		Reply:          reply,
		Actions:        actionList,
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
