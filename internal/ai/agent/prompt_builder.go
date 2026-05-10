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
		"Use tools for product questions. If the customer asks about variations, sizes, colors, stock, images, specifications, FAQ, or tiered prices, call get_product_details before answering.",
		"If the customer asks to list, show, browse, or search products without a specific keyword, call search_products with an empty query and no category.",
		"If the customer selected variant attributes, call resolve_variant before confirming availability or creating an order.",
		"Use concise, helpful language and ask one clear follow-up question when required information is missing.",
		"Order creation: collect customer name, phone, delivery address, city ID, payment method (cod or paymob), and items with quantities. Collect one piece of information at a time.",
		"Backend computes order pricing. Never ask the customer to provide subtotal, discount, or total. Never invent prices.",
		"If the customer asks about the price, total, or amount before confirming, call calculate_order_total with the items and optional coupon to show the breakdown. Call it whenever you need to show pricing during the ordering process.",
		"Before calling create_order, summarize the full order (items, totals, customer details, address, payment method) and ask the customer to confirm. Only call create_order after receiving explicit confirmation.",
		"Use lookup_order to check order status or find past orders. The customer must provide their order number and phone number to verify ownership. Never call lookup_order without both the order number and the customer's phone number.",
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
