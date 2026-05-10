package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		GenerationConfig: googleGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("marshal google request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("create google request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("google request failed: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("read google response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ai.GenerateResponse{}, fmt.Errorf("google response status %d: %s", res.StatusCode, string(resBody))
	}

	var parsed googleGenerateResponse
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return ai.GenerateResponse{}, fmt.Errorf("decode google response: %w", err)
	}

	return ai.GenerateResponse{Content: parsed.Text(), Raw: parsed}, nil
}

type googleGenerateRequest struct {
	SystemInstruction googleContent          `json:"systemInstruction,omitempty"`
	Contents          []googleContent        `json:"contents"`
	GenerationConfig  googleGenerationConfig `json:"generationConfig,omitempty"`
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
	Text string `json:"text"`
}

type googleGenerateResponse struct {
	Candidates []struct {
		Content googleContent `json:"content"`
	} `json:"candidates"`
}

func (r googleGenerateResponse) Text() string {
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return r.Candidates[0].Content.Parts[0].Text
}

func googleSystemInstruction(prompt string) googleContent {
	if prompt == "" {
		return googleContent{}
	}
	return googleContent{Parts: []googlePart{{Text: prompt}}}
}

func googleContents(messages []ai.Message) []googleContent {
	contents := make([]googleContent, 0, len(messages))
	for _, message := range messages {
		if message.Content == "" || message.Role == ai.RoleSystem {
			continue
		}
		role := "user"
		if message.Role == ai.RoleAssistant {
			role = "model"
		}
		contents = append(contents, googleContent{
			Role:  role,
			Parts: []googlePart{{Text: message.Content}},
		})
	}
	return contents
}
