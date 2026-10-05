package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/services"

	"gorm.io/gorm"
)

type ConversationService struct {
	db *gorm.DB
}

func NewConversationService(db *gorm.DB) *ConversationService {
	return &ConversationService{db: db}
}

func (s *ConversationService) StartOrContinue(ctx context.Context, conversationID int64, visitorID, sessionID string) (int64, error) {
	return services.EnsureConversation(ctx, s.db, sessionID, visitorID)
}

func (s *ConversationService) recordChatStarted(ctx context.Context, visitorID, sessionID *string, conversationID int64) {
	if visitorID == nil && sessionID == nil {
		return
	}
	metadata, _ := json.Marshal(map[string]any{"conversationId": conversationID})
	row := map[string]any{
		"visitor_id": visitorID,
		"session_id": sessionID,
		"event_type": "chat_started",
		"path":       "chat",
		"metadata":   string(metadata),
	}
	_ = s.db.WithContext(ctx).Table("analytics_events").Create(&row).Error
}

func (s *ConversationService) SaveMessage(ctx context.Context, conversationID int64, role ai.Role, content string, metadata map[string]any) error {
	row := map[string]any{
		"conversation_id": conversationID,
		"role":            string(role),
		"content":         content,
	}
	if len(metadata) > 0 {
		encoded, _ := json.Marshal(metadata)
		row["metadata"] = string(encoded)
	}
	if err := s.db.WithContext(ctx).Table("messages").Create(&row).Error; err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	s.touchConversation(ctx, conversationID)
	return nil
}

func (s *ConversationService) SaveAssistantMessage(ctx context.Context, conversationID int64, content string, toolCalls []ai.ToolCall, toolResults []ai.ToolResult, usage *ai.Usage, provider, model string) error {
	row := map[string]any{
		"conversation_id": conversationID,
		"role":            "assistant",
		"content":         content,
		"provider":        provider,
		"model":           model,
	}
	if len(toolCalls) > 0 {
		encoded, _ := json.Marshal(toolCalls)
		row["tool_calls"] = string(encoded)
	}
	if len(toolResults) > 0 {
		encoded, _ := json.Marshal(toolResults)
		row["tool_results"] = string(encoded)
	}
	if usage != nil {
		row["usage_prompt_tokens"] = usage.PromptTokens
		row["usage_completion_tokens"] = usage.CompletionTokens
		if usage.CacheWriteTokens > 0 {
			row["usage_cache_write_tokens"] = usage.CacheWriteTokens
		}
		if usage.CacheReadTokens > 0 {
			row["usage_cache_read_tokens"] = usage.CacheReadTokens
		}
		if usage.ReasoningTokens > 0 {
			row["usage_reasoning_tokens"] = usage.ReasoningTokens
		}
	}
	if err := s.db.WithContext(ctx).Table("messages").Create(&row).Error; err != nil {
		return fmt.Errorf("save assistant message: %w", err)
	}
	s.touchConversation(ctx, conversationID)
	return nil
}

func (s *ConversationService) RecentMessages(ctx context.Context, conversationID int64, limit int) ([]ai.Message, error) {
	if limit <= 0 {
		limit = 20
	}

	type messageRow struct {
		Role        string
		Content     string
		ToolCalls   *string
		ToolResults *string
		Metadata    *string
	}
	var rows []messageRow
	if err := s.db.WithContext(ctx).Table("messages").
		Select("role, content, tool_calls, tool_results, metadata").
		Where("conversation_id = ? AND content IS NOT NULL", conversationID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	messages := make([]ai.Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		role := ai.Role(row.Role)
		if role == "" {
			role = ai.RoleUser
		}
		msg := ai.Message{Role: role, Content: row.Content}

		if row.ToolCalls != nil {
			var calls []ai.ToolCall
			if err := json.Unmarshal([]byte(*row.ToolCalls), &calls); err == nil {
				msg.ToolCalls = calls
			}
		}
		if row.ToolResults != nil {
			var results []ai.ToolResult
			if err := json.Unmarshal([]byte(*row.ToolResults), &results); err == nil && len(results) > 0 {
				if msg.Metadata == nil {
					msg.Metadata = map[string]any{}
				}
				msg.Metadata["tool_results"] = results
			}
		}
		if row.Metadata != nil {
			var meta map[string]any
			if err := json.Unmarshal([]byte(*row.Metadata), &meta); err == nil && len(meta) > 0 {
				if msg.Metadata == nil {
					msg.Metadata = meta
				} else {
					for k, v := range meta {
						msg.Metadata[k] = v
					}
				}
			}
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *ConversationService) touchConversation(ctx context.Context, conversationID int64) {
	_ = s.db.WithContext(ctx).Table("conversations").Where("id = ?", conversationID).Updates(map[string]any{
		"last_message_at": gorm.Expr("NOW()"),
		"updated_at":      gorm.Expr("NOW()"),
	}).Error
}
