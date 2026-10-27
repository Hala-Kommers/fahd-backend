-- +goose Up
ALTER TABLE bot_config ALTER COLUMN model SET DEFAULT 'gemini-3.8-flash';
UPDATE bot_config
SET model = 'gemini-3.8-flash', updated_at = NOW()
WHERE provider IN ('google', 'gemini')
  AND model IN ('gemini-2.0-flash', 'gemini-2.0-flash-001', 'google/gemini-2.0-flash-001', 'models/gemini-2.0-flash');

-- +goose Down
ALTER TABLE bot_config ALTER COLUMN model SET DEFAULT 'gemini-2.0-flash';
-- Keep existing configured models: rolling back code must not select a retired model.
