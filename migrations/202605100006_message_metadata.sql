-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS metadata JSONB;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS metadata;
