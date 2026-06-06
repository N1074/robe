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

## Current intended state

The current intended version includes:

- Go module: `github.com/N1074/robe`
- HTTP server with `/health`
- `.env` based configuration
- Telegram bot adapter
- private Telegram access using `TELEGRAM_ALLOWED_USER_ID`
- `/start`, `/help`, `/ping`, `/status`
- `/ask <question>`
- command handling in `internal/core`
- `/status` reports env, LLM provider/model and Telegram access mode without secrets
- local LLM integration through Ollama
- tested with `qwen3:14b`
- Makefile with `run`, `fmt`, `test`, `vet`, `check`
- config, core command and LLM response cleanup tests
- README with architecture and roadmap

Current runtime flow:

Telegram -> Robe Go server -> Telegram adapter -> core assistant -> Ollama LLM adapter -> qwen3:14b -> Telegram reply

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
    git pull --ff-only
    make check
    make run

Server smoke checks after `make run`:

- `curl http://localhost:8080/health`
- Telegram `/ping`
- Telegram `/help`
- Telegram `/status`
- Telegram `/ask responde solo OK`
- confirm Telegram does not show model thinking text
- confirm `/status` does not expose tokens or secrets

## Conservative development workflow

Work conservatively and keep changes easy to review.

Before changing code:

- check `git status`
- read the smallest relevant set of files first
- identify whether the task is code, docs, tests or runtime/deployment
- avoid unrelated refactors
- preserve user or local changes unless explicitly told to discard them

While working:

- batch related file reads and searches where practical
- avoid repeated broad scans after the code shape is understood
- prefer targeted `rg`, `go test ./package`, and focused diffs during iteration
- use full `make check` or equivalent before finishing substantial work
- keep code simple, explicit and auditable
- let `gofmt` be authoritative for Go formatting
- do not spend extra cycles chasing harmless line-ending/index noise unless it affects the actual diff, commit contents or runtime behavior

Token efficiency matters. Prefer one good pass over several noisy passes. Group related investigation, implementation and verification steps so more time can be spent programming instead of re-reading the same context. Be flexible: avoid low-value cleanup rituals when they cost more attention than they save.

At the end of any substantial task:

- update `AGENTS.md` if workflow, architecture, safety model, runtime assumptions or roadmap changed
- update `README.md` if user-facing behavior, setup, commands, architecture or roadmap changed
- add or update tests for the behavior that changed
- run the most appropriate checks and report exactly what passed

## Code quality rules

Prefer small, explicit, testable code.

Before committing:

    make check

This runs:

- gofmt
- go test ./...
- go vet ./...

Useful Make targets:

- `make run`: run the server in the foreground using local `.env`
- `make build`: build `bin/robe-server`
- `make health`: call `http://localhost:8080/health`

Test coverage should scale with risk:

- command behavior belongs in core unit tests
- adapter behavior should be tested when parsing, filtering or transport-specific behavior changes
- LLM response cleanup and safety-sensitive logic should have focused tests
- future confirmation gates, tool execution and audit behavior must be tested before being treated as stable

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

## Current core state

Command handling now lives in `internal/core`.

Telegram calls `core.Assistant.HandleText` and should remain a thin transport adapter.

Current commands:

- `/start`
- `/help`
- `/ping`
- `/status`
- `/ask <question>`

Core tests should continue to cover:

- empty message
- unknown command
- `/ping`
- `/status`
- `/status` restricted and setup-open access modes
- `/help`
- `/ask` with empty prompt
- `/ask` with mock LLM success
- `/ask` with mock LLM error

## Important current warning

There may be unfinished local working tree changes on the server from an attempted refactor.

Before continuing from a new workflow, check:

    git status

If the working tree contains broken WIP changes and they are not needed, inspect them before restoring anything. Do not remove `internal/core` if it is part of the current checked-out version.

For old pre-core WIP only, the historical recovery command was:

    git restore cmd/robe-server/main.go internal/adapters/telegram/bot.go
    rm -rf internal/core
    make check

Only use destructive cleanup when the WIP changes are intentionally being discarded and the target stable version is known.

## Near-term roadmap

v0.1:

- local Telegram assistant using Ollama

v0.1.1:

- thin Telegram adapter
- core assistant command handling
- core command tests
- safer `/status`
- LLM thinking cleanup tests

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
