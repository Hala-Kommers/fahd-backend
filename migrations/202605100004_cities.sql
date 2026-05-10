-- +goose Up
CREATE TABLE IF NOT EXISTS cities (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO cities (name, sort_order)
VALUES
  ('Riyadh', 1),
  ('Jeddah', 2),
  ('Dammam', 3),
  ('Makkah', 4),
  ('Madinah', 5)
ON CONFLICT (name) DO NOTHING;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS city_id BIGINT REFERENCES cities(id) ON DELETE SET NULL;
ALTER TABLE orders DROP COLUMN IF EXISTS address_city;

CREATE INDEX IF NOT EXISTS idx_cities_active_sort ON cities(is_active, sort_order, name);
CREATE INDEX IF NOT EXISTS idx_orders_city_id ON orders(city_id);

-- +goose Down
DROP INDEX IF EXISTS idx_orders_city_id;
DROP INDEX IF EXISTS idx_cities_active_sort;
ALTER TABLE orders DROP COLUMN IF EXISTS city_id;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS address_city TEXT NOT NULL DEFAULT '';
DROP TABLE IF EXISTS cities;
