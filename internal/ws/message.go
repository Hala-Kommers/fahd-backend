package ws

type ClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Token   string `json:"token,omitempty"`
}

type ServerMessage struct {
	Type       string         `json:"type"`
	SessionID  string         `json:"session_id,omitempty"`
	Token      string         `json:"token,omitempty"`
	ExpiresAt  int64          `json:"expires_at,omitempty"`
	Content    string         `json:"content,omitempty"`
	Actions    []any          `json:"actions,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	ToolCalls  []any          `json:"tool_calls,omitempty"`
	ToolResults []any         `json:"tool_results,omitempty"`
	Error      string         `json:"error,omitempty"`
}
