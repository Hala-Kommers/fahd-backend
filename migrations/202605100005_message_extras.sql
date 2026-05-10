-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_calls JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_results JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage_prompt_tokens INT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage_completion_tokens INT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage_cache_write_tokens INT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage_cache_read_tokens INT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage_reasoning_tokens INT;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS tool_calls;
ALTER TABLE messages DROP COLUMN IF EXISTS tool_results;
ALTER TABLE messages DROP COLUMN IF EXISTS usage_prompt_tokens;
ALTER TABLE messages DROP COLUMN IF EXISTS usage_completion_tokens;
ALTER TABLE messages DROP COLUMN IF EXISTS usage_cache_write_tokens;
ALTER TABLE messages DROP COLUMN IF EXISTS usage_cache_read_tokens;
ALTER TABLE messages DROP COLUMN IF EXISTS usage_reasoning_tokens;
