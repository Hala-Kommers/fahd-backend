package agent

import (
	"context"
	"testing"
)

func TestOfferQuestionDoesNotOpenDialog(t *testing.T) {
	s := &Service{}
	for _, text := range []string{"هل العرض يشمل الشحن؟", "متى ينتهي العرض؟", "العرض غالي"} {
		if _, ok := s.directResponse(context.Background(), MessageRequest{Message: text, Context: map[string]any{"productId": float64(1)}}, 1); ok {
			t.Fatal(text)
		}
	}
}
func TestStructuredActions(t *testing.T) {
	text, actions := parseActions("اختر العرض [ACTION:show_offers] [ACTION:checkout]")
	if text != "اختر العرض" || len(actions) != 2 {
		t.Fatal(text, actions)
	}
}

func TestContextualQuickReply(t *testing.T) {
	text, actions := parseActions("الشحن حسب المدينة [ACTION:quick_reply:اختر المدينة|أريد معرفة الشحن للرياض] [ACTION:quick_reply:ناقص]")
	if text != "الشحن حسب المدينة" || len(actions) != 1 || actions[0].Type != "quick_reply" {
		t.Fatal(text, actions)
	}
	if actions[0].Payload.(map[string]any)["message"] != "أريد معرفة الشحن للرياض" {
		t.Fatal(actions)
	}
}
