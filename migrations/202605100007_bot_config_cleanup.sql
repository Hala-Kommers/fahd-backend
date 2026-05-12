-- +goose Up
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS custom_instructions TEXT;
ALTER TABLE bot_config DROP COLUMN IF EXISTS system_prompt;
ALTER TABLE bot_config DROP COLUMN IF EXISTS system;
ALTER TABLE bot_config DROP COLUMN IF EXISTS templates;
ALTER TABLE bot_config DROP COLUMN IF EXISTS closing;
ALTER TABLE bot_config DROP COLUMN IF EXISTS settings_json;

-- +goose Down
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS system_prompt TEXT;
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS system JSONB;
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS templates JSONB;
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS closing JSONB;
ALTER TABLE bot_config ADD COLUMN IF NOT EXISTS settings_json JSONB;
ALTER TABLE bot_config DROP COLUMN IF EXISTS custom_instructions;
