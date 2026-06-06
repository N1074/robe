# Robe

Robe is a local-first personal assistant service written in Go.

The project is designed as a small orchestration layer for private assistant workflows: Telegram input, local LLM inference through Ollama, and future adapters for calendar, email, search, voice and visual context.

Current status: **v0.1 development**.

## Goals

Robe is intended to be:

- local-first where practical
- explicit about permissions and side effects
- modular through adapters
- safe by default for sensitive actions
- easy to run on a home server
- presentable as a maintainable Go service

## Current capabilities

Implemented:

- HTTP server with `/health`
- private Telegram bot adapter
- `/ping`, `/start`, `/help`, `/status`
- `/ask <question>` using a local Ollama model
- `.env` based configuration
- basic project quality commands through `make`
- initial config tests

Planned:

- Google Calendar read
- Google Calendar event creation with confirmation
- Gmail read-only search and summarization
- web search adapter
- audit log
- confirmation gate for sensitive actions
- voice input and TTS
- mobile / glasses bridge

## Architecture

Current v0.1 flow:

Telegram → Robe Go server → Telegram adapter → Ollama LLM adapter → Local model response → Telegram reply

Target architecture:

Input adapters:

- Telegram
- HTTP / mobile
- future voice / glasses bridge

Core:

- session handling
- command and intent routing
- confirmation gate
- audit logging

Tool adapters:

- local LLM via Ollama
- Google Calendar
- Gmail read-only
- web search
- local storage

## Requirements

- Go 1.25+
- Ollama running locally
- Telegram bot token
- A local model available in Ollama, currently tested with `qwen3:14b`

The current Ollama endpoint used by this deployment is:

`http://172.17.0.1:11434`

This is useful when Ollama is exposed on the Docker bridge host interface for containers or local services.

## Configuration

Create a local `.env` file from the example:

    cp .env.example .env
    chmod 600 .env

Example:

    ROBE_ENV=dev
    ROBE_HTTP_ADDR=:8080

    TELEGRAM_BOT_TOKEN=
    TELEGRAM_ALLOWED_USER_ID=

    LLM_PROVIDER=ollama
    LLM_BASE_URL=http://172.17.0.1:11434
    LLM_MODEL=qwen3:14b
    LLM_NUM_PREDICT=512
    LLM_TEMPERATURE=0.2

`.env` must not be committed.

## Telegram setup

Create a bot with `@BotFather`, add the token to `.env`, then start Robe and send a message to the bot.

If `TELEGRAM_ALLOWED_USER_ID` is empty, Robe logs the detected Telegram user ID. Add that value to `.env` to restrict access to your account.

## Running

    make run

Health check:

    curl http://localhost:8080/health

Telegram commands:

- `/start`
- `/help`
- `/ping`
- `/status`
- `/ask <question>`

## Quality checks

    make check

This runs:

- `gofmt`
- `go test ./...`
- `go vet ./...`

## Safety model

Robe should not allow the LLM to directly execute sensitive actions.

The intended policy is:

- read operations may be executed directly when authorized
- write operations require explicit confirmation
- destructive actions are disabled until specifically implemented
- email deletion, email sending, calendar modification and external posting require confirmation gates
- all future tool executions should be auditable

## Development roadmap

### v0.1

Local Telegram assistant using Ollama.

### v0.2

Google Calendar read support.

### v0.3

Calendar event creation with confirmation.

### v0.4

Gmail read-only search and thread summarization.

### v0.5

Web search adapter.

### v0.6

Voice input, TTS and mobile bridge.

### Later

Ray-Ban / glasses bridge as an input-output adapter, not as the core of the assistant.
