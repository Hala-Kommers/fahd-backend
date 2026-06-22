package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/services"
)

type CreateOrderTool struct {
	orders *services.OrderService
}

func NewCreateOrderTool(orders *services.OrderService) *CreateOrderTool {
	return &CreateOrderTool{orders: orders}
}

func (t *CreateOrderTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_order",
		Description: "Create a store order. Before calling, summarize all details and ask the customer to confirm. Returns full order details with computed pricing.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"items", "customerName", "customerPhone", "addressRaw", "cityId", "paymentMethod"},
			"properties": map[string]any{
				"items": map[string]any{
					"type":        "array",
					"description": "Order items. Each item must have a productId and qty; optionally a variantId.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"productId": map[string]any{"type": "integer", "description": "Product ID."},
							"variantId": map[string]any{"type": "integer", "description": "Optional variant ID."},
							"qty":       map[string]any{"type": "integer", "description": "Quantity."},
						},
						"required": []string{"productId", "qty"},
					},
				},
				"customerName":    map[string]any{"type": "string", "description": "Full customer name."},
				"customerPhone":   map[string]any{"type": "string", "description": "Customer phone number."},
				"customerEmail":   map[string]any{"type": "string", "description": "Optional customer email."},
				"addressRaw":      map[string]any{"type": "string", "description": "Full delivery address."},
				"addressZone":     map[string]any{"type": "string", "description": "Optional city within the selected state/wilaya."},
				"addressDistrict": map[string]any{"type": "string", "description": "Optional neighborhood/district within the selected city."},
				"cityId":          map[string]any{"type": "integer", "description": "City ID from the store cities list."},
				"paymentMethod":   map[string]any{"type": "string", "description": "Payment method: 'cod' or 'paymob'.", "enum": []string{"cod", "paymob"}},
				"couponCode":      map[string]any{"type": "string", "description": "Optional coupon code."},
			},
		},
	}
}

func (t *CreateOrderTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	itemsRaw, ok := arguments["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return ai.ToolResult{}, fmt.Errorf("items array is required")
	}
	itemsJSON, _ := json.Marshal(itemsRaw)
	var items []services.OrderItemInput
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return ai.ToolResult{}, fmt.Errorf("invalid items: %w", err)
	}

	input := services.CreateOrderInput{
		Items:           items,
		CustomerName:    stringArg(arguments, "customerName"),
		CustomerPhone:   stringArg(arguments, "customerPhone"),
		CustomerEmail:   stringArg(arguments, "customerEmail"),
		AddressRaw:      stringArg(arguments, "addressRaw"),
		AddressZone:     stringPtrArg(arguments, "addressZone"),
		AddressDistrict: stringPtrArg(arguments, "addressDistrict"),
		CityID:          int64Arg(arguments, "cityId"),
		ConversationID:  int64Arg(arguments, "conversationId"),
		PaymentMethod:   stringArg(arguments, "paymentMethod"),
		CouponCode:      stringArg(arguments, "couponCode"),
	}

	data, err := t.orders.Create(ctx, input)
	if err != nil {
		return ai.ToolResult{}, err
	}
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}

type CalculateOrderTotalTool struct {
	orders *services.OrderService
}

func NewCalculateOrderTotalTool(orders *services.OrderService) *CalculateOrderTotalTool {
	return &CalculateOrderTotalTool{orders: orders}
}

func (t *CalculateOrderTotalTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "calculate_order_total",
		Description: "Calculate the order total (subtotal, discount, grand total) before creating the order. Use this when the customer asks about the price, total, or amount before confirming the order. Does not create the order.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"items"},
			"properties": map[string]any{
				"items": map[string]any{
					"type":        "array",
					"description": "Order items. Each item must have a productId and qty; optionally a variantId.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"productId": map[string]any{"type": "integer", "description": "Product ID."},
							"variantId": map[string]any{"type": "integer", "description": "Optional variant ID."},
							"qty":       map[string]any{"type": "integer", "description": "Quantity."},
						},
						"required": []string{"productId", "qty"},
					},
				},
				"couponCode": map[string]any{"type": "string", "description": "Optional coupon code to apply."},
			},
		},
	}
}

func (t *CalculateOrderTotalTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	itemsRaw, ok := arguments["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return ai.ToolResult{}, fmt.Errorf("items array is required")
	}
	itemsJSON, _ := json.Marshal(itemsRaw)
	var items []services.OrderItemInput
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return ai.ToolResult{}, fmt.Errorf("invalid items: %w", err)
	}

	input := services.CreateOrderInput{
		Items:      items,
		CouponCode: stringArg(arguments, "couponCode"),
	}

	data, err := t.orders.CalculateTotal(ctx, input)
	if err != nil {
		return ai.ToolResult{}, err
	}
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}

type LookupOrderTool struct {
	orders *services.OrderService
}

func NewLookupOrderTool(orders *services.OrderService) *LookupOrderTool {
	return &LookupOrderTool{orders: orders}
}

func (t *LookupOrderTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "lookup_order",
		Description: "Look up an existing order by order number. Requires the customer's phone number for verification. Returns order status and basic details.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"orderNumber", "customerPhone"},
			"properties": map[string]any{
				"orderNumber":   map[string]any{"type": "string", "description": "Order number (e.g. ORD-...)."},
				"customerPhone": map[string]any{"type": "string", "description": "Customer phone number for ownership verification."},
			},
		},
	}
}

func (t *LookupOrderTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	orderID := int64Arg(arguments, "orderId")
	orderNumber := stringArg(arguments, "orderNumber")
	customerPhone := stringArg(arguments, "customerPhone")
	data, err := t.orders.Lookup(ctx, orderID, orderNumber, customerPhone)
	if err != nil {
		return ai.ToolResult{}, err
	}
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}
