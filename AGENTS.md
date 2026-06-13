# AGENTS.md

## Project

Robe is a local-first personal assistant service written in Go.

The repository is intended to become a maintainable orchestration layer for private assistant workflows:

- Telegram input/output
- local LLM inference through Ollama
- Google Calendar integration
- Gmail search/show and controlled label integration
- future web search integration
- confirmation gates for sensitive actions
- voice/TTS/mobile/glasses bridge

The project should remain simple, explicit and auditable. Avoid turning it into an opaque autonomous agent.

Primary governance references:

- `docs/ARCHITECTURE_GOVERNANCE.md`
- `docs/ROADMAP.md`
- `docs/INTENT_PROTOCOL.md`
- `docs/llm_traits/README.md`

## Current intended state

The current intended version includes:

- Go module: `github.com/N1074/robe`
- Go 1.25.8+
- HTTP server with `/health`
- `.env` based configuration
- Telegram bot adapter
- private Telegram access using `TELEGRAM_ALLOWED_USER_ID`
- `/start`, `/help`, `/ping`, `/status`
- `/ask <question>`
- command handling in `internal/core`
- `/status` reports env, LLM provider/model and Telegram access mode without secrets
- Google Calendar read/create/delete commands with explicit confirmation for writes
- Gmail search/show commands and controlled label support
- natural-language intent routing through the local LLM
- Telegram voice/audio input through configurable local STT
- structured local memory backed by Postgres
- LLM-proposed memory creation validated and executed by Robe Core
- optional Ollama embeddings for memory-assisted LLM retrieval
- central Core permission engine
- PostgreSQL audit events for memory writes and calendar write proposals/execution
- Core redaction contract for external content before LLM prompt injection
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
- `LLM_NUM_PREDICT=1024` is currently recommended to avoid empty responses, especially with intent parsing.
- Telegram should only display final content, not thinking.

Existing lighter model:

    dolphin-mistral:latest

Recommended embedding model:

    nomic-embed-text

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
- Telegram `/calendar today`
- Telegram `/calendar create Test | 2026-06-07 10:00 | 2026-06-07 10:15`
- Telegram natural language: `crea una cita mañana a las 12 con el dentista`
- Telegram voice message: `crea una cita mañana a las 12 con el dentista`
- Telegram `/calendar delete <event_id>`
- Telegram `/pending`, `/confirm <token>`, `/cancel <token>`
- Telegram `/remember <text>` and `/memories <query>`
- Telegram `/askmem <query> | <question>` when memory is configured; with embeddings enabled it should report used memory IDs
- Telegram natural memory request with a configured project alias, for example: `recuerda que para example-project quiero hablar en kilos, no en cajas`
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
- update `docs/INTENT_PROTOCOL.md` when LLM/Core action JSON changes
- update `docs/ARCHITECTURE_GOVERNANCE.md` or `docs/ROADMAP.md` when governance, compliance, domain architecture or long-term sequencing changes
- update `docs/llm_traits` when reusable LLM behavior traits change
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
- `make google-auth`: create/update the local Google Calendar OAuth token
- `make build`: build `bin/robe-server`
- `make health`: call `http://localhost:8080/health`
- `make db-up`: start local Postgres
- `make db-down`: stop local Postgres
- `make db-logs`: follow Postgres logs
- `make db-psql`: open psql in the Postgres container

Embedding configuration:

- `EMBEDDING_PROVIDER=ollama` enables explicit memory embeddings
- `EMBEDDING_BASE_URL` defaults to the Ollama base URL
- `EMBEDDING_MODEL=nomic-embed-text` is the current recommended local embedding model
- embeddings are stored with memory rows in Postgres and may be reindexed later if the model changes
- embeddings support compact context injection; this is not RAG yet
- embedding failures must not prevent explicit memory storage or ordinary LLM answers; fall back to non-semantic retrieval where practical

Project alias configuration:

- Core must remain project-agnostic; do not hardcode user projects, client names, home/career labels or personal domains in Go code.
- `MEMORY_PROJECT_ALIASES` is the private runtime place for project inference hints.
- Format: `project=alias1,alias2;other-project=alias3`.
- `.env` is not committed, so personal project aliases belong there or in server-side secrets/configuration.

On Windows PowerShell, `make` may not be installed and local script execution may be restricted. Use `powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 run`, `google-auth`, `check`, `build`, `health`, `db-up`, `db-down`, `db-logs` or `db-psql` as local equivalents. Keep `make` as the Ubuntu server workflow.

Test coverage should scale with risk:

- command behavior belongs in core unit tests
- adapter behavior should be tested when parsing, filtering or transport-specific behavior changes
- LLM response cleanup and safety-sensitive logic should have focused tests
- future confirmation gates, tool execution and audit behavior must be tested before being treated as stable
- in `internal/core`, keep tests mirrored by code file for readability: `assistant_test.go`, `calendar_test.go`, `intent_test.go`, `memory_test.go` and `governance_test.go`
- shared core test doubles belong in `test_helpers_test.go`, not at the bottom of a feature test file

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

Professional architecture rule:

- Core owns orchestration, permissions, validation, state and execution.
- LLM proposes actions and context transformations.
- Adapters perform I/O and translate provider-specific APIs.
- PostgreSQL is the source of truth for durable state.
- No module should invent its own permission, confirmation or audit model.
- Personal project aliases, private domains, user-specific labels and secrets must not be hardcoded in Go code or committed documentation.
- LLM traits are context guidance only; they never grant tool permissions or bypass Core validation.

Planned adapters:

- LLM via Ollama
- Google Calendar
- Gmail search/show and controlled labels
- web search
- local storage
- local STT command adapter
- Postgres memory store
- Postgres audit event store
- Ollama embeddings adapter
- STT/TTS
- future mobile/glasses bridge

## Safety model

The LLM must not directly execute sensitive actions.

Policy:

- read operations may execute directly when authorized
- write operations require confirmation
- memory writes are an exception only when the user explicitly asks Robe to remember durable context
- the LLM may propose `create_memory`, but Robe Core owns validation, normalization and persistence
- the LLM must never write directly to Postgres
- no silent autonomous memory writes unless explicitly enabled in a later design
- invalid explicit memory proposals should be reported as not saved rather than silently persisted
- destructive actions are disabled until deliberately implemented
- email sending requires confirmation
- email deletion is not allowed in early versions
- calendar event creation requires confirmation
- calendar event deletion requires confirmation
- natural-language calendar write intents require the same explicit confirmation tokens as command-based writes
- voice-transcribed calendar write intents require the same explicit confirmation tokens as text intents
- external posting requires confirmation
- future tool executions should be auditable
- current memory writes and calendar write proposals/execution are audited when Postgres is configured
- new side-effecting tools must use the Core permission engine and audit event model instead of inventing local policy
- external content destined for LLM prompt injection must pass through Core redaction
- Gmail natural-language commands are read-only in early versions; sending, deletion, archive and unsubscribe execution are not implemented
- Gmail label mutation is reserved for Core-owned review workflows and must be validated and audited before automatic use
- Gmail review labels must stay controlled under `Robe/...`; project-specific or user-specific labels should come from database-backed rules, not free-form LLM output
- raw email sender names and addresses are Core-private; prompts, summaries and normal Robe responses should use safe aliases such as `Maria S. B.`
- `/email show` is safe by default; raw message display requires `/email show raw <message_id>`
- contact relationship/category proposals from the LLM must be validated by Core before writing to `ContactDirectory`
- email review automation must start in dry-run mode with audit records before any scheduler is enabled
- `CONTACT_ENCRYPTION_KEY` enables encrypted storage for contact private fields; without it, new raw contact identity values should not be persisted in private columns
- `CONTACT_ENCRYPTION_PREVIOUS_KEYS` allows explicit contact encryption rotation into the current key; do not run rotation automatically during normal startup
- durable multi-account email configuration belongs in Postgres `email_accounts`; scheduler implementation is technical debt and must wait until manual dry-run review has been tested carefully

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
- `/calendar today`
- `/calendar tomorrow`
- `/calendar week`
- `/calendar create <title> | <start> | <end> [| location] [| description]`
- `/calendar delete <event_id>`
- `/email search <query>`
- `/email show <message_id>`
- `/email show raw <message_id>`
- `/email review dry-run`
- natural-language email search/show intents parsed by the LLM
- `/pending`
- `/confirm <token>`
- `/cancel <token>`
- natural-language calendar create/list/delete intents parsed by the LLM
- Telegram voice/audio messages transcribed to text before core handling
- STT commands should print only the transcript on stdout; logs belong on stderr. Robe also filters common `whisper.cpp` log lines defensively.
- `/remember <text>` stores manual memory; `/memories <query>` searches memory
- `/askmem <memory query> | <question>` injects bounded, explicit memory context into an LLM answer and reports used memory IDs
- natural-language memory requests can become `create_memory` intents when explicit phrases are present, such as `recuerda que`, `ten en cuenta que`, `from now on`, `de ahora en adelante` or `a partir de ahora`
- when embeddings are configured, explicit memory creation stores a vector and normal LLM answers may receive compact relevant memory context
- embeddings are retrieval metadata, not autonomous memory
- `/forget <memory_id>` archives a memory; it does not physically delete from Postgres
- `/memory show <id>`, `/memory tag <id> <tag>` and `/memory archive <id>` support memory curation
- structured memory supports kind, project scope, tags, source, confidence, importance, status and timestamps
- supported memory kinds are `preference`, `fact`, `decision`, `constraint`, `task_context`, `contact_context` and `operational_note`
- project scopes are user-defined through `/project create` and optional private `MEMORY_PROJECT_ALIASES`
- projects are explicit; global memories have no project
- side-effecting actions are classified by the Core permission engine
- audit events are written through a Core-owned `AuditLogger`
- Postgres persists audit events in `audit_events` when the Postgres store is configured
- `RedactExternalContentForPrompt` is the Core-owned redaction contract for future email, web and RAG content before LLM prompt injection

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
- calendar list with mock Calendar
- calendar create proposal without execution
- calendar delete proposal without execution
- `/confirm <token>` execution
- `/cancel <token>`
- `/pending`
- natural-language calendar create intent without execution
- natural-language calendar list intent
- natural-language email search/show intent with mock Email
- `/remember` with mock MemoryStore
- `/memories` with mock MemoryStore
- `/memory show/tag/archive` with mock MemoryStore
- project create/use/list behavior with mock MemoryStore
- memory embeddings and semantic `/askmem` filters with mock Embedder
- natural-language memory create intent with validation before persistence
- compact memory retrieval before LLM answers, including global and project-scoped memory
- rejected memory create proposals for missing explicit intent, sensitive content or unsupported kind
- permission classification for memory and calendar actions
- audit records for memory writes, rejected memory proposals, calendar proposals, confirmation execution and cancellation

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

- Google Calendar read commands

v0.3:

- Calendar event creation and deletion with confirmation gate

v0.3.1:

- Natural-language intent routing for calendar reads and proposals

v0.4:

- Gmail search/show, controlled review labels and initial review summaries

v0.5:

- web search adapter

v0.6:

- STT/TTS and mobile bridge

v0.7:

- manual local memory through Postgres
- structured project-aware memory before autonomous memory behavior
- LLM-proposed memory actions validated and persisted by Robe Core
- optional embeddings for memory-assisted LLM context
- project context and future RAG as core/tool/storage capabilities, not Telegram logic

v0.8:

- central permission engine
- audit event model in Core
- Postgres `audit_events` persistence
- audit tests for memory writes and calendar write lifecycle

Later:

- Ray-Ban / glasses bridge as an input-output adapter, not as the core of the assistant
