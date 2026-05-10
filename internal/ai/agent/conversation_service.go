package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"fahd-backend/internal/ai"

	"gorm.io/gorm"
)

type ConversationService struct {
	db *gorm.DB
}

func NewConversationService(db *gorm.DB) *ConversationService {
	return &ConversationService{db: db}
}

func (s *ConversationService) StartOrContinue(ctx context.Context, conversationID int64) (int64, error) {
	if conversationID > 0 {
		var exists int64
		if err := s.db.WithContext(ctx).Table("conversations").Where("id = ?", conversationID).Count(&exists).Error; err != nil {
			return 0, fmt.Errorf("check conversation: %w", err)
		}
		if exists > 0 {
			return conversationID, nil
		}
	}

	var created struct{ ID int64 }
	if err := s.db.WithContext(ctx).Raw("INSERT INTO conversations (status) VALUES (?) RETURNING id", "active").Scan(&created).Error; err != nil {
		return 0, fmt.Errorf("create conversation: %w", err)
	}
	return created.ID, nil
}

func (s *ConversationService) SaveMessage(ctx context.Context, conversationID int64, role ai.Role, content string) error {
	row := map[string]any{
		"conversation_id": conversationID,
		"role":            string(role),
		"content":         content,
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
	}
	var rows []messageRow
	if err := s.db.WithContext(ctx).Table("messages").
		Select("role, content, tool_calls, tool_results").
		Where("conversation_id = ? AND content IS NOT NULL", conversationID).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	messages := make([]ai.Message, 0, len(rows))
	for _, row := range rows {
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
