# Fahd Backend API Endpoints

Base URL examples use:

```txt
http://localhost:8080
```

Protected admin endpoints require:

```http
Authorization: Bearer <access_token>
```

## Health

### GET `/health`

Checks API availability.

Response:

```json
{
  "status": "ok"
}
```

## Auth

### POST `/api/auth/login`

Logs in an admin and returns JWT access + refresh tokens.

Body:

```json
{
  "username": "admin",
  "password": "password"
}
```

Response:

```json
{
  "data": {
    "token": "<access_token>",
    "refreshToken": "<refresh_token>",
    "user": {
      "id": 1,
      "username": "admin",
      "name": "Admin User",
      "email": "admin@fahd.local",
      "role": "admin"
    }
  }
}
```

### POST `/api/auth/refresh`

Rotates refresh token and returns a new access token pair.

Body:

```json
{
  "refreshToken": "<refresh_token>"
}
```

Response:

```json
{
  "data": {
    "token": "<new_access_token>",
    "refreshToken": "<new_refresh_token>"
  }
}
```

### POST `/api/auth/logout`

Revokes a refresh token when provided.

Body:

```json
{
  "refreshToken": "<refresh_token>"
}
```

Response:

```json
{
  "data": {
    "ok": true
  }
}
```

### GET `/api/auth/me`

Returns current admin user from access token.

Headers:

```http
Authorization: Bearer <access_token>
```

Response:

```json
{
  "data": {
    "id": 1,
    "username": "admin",
    "name": "Admin User",
    "email": "admin@fahd.local",
    "role": "admin"
  }
}
```

## Storefront

### GET `/api/products`

Returns paginated active products for storefront listing.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `search` | string | No | Searches product title or SKU. |
| `category` | string | No | Category slug, for example `perfumes`. |
| `sort` | string | No | Supported: `updated_desc`, `price_asc`, `price_desc`, `title_asc`. |
| `minPrice` | number | No | Minimum product price. |
| `maxPrice` | number | No | Maximum product price. |

Example:

```http
GET /api/products?page=1&limit=12&search=oud&category=perfumes&sort=price_desc
```

Response:

```json
{
  "data": [
    {
      "id": 1,
      "title": "Oud Signature",
      "slug": "oud-signature",
      "sku": "P-1001",
      "status": "active",
      "categoryId": 1,
      "categoryName": "Perfumes",
      "primaryImage": "https://images.unsplash.com/photo-1594035910387-fea47794261f",
      "price": 120,
      "compareAt": 160,
      "currency": "SAR",
      "stockTotal": 50,
      "isFeatured": true,
      "salesCount": 0,
      "rating": 0
    }
  ],
  "meta": {
    "page": 1,
    "limit": 12,
    "total": 1,
    "totalPages": 1
  }
}
```

### GET `/api/products/:id`

Returns full product details.

Path params:

| Param | Type | Description |
| --- | --- | --- |
| `id` | number | Product ID. |

Response:

```json
{
  "data": {
    "id": 1,
    "title": "Oud Signature",
    "slug": "oud-signature",
    "sku": "P-1001",
    "status": "active",
    "descriptionShort": "Warm oud blend with amber notes",
    "descriptionLong": "A refined oud fragrance suitable for daily and evening use.",
    "categoryId": 1,
    "categoryName": "Perfumes",
    "cost": 65,
    "price": 120,
    "compareAt": 160,
    "currency": "SAR",
    "inventoryMode": "global",
    "stockTotal": 50,
    "lowStockThreshold": 5,
    "isFeatured": true,
    "hasVariants": false,
    "variantOptions": [],
    "specs": [{ "key": "volume", "value": "100ml" }],
    "faq": [{ "question": "How long does it last?", "answer": "6-8 hours" }],
    "usageInstructions": "Spray on pulse points from 15cm distance.",
    "images": [
      {
        "id": 1,
        "productId": 1,
        "url": "https://images.unsplash.com/photo-1594035910387-fea47794261f",
        "isPrimary": true,
        "sortOrder": 0
      }
    ],
    "variants": [],
    "pricingTiers": [
      {
        "id": 1,
        "productId": 1,
        "qty": 1,
        "label": "قطعة واحدة",
        "originalPrice": 160,
        "finalPrice": 120
      }
    ]
  }
}
```

### GET `/api/categories`

Returns all categories.

Response:

```json
{
  "data": [
    {
      "id": 1,
      "name": "Perfumes",
      "slug": "perfumes",
      "icon": "sparkles"
    }
  ]
}
```

### POST `/api/coupons/validate`

Validates coupon availability. The `discount` value in the response is the configured coupon value, not the computed monetary discount.

Body:

```json
{
  "code": "WELCOME10",
  "subtotal": 500
}
```

Response for 10 percent coupon:

```json
{
  "data": {
    "code": "WELCOME10",
    "discount": 10,
    "type": "percentage"
  }
}
```

### POST `/api/orders`

Creates an order. Frontend must not send totals. Backend computes prices, discount, grand total, and creates order items.

Body fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `paymentMethod` | string | Yes | `COD` or `Paymob`. |
| `customerName` | string | Yes | Customer name. |
| `customerPhone` | string | Yes | Customer phone. |
| `customerEmail` | string | No | Customer email. |
| `addressRaw` | string | Yes | Full address text. |
| `addressCity` | string | No | City name. |
| `couponCode` | string | No | Coupon code. |
| `items` | array | Yes | Order items. |

Item fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `productId` | number | Yes | Product ID. |
| `variantId` | number | No | Variant ID if selected. |
| `qty` | number | Yes | Quantity, must be greater than 0. |

Request:

```json
{
  "paymentMethod": "COD",
  "customerName": "Ahmed",
  "customerPhone": "+966500000000",
  "customerEmail": "ahmed@example.com",
  "addressRaw": "Riyadh, Al Malqa",
  "addressCity": "Riyadh",
  "couponCode": "WELCOME10",
  "items": [
    { "productId": 1, "qty": 1 }
  ]
}
```

Response:

```json
{
  "data": {
    "id": 12,
    "orderNumber": "ORD-1710000000000000000",
    "status": "new",
    "paymentMethod": "COD",
    "subtotal": 120,
    "shipping": 0,
    "discount": 12,
    "grandTotal": 108,
    "currency": "SAR",
    "customerName": "Ahmed",
    "customerPhone": "+966500000000",
    "addressRaw": "Riyadh, Al Malqa"
  }
}
```

### GET `/api/orders/:id`

Returns public-safe order summary.

Path params:

| Param | Type | Description |
| --- | --- | --- |
| `id` | number | Order ID. |

Response:

```json
{
  "data": {
    "id": 12,
    "orderNumber": "ORD-1710000000000000000",
    "status": "new",
    "paymentMethod": "COD",
    "subtotal": 120,
    "shipping": 0,
    "discount": 12,
    "grandTotal": 108,
    "currency": "SAR"
  }
}
```

### POST `/api/chat/start`

Starts a conversation.

Response:

```json
{
  "data": {
    "conversationId": 1
  }
}
```

### POST `/api/chat/message`

Sends a message to an existing conversation.

Body:

```json
{
  "conversationId": 1,
  "message": "Do you have oud offers?"
}
```

Response:

```json
{
  "data": {
    "conversationId": 1,
    "reply": "Thanks for your message. AI replies will be enabled in Phase 7."
  }
}
```

## Admin Products

All admin product endpoints require `Authorization: Bearer <access_token>`.

### GET `/api/admin/products`

Returns paginated products for admin product management.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `status` | string | No | Filter by `active`, `draft`, or `archived`. |

Response:

```json
{
  "data": [
    {
      "id": 1,
      "title": "Oud Signature",
      "isActive": true,
      "categoryName": "Perfumes",
      "sku": "P-1001",
      "price": 120,
      "stockTotal": 50,
      "primaryImage": "https://images.unsplash.com/photo-1594035910387-fea47794261f"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "totalPages": 1
  }
}
```

### GET `/api/admin/products/:id`

Returns full admin product details. This shape can be reused by create/update forms.

Response:

```json
{
  "data": {
    "id": 1,
    "title": "Oud Signature",
    "slug": "oud-signature",
    "sku": "P-1001",
    "status": "active",
    "isActive": true,
    "category": { "id": 1, "name": "Perfumes" },
    "descriptionShort": "Warm oud blend with amber notes",
    "descriptionLong": "A refined oud fragrance suitable for daily and evening use.",
    "pricing": {
      "cost": 65,
      "price": 120,
      "compareAt": 160,
      "currency": "SAR"
    },
    "pricingTiers": [
      { "qty": 1, "label": "قطعة واحدة", "originalPrice": 160, "finalPrice": 120 }
    ],
    "images": [
      { "url": "https://cdn.example.com/p1.jpg", "isPrimary": true, "sortOrder": 0 }
    ],
    "inventory": {
      "mode": "global",
      "stockTotal": 50,
      "lowStockThreshold": 5
    },
    "hasVariants": false,
    "variantOptions": [],
    "variants": [],
    "specs": [{ "key": "volume", "value": "100ml" }],
    "faq": [{ "question": "How long does it last?", "answer": "6-8 hours" }],
    "usageInstructions": "Spray on pulse points."
  }
}
```

### POST `/api/admin/products`

Creates product with all nested details.

Body fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `title` | string | Yes | Product title. |
| `slug` | string | Yes | URL slug. |
| `sku` | string | Yes | Unique SKU. |
| `status` | string | No | `active`, `draft`, `archived`. Default: `draft`. |
| `category.id` | number | No | Existing category ID. |
| `descriptionShort` | string | No | Short description. |
| `descriptionLong` | string | No | Full description. |
| `pricing.price` | number | Yes | Selling price. |
| `pricing.cost` | number | No | Internal cost. |
| `pricing.compareAt` | number | No | Original/struck-through price. |
| `pricing.currency` | string | No | Default: `SAR`. |
| `inventory.mode` | string | No | `global` or `variant`. Default: `global`. |
| `inventory.stockTotal` | number | Yes | Total stock. |
| `inventory.lowStockThreshold` | number | No | Low stock threshold. |
| `pricingTiers` | array | No | Tiered prices. |
| `images` | array | No | Product images. |
| `variants` | array | No | Product variants. |
| `specs` | array | No | Key/value specs. |
| `faq` | array | No | Product FAQ. |

Request:

```json
{
  "title": "Saffron Musk",
  "slug": "saffron-musk",
  "sku": "P-2001",
  "status": "active",
  "category": { "id": 1 },
  "descriptionShort": "Fresh saffron notes",
  "descriptionLong": "A luxurious blend of saffron and white musk.",
  "pricing": { "cost": 70, "price": 145, "compareAt": 190, "currency": "SAR" },
  "inventory": { "mode": "global", "stockTotal": 80, "lowStockThreshold": 10 },
  "isFeatured": true,
  "hasVariants": true,
  "variantOptions": ["size"],
  "specs": [{ "key": "volume", "value": "75ml" }],
  "faq": [{ "question": "Is it unisex?", "answer": "Yes" }],
  "usageInstructions": "Apply to pulse points.",
  "pricingTiers": [
    { "qty": 1, "label": "قطعة واحدة", "originalPrice": 190, "finalPrice": 145 }
  ],
  "images": [
    { "url": "https://cdn.example.com/products/p2001-main.jpg", "isPrimary": true, "sortOrder": 0 }
  ],
  "variants": [
    {
      "sku": "P-2001-75ML",
      "attributes": { "size": "75ml" },
      "priceOverride": 145,
      "stock": 80,
      "isActive": true
    }
  ]
}
```

Response:

```json
{
  "data": {
    "id": 22
  }
}
```

### PATCH `/api/admin/products/:id`

Updates product. Nested arrays (`pricingTiers`, `images`, `variants`) replace existing rows.

Body: same as `POST /api/admin/products`.

Response:

```json
{
  "data": {
    "id": 22
  }
}
```

### DELETE `/api/admin/products/:id`

Deletes product. Related images, variants, and pricing tiers are removed by cascade.

Response:

```json
{
  "data": {
    "id": "22",
    "deleted": true
  }
}
```

## Admin Orders

All admin order endpoints require `Authorization: Bearer <access_token>`.

### GET `/api/admin/orders`

Returns paginated orders for admin order management.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `search` | string | No | Searches by order number, customer name, or customer phone. |
| `status` | string | No | Filter by order status: `new`, `confirmed`, `processing`, `shipped`, `delivered`, `returned`, `cancelled`. |

Example:

```http
GET /api/admin/orders?page=1&limit=20&search=ahmed&status=new
```

Response:

```json
{
  "data": [
    {
      "id": 12,
      "orderNumber": "ORD-1710000000000000000",
      "createdAt": "2026-05-10T10:00:00Z",
      "customerName": "Ahmed",
      "customerPhone": "+966500000000",
      "addressCity": "Riyadh",
      "total": 108,
      "paymentMethod": "COD",
      "status": "new",
      "confidence": 0,
      "risk": 0
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "totalPages": 1
  }
}
```

### GET `/api/admin/orders/:id`

Returns full order details with order items.

Response:

```json
{
  "data": {
    "id": 12,
    "order_number": "ORD-1710000000000000000",
    "customer_name": "Ahmed",
    "customer_phone": "+966500000000",
    "address_city": "Riyadh",
    "grand_total": 108,
    "payment_method": "COD",
    "status": "new",
    "items": [
      {
        "id": 1,
        "order_id": 12,
        "product_id": 1,
        "sku": "P-1001",
        "title": "Oud Signature",
        "qty": 1,
        "unit_price": 120,
        "line_total": 120
      }
    ]
  }
}
```

### PATCH `/api/admin/orders/:id`

Updates order fields, typically `status`, `payment_status`, `notes`, `tags`, or risk fields.

Request:

```json
{
  "status": "confirmed",
  "notes": "Customer confirmed by phone"
}
```

Response:

```json
{
  "data": {
    "id": "12"
  }
}
```

## Common Error Shape

Errors return JSON with an `error` string.

```json
{
  "error": "invalid credentials"
}
```
