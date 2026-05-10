package ai

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role           `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string         `json:"toolCallId,omitempty"`
	Name       string         `json:"name"`
	Content    string         `json:"content"`
	Data       map[string]any `json:"data,omitempty"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, arguments map[string]any) (ToolResult, error)
}

type GenerateRequest struct {
	SystemPrompt string           `json:"systemPrompt"`
	Messages     []Message        `json:"messages"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	Temperature  float64          `json:"temperature"`
	MaxTokens    int              `json:"maxTokens"`
}

type GenerateResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	Raw       any        `json:"-"`
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
