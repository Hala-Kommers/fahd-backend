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

### GET `/api/cities`

Returns active cities for storefront checkout city selection.

Response:

```json
{
  "data": [
    {
      "id": 1,
      "name": "Riyadh",
      "isActive": true,
      "sortOrder": 1
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
| `cityId` | number | Yes | Selected state/wilaya (`الولاية`) ID from `GET /api/cities`. |
| `addressZone` | string | No | City (`المدينة`) inside the selected state/wilaya. |
| `addressDistrict` | string | No | Neighborhood/district (`الحي`) inside the selected city. |
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
  "cityId": 1,
  "addressZone": "Al Malqa",
  "addressDistrict": "Al Aqiq",
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
    "addressRaw": "Riyadh, Al Malqa",
    "cityId": 1,
    "addressZone": "Al Malqa",
    "addressDistrict": "Al Aqiq"
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

### WebSocket `/api/ws/chat`

Real-time chat endpoint using WebSocket protocol for AI sales assistant communication.

#### Protocol

**Connection:** Open a WebSocket to `ws://host:port/api/ws/chat`.

**1. Initialize session (first message):**

Frontend sends:
```json
{ "type": "init" }
```

Backend responds:
```json
{
  "type": "session_created",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": 1710003600
}
```

Frontend stores `session_id` and `token` for reconnection.

**2. Send a message:**

```json
{ "type": "message", "content": "Do you have oud offers?" }
```

With page context (e.g. opened from a product page):

```json
{
  "type": "message",
  "content": "Does it come in blue?",
  "context": { "productId": 2, "variantId": 2 }
}
```

With city context (e.g. user selected a city from a dropdown):

```json
{
  "type": "message",
  "content": "أريد طلب هذا المنتج",
  "context": { "productId": 2, "variantId": 2, "cityId": 5 }
}
```

##### Context Fields

The optional `context` object tells the AI about the customer's current page/selection state so it doesn't need to ask for information the frontend already knows.

| Field | Type | Description |
| --- | --- | --- |
| `productId` | number | The product the customer is currently viewing. AI uses this ID directly in tool calls without asking. |
| `variantId` | number | The selected product variant (if applicable). |
| `cityId` | number | The state/wilaya (`الولاية`) selected by the customer from the checkout dropdown. AI uses this when creating orders without asking for the state/wilaya. |

Backend acknowledges immediately:
```json
{ "type": "message_received" }
```

**3. Receive AI response stream:**

```json
{ "type": "ai_typing" }

{ "type": "ai_chunk", "content": "Yes" }
{ "type": "ai_chunk", "content": "Yes, we have" }
{ "type": "ai_chunk", "content": "Yes, we have Oud Signature" }
{ "type": "ai_chunk", "content": "Yes, we have Oud Signature for 120 SAR." }

{ "type": "ai_done", "actions": [{"type":"show_product","payload":{"productId":1}}], "meta": {"needsHuman":false,"orderCreated":false,"orderId":null} }
```

**4. Reconnect (after disconnect):**

Frontend reopens WebSocket and sends:
```json
{
  "type": "auth",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

Backend validates token signature and responds:
```json
{ "type": "auth_ok", "session_id": "550e8400-e29b-41d4-a716-446655440000" }
```
or on failure:
```json
{ "type": "auth_error", "error": "token validation failed" }
```

**5. Heartbeat:**

Frontend sends `{ "type": "ping" }`, backend responds `{ "type": "pong" }`.

#### Client-to-Server Message Types

| Type | Fields | Description |
| --- | --- | --- |
| `init` | none | Create a new anonymous chat session. First message after connecting. |
| `auth` | `session_id`, `token` | Re-authenticate an existing session after reconnect. |
| `history` | none | Request conversation message history (most recent first). Must be authenticated first. |
| `message` | `content`, `context` | Send a chat message to the AI assistant. Optional `context` object can include `productId`, `variantId`, and `cityId` (see [Context Fields](#context-fields)). |
| `ping` | none | Heartbeat keepalive. |

#### Server-to-Client Event Types

| Type | Fields | Description |
| --- | --- | --- |
| `session_created` | `session_id`, `token`, `expires_at` | New session created after `init`. |
| `auth_ok` | `session_id` | Session authentication successful. |
| `auth_error` | `error` | Session authentication failed. |
| `history` | `messages` | Conversation message history in descending order. Each message includes `role`, `content`, `toolCalls`, `toolResults`, usage tokens, `provider`, `model`, `createdAt`. |
| `message_received` | none | Message accepted and queued for processing. |
| `ai_typing` | none | AI provider is processing the request. |
| `ai_chunk` | `content` | Incremental AI response text. Each chunk contains the full text so far. |
| `ai_done` | `actions`, `meta` | AI response complete. Contains any actions and metadata. |
| `ai_error` | `error` | AI processing error. |
| `pong` | none | Heartbeat response. |
| `error` | `error` | Protocol error (invalid message, rate limit, etc.). |

##### Actions in `ai_done`

The `actions` array in `ai_done` events contains structured objects the frontend should use to render interactive UI elements.

Each action is an object with `type` and optional `payload`:

| Type | Payload | Description |
| --- | --- | --- |
| `address_form` | none | Frontend should display a form for collecting customer name, phone, delivery address, city, and payment method. When the user submits the form, send the collected values as a normal `message` to the AI. |
| `order_confirmation` | none | Frontend should display a confirm order button under the AI message. When the user taps confirm, send an affirmative message (e.g. "confirm order") to the AI. |
| `show_product` | `{"productId": 5}` | Frontend should display a button or card linking to the specified product. |

`ai_done` example with actions:

```json
{
  "type": "ai_done",
  "actions": [
    { "type": "address_form" }
  ],
  "meta": {
    "needsHuman": false,
    "orderCreated": false,
    "orderId": null
  }
}
```

```json
{
  "type": "ai_done",
  "actions": [
    { "type": "order_confirmation" }
  ],
  "meta": {
    "needsHuman": false,
    "orderCreated": false,
    "orderId": null
  }
}
```

```json
{
  "type": "ai_done",
  "actions": [
    { "type": "show_product", "payload": { "productId": 1 } }
  ],
  "meta": {
    "needsHuman": false,
    "orderCreated": false,
    "orderId": null
  }
}
```

#### Security Notes

- **Session tokens are HMAC SHA256 signed JWTs** using the server's `JWT_SECRET`.
- A session UUID alone is **not authorization** — the signed token is required.
- Tokens expire after the configured `SESSION_TTL` (default 1 hour).
- The frontend must store both `session_id` and `token` and send `auth` on reconnect.
- Rate limiting is applied per session.

#### Frontend Example (JavaScript)

```javascript
class ChatClient {
  constructor(url) {
    this.url = url;
    this.sessionId = null;
    this.token = null;
    this.ws = null;
    this.listeners = {};
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      if (this.sessionId && this.token) {
        // Reconnect: authenticate existing session
        this.send({ type: "auth", session_id: this.sessionId, token: this.token });
      } else {
        // New session
        this.send({ type: "init" });
      }
    };

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      if (msg.type === "session_created") {
        this.sessionId = msg.session_id;
        this.token = msg.token;
        // Persist in localStorage for reconnection
        localStorage.setItem("chat_session_id", msg.session_id);
        localStorage.setItem("chat_token", msg.token);
      }

      if (msg.type === "auth_ok") {
        this.sessionId = msg.session_id;
      }

      const handler = this.listeners[msg.type];
      if (handler) handler(msg);
    };

    this.ws.onclose = () => {
      // Auto-reconnect after 1 second
      setTimeout(() => this.connect(), 1000);
    };
  }

  sendMessage(content) {
    this.send({ type: "message", content });
  }

  send(data) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  on(event, callback) {
    this.listeners[event] = callback;
  }

  close() {
    if (this.ws) this.ws.close();
  }
}

// Usage
const client = new ChatClient("ws://localhost:8080/api/ws/chat");

// Restore persisted session
const savedSession = localStorage.getItem("chat_session_id");
const savedToken = localStorage.getItem("chat_token");
if (savedSession && savedToken) {
  client.sessionId = savedSession;
  client.token = savedToken;
}

client.on("ai_chunk", (msg) => {
  document.getElementById("chat-reply").textContent = msg.content;
});

client.on("ai_done", (msg) => {
  console.log("Actions:", msg.actions);
  console.log("Meta:", msg.meta);
});

client.on("ai_error", (msg) => {
  console.error("AI error:", msg.error);
});

client.on("session_created", (msg) => {
  console.log("Session:", msg.session_id);
});

client.connect();

// Send message
client.sendMessage("Do you have oud offers?");
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

## Admin Coupons

All admin coupon endpoints require `Authorization: Bearer <access_token>`.

Coupon types are validated at the application level and must be `percentage` or `fixed`.

### GET `/api/admin/coupons`

Returns paginated coupons for admin coupon management.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `code` | string | No | Partial coupon code match. |
| `type` | string | No | Filter by `percentage` or `fixed`. |
| `isActive` | boolean | No | Filter active/inactive coupons. Accepts `true` or `false`. |
| `search` | string | No | Backward-compatible partial code search. |

Example:

```http
GET /api/admin/coupons?page=1&limit=20&code=WELCOME&type=percentage&isActive=true
```

Response:

```json
{
  "data": [
    {
      "id": 1,
      "code": "WELCOME10",
      "type": "percentage",
      "value": 10,
      "minOrder": 100,
      "maxDiscountAmount": 50,
      "usageLimit": 500,
      "usageCount": 12,
      "startsAt": "2026-05-10T00:00:00Z",
      "expiresAt": "2026-06-10T00:00:00Z",
      "isActive": true,
      "createdAt": "2026-05-10T10:00:00Z",
      "updatedAt": "2026-05-10T10:00:00Z"
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

### GET `/api/admin/coupons/:code`

Returns one coupon by code. Code matching is case-insensitive.

Response:

```json
{
  "data": {
    "id": 1,
    "code": "WELCOME10",
    "type": "percentage",
    "value": 10,
    "minOrder": 100,
    "maxDiscountAmount": 50,
    "usageLimit": 500,
    "usageCount": 12,
    "startsAt": "2026-05-10T00:00:00Z",
    "expiresAt": "2026-06-10T00:00:00Z",
    "isActive": true,
    "createdAt": "2026-05-10T10:00:00Z",
    "updatedAt": "2026-05-10T10:00:00Z"
  }
}
```

### POST `/api/admin/coupons`

Creates a coupon. `code`, `type`, and `value` are required.

Body fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `code` | string | Yes | Unique coupon code. Stored uppercase. |
| `type` | string | Yes | `percentage` or `fixed`. |
| `value` | number | Yes | Discount value. Must be greater than `0`. Percentage cannot exceed `100`. |
| `minOrder` | number | No | Minimum subtotal required. Must be non-negative. |
| `maxDiscountAmount` | number | No | Maximum discount cap. Must be non-negative. Usually for percentage coupons. |
| `usageLimit` | number | No | Maximum total redemptions. Must be non-negative. |
| `startsAt` | string | No | ISO timestamp when coupon starts. |
| `expiresAt` | string | No | ISO timestamp when coupon expires. Must be after `startsAt` when both are sent. |
| `isActive` | boolean | No | Defaults to database default `true` when omitted. |

Request:

```json
{
  "code": "WELCOME10",
  "type": "percentage",
  "value": 10,
  "minOrder": 100,
  "maxDiscountAmount": 50,
  "usageLimit": 500,
  "startsAt": "2026-05-10T00:00:00Z",
  "expiresAt": "2026-06-10T00:00:00Z",
  "isActive": true
}
```

Response: same shape as `GET /api/admin/coupons/:code` with status `201`.

### PATCH `/api/admin/coupons/:code`

Updates a coupon by code. Code matching is case-insensitive. The coupon `code` itself cannot be changed; create a new coupon if a new code is needed.

Body: any subset of `type`, `value`, `minOrder`, `maxDiscountAmount`, `usageLimit`, `startsAt`, `expiresAt`, and `isActive`.

Request:

```json
{
  "value": 15,
  "maxDiscountAmount": 75,
  "isActive": true
}
```

Response: same shape as `GET /api/admin/coupons/:code`.

### DELETE `/api/admin/coupons/:code`

Deletes a coupon by code. Code matching is case-insensitive.

Response:

```json
{
  "data": {
    "code": "WELCOME10",
    "deleted": true
  }
}
```

## Admin Orders

All admin order endpoints require `Authorization: Bearer <access_token>`.

### GET `/api/admin/orders`

Returns paginated orders for admin order management. This endpoint does not create orders.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `orderNumber` | string | No | Partial order number match. |
| `name` | string | No | Partial customer name match. |
| `phone` | string | No | Partial customer phone match. |
| `status` | string | No | Filter by order status: `new`, `confirmed`, `shipped`, `delivered`, `returned`, `cancelled`. |
| `city` | string/number | No | Filter by state/wilaya (`الولاية`). Pass a city ID or partial city name. |
| `search` | string | No | Backward-compatible search across order number, customer name, and customer phone. |

Example:

```http
GET /api/admin/orders?page=1&limit=20&orderNumber=ORD&name=Ahmed&phone=966&status=new&city=1
```

Response:

```json
{
  "data": [
    {
      "id": 12,
      "orderNumber": "ORD-1710000000000000000",
      "conversationId": 3,
      "createdAt": "2026-05-10T10:00:00Z",
      "customerName": "Ahmed",
      "customerPhone": "+966500000000",
      "customerEmail": "ahmed@example.com",
      "cityId": 1,
      "addressCity": "Riyadh",
      "addressZone": "Al Malqa",
      "addressDistrict": "Al Aqiq",
      "total": 108,
      "currency": "SAR",
      "paymentMethod": "COD",
      "paymentStatus": "pending",
      "status": "new"
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

Returns full order details with order items and the linked conversation when the order was created from AI chat.

Response:

```json
{
  "data": {
    "id": 12,
    "orderNumber": "ORD-1710000000000000000",
    "conversationId": 3,
    "createdAt": "2026-05-10T10:00:00Z",
    "updatedAt": "2026-05-10T10:00:00Z",
    "status": "new",
    "paymentMethod": "COD",
    "paymentStatus": "pending",
    "subtotal": 120,
    "shipping": 0,
    "discount": 12,
    "grandTotal": 108,
    "currency": "SAR",
    "couponCode": "WELCOME10",
    "customerName": "Ahmed",
    "customerPhone": "+966500000000",
    "customerEmail": "ahmed@example.com",
    "addressRaw": "Riyadh, Al Malqa",
    "cityId": 1,
    "addressCity": "Riyadh",
    "addressZone": "Al Malqa",
    "addressDistrict": "Al Aqiq",
    "items": [
      {
        "id": 1,
        "productId": 1,
        "variantId": null,
        "sku": "P-1001",
        "title": "Oud Signature",
        "qty": 1,
        "unitPrice": 120,
        "lineTotal": 120,
        "product": {
          "id": 1,
          "title": "Oud Signature",
          "slug": "oud-signature",
          "sku": "P-1001",
          "primaryImage": "https://images.unsplash.com/photo-1594035910387-fea47794261f"
        }
      }
    ],
    "conversation": {
      "id": 3,
      "title": "Order conversation",
      "customerName": "Ahmed",
      "customerPhone": "+966500000000",
      "channel": "web",
      "status": "active",
      "lastMessageAt": "2026-05-10T10:00:00Z",
      "createdAt": "2026-05-10T10:00:00Z",
      "updatedAt": "2026-05-10T10:00:00Z",
      "messages": [
        {
          "id": 1,
          "role": "user",
          "content": "I want to order Oud Signature",
          "toolCalls": null,
          "toolResults": null,
          "usagePromptTokens": 0,
          "usageCompletionTokens": 0,
          "usageCacheWriteTokens": 0,
          "usageCacheReadTokens": 0,
          "usageReasoningTokens": 0,
          "provider": "",
          "model": "",
          "createdAt": "2026-05-10T10:00:00Z"
        },
        {
          "id": 2,
          "role": "assistant",
          "content": "Your order has been created.",
          "toolCalls": "[{\"name\":\"create_order\"}]",
          "toolResults": "[{\"name\":\"create_order\"}]",
          "usagePromptTokens": 500,
          "usageCompletionTokens": 150,
          "usageCacheWriteTokens": 0,
          "usageCacheReadTokens": 0,
          "usageReasoningTokens": 0,
          "provider": "google",
          "model": "gemini-2.0-flash",
          "createdAt": "2026-05-10T10:00:01Z"
        }
      ]
    }
  }
}
```

### PATCH `/api/admin/orders/:id`

Updates order status only. Status is validated at the application level and must be one of: `new`, `confirmed`, `shipped`, `delivered`, `returned`, `cancelled`.

Request:

```json
{
  "status": "confirmed"
}
```

Response:

```json
{
  "data": {
    "id": "12",
    "status": "confirmed"
  }
}
```

## Admin AI & Conversations

All admin AI and conversation endpoints require `Authorization: Bearer <access_token>`.

All paginated endpoints return:

```json
{
  "data": [],
  "meta": { "page": 1, "limit": 20, "total": 0, "totalPages": 0 }
}
```

Conversation statuses are `active`, `closed`, or `archived`.

Message roles are `user`, `assistant`, `admin`, `system`, or `tool` depending on source.

### GET `/api/admin/bot/config`

Returns the latest bot configuration. The stored API key is never returned; use `hasApiKey` to know if one exists.

Response fields:

| Field | Type | Description |
| --- | --- | --- |
| `id` | number | Config row ID. |
| `provider` | string | AI provider, currently expected to be `google`. |
| `model` | string | Provider model name. |
| `hasApiKey` | boolean | Whether an API key is stored. |
| `temperature` | number | Generation temperature. |
| `maxTokens` | number | Max model output tokens. |
| `enabled` | boolean | Whether the assistant is enabled. |
| `persona` | object | Persona settings used by the prompt builder. |
| `customInstructions` | string/null | Extra instructions appended to the assistant behavior. |
| `createdAt` | string | ISO timestamp. |
| `updatedAt` | string | ISO timestamp. |

Response:

```json
{
  "data": {
    "id": 1,
    "provider": "google",
    "model": "gemini-2.0-flash",
    "hasApiKey": true,
    "temperature": 0.7,
    "maxTokens": 1000,
    "enabled": true,
    "persona": {"tone": "friendly_saudi", "style": "concise", "botName": "فهد", "language": "ar-SA", "emojiLevel": "medium"},
    "customInstructions": "Answer in Arabic only.",
    "createdAt": "2026-05-10T10:00:00Z",
    "updatedAt": "2026-05-10T10:00:00Z"
  }
}
```

If no config exists yet:
```json
{ "data": {} }
```

### PATCH `/api/admin/bot/config`

Updates bot configuration fields. Only send fields that should change.

When sending a new API key, it is encrypted before storage and never returned in responses.
If `apiKey` is set to an empty string, the stored key is cleared.
If `apiKey` is omitted, the existing key (if any) is preserved.

Body (all fields optional):

| Field | Type | Description |
| --- | --- | --- |
| `provider` | string | AI provider, e.g. `google`. |
| `model` | string | Model name, e.g. `gemini-2.0-flash`. |
| `temperature` | number | Generation temperature (0.0–1.0). |
| `maxTokens` | number | Max output tokens. |
| `enabled` | boolean | Enable/disable the sales assistant. |
| `persona` | object | JSON persona settings (tone, style, botName, language, emojiLevel). |
| `customInstructions` | string | Custom instructions appended to the AI system prompt. |
| `apiKey` | string | API key for the AI provider. Encrypted before storage. Send `""` to clear. |

Do not send unknown fields. The backend maps `maxTokens` to `max_tokens` and `customInstructions` to `custom_instructions` internally.

Request:

```json
{
  "provider": "google",
  "model": "gemini-2.0-flash",
  "temperature": 0.7,
  "maxTokens": 1000,
  "enabled": true,
  "persona": {"tone": "friendly_saudi", "style": "concise", "botName": "فهد", "language": "ar-SA", "emojiLevel": "medium"},
  "customInstructions": "Answer in Arabic only.",
  "apiKey": "your-api-key-here"
}
```

Response:

```json
{
  "data": {
    "id": 1
  }
}
```

### POST `/api/admin/bot/test-connection`

Returns a lightweight provider check for the currently stored config. This endpoint currently confirms which provider will be used; it does not send a live model prompt.

Response:

```json
{
  "data": {
    "ok": true,
    "provider": "google"
  }
}
```

### GET `/api/admin/ai/stats`

Aggregated AI usage statistics with optional filters and breakdowns. Filters apply consistently to message totals, distinct conversation totals, usage sums, provider/model breakdowns, and conversation status breakdowns.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `from` | string | No | Start date (ISO 8601) to filter message usage. |
| `to` | string | No | End date (ISO 8601) to filter message usage. |
| `provider` | string | No | Filter usage by provider name (e.g. `google`). |
| `model` | string | No | Filter usage by model name (e.g. `gemini-2.0-flash`). |

Example:

```http
GET /api/admin/ai/stats?from=2026-05-01T00:00:00Z&to=2026-05-31T23:59:59Z&provider=google&model=gemini-2.0-flash
```

Response:

```json
{
  "data": {
    "messages": 42,
    "conversations": 5,
    "usage": {
      "promptTokens": 12500,
      "completionTokens": 3400,
      "cacheWriteTokens": 0,
      "cacheReadTokens": 0,
      "reasoningTokens": 0
    },
    "byProvider": [
      { "provider": "google", "totalMessages": 42, "promptTokens": 12500, "completionTokens": 3400 }
    ],
    "byModel": [
      { "model": "gemini-2.0-flash", "totalMessages": 35, "promptTokens": 10500, "completionTokens": 2800 }
    ],
    "byStatus": [
      { "status": "active", "totalConversations": 3 },
      { "status": "closed", "totalConversations": 2 }
    ]
  }
}
```

### GET `/api/admin/conversations`

Paginated list of conversations with last message, message count, and token usage. Use this for the conversations index page.

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `20`, max: `100`. |
| `status` | string | No | Filter by `active`, `closed`, or `archived`. |

Example:

```http
GET /api/admin/conversations?page=1&limit=20&status=active
```

Response:

```json
{
  "data": [
    {
      "id": 1,
      "status": "active",
      "customerName": "Ahmed",
      "customerPhone": "+966500000000",
      "createdAt": "2026-05-10T10:00:00Z",
      "updatedAt": "2026-05-10T10:05:00Z",
      "messageCount": 5,
      "lastMessage": "Yes, we have Oud Signature for 120 SAR.",
      "lastMessageRole": "assistant",
      "totalPromptTokens": 2500,
      "totalCompletionTokens": 800
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 5,
    "totalPages": 1
  }
}
```

### GET `/api/admin/conversations/:id`

Returns full conversation details with all messages including tool calls, tool results, token usage, provider, and model.

Path params:

| Param | Type | Description |
| --- | --- | --- |
| `id` | number | Conversation ID. |

Response:

```json
{
  "data": {
    "id": 1,
    "title": "Order conversation",
    "customerName": "Ahmed",
    "customerPhone": "+966500000000",
    "channel": "web",
    "status": "active",
    "lastMessageAt": "2026-05-10T10:05:00Z",
    "createdAt": "2026-05-10T10:00:00Z",
    "updatedAt": "2026-05-10T10:05:00Z",
    "messages": [
      {
        "id": 1,
        "role": "user",
        "content": "Do you have oud offers?",
        "toolCalls": null,
        "toolResults": null,
        "usagePromptTokens": 0,
        "usageCompletionTokens": 0,
        "usageCacheWriteTokens": 0,
        "usageCacheReadTokens": 0,
        "usageReasoningTokens": 0,
        "provider": "",
        "model": "",
        "createdAt": "2026-05-10T10:00:00Z"
      },
      {
        "id": 2,
        "role": "assistant",
        "content": "Yes, we have Oud Signature for 120 SAR.",
        "toolCalls": "[{\"id\":\"abc123\",\"name\":\"search_products\",\"arguments\":{\"query\":\"oud\"}}]",
        "toolResults": "[{\"toolCallId\":\"abc123\",\"name\":\"search_products\",\"content\":\"{\\\"data\\\":[...]}\"}]",
        "usagePromptTokens": 500,
        "usageCompletionTokens": 150,
        "usageCacheWriteTokens": 0,
        "usageCacheReadTokens": 0,
        "usageReasoningTokens": 0,
        "provider": "google",
        "model": "gemini-2.0-flash",
        "createdAt": "2026-05-10T10:00:01Z"
      }
    ]
  }
}
```

### GET `/api/admin/conversations/:id/messages`

Paginated message history for a specific conversation.

Path params:

| Param | Type | Description |
| --- | --- | --- |
| `id` | number | Conversation ID. |

Query params:

| Param | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | number | No | Page number. Default: `1`. |
| `limit` | number | No | Items per page. Default: `50`, max: `200`. |
| `role` | string | No | Filter by message role (`user`, `assistant`, `tool`). |

Example:

```http
GET /api/admin/conversations/1/messages?page=1&limit=50&role=assistant
```

Response:

```json
{
  "data": [
    {
      "id": 1,
      "role": "user",
      "content": "Do you have oud offers?",
      "toolCalls": null,
      "toolResults": null,
      "usagePromptTokens": 0,
      "usageCompletionTokens": 0,
      "usageCacheWriteTokens": 0,
      "usageCacheReadTokens": 0,
      "usageReasoningTokens": 0,
      "provider": "",
      "model": "",
      "createdAt": "2026-05-10T10:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 50,
    "total": 5,
    "totalPages": 1
  }
}
```

### POST `/api/admin/conversations/:id/close`

Closes a conversation (sets status to `closed`).

Path params:

| Param | Type | Description |
| --- | --- | --- |
| `id` | number | Conversation ID. |

Response:

```json
{
  "data": {
    "id": "1",
    "status": "closed"
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
