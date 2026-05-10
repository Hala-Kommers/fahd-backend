package agent

import (
	"context"
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
	_ = s.db.WithContext(ctx).Table("conversations").Where("id = ?", conversationID).Updates(map[string]any{
		"last_message_at": gorm.Expr("NOW()"),
		"updated_at":       gorm.Expr("NOW()"),
	}).Error
	return nil
}

func (s *ConversationService) RecentMessages(ctx context.Context, conversationID int64, limit int) ([]ai.Message, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows []struct {
		Role    string
		Content string
	}
	err := s.db.WithContext(ctx).Table("messages").
		Select("role, content").
		Where("conversation_id = ? AND content IS NOT NULL", conversationID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	messages := make([]ai.Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		role := ai.Role(rows[i].Role)
		if role == "" {
			role = ai.RoleUser
		}
		messages = append(messages, ai.Message{Role: role, Content: rows[i].Content})
	}
	return messages, nil
}
