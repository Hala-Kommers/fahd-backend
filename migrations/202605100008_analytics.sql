-- +goose Up
CREATE TABLE IF NOT EXISTS analytics_events (
    id BIGSERIAL PRIMARY KEY,
    visitor_id TEXT,
    session_id TEXT,
    event_type TEXT NOT NULL CHECK (event_type IN ('page_view', 'chat_started', 'order_created')),
    path TEXT,
    referrer TEXT,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS visitor_id TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS session_id TEXT;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS visitor_id TEXT;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS session_id TEXT;

CREATE INDEX IF NOT EXISTS idx_analytics_events_created_at ON analytics_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_type_created_at ON analytics_events(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_visitor_id ON analytics_events(visitor_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_session_id ON analytics_events(session_id);
CREATE INDEX IF NOT EXISTS idx_orders_visitor_id ON orders(visitor_id);
CREATE INDEX IF NOT EXISTS idx_orders_session_id ON orders(session_id);
CREATE INDEX IF NOT EXISTS idx_conversations_visitor_id ON conversations(visitor_id);
CREATE INDEX IF NOT EXISTS idx_conversations_session_id ON conversations(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_conversations_session_id;
DROP INDEX IF EXISTS idx_conversations_visitor_id;
DROP INDEX IF EXISTS idx_orders_session_id;
DROP INDEX IF EXISTS idx_orders_visitor_id;
DROP INDEX IF EXISTS idx_analytics_events_session_id;
DROP INDEX IF EXISTS idx_analytics_events_visitor_id;
DROP INDEX IF EXISTS idx_analytics_events_type_created_at;
DROP INDEX IF EXISTS idx_analytics_events_created_at;
ALTER TABLE conversations DROP COLUMN IF EXISTS session_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS visitor_id;
ALTER TABLE orders DROP COLUMN IF EXISTS session_id;
ALTER TABLE orders DROP COLUMN IF EXISTS visitor_id;
DROP TABLE IF EXISTS analytics_events;
