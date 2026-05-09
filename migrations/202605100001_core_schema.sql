-- +goose Up

-- users
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT UNIQUE,
    name TEXT,
    email TEXT UNIQUE,
    phone TEXT,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- categories
CREATE TABLE IF NOT EXISTS categories (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    icon TEXT,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- products
CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    sku TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'draft', 'archived')),
    description_short TEXT,
    description_long TEXT,
    category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL,

    -- pricing
    cost NUMERIC(12,2),
    price NUMERIC(12,2) NOT NULL,
    compare_at NUMERIC(12,2),
    currency TEXT NOT NULL DEFAULT 'SAR',

    -- inventory
    inventory_mode TEXT NOT NULL DEFAULT 'global' CHECK (inventory_mode IN ('global', 'variant')),
    stock_total INT NOT NULL DEFAULT 0,
    low_stock_threshold INT NOT NULL DEFAULT 5,

    -- merchandising
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    has_variants BOOLEAN NOT NULL DEFAULT FALSE,
    variant_options JSONB NOT NULL DEFAULT '[]'::jsonb,
    specs JSONB NOT NULL DEFAULT '[]'::jsonb,
    faq JSONB NOT NULL DEFAULT '[]'::jsonb,
    usage_instructions TEXT,

    sales_count INT NOT NULL DEFAULT 0,
    rating NUMERIC(3,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- normalized images table
CREATE TABLE IF NOT EXISTS product_images (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- normalized variants table
CREATE TABLE IF NOT EXISTS product_variants (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    price_override NUMERIC(12,2),
    stock INT NOT NULL DEFAULT 0,
    image TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id, sku)
);

-- normalized pricing tiers (source of truth for pricing tiers)
CREATE TABLE IF NOT EXISTS pricing_tiers (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    qty INT NOT NULL CHECK (qty > 0),
    label TEXT,
    original_price NUMERIC(12,2) NOT NULL,
    final_price NUMERIC(12,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id, qty)
);

-- coupons
CREATE TABLE IF NOT EXISTS coupons (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('percentage', 'fixed')),
    value NUMERIC(12,2) NOT NULL CHECK (value >= 0),
    min_order NUMERIC(12,2),
    max_discount_amount NUMERIC(12,2),
    usage_limit INT,
    usage_count INT NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- orders
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_number TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    status TEXT NOT NULL CHECK (status IN ('new', 'confirmed', 'processing', 'shipped', 'delivered', 'returned', 'cancelled')),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('COD', 'Paymob')),
    payment_status TEXT NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending', 'paid', 'failed', 'refunded')),

    subtotal NUMERIC(12,2) NOT NULL,
    shipping NUMERIC(12,2) NOT NULL,
    discount NUMERIC(12,2) NOT NULL,
    grand_total NUMERIC(12,2) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'SAR',
    coupon_code TEXT,

    customer_name TEXT NOT NULL,
    customer_phone TEXT NOT NULL,
    customer_email TEXT,

    address_raw TEXT NOT NULL,
    address_city TEXT NOT NULL,
    address_district TEXT,
    address_street TEXT,
    address_building_no TEXT,
    address_landmark TEXT,
    address_confidence NUMERIC(5,2) NOT NULL DEFAULT 0,

    risk_score NUMERIC(5,2) NOT NULL DEFAULT 0,
    risk_flags JSONB NOT NULL DEFAULT '[]'::jsonb,
    otp_status TEXT NOT NULL DEFAULT 'none' CHECK (otp_status IN ('none', 'sent', 'verified')),

    notes TEXT,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    activity_log JSONB NOT NULL DEFAULT '[]'::jsonb,
    meta_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT REFERENCES products(id) ON DELETE SET NULL,
    variant_id BIGINT REFERENCES product_variants(id) ON DELETE SET NULL,
    sku TEXT,
    title TEXT NOT NULL,
    variant JSONB,
    qty INT NOT NULL CHECK (qty > 0),
    unit_price NUMERIC(12,2) NOT NULL,
    line_total NUMERIC(12,2) NOT NULL,
    snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- policies and faq
CREATE TABLE IF NOT EXISTS policies (
    id BIGSERIAL PRIMARY KEY,
    key TEXT UNIQUE,
    title TEXT NOT NULL,
    content JSONB NOT NULL DEFAULT '[]'::jsonb,
    cities JSONB,
    body TEXT,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applies_to TEXT NOT NULL DEFAULT 'all' CHECK (applies_to IN ('all', 'category', 'product')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS global_faq (
    id BIGSERIAL PRIMARY KEY,
    question TEXT,
    answer TEXT,
    items JSONB,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT faq_shape_check CHECK (
        (items IS NOT NULL) OR (question IS NOT NULL AND answer IS NOT NULL)
    )
);

-- conversations and messages
CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    title TEXT,
    customer_name TEXT,
    customer_phone TEXT,
    channel TEXT NOT NULL DEFAULT 'web',
    status TEXT NOT NULL CHECK (status IN ('active', 'closed', 'archived')),
    sentiment TEXT CHECK (sentiment IN ('positive', 'neutral', 'negative')),
    summary TEXT,
    last_message_at TIMESTAMPTZ,
    meta_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender TEXT CHECK (sender IN ('user', 'fahd')),
    role TEXT CHECK (role IN ('user', 'assistant', 'admin', 'system')),
    text TEXT,
    content TEXT,
    metadata JSONB,
    tokens_used INT,
    provider TEXT,
    model TEXT,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT message_shape_check CHECK (
        (text IS NOT NULL AND sender IS NOT NULL) OR (content IS NOT NULL AND role IS NOT NULL)
    )
);

-- bot config
CREATE TABLE IF NOT EXISTS bot_config (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'google',
    model TEXT NOT NULL DEFAULT 'gemini-2.0-flash',
    api_key TEXT,
    temperature NUMERIC(3,2) NOT NULL DEFAULT 0.70,
    max_tokens INT NOT NULL DEFAULT 1000,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    system_prompt TEXT,
    persona JSONB NOT NULL DEFAULT '{}'::jsonb,
    system JSONB NOT NULL DEFAULT '{}'::jsonb,
    templates JSONB NOT NULL DEFAULT '{}'::jsonb,
    closing JSONB NOT NULL DEFAULT '{}'::jsonb,
    settings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- settings
CREATE TABLE IF NOT EXISTS settings (
    id BIGSERIAL PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT,
    value_json JSONB,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT settings_value_check CHECK (value IS NOT NULL OR value_json IS NOT NULL)
);

-- chat order drafts
CREATE TABLE IF NOT EXISTS chat_order_drafts (
    conversation_id BIGINT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    product_title TEXT,
    product_sku TEXT,
    product_id BIGINT REFERENCES products(id) ON DELETE SET NULL,
    unit_price NUMERIC(12,2),
    qty INT,
    variant JSONB,
    customer_name TEXT,
    customer_phone TEXT,
    address_raw TEXT,
    city TEXT,
    payment_method TEXT CHECK (payment_method IN ('COD', 'Online')),
    coupon_code TEXT,
    discount NUMERIC(12,2),
    step TEXT NOT NULL DEFAULT 'idle' CHECK (step IN (
      'idle', 'collecting_product', 'collecting_name', 'collecting_phone',
      'collecting_address', 'collecting_payment', 'confirming', 'done'
    )),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- indexes
CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_status ON products(status);
CREATE INDEX IF NOT EXISTS idx_products_active_featured ON products(status, is_featured);
CREATE INDEX IF NOT EXISTS idx_products_price ON products(price);
CREATE INDEX IF NOT EXISTS idx_product_images_product_id ON product_images(product_id);
CREATE INDEX IF NOT EXISTS idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX IF NOT EXISTS idx_pricing_tiers_product_id ON pricing_tiers(product_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_phone ON orders(customer_phone);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_coupons_code_active ON coupons(code, is_active);
CREATE INDEX IF NOT EXISTS idx_conversations_last_message_at ON conversations(last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_timestamp ON messages(conversation_id, timestamp DESC);

-- +goose Down
DROP TABLE IF EXISTS chat_order_drafts;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS bot_config;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS global_faq;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS coupons;
DROP TABLE IF EXISTS pricing_tiers;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;
