package providers

import (
	"strings"
	"testing"

	"fahd-backend/internal/ai"
)

func TestRealStream(t *testing.T) {
	raw := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"هلا \"}]}}]}\n\ndata: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"فيك\"}]}}],\"usageMetadata\":{\"promptTokenCount\":3,\"candidatesTokenCount\":2}}\n\n"
	calls := []string{}
	r, e := readGoogleStream(strings.NewReader(raw), func(s string) { calls = append(calls, s) })
	if e != nil || len(calls) != 2 || calls[0] != "هلا " || r.Content != "هلا فيك" || r.Usage.CompletionTokens != 2 {
		t.Fatal(r, e, calls)
	}
}
func TestStreamError(t *testing.T) {
	if _, e := readGoogleStream(strings.NewReader("data: {broken}\n\n"), func(string) {}); e == nil {
		t.Fatal("malformed stream accepted")
	}
}

func TestGoogleContentsPreservesGeminiToolSignature(t *testing.T) {
	contents := googleContents([]ai.Message{
		{Role: ai.RoleUser, Content: "اعرض المنتج"},
		{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
			Name:             "get_product_details",
			Arguments:        map[string]any{"productId": float64(2)},
			ThoughtSignature: "signature-a",
		}}},
		{Role: ai.RoleTool, Content: `{"name":"get_product_details"}`, Metadata: map[string]any{"name": "get_product_details"}},
	})
	if len(contents) != 3 {
		t.Fatalf("expected user, model and tool response contents, got %d", len(contents))
	}
	if contents[1].Role != "model" || contents[1].Parts[0].ThoughtSignature != "signature-a" {
		t.Fatalf("thought signature was not preserved: %#v", contents[1])
	}
	if contents[2].Role != "user" || contents[2].Parts[0].FunctionResponse == nil {
		t.Fatalf("tool response must be a user content part: %#v", contents[2])
	}
}

func TestGoogleContentsGroupsParallelToolResponses(t *testing.T) {
	contents := googleContents([]ai.Message{
		{Role: ai.RoleUser, Content: "قارن المنتجين"},
		{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{
			{Name: "get_product_details", Arguments: map[string]any{"productId": 1}},
			{Name: "get_product_details", Arguments: map[string]any{"productId": 2}},
		}},
		{Role: ai.RoleTool, Content: `{"productId":1}`, Metadata: map[string]any{"name": "get_product_details"}},
		{Role: ai.RoleTool, Content: `{"productId":2}`, Metadata: map[string]any{"name": "get_product_details"}},
	})
	if len(contents) != 3 {
		t.Fatalf("expected one user, one call and one grouped response turn, got %d", len(contents))
	}
	if got := len(contents[1].Parts); got != 2 {
		t.Fatalf("expected two function calls in one model turn, got %d", got)
	}
	if contents[2].Role != "user" || len(contents[2].Parts) != 2 {
		t.Fatalf("expected two function responses in one user turn: %#v", contents[2])
	}
}

func TestGoogleContentsDropsIncompleteLeadingToolCycle(t *testing.T) {
	contents := googleContents([]ai.Message{
		{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{Name: "orphan_call"}}},
		{Role: ai.RoleTool, Content: `{"stale":true}`, Metadata: map[string]any{"name": "orphan_call"}},
		{Role: ai.RoleUser, Content: "رسالة جديدة"},
		{Role: ai.RoleAssistant, Content: "رد سليم"},
	})
	if len(contents) != 2 || contents[0].Role != "user" || contents[1].Role != "model" {
		t.Fatalf("expected history to start at the complete user turn: %#v", contents)
	}
}

func TestGoogleContentsDropsPartiallyAnsweredToolCycle(t *testing.T) {
	contents := googleContents([]ai.Message{
		{Role: ai.RoleUser, Content: "قارن المنتجين"},
		{Role: ai.RoleAssistant, Content: "ما قدرت أكمل المقارنة", ToolCalls: []ai.ToolCall{
			{Name: "get_product_details", Arguments: map[string]any{"productId": 1}},
			{Name: "get_product_details", Arguments: map[string]any{"productId": 2}},
		}},
		{Role: ai.RoleTool, Content: `{"productId":1}`, Metadata: map[string]any{"name": "get_product_details"}},
	})
	if len(contents) != 2 || contents[1].Parts[0].Text != "ما قدرت أكمل المقارنة" {
		t.Fatalf("partial tool cycle must be removed while keeping safe text: %#v", contents)
	}
}

func TestGoogleContentsReplaysPersistedCompactToolTurn(t *testing.T) {
	contents := googleContents([]ai.Message{
		{Role: ai.RoleUser, Content: "وش المتوفر؟"},
		{Role: ai.RoleAssistant, Content: "هذه المنتجات", ToolCalls: []ai.ToolCall{
			{Name: "list_products"},
			{Name: "get_product_details", Arguments: map[string]any{"productId": 2}},
		}, Metadata: map[string]any{"tool_results": []ai.ToolResult{
			{Name: "list_products", Content: `{"count":2}`},
			{Name: "get_product_details", Content: `{"productId":2}`},
		}}},
		{Role: ai.RoleUser, Content: "تمام"},
	})
	if len(contents) != 5 {
		t.Fatalf("expected user, calls, grouped responses, final text and next user, got %d", len(contents))
	}
	if contents[2].Role != "user" || len(contents[2].Parts) != 2 {
		t.Fatalf("persisted responses were not grouped: %#v", contents[2])
	}
	if contents[3].Role != "model" || contents[3].Parts[0].Text != "هذه المنتجات" {
		t.Fatalf("final assistant text was not preserved: %#v", contents[3])
	}
}
