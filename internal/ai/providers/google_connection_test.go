package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fahd-backend/internal/ai"
)

func TestGoogleGenerationUsesNormalizedModelAndHeaderKey(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", stream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantPath := "/models/gemini-3.8-flash:generateContent"
				if stream {
					wantPath = "/models/gemini-3.8-flash:streamGenerateContent"
				}
				if r.URL.Path != wantPath || r.Header.Get("x-goog-api-key") != "test-key" || r.URL.Query().Has("key") {
					t.Errorf("wrong model path or key transport: %s", r.URL.Path)
				}
				if stream {
					if r.URL.Query().Get("alt") != "sse" {
						t.Error("stream must request SSE")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"OK\"}]}}]}\n\n")
				} else {
					fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"OK"}]}}]}`)
				}
			}))
			defer server.Close()
			p := NewGoogleProvider(GoogleConfig{APIKey: " \ttest-key\n", Model: " models/gemini-3.8-flash "})
			p.baseURL = server.URL
			req := ai.GenerateRequest{Messages: []ai.Message{{Role: ai.RoleUser, Content: "Reply OK"}}, MaxTokens: 1024}
			streamed := ""
			if stream {
				req.OnText = func(s string) { streamed = s }
			}
			res, err := p.Generate(context.Background(), req)
			if err != nil || res.Content != "OK" || (stream && streamed != "OK") {
				t.Fatalf("generation failed: response=%#v error=%v", res, err)
			}
		})
	}
}

func TestGoogleErrorDistinguishesKeyModelAndQuota(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   ai.ErrorCode
	}{
		{"invalid key returned as 400", 400, `{"error":{"message":"Invalid request", "details":[{"reason":"API_KEY_INVALID"}]}}`, ai.ErrorCodeProviderAuth},
		{"leaked key", 403, `{"error":{"message":"Your API key was reported as leaked."}}`, ai.ErrorCodeProviderAuth},
		{"retired model", 404, `{"error":{"message":"Model is no longer available"}}`, ai.ErrorCodeProviderModel},
		{"quota", 429, `{"error":{"message":"Quota exceeded for metric"}}`, ai.ErrorCodeProviderQuota},
		{"billing", 402, `{"error":{"message":"Insufficient credits"}}`, ai.ErrorCodeProviderQuota},
		{"temporary limit", 429, `{"error":{"message":"Too many requests"}}`, ai.ErrorCodeProviderRate},
		{"invalid token setting", 400, `{"error":{"message":"Output token limit exceeded"}}`, ai.ErrorCodeProviderError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err ai.ProviderError
			if !errors.As(googleProviderError(tc.status, []byte(tc.body)), &err) || err.Code != tc.code {
				t.Fatalf("wrong error category: %#v", err)
			}
		})
	}
}

func TestGoogleModelsUsesPaginationAndFiltersNonChatModels(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/models" || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("models must use the official models path and header key")
		}
		if r.URL.Query().Get("pageToken") == "page 2" {
			fmt.Fprint(w, `{"models":[{"name":"models/gemini-3.8-flash","displayName":"Gemini 3.8 Flash","supportedGenerationMethods":["generateContent"]},{"name":"models/gemini-3.6-flash","supportedGenerationMethods":["generateContent"]}]}`)
			return
		}
		fmt.Fprint(w, `{"models":[{"name":"models/gemini-3.6-flash","supportedGenerationMethods":["generateContent"]},{"name":"models/gemini-3.8-flash-image","supportedGenerationMethods":["generateContent"]},{"name":"models/gemini-live","supportedGenerationMethods":["generateContent"]},{"name":"models/gemini-embedding","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"page 2"}`)
	}))
	defer server.Close()
	p := NewGoogleProvider(GoogleConfig{APIKey: "test-key"})
	p.baseURL = server.URL
	models, err := p.ListModels(context.Background())
	if err != nil || requests != 2 || len(models) != 2 || models[0].ID != ai.DefaultGoogleModel || models[1].ID != "gemini-3.6-flash" {
		t.Fatalf("wrong chat models: models=%#v requests=%d error=%v", models, requests, err)
	}
}
