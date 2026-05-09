package http

import "time"

type Category struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Icon string `json:"icon"`
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
	Price            float64   `json:"price"`
	CompareAt        *float64  `json:"compareAt"`
	Currency         string    `json:"currency"`
	StockTotal       int       `json:"stockTotal"`
	IsFeatured       bool      `json:"isFeatured"`
	SalesCount       int       `json:"salesCount"`
	Rating           float64   `json:"rating"`
	UpdatedAt        time.Time `json:"updatedAt"`
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
	CreatedAt    time.Time `json:"createdAt"`
}

type Conversation struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
