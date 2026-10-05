package tools

import (
	"context"
	"fmt"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/services"
)

type SearchProductsTool struct {
	products *services.ProductService
}

func NewSearchProductsTool(products *services.ProductService) *SearchProductsTool {
	return &SearchProductsTool{products: products}
}

func (t *SearchProductsTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "search_products",
		Description: "Search active store products by optional text query and optional category. Use this before recommending products. If the user asks generally to show/list/search products, call this with an empty query.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":      map[string]any{"type": "string", "description": "Search text, product name, SKU, or description keywords."},
				"category":   map[string]any{"type": "string", "description": "Optional category slug or name."},
				"maxPrice":   map[string]any{"type": "number", "description": "Maximum price within customer budget in SAR."},
				"maxResults": map[string]any{"type": "integer", "minimum": 1, "maximum": 3},
			},
		},
	}
}

func (t *SearchProductsTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	data, err := t.products.Search(ctx, stringArg(arguments, "query"), stringArg(arguments, "category"), intArg(arguments, "maxResults"), float64(intArg(arguments, "maxPrice")))
	if err != nil {
		return ai.ToolResult{}, err
	}
	t.products.ApplySessionPrices(ctx, data, stringArg(arguments, "sessionId"))
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}

type GetProductDetailsTool struct {
	products *services.ProductService
}

func NewGetProductDetailsTool(products *services.ProductService) *GetProductDetailsTool {
	return &GetProductDetailsTool{products: products}
}

func (t *GetProductDetailsTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_product_details",
		Description: "Get full active product details including images, variants, specs, FAQ, inventory, and tiered pricing.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"productId"},
			"properties": map[string]any{
				"productId": map[string]any{"type": "integer", "description": "Product ID."},
			},
		},
	}
}

func (t *GetProductDetailsTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	productID := int64Arg(arguments, "productId")
	if productID <= 0 {
		return ai.ToolResult{}, fmt.Errorf("productId is required")
	}
	data, err := t.products.Details(ctx, productID)
	if err != nil {
		return ai.ToolResult{}, err
	}
	t.products.ApplySessionPrices(ctx, data, stringArg(arguments, "sessionId"))
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}

type CompareProductsTool struct {
	products *services.ProductService
}

func NewCompareProductsTool(products *services.ProductService) *CompareProductsTool {
	return &CompareProductsTool{products: products}
}

func (t *CompareProductsTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "compare_products",
		Description: "Compare up to five active products by price, stock, category, rating, and tiered pricing.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"productIds"},
			"properties": map[string]any{
				"productIds": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			},
		},
	}
}

func (t *CompareProductsTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	data, err := t.products.Compare(ctx, int64SliceArg(arguments, "productIds"))
	if err != nil {
		return ai.ToolResult{}, err
	}
	t.products.ApplySessionPrices(ctx, data, stringArg(arguments, "sessionId"))
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}

type ResolveVariantTool struct {
	products *services.ProductService
}

func NewResolveVariantTool(products *services.ProductService) *ResolveVariantTool {
	return &ResolveVariantTool{products: products}
}

func (t *ResolveVariantTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "resolve_variant",
		Description: "Resolve a product variant from selected attributes like color, size, or volume. Use before order creation when product has variants.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"productId", "selectedAttributes"},
			"properties": map[string]any{
				"productId":          map[string]any{"type": "integer"},
				"selectedAttributes": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			},
		},
	}
}

func (t *ResolveVariantTool) Execute(ctx context.Context, arguments map[string]any) (ai.ToolResult, error) {
	productID := int64Arg(arguments, "productId")
	if productID <= 0 {
		return ai.ToolResult{}, fmt.Errorf("productId is required")
	}
	data, err := t.products.ResolveVariant(ctx, productID, stringMapArg(arguments, "selectedAttributes"))
	if err != nil {
		return ai.ToolResult{}, err
	}
	t.products.ApplySessionPrices(ctx, data, stringArg(arguments, "sessionId"))
	content, err := jsonContent(data)
	return ai.ToolResult{Name: t.Definition().Name, Content: content, Data: data}, err
}
