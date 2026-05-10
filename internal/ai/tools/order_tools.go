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
				"customerName":  map[string]any{"type": "string", "description": "Full customer name."},
				"customerPhone": map[string]any{"type": "string", "description": "Customer phone number."},
				"customerEmail": map[string]any{"type": "string", "description": "Optional customer email."},
				"addressRaw":    map[string]any{"type": "string", "description": "Full delivery address."},
				"cityId":        map[string]any{"type": "integer", "description": "City ID from the store cities list."},
				"paymentMethod": map[string]any{"type": "string", "description": "Payment method: 'cod' or 'paymob'.", "enum": []string{"cod", "paymob"}},
				"couponCode":    map[string]any{"type": "string", "description": "Optional coupon code."},
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
		Items:         items,
		CustomerName:  stringArg(arguments, "customerName"),
		CustomerPhone: stringArg(arguments, "customerPhone"),
		CustomerEmail: stringArg(arguments, "customerEmail"),
		AddressRaw:    stringArg(arguments, "addressRaw"),
		CityID:        int64Arg(arguments, "cityId"),
		PaymentMethod: stringArg(arguments, "paymentMethod"),
		CouponCode:    stringArg(arguments, "couponCode"),
	}

	data, err := t.orders.Create(ctx, input)
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
		Description: "Look up an existing order by ID or order number. Returns order status and basic details.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"orderId":     map[string]any{"type": "integer", "description": "Order ID."},
				"orderNumber": map[string]any{"type": "string", "description": "Order number (e.g. ORD-...)."},
			},
		},
	}
}

func (t *LookupOrderTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	orderID := int64Arg(arguments, "orderId")
	orderNumber := stringArg(arguments, "orderNumber")
	data, err := t.orders.Lookup(ctx, orderID, orderNumber)
	if err != nil {
		return ai.ToolResult{}, err
	}
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}
