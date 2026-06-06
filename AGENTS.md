# AGENTS.md

## Project

Robe is a local-first personal assistant service written in Go.

The repository is intended to become a maintainable orchestration layer for private assistant workflows:

- Telegram input/output
- local LLM inference through Ollama
- future Google Calendar integration
- future Gmail read-only integration
- future web search integration
- future confirmation gates for sensitive actions
- future voice/TTS/mobile/glasses bridge

The project should remain simple, explicit and auditable. Avoid turning it into an opaque autonomous agent.

## Current stable state

The latest pushed stable version includes:

- Go module: `github.com/N1074/robe`
- HTTP server with `/health`
- `.env` based configuration
- Telegram bot adapter
- private Telegram access using `TELEGRAM_ALLOWED_USER_ID`
- `/start`, `/help`, `/ping`, `/status`
- `/ask <question>`
- local LLM integration through Ollama
- tested with `qwen3:14b`
- Makefile with `run`, `fmt`, `test`, `vet`, `check`
- initial config tests
- README with architecture and roadmap

Current runtime flow:

Telegram -> Robe Go server -> Telegram adapter -> Ollama LLM adapter -> qwen3:14b -> Telegram reply

## Local runtime environment

The project currently runs on a local Ubuntu server.

Project path on server:

    /opt/ai/projects/robe

Ollama endpoint currently used by the server:

    http://172.17.0.1:11434

This is because Ollama is configured through systemd with:

    OLLAMA_HOST=172.17.0.1:11434

Current main model:

    qwen3:14b

Observed behavior:

- qwen3:14b uses roughly 8.3 GB VRAM when loaded.
- It may emit internal `thinking` before final content.
- Low `num_predict` values can result in empty content because generation is consumed by thinking.
- `LLM_NUM_PREDICT=512` is currently used to avoid empty responses.
- Telegram should only display final content, not thinking.

Existing lighter model:

    dolphin-mistral:latest

## Development workflow recommendation

GitHub should be the source of truth.

Recommended workflow:

- develop on the main PC using an IDE
- push to GitHub
- pull on the server
- run `make check`
- run or restart Robe on the server

The server should be treated as runtime/staging, not as the primary IDE.

Manual server workflow:

    cd /opt/ai/projects/robe
    git pull
    make check
    make run

## Code quality rules

Prefer small, explicit, testable code.

Before committing:

    make check

This runs:

- gofmt
- go test ./...
- go vet ./...

Avoid committing secrets.

Never commit:

- `.env`
- tokens
- Telegram bot token
- Google OAuth secrets
- local database files
- logs
- model files

## Architecture direction

Telegram must remain a transport adapter only.

Telegram responsibilities:

- receive messages
- check authorized user
- send responses
- push notifications
- forward confirmation commands

Telegram must not contain business logic.

Core responsibilities:

- command handling
- intent routing
- session behavior
- deciding which tool to call
- deciding whether confirmation is required
- returning transport-agnostic responses

Target flow:

    Telegram adapter -> core assistant -> tool adapters -> core response -> Telegram adapter

Tool adapters should remain behind simple interfaces.

Planned adapters:

- LLM via Ollama
- Google Calendar
- Gmail read-only
- web search
- local storage
- future STT/TTS
- future mobile/glasses bridge

## Safety model

The LLM must not directly execute sensitive actions.

Policy:

- read operations may execute directly when authorized
- write operations require confirmation
- destructive actions are disabled until deliberately implemented
- email sending requires confirmation
- email deletion is not allowed in early versions
- calendar event creation requires confirmation
- calendar event deletion is not allowed in early versions
- external posting requires confirmation
- future tool executions should be auditable

Confirmation flow should eventually look like:

    user request -> action plan -> pending confirmation -> explicit confirm token -> execute

Avoid accepting ambiguous confirmations like "yes" unless tied to a specific pending action.

## Current refactor plan

Next desired refactor:

- create `internal/core`
- move command handling out of Telegram
- make Telegram call `core.Assistant.HandleText`
- add unit tests for core command behavior
- keep Telegram as a thin adapter

Expected commands after refactor:

- `/start`
- `/help`
- `/ping`
- `/status`
- `/ask <question>`

Core tests should cover:

- empty message
- unknown command
- `/ping`
- `/status`
- `/help`
- `/ask` with empty prompt
- `/ask` with mock LLM success
- `/ask` with mock LLM error

## Important current warning

There may be unfinished local working tree changes on the server from an attempted refactor.

Before continuing from a new workflow, check:

    git status

If the working tree contains broken WIP changes and they are not needed, restore to the latest pushed stable state before pulling or continuing:

    git restore cmd/robe-server/main.go internal/adapters/telegram/bot.go
    rm -rf internal/core
    make check

Only do this if the WIP changes are intentionally being discarded.

## Near-term roadmap

v0.1:

- local Telegram assistant using Ollama

v0.1.1:

- refactor Telegram into thin adapter
- introduce core assistant
- add core tests
- improve `/status`

v0.2:

- Google Calendar read-only

v0.3:

- Calendar event creation with confirmation gate

v0.4:

- Gmail read-only search and summarization

v0.5:

- web search adapter

v0.6:

- STT/TTS and mobile bridge

Later:

- Ray-Ban / glasses bridge as an input-output adapter, not as the core of the assistant
