-- +goose Up
CREATE TABLE session_offers (
  session_id TEXT NOT NULL,
  product_id BIGINT NOT NULL REFERENCES products(id),
  ends_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '2 hours',
  PRIMARY KEY (session_id, product_id)
);
-- +goose Down
DROP TABLE session_offers;
