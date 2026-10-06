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
		{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
			Name:             "get_product_details",
			Arguments:        map[string]any{"productId": float64(2)},
			ThoughtSignature: "signature-a",
		}}},
		{Role: ai.RoleTool, Content: `{"name":"get_product_details"}`, Metadata: map[string]any{"name": "get_product_details"}},
	})
	if len(contents) != 2 {
		t.Fatalf("expected model and tool response contents, got %d", len(contents))
	}
	if contents[0].Role != "model" || contents[0].Parts[0].ThoughtSignature != "signature-a" {
		t.Fatalf("thought signature was not preserved: %#v", contents[0])
	}
	if contents[1].Role != "user" || contents[1].Parts[0].FunctionResponse == nil {
		t.Fatalf("tool response must be a user content part: %#v", contents[1])
	}
}
