package providers

import (
	"strings"
	"testing"
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
