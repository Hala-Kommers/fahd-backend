package ws

import "time"

type ClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Token   string `json:"token,omitempty"`
}

type HistoryMessage struct {
	ID                    int64     `json:"id"`
	Role                  string    `json:"role"`
	Content               string    `json:"content"`
	ToolCalls             *string   `json:"toolCalls"`
	ToolResults           *string   `json:"toolResults"`
	UsagePromptTokens     int       `json:"usagePromptTokens"`
	UsageCompletionTokens int       `json:"usageCompletionTokens"`
	UsageCacheWriteTokens int       `json:"usageCacheWriteTokens"`
	UsageCacheReadTokens  int       `json:"usageCacheReadTokens"`
	UsageReasoningTokens  int       `json:"usageReasoningTokens"`
	Provider              string    `json:"provider"`
	Model                 string    `json:"model"`
	CreatedAt             time.Time `json:"createdAt"`
}

type ServerMessage struct {
	Type       string            `json:"type"`
	SessionID  string            `json:"session_id,omitempty"`
	Token      string            `json:"token,omitempty"`
	ExpiresAt  int64             `json:"expires_at,omitempty"`
	Content    string            `json:"content,omitempty"`
	Actions    []any             `json:"actions,omitempty"`
	Meta       map[string]any    `json:"meta,omitempty"`
	ToolCalls  []any             `json:"tool_calls,omitempty"`
	ToolResults []any            `json:"tool_results,omitempty"`
	Messages   []HistoryMessage  `json:"messages,omitempty"`
	Error      string            `json:"error,omitempty"`
}
