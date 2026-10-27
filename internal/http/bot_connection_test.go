package http

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"fahd-backend/internal/ai"
)

func TestBotConnectionNormalizesPastedSettings(t *testing.T) {
	payload := map[string]any{"provider": " Gemini ", "model": " google/models/gemini-3.8-flash ", "apiKey": " \ttest-key\n"}
	if err := normalizeBotConnectionPayload(payload); err != nil {
		t.Fatal(err)
	}
	if payload["apiKey"] != "test-key" || payload["model"] != "gemini-3.8-flash" || payload["provider"] != "google" {
		t.Fatalf("settings were not normalized: %#v", payload)
	}
}

func TestBotConnectionRejectsMalformedKeyAndModel(t *testing.T) {
	for _, payload := range []map[string]any{
		{"apiKey": "broken\nkey"}, {"apiKey": 123}, {"apiKey": "first", "api_key": "second"},
		{"model": "gemini-flash?key=secret"}, {"provider": "unsupported"},
	} {
		if err := normalizeBotConnectionPayload(payload); err == nil {
			t.Fatal("invalid connection payload accepted")
		}
	}
}

func TestBotConnectionFailureIsActionableAndRedactsKey(t *testing.T) {
	for _, tc := range []struct {
		name          string
		err           error
		code, message string
	}{
		{"bad key", ai.ProviderError{Code: ai.ErrorCodeProviderAuth, Provider: "google", StatusCode: 400, Message: "bad test-key"}, "provider_auth", "رفض المفتاح"},
		{"model unavailable", ai.ProviderError{Code: ai.ErrorCodeProviderModel, Provider: "google", StatusCode: 404, Message: "not found"}, "provider_model_unavailable", "الموديل المحدد غير متاح"},
		{"quota", ai.ProviderError{Code: ai.ErrorCodeProviderQuota, Provider: "google", StatusCode: 429, Message: "quota exceeded"}, "provider_quota", "حصة Google"},
		{"timeout", fmt.Errorf("request failed: %w", context.DeadlineExceeded), "provider_timeout", "لا يعني أن المفتاح غير صحيح"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := googleConnectionFailure(tc.err, "test-key")
			if failure["ok"] != false || failure["code"] != tc.code || !strings.Contains(failure["error"].(string), tc.message) || strings.Contains(failure["details"].(string), "test-key") {
				t.Fatalf("unsafe or misleading connection failure: %#v", failure)
			}
		})
	}
}
