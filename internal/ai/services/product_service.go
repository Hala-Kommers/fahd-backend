package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type ProductService struct {
	db *gorm.DB
}

func NewProductService(db *gorm.DB) *ProductService {
	return &ProductService{db: db}
}

func (s *ProductService) Search(ctx context.Context, query, category string, maxResults int, maxPrice float64) (map[string]any, error) {
	if maxResults <= 0 {
		maxResults = 3
	}
	if maxResults > 3 {
		maxResults = 3
	}

	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)
	if isGenericProductQuery(query) {
		query = ""
	}

	rows, err := s.search(ctx, query, category, maxResults, maxPrice)
	if err != nil {
		return nil, err
	}

	// If the model guessed an invalid category from natural language, retry without it.
	if len(rows) == 0 && category != "" {
		rows, err = s.search(ctx, query, "", maxResults, maxPrice)
		if err != nil {
			return nil, err
		}
	}

	return map[string]any{"count": len(rows), "products": rows}, nil
}

func (s *ProductService) search(ctx context.Context, query, category string, maxResults int, maxPrice float64) ([]map[string]any, error) {
	var rows []map[string]any
	db := s.db.WithContext(ctx).Table("products p").
		Select("p.id, p.title, p.slug, p.sku, p.price, p.compare_at, p.currency, p.stock_total, c.name AS category_name, img.url AS primary_image").
		Joins("LEFT JOIN categories c ON c.id = p.category_id").
		Joins("LEFT JOIN LATERAL (SELECT url FROM product_images i WHERE i.product_id = p.id ORDER BY i.is_primary DESC, i.sort_order ASC, i.id ASC LIMIT 1) img ON TRUE").
		Where("p.status = ?", "active").
		Order("p.title ASC").
		Limit(maxResults)

	if maxPrice > 0 {
		db = db.Where("p.price<=?", maxPrice)
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + NormalizeSearch(q) + "%"
		db = db.Where("normalize_arabic(p.title || ' ' || p.sku || ' ' || COALESCE(p.description_short,'') || ' ' || COALESCE(p.description_long,'')) ILIKE ?", like)
	}
	if cat := strings.TrimSpace(category); cat != "" {
		db = db.Where("c.slug = ? OR c.name ILIKE ?", cat, cat)
	}

	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	return rows, nil
}

func (s *ProductService) Details(ctx context.Context, productID int64) (map[string]any, error) {
	product := map[string]any{}
	if err := s.db.WithContext(ctx).Table("products p").
		Select("p.*, c.name AS category_name").
		Joins("LEFT JOIN categories c ON c.id = p.category_id").
		Where("p.id = ? AND p.status = ?", productID, "active").
		Take(&product).Error; err != nil {
		return nil, fmt.Errorf("get product details: %w", err)
	}

	attachProductRelations(ctx, s.db, productID, product)
	decodeJSONFields(product, "variant_options", "specs", "faq")

	return product, nil
}

func (s *ProductService) Compare(ctx context.Context, productIDs []int64) (map[string]any, error) {
	ids := make([]int64, 0, len(productIDs))
	seen := map[int64]bool{}
	for _, id := range productIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]any{"count": 0, "products": []any{}}, nil
	}
	if len(ids) > 5 {
		ids = ids[:5]
	}

	var rows []map[string]any
	if err := s.db.WithContext(ctx).Table("products p").
		Select("p.id, p.title, p.sku, p.price, p.compare_at, p.currency, p.stock_total, p.rating, c.name AS category_name").
		Joins("LEFT JOIN categories c ON c.id = p.category_id").
		Where("p.id IN ? AND p.status = ?", ids, "active").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("compare products: %w", err)
	}

	for _, row := range rows {
		if id, ok := int64FromAny(row["id"]); ok {
			var tiers []map[string]any
			_ = s.db.WithContext(ctx).Table("pricing_tiers").
				Select("qty, label, original_price, final_price").
				Where("product_id = ?", id).
				Order("qty ASC").Find(&tiers).Error
			row["pricing_tiers"] = tiers
		}
	}

	return map[string]any{"count": len(rows), "products": rows}, nil
}

func (s *ProductService) ResolveVariant(ctx context.Context, productID int64, selected map[string]string) (map[string]any, error) {
	var variants []map[string]any
	if err := s.db.WithContext(ctx).Table("product_variants").
		Where("product_id = ? AND is_active = TRUE", productID).
		Order("id ASC").Find(&variants).Error; err != nil {
		return nil, fmt.Errorf("load variants: %w", err)
	}

	if len(variants) == 0 {
		return map[string]any{"found": false, "resolved": false, "message": "Product has no active variants."}, nil
	}
	if len(variants) == 1 && len(selected) == 0 {
		decodeJSONFields(variants[0], "attributes")
		return map[string]any{"found": true, "resolved": true, "variant": variants[0], "missingOptions": []string{}}, nil
	}

	normalizedSelected := normalizeStringMap(selected)
	availableOptions := map[string][]string{}
	for _, variant := range variants {
		attrs := attributesFromRow(variant)
		for key, value := range attrs {
			availableOptions[key] = appendUnique(availableOptions[key], value)
		}

		if attributesMatch(attrs, normalizedSelected) {
			decodeJSONFields(variant, "attributes")
			return map[string]any{"found": true, "resolved": true, "variant": variant, "missingOptions": []string{}}, nil
		}
	}

	missing := missingOptions(availableOptions, normalizedSelected)
	return map[string]any{
		"found":            true,
		"resolved":         false,
		"missingOptions":   missing,
		"availableOptions": availableOptions,
		"message":          "No variant matches the selected options.",
	}, nil
}

func attachProductRelations(ctx context.Context, db *gorm.DB, productID int64, product map[string]any) {
	var images []map[string]any
	_ = db.WithContext(ctx).Table("product_images").Where("product_id = ?", productID).Order("is_primary DESC, sort_order ASC, id ASC").Find(&images).Error
	product["images"] = images

	var tiers []map[string]any
	_ = db.WithContext(ctx).Table("pricing_tiers").Where("product_id = ?", productID).Order("qty ASC").Find(&tiers).Error
	product["pricing_tiers"] = tiers

	var variants []map[string]any
	_ = db.WithContext(ctx).Table("product_variants").Where("product_id = ? AND is_active = TRUE", productID).Order("id ASC").Find(&variants).Error
	for _, variant := range variants {
		decodeJSONFields(variant, "attributes")
	}
	product["variants"] = variants
}

func decodeJSONFields(row map[string]any, keys ...string) {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok || raw == nil {
			continue
		}
		var bytes []byte
		switch v := raw.(type) {
		case []byte:
			bytes = v
		case string:
			bytes = []byte(v)
		default:
			continue
		}
		var decoded any
		if err := json.Unmarshal(bytes, &decoded); err == nil {
			row[key] = decoded
		}
	}
}

func attributesFromRow(row map[string]any) map[string]string {
	decodeJSONFields(row, "attributes")
	attrs := map[string]string{}
	if value, ok := row["attributes"].(map[string]any); ok {
		for k, v := range value {
			attrs[strings.ToLower(strings.TrimSpace(k))] = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		}
	}
	return attrs
}

func normalizeStringMap(input map[string]string) map[string]string {
	output := map[string]string{}
	for k, v := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		value := strings.ToLower(strings.TrimSpace(v))
		if key != "" && value != "" {
			output[key] = value
		}
	}
	return output
}

func attributesMatch(attrs, selected map[string]string) bool {
	if len(selected) == 0 {
		return false
	}
	for key, value := range selected {
		if attrs[key] != value {
			return false
		}
	}
	return true
}

func missingOptions(options map[string][]string, selected map[string]string) []string {
	missing := []string{}
	for key := range options {
		if selected[key] == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func isGenericProductQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	generic := map[string]bool{
		"product":         true,
		"products":        true,
		"item":            true,
		"items":           true,
		"catalog":         true,
		"store products":  true,
		"show products":   true,
		"search products": true,
		"all products":    true,
		"منتجات":          true,
		"المنتجات":        true,
		"كل المنتجات":     true,
	}
	return generic[q]
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// ApplySessionPrices keeps product tools consistent with the canonical checkout price.
func (s *ProductService) ApplySessionPrices(ctx context.Context, data map[string]any, sessionID string) {
	if sessionID == "" {
		return
	}
	apply := func(row map[string]any) {
		id, ok := int64FromAny(row["id"])
		if !ok {
			return
		}
		var expired struct {
			Price   float64
			Expired bool
		}
		err := s.db.WithContext(ctx).Raw(`SELECT p.compare_at AS price, TRUE AS expired FROM products p JOIN session_offers o ON o.product_id=p.id WHERE p.id=? AND o.session_id=? AND o.ends_at<=NOW() AND NOT p.has_variants AND p.compare_at>p.price`, id, sessionID).Scan(&expired).Error
		if err == nil && expired.Expired {
			row["price"] = expired.Price
			row["compare_at"] = nil
			row["pricing_tiers"] = []any{}
			row["offer_expired"] = true
		}
	}
	if rows, ok := data["products"].([]map[string]any); ok {
		for _, row := range rows {
			apply(row)
		}
	} else {
		apply(data)
	}
}
