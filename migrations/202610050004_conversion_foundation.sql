-- +goose Up
ALTER TABLE orders ADD COLUMN idempotency_key TEXT UNIQUE;
ALTER TABLE orders ADD COLUMN request_hash TEXT;
ALTER TABLE orders ADD COLUMN tracking_token TEXT;
ALTER TABLE orders ADD COLUMN is_test BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE orders ADD COLUMN stock_reserved BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE cities ADD COLUMN shipping_fee NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (shipping_fee >= 0);
ALTER TABLE cities ADD COLUMN delivery_estimate TEXT;
CREATE TABLE checkout_quotes (
 id TEXT PRIMARY KEY, request_hash TEXT NOT NULL, pricing_hash TEXT NOT NULL,
 session_id TEXT NOT NULL DEFAULT '', expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE chat_engagements (
 conversation_id BIGINT PRIMARY KEY REFERENCES conversations(id), session_id TEXT NOT NULL UNIQUE,
 visitor_id TEXT, product_id BIGINT, source TEXT NOT NULL DEFAULT 'direct',
 device TEXT NOT NULL DEFAULT 'unknown', returning_customer BOOLEAN NOT NULL DEFAULT FALSE,
 experiment TEXT NOT NULL DEFAULT 'baseline', is_test BOOLEAN NOT NULL DEFAULT FALSE,
 engaged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX chat_engagements_date ON chat_engagements(engaged_at);
CREATE INDEX checkout_quotes_expiry ON checkout_quotes(expires_at);
ALTER TABLE analytics_events DROP CONSTRAINT analytics_events_event_type_check;
ALTER TABLE analytics_events ADD CONSTRAINT analytics_events_event_type_check CHECK(event_type IN
 ('page_view','chat_started','chat_engaged','offer_viewed','offer_selected','checkout_started','order_created','order_confirmed','order_delivered','order_cancelled','order_returned','checkout_error','complement_clicked','web_vital','bot_latency'));
CREATE INDEX analytics_funnel ON analytics_events(session_id,event_type,created_at);
ALTER TABLE products ADD COLUMN video_url TEXT;
ALTER TABLE products ADD COLUMN related_product_ids JSONB NOT NULL DEFAULT '[]';
CREATE TABLE product_reviews (
 id BIGSERIAL PRIMARY KEY, order_id BIGINT NOT NULL REFERENCES orders(id), product_id BIGINT NOT NULL REFERENCES products(id),
 rating INT NOT NULL CHECK(rating BETWEEN 1 AND 5), body TEXT NOT NULL, display_name TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(order_id,product_id)
);
-- +goose Down
DROP TABLE product_reviews;
ALTER TABLE products DROP COLUMN video_url, DROP COLUMN related_product_ids;
DROP INDEX analytics_funnel;
ALTER TABLE analytics_events DROP CONSTRAINT analytics_events_event_type_check;
DELETE FROM analytics_events WHERE event_type NOT IN ('page_view','chat_started','order_created');
ALTER TABLE analytics_events ADD CONSTRAINT analytics_events_event_type_check CHECK(event_type IN ('page_view','chat_started','order_created'));
DROP TABLE chat_engagements;
DROP TABLE checkout_quotes;
ALTER TABLE cities DROP COLUMN shipping_fee, DROP COLUMN delivery_estimate;
ALTER TABLE orders DROP COLUMN idempotency_key, DROP COLUMN request_hash, DROP COLUMN tracking_token, DROP COLUMN is_test, DROP COLUMN stock_reserved;
