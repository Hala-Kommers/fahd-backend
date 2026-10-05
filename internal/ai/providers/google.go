package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fahd-backend/internal/ai"
)

type GoogleConfig struct {
	APIKey string
	Model  string
}

type GoogleProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGoogleProvider(cfg GoogleConfig) *GoogleProvider {
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GoogleProvider{
		apiKey: cfg.APIKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) Generate(ctx context.Context, req ai.GenerateRequest) (ai.GenerateResponse, error) {
	if p.apiKey == "" {
		return ai.GenerateResponse{}, fmt.Errorf("google api key is required")
	}

	payload := googleGenerateRequest{
		SystemInstruction: googleSystemInstruction(req.SystemPrompt),
		Contents:          googleContents(req.Messages),
		Tools:             googleTools(req.Tools),
		GenerationConfig: googleGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("marshal google request: %w", err)
	}

	method := "generateContent"
	if req.OnText != nil {
		method = "streamGenerateContent?alt=sse"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:%s", p.model, method)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("create google request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("google request failed: %w", err)
	}
	defer res.Body.Close()

	if req.OnText != nil && res.StatusCode >= 200 && res.StatusCode < 300 {
		return readGoogleStream(res.Body, req.OnText)
	}
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("read google response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ai.GenerateResponse{}, googleProviderError(res.StatusCode, resBody)
	}

	var parsed googleGenerateResponse
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("decode google response: %w", err)
	}

	usage := parseUsage(parsed.UsageMetadata)
	return ai.GenerateResponse{Content: parsed.Text(), ToolCalls: parsed.ToolCalls(), Usage: usage, Raw: parsed}, nil
}

func googleProviderError(statusCode int, body []byte) error {
	message := string(body)
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		message = parsed.Error.Message
	}

	code := ai.ErrorCodeProviderError
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = ai.ErrorCodeProviderAuth
	case http.StatusTooManyRequests:
		code = ai.ErrorCodeProviderRate
	}

	lower := strings.ToLower(message)
	if strings.Contains(lower, "quota") || strings.Contains(lower, "exceeded") {
		code = ai.ErrorCodeProviderQuota
	}

	return ai.ProviderError{
		Code:       code,
		Provider:   "google",
		StatusCode: statusCode,
		Message:    message,
	}
}

type googleGenerateRequest struct {
	SystemInstruction googleContent          `json:"systemInstruction,omitempty"`
	Contents          []googleContent        `json:"contents"`
	Tools             []googleTool           `json:"tools,omitempty"`
	GenerationConfig  googleGenerationConfig `json:"generationConfig,omitempty"`
}

type googleTool struct {
	FunctionDeclarations []googleFunctionDeclaration `json:"functionDeclarations"`
}

type googleFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type googleGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type googleFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type googleFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type googleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type googleGenerateResponse struct {
	Candidates []struct {
		Content googleContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata *googleUsageMetadata `json:"usageMetadata,omitempty"`
}

func (r googleGenerateResponse) Text() string {
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, part := range r.Candidates[0].Content.Parts {
		b.WriteString(part.Text)
	}
	return b.String()
}

func (r googleGenerateResponse) ToolCalls() []ai.ToolCall {
	if len(r.Candidates) == 0 {
		return nil
	}
	calls := []ai.ToolCall{}
	for i, part := range r.Candidates[0].Content.Parts {
		if part.FunctionCall == nil {
			continue
		}
		calls = append(calls, ai.ToolCall{
			ID:        fmt.Sprintf("google_tool_call_%d", i+1),
			Name:      part.FunctionCall.Name,
			Arguments: part.FunctionCall.Args,
		})
	}
	return calls
}

func googleSystemInstruction(prompt string) googleContent {
	if prompt == "" {
		return googleContent{}
	}
	return googleContent{Parts: []googlePart{{Text: prompt}}}
}

func googleContents(messages []ai.Message) []googleContent {
	contents := make([]googleContent, 0, len(messages)*3)
	for _, message := range messages {
		if message.Content == "" && len(message.ToolCalls) == 0 || message.Role == ai.RoleSystem {
			continue
		}
		switch message.Role {
		case ai.RoleTool:
			name := "tool_result"
			if rawName, ok := message.Metadata["name"].(string); ok && rawName != "" {
				name = rawName
			}
			var responseData map[string]any
			if err := json.Unmarshal([]byte(message.Content), &responseData); err != nil || responseData == nil {
				responseData = map[string]any{"result": message.Content}
			}
			contents = append(contents, googleContent{
				Role: "function",
				Parts: []googlePart{{
					FunctionResponse: &googleFunctionResponse{
						Name:     name,
						Response: responseData,
					},
				}},
			})
		case ai.RoleAssistant:
			// Compact format: message may bundle tool_calls + tool_results + final text
			if len(message.ToolCalls) > 0 {
				parts := make([]googlePart, 0, len(message.ToolCalls))
				for _, tc := range message.ToolCalls {
					parts = append(parts, googlePart{
						FunctionCall: &googleFunctionCall{Name: tc.Name, Args: tc.Arguments},
					})
				}
				contents = append(contents, googleContent{Role: "model", Parts: parts})

				if results, ok := message.Metadata["tool_results"].([]ai.ToolResult); ok && len(results) > 0 {
					for _, r := range results {
						var responseData map[string]any
						if err := json.Unmarshal([]byte(r.Content), &responseData); err != nil || responseData == nil {
							responseData = map[string]any{"result": r.Content}
						}
						contents = append(contents, googleContent{
							Role: "function",
							Parts: []googlePart{{
								FunctionResponse: &googleFunctionResponse{
									Name:     r.Name,
									Response: responseData,
								},
							}},
						})
					}
				}
			}
			// Emit text part (even without tool_calls for plain assistant replies)
			if message.Content != "" {
				contents = append(contents, googleContent{Role: "model", Parts: []googlePart{{Text: message.Content}}})
			}
		default:
			if message.Content == "" {
				continue
			}
			contents = append(contents, googleContent{
				Role:  "user",
				Parts: []googlePart{{Text: message.Content}},
			})
		}
	}
	return contents
}

func googleTools(definitions []ai.ToolDefinition) []googleTool {
	if len(definitions) == 0 {
		return nil
	}
	declarations := make([]googleFunctionDeclaration, 0, len(definitions))
	for _, definition := range definitions {
		declarations = append(declarations, googleFunctionDeclaration{
			Name:        definition.Name,
			Description: definition.Description,
			Parameters:  googleSchema(definition.Parameters),
		})
	}
	return []googleTool{{FunctionDeclarations: declarations}}
}

func googleSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	return sanitizeGoogleSchema(schema).(map[string]any)
}

func parseUsage(m *googleUsageMetadata) *ai.Usage {
	if m == nil {
		return nil
	}
	return &ai.Usage{
		PromptTokens:     m.PromptTokenCount,
		CompletionTokens: m.CandidatesTokenCount,
	}
}

func sanitizeGoogleSchema(value any) any {
	switch v := value.(type) {
	case map[string]any:
		cleaned := map[string]any{}
		for key, item := range v {
			if key == "additionalProperties" {
				continue
			}
			cleaned[key] = sanitizeGoogleSchema(item)
		}
		return cleaned
	case []any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, sanitizeGoogleSchema(item))
		}
		return items
	default:
		return value
	}
}

func readGoogleStream(r io.Reader, onText func(string)) (ai.GenerateResponse, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	result := ai.GenerateResponse{}
	var data strings.Builder
	consume := func() error {
		raw := strings.TrimSpace(data.String())
		data.Reset()
		if raw == "" || raw == "[DONE]" {
			return nil
		}
		var chunk googleGenerateResponse
		if e := json.Unmarshal([]byte(raw), &chunk); e != nil {
			return e
		}
		text := chunk.Text()
		if text != "" {
			result.Content += text
			onText(result.Content)
		}
		for _, call := range chunk.ToolCalls() {
			call.ID = fmt.Sprintf("google_tool_call_%d", len(result.ToolCalls)+1)
			result.ToolCalls = append(result.ToolCalls, call)
		}
		if chunk.UsageMetadata != nil {
			result.Usage = parseUsage(chunk.UsageMetadata)
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if e := consume(); e != nil {
				return result, e
			}
		} else if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimPrefix(line, "data:"))
			data.WriteByte('\n')
		}
	}
	if e := scanner.Err(); e != nil {
		return result, e
	}
	if e := consume(); e != nil {
		return result, e
	}
	return result, nil
}
