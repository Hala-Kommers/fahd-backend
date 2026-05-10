package http

import "time"

type Category struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Icon string `json:"icon"`
}

type City struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"isActive"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Product struct {
	ID               int64     `gorm:"primaryKey" json:"id"`
	Title            string    `json:"title"`
	Slug             string    `json:"slug"`
	SKU              string    `json:"sku"`
	Status           string    `json:"status"`
	DescriptionShort string    `json:"descriptionShort"`
	DescriptionLong  string    `json:"descriptionLong"`
	CategoryID       *int64    `json:"categoryId"`
	Cost             *float64  `json:"cost"`
	Price            float64   `json:"price"`
	CompareAt        *float64  `json:"compareAt"`
	Currency         string    `json:"currency"`
	InventoryMode    string    `json:"inventoryMode"`
	StockTotal       int       `json:"stockTotal"`
	LowStockThreshold int      `json:"lowStockThreshold"`
	IsFeatured       bool      `json:"isFeatured"`
	HasVariants      bool      `json:"hasVariants"`
	VariantOptions   []byte    `json:"variantOptions"`
	Specs            []byte    `json:"specs"`
	FAQ              []byte    `json:"faq"`
	UsageInstructions *string  `json:"usageInstructions"`
	SalesCount       int       `json:"salesCount"`
	Rating           float64   `json:"rating"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ProductImage struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	ProductID int64     `json:"productId"`
	URL       string    `json:"url"`
	IsPrimary bool      `json:"isPrimary"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProductVariant struct {
	ID            int64      `gorm:"primaryKey" json:"id"`
	ProductID     int64      `json:"productId"`
	SKU           string     `json:"sku"`
	Attributes    []byte     `json:"attributes"`
	PriceOverride *float64   `json:"priceOverride"`
	Stock         int        `json:"stock"`
	Image         *string    `json:"image"`
	IsActive      bool       `json:"isActive"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type PricingTier struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	ProductID     int64     `json:"productId"`
	Qty           int       `json:"qty"`
	Label         *string   `json:"label"`
	OriginalPrice float64   `json:"originalPrice"`
	FinalPrice    float64   `json:"finalPrice"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Coupon struct {
	Code      string     `gorm:"primaryKey" json:"code"`
	Type      string     `json:"type"`
	Value     float64    `json:"value"`
	MinOrder  *float64   `json:"minOrder"`
	ExpiresAt *time.Time `json:"expiresAt"`
	IsActive  bool       `json:"isActive"`
}

type Order struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	OrderNumber  string    `json:"orderNumber"`
	Status       string    `json:"status"`
	PaymentMethod string   `json:"paymentMethod"`
	Subtotal     float64   `json:"subtotal"`
	Shipping     float64   `json:"shipping"`
	Discount     float64   `json:"discount"`
	GrandTotal   float64   `json:"grandTotal"`
	Currency     string    `json:"currency"`
	CustomerName string    `json:"customerName"`
	CustomerPhone string   `json:"customerPhone"`
	AddressRaw   string    `json:"addressRaw"`
	CityID       *int64    `json:"cityId"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Conversation struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
