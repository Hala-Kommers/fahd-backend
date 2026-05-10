package tools

import (
	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/services"

	"gorm.io/gorm"
)

type Registry struct {
	tools []ai.Tool
}

func NewRegistry(db *gorm.DB) *Registry {
	productService := services.NewProductService(db)
	orderService := services.NewOrderService(db)
	return &Registry{tools: []ai.Tool{
		NewSearchProductsTool(productService),
		NewGetProductDetailsTool(productService),
		NewCompareProductsTool(productService),
		NewResolveVariantTool(productService),
		NewCreateOrderTool(orderService),
		NewLookupOrderTool(orderService),
	}}
}

func (r *Registry) Tools() []ai.Tool {
	return r.tools
}

func (r *Registry) Definitions() []ai.ToolDefinition {
	definitions := make([]ai.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func (r *Registry) Find(name string) (ai.Tool, bool) {
	for _, tool := range r.tools {
		if tool.Definition().Name == name {
			return tool, true
		}
	}
	return nil, false
}
