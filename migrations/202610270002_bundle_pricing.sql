-- +goose Up
ALTER TABLE products ADD COLUMN free_shipping BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE products ADD COLUMN bundle_only BOOLEAN NOT NULL DEFAULT FALSE;
-- final_price remains a per-piece price for existing quantity discounts.
-- bundle_price is the exact total for this quantity; never derive it from a rounded unit price.
ALTER TABLE pricing_tiers ADD COLUMN bundle_price NUMERIC(12,2) CHECK (bundle_price > 0);

-- +goose Down
ALTER TABLE pricing_tiers DROP COLUMN bundle_price;
ALTER TABLE products DROP COLUMN bundle_only;
ALTER TABLE products DROP COLUMN free_shipping;
