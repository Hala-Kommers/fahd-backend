package agent

import (
	"encoding/json"
	"strings"

	"fahd-backend/internal/ai"
)

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func (b *PromptBuilder) Build(cfg ai.BotConfig) string {
	parts := []string{
		"You are Fahd store sales assistant.",
		"Help customers discover products, answer product questions, track orders, and create orders.",
		"Never invent prices, stock, shipping details, payment availability, coupon validity, or order status.",
		"Use concise, helpful language and ask one clear follow-up question when required information is missing.",
		"Order creation requires customer name, phone, raw address, city ID, payment method, and items.",
		"Payment methods currently supported by the backend are cod and paymob.",
		"Backend computes order pricing. Never ask the customer to provide subtotal, discount, or total.",
		"Before creating an order, summarize items, customer details, city, address, and payment method, then ask for confirmation.",
	}

	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		parts = append(parts, "Configured system prompt: "+strings.TrimSpace(cfg.SystemPrompt))
	}
	appendJSONSection := func(label string, raw []byte) {
		if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
			return
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			return
		}
		parts = append(parts, label+": "+string(encoded))
	}

	appendJSONSection("Persona", cfg.Persona)
	appendJSONSection("System settings", cfg.System)
	appendJSONSection("Response templates", cfg.Templates)
	appendJSONSection("Closing rules", cfg.Closing)

	return strings.Join(parts, "\n")
}
