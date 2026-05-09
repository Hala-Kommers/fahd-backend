**Go Backend Plan**

Goal: create a separate Go backend for `fahd-store`, replacing the current Node/Express backend while serving the new React frontend through real APIs later.

**Recommended Stack**

- Go HTTP framework: `chi` or `gin`
- Recommended: `chi`
- Reason: small, explicit, clean for REST APIs, easy middleware composition.

- Database: PostgreSQL
- ORM/query layer: `sqlc` or `gorm`
- Recommended: `sqlc`
- Reason: safer SQL, strong typing, less magic, better long-term maintainability.

- Migrations: `goose` or `atlas`
- Recommended: `goose`
- Auth: cookie/session-based admin auth initially
- Validation: `go-playground/validator` or manual validation
- WebSocket: `nhooyr.io/websocket` or `gorilla/websocket`
- AI integration: backend-owned service layer, not exposed to frontend

**Proposed Backend Structure**

```txt
fahd-backend/
  cmd/
    api/
      main.go
  internal/
    config/
    db/
    http/
      middleware/
      handlers/
      routes.go
    auth/
    products/
    orders/
    coupons/
    chat/
    admin/
    ai/
    policies/
    websocket/
  migrations/
  sql/
    queries/
  pkg/
  go.mod
  go.sum
  .env.example
```

**Phase 1: Backend Foundation**

Tasks:

1. Create new Go project.
2. Add config loading from env:
   - `DATABASE_URL`
   - `PORT`
   - `SESSION_SECRET`
   - `FRONTEND_ORIGIN`
   - `AI_PROVIDER`
   - `AI_API_KEY`
3. Add HTTP server with:
   - request logging
   - panic recovery
   - CORS for React frontend
   - JSON error responses
4. Add health endpoint:
   - `GET /health`
5. Add database connection pool.
6. Add migration system.

Deliverable:

- Go API starts successfully.
- Can connect to PostgreSQL.
- `GET /health` works.

**Phase 2: Database Schema**

Migrate the current shared schema into real PostgreSQL tables.

Core tables:

- `users`
- `products`
- `product_images`
- `product_variants`
- `categories`
- `orders`
- `order_items`
- `coupons`
- `policies`
- `global_faq`
- `conversations`
- `messages`
- `bot_config`
- `settings`

Important design choice:

- Store product variants/specs/FAQ either normalized or JSONB.
- Recommended:
  - products: normal columns for core fields
  - images: separate table
  - variants/specs/faq/pricing tiers: `jsonb` initially
- Reason: faster migration, flexible admin UI, easier to evolve.

Deliverable:

- DB migrations represent current app data model.
- Seed data can populate the storefront/admin UI.

**Phase 3: Customer Storefront APIs**

Implement APIs needed by the React customer UI.

Endpoints:

```txt
GET    /api/products
GET    /api/products/:id
GET    /api/categories
POST   /api/coupons/validate
POST   /api/orders
GET    /api/orders/:id
POST   /api/chat/start
POST   /api/chat/message
```

Behavior:

- Product listing supports:
  - search
  - category
  - sorting
  - min/max price
- Order creation validates:
  - customer name
  - phone
  - address
  - items
  - payment method
  - coupon
- Order tracking returns public-safe order details only.
- Chat endpoints create conversations and store messages.

Deliverable:

- Frontend can replace mock storefront API with real backend.

**Phase 4: Admin Auth**

Implement admin login/session system.

Endpoints:

```txt
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me
```

Recommended approach:

- HTTP-only secure cookie session.
- Passwords hashed with `bcrypt`.
- Middleware: `RequireAdmin`.

Deliverable:

- Admin UI can authenticate against Go backend.
- Protected admin routes reject unauthenticated requests.

**Phase 5: Admin APIs**

Implement admin APIs already expected by the React admin UI.

Products:

```txt
GET    /api/admin/products
POST   /api/admin/products
PATCH  /api/admin/products/:id
DELETE /api/admin/products/:id
```

Orders:

```txt
GET   /api/admin/orders
GET   /api/admin/orders/:id
PATCH /api/admin/orders/:id
```

Coupons:

```txt
GET    /api/admin/coupons
POST   /api/admin/coupons
PATCH  /api/admin/coupons/:code
DELETE /api/admin/coupons/:code
```

Policies/FAQ:

```txt
GET    /api/admin/policies
POST   /api/admin/policies
PATCH  /api/admin/policies/:id
DELETE /api/admin/policies/:id
GET    /api/admin/faq
PATCH  /api/admin/faq
```

Bot config:

```txt
GET   /api/admin/bot/config
PATCH /api/admin/bot/config
POST  /api/admin/bot/test-connection
GET   /api/admin/ai/stats
```

Conversations:

```txt
GET /api/admin/conversations
GET /api/admin/conversations/:id
```

Deliverable:

- Admin UI works with real backend data.

**Phase 6: WebSocket / Live Admin Updates**

Implement real-time admin notifications.

Endpoint:

```txt
/ws
```

Events:

- `new_order`
- `order_updated`
- `new_message`
- `conversation_updated`

Behavior:

- Admin opens WebSocket after login.
- Backend broadcasts new orders/messages.
- Frontend invalidates React Query caches.

Deliverable:

- Admin dashboard receives live updates.

**Phase 7: AI Service Layer**

Backend owns AI integration.

Responsibilities:

- Load bot config from DB.
- Build prompts using:
  - product catalog
  - product FAQ
  - global FAQ
  - policies
  - conversation history
- Save user and AI messages.
- Return AI reply to frontend.
- Optionally create draft order intent.

Internal package:

```txt
internal/ai/
  service.go
  prompts.go
  providers/
    google.go
```

Recommended first provider:

- Google Gemini, matching the old project.

Deliverable:

- `/api/chat/message` returns real AI responses.
- Admin bot settings affect behavior.

**Phase 8: Frontend Integration Switch**

Once backend is ready, update frontend from mock mode to real API mode.

Frontend change should be small:

- Add `VITE_API_BASE_URL`
- Update `queryClient.ts` to call real backend when mode is `real`.
- Keep mock mode for local demos.

Deliverable:

- Same frontend can run with either mock data or Go backend.

**Phase 9: Deployment**

Recommended deployment shape:

```txt
fahd-frontend  -> static hosting / CDN
fahd-backend   -> Go API server
postgres       -> managed PostgreSQL
```

Backend deployment needs:

- Dockerfile
- `.env.example`
- migration command
- seed command
- health check
- structured logs

Deliverable:

- Backend can run locally and deploy cleanly.

**Execution Order**

1. Go project foundation.
2. PostgreSQL schema/migrations.
3. Storefront APIs.
4. Admin auth.
5. Admin APIs.
6. WebSocket.
7. AI service.
8. Frontend real API integration.
9. Deployment setup.

**Main Decisions Needed**

1. Do you want the Go backend in a new folder like `D:\fahd-backend`, or inside the old `D:\fahd-store` repo?
2. Do you want to use PostgreSQL from the current project, or start with a fresh database?
3. Should the first backend implementation prioritize storefront APIs first, or admin/auth first?

My recommendation:

- Create a new project: `D:\fahd-backend`
- Use PostgreSQL
- Implement storefront APIs first, then admin/auth, then AI/WebSocket.