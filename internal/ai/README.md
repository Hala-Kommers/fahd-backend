# AI Foundation

This package is intentionally provider-agnostic.

The sales agent should depend on `ai.Provider`, `ai.Message`, `ai.Tool`, and `ai.GenerateRequest`, not on Google, OpenAI, Anthropic, or any model-specific SDK.

## Runtime Selection

Provider/model settings are loaded from the `bot_config` table:

- `provider`
- `model`
- `api_key`
- `temperature`
- `max_tokens`
- `enabled`
- prompt/persona JSON fields

`providers.NewProvider(botConfig)` is the only place that should decide which provider implementation to instantiate.

## Current Provider

Implemented:

- `google` / `gemini`

Planned:

- `openai`
- `anthropic`

## Next Step

Add the sales agent runtime that:

1. Loads bot config.
2. Builds a system prompt.
3. Loads conversation messages.
4. Registers commerce tools.
5. Calls the selected provider.
6. Executes requested tool calls.
7. Persists assistant replies.

## Logging

AI failures use structured `slog` logs configured by:

- `APP_ENV=development|production`
- `LOG_LEVEL=debug|info|warn|error`

Production uses JSON logs. Development uses text logs.

Do not log API keys, full prompts, raw customer messages, phone numbers, or addresses. Provider errors are logged with safe metadata such as provider, model, status code, and provider error category.
