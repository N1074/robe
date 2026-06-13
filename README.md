# Robe

Robe is a local-first personal assistant service written in Go.

The project is designed as a small orchestration layer for private assistant workflows: Telegram input, local LLM inference through Ollama, and future adapters for calendar, email, search, voice and visual context.

Current status: **v0.7 development**.

## Goals

Robe is intended to be:

- local-first where practical
- explicit about permissions and side effects
- modular through adapters
- safe by default for sensitive actions
- easy to run on a home server
- presentable as a maintainable Go service

## Governance

Architecture and compliance rules live in:

- [Architecture and Governance](docs/ARCHITECTURE_GOVERNANCE.md)
- [Roadmap](docs/ROADMAP.md)
- [Intent Protocol](docs/INTENT_PROTOCOL.md)
- [LLM Traits](docs/llm_traits/README.md)

Short version: the Core owns orchestration, permissions, persistence and execution. The LLM proposes; Core validates and executes. PostgreSQL is the source of truth for durable state.

## Current capabilities

Implemented:

- HTTP server with `/health`
- private Telegram bot adapter
- command handling through `internal/core`
- `/ping`, `/start`, `/help`, `/status`
- `/ask <question>` using a local Ollama model
- Google Calendar read/create/delete behind confirmation gates
- Gmail search/show commands and controlled label support
- natural-language intent routing through the local LLM
- Telegram voice/audio input through configurable local STT
- structured local memory backed by Postgres
- optional Ollama embeddings for memory-assisted LLM retrieval
- central permission engine for Core actions
- PostgreSQL audit trail for memory writes and calendar write proposals/execution
- deterministic Core redaction for memory context before LLM prompt injection
- `.env` based configuration
- basic project quality commands through `make`
- config, core command and LLM response cleanup tests

Planned:

- Gmail summarization
- web search adapter
- broader PII redaction for future email, web and RAG inputs
- task system
- coaching system
- TTS
- mobile / glasses bridge
- project context and future RAG

## Architecture

Current text flow:

Telegram -> Robe Go server -> Telegram adapter -> core assistant -> Ollama LLM adapter -> Local model response -> Telegram reply

Current voice flow:

Telegram voice/audio -> Telegram adapter -> STT adapter -> core assistant -> intent/LLM/tools -> Telegram reply

Architecture:

Input adapters:

- Telegram
- HTTP / mobile
- voice through Telegram STT
- future mobile / glasses bridge

Core:

- command and intent routing
- session handling
- confirmation gate
- permission decisions
- audit event recording

Tool adapters:

- local LLM via Ollama
- local embeddings via Ollama
- Google Calendar
- Gmail search/show and controlled labels
- web search
- Postgres memory store

## Requirements

- Go 1.25.8+
- Ollama running locally
- Telegram bot token
- Optional local STT command for voice/audio input
- A local model available in Ollama, currently tested with `qwen3:14b`
- Optional embedding model in Ollama, for example `nomic-embed-text`

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
    LLM_NUM_PREDICT=1024
    LLM_TEMPERATURE=0.2
    PROMPTS_DIR=

    CALENDAR_PROVIDER=google
    CALENDAR_ID=primary
    CALENDAR_CREDENTIALS_FILE=/opt/ai/projects/robe/secrets/google-calendar-credentials.json
    CALENDAR_TOKEN_FILE=/opt/ai/projects/robe/secrets/google-calendar-token.json
    CALENDAR_TIMEZONE=Europe/Madrid

    EMAIL_PROVIDER=gmail
    GMAIL_CREDENTIALS_FILE=/opt/ai/projects/robe/secrets/google-gmail-credentials.json
    GMAIL_TOKEN_FILE=/opt/ai/projects/robe/secrets/google-gmail-token.json
    GMAIL_USER_ID=me

    STT_PROVIDER=command
    STT_COMMAND=/opt/ai/bin/transcribe-audio
    STT_ARGS={audio}
    STT_TIMEOUT_SECONDS=120

    MEMORY_PROVIDER=postgres
    DATABASE_URL=postgres://robe:robe_dev_password@localhost:5432/robe?sslmode=disable
    MEMORY_PROJECT_ALIASES=garden=veg,orchard;writing=novel,draft

    EMBEDDING_PROVIDER=ollama
    EMBEDDING_BASE_URL=http://172.17.0.1:11434
    EMBEDDING_MODEL=nomic-embed-text

`.env` must not be committed.

`PROMPTS_DIR` is optional. When set, Robe loads `system_chat.txt` and `system_intent.txt` from that directory; otherwise it uses the embedded defaults in `internal/adapters/llm/prompts`.

## Local database

Robe uses Postgres for local memory and audit events when `MEMORY_PROVIDER=postgres`.

Start the database:

    make db-up

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 db-up

Useful database commands:

    make db-logs
    make db-psql
    make db-down

## Project aliases

Robe Core is project-agnostic: personal project names and aliases should not be hardcoded in Go code or committed documentation.

Create project records with `/project create <slug> | <name>`. If you want natural language such as "for garden..." to map to a project, configure private aliases in `.env`:

    MEMORY_PROJECT_ALIASES=garden=veg,orchard;writing=novel,draft

The format is `project=alias1,alias2;other-project=alias3`. Keep real personal project aliases in `.env` or server-side configuration, not in the repository.

## Memory embeddings

Embeddings are optional and are intended for LLM context retrieval, not for autonomous memory writes or RAG.

When `EMBEDDING_PROVIDER=ollama` is configured, explicit memory creation stores an embedding alongside the structured memory record. Normal LLM answers, natural conversation and `/askmem <memory query> | <question>` can use semantic similarity to select bounded memory context. PostgreSQL remains the source of truth; the embedding is metadata on the audited memory row.

Install the embedding model on the Ubuntu server:

    ollama pull nomic-embed-text

Then set:

    EMBEDDING_PROVIDER=ollama
    EMBEDDING_BASE_URL=http://172.17.0.1:11434
    EMBEDDING_MODEL=nomic-embed-text

Existing memories without embeddings remain searchable by text. If embedding generation fails, Robe still stores explicit memories without a vector and normal answers fall back to non-semantic retrieval. To give old memories semantic retrieval, re-save or backfill them later with an explicit maintenance command.

## Google Calendar setup

Calendar integration uses Google OAuth credentials and a local token file.

Generate the token on the server after configuring `CALENDAR_CREDENTIALS_FILE` and `CALENDAR_TOKEN_FILE`:

    make google-auth

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 google-auth

Open the printed URL, approve Calendar access, paste the authorization code, then keep the generated token file under `secrets/` or another non-committed path.

## Gmail setup

Gmail integration supports search, message display and controlled Robe labels for future automated review. It does not send, delete, archive or unsubscribe.

Generate the Gmail token after configuring `GMAIL_CREDENTIALS_FILE` and `GMAIL_TOKEN_FILE`:

    GOOGLE_AUTH_TARGET=gmail make google-auth

On Windows PowerShell:

    $env:GOOGLE_AUTH_TARGET="gmail"; powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 google-auth

Open the printed URL, approve Gmail modify access, paste the authorization code, then keep the generated token file under `secrets/` or another non-committed path. Existing tokens created for read-only Gmail access must be regenerated because Robe now needs label permissions.

Current controlled labels are:

- `Robe/Reviewed`
- `Robe/Important`
- `Robe/NeedsAttention`
- `Robe/Category/Admin`
- `Robe/Category/People`
- `Robe/Category/OnlinePurchases`
- `Robe/Category/Finance`
- `Robe/Category/Projects`
- `Robe/Category/Notifications`
- `Robe/Category/Other`

These labels are intentionally broad. Project-specific or user-specific labels should come later from database-backed rules, not from free-form LLM output.

Email sender identity is split between Core-private and Robe-facing forms. Core may retain the raw display name and email address for lookup and future contact rules, but prompts, summaries and ordinary Robe responses should use a safe alias. For example, `Maria Sanchez Barroso <maria@example.com>` becomes `Maria S. B.` outside Core. If Gmail provides only an email address, Robe uses `Unknown sender` instead of exposing the local part or domain.

## Telegram setup

Create a bot with `@BotFather`, add the token to `.env`, then start Robe and send a message to the bot.

If `TELEGRAM_ALLOWED_USER_ID` is empty, Robe logs the detected Telegram user ID. Add that value to `.env` to restrict access to your account.

## Running

    make run

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 run

Build a local binary:

    make build

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 build

Health check:

    curl http://localhost:8080/health

Or, if the server is already running:

    make health

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 health

Telegram commands:

- `/start`
- `/help`
- `/ping`
- `/status` shows environment, LLM provider/model and access mode
- `/ask <question>`
- `/askmem <memory query> | <question>`
- `/remember <text>`
- `/memories <query>`
- `/forget <memory_id>`
- `/memory show <id>`
- `/memory archive <id>`
- `/memory tag <id> <tag>`
- `/project list`
- `/project create <slug> | <name>`
- `/project use <slug>`
- `/project status`
- `/calendar today`
- `/calendar tomorrow`
- `/calendar week`
- `/calendar create <title> | <start> | <end> [| location] [| description]`
- `/calendar delete <event_id>`
- `/email search <query>`
- `/email show <message_id>`
- `/email show raw <message_id>`
- `/email review dry-run`
- `/pending`
- `/confirm <token>`
- `/cancel <token>`

Natural language also works for supported calendar intents. For example:

    crea una cita de calendario para mañana a las 12 con el dentista
    que tengo mañana en el calendario

Calendar create/delete requests made in natural language still return a proposal and require `/confirm <token>`.

Natural language also works for email search when Gmail is configured. For example:

    busca correos de alice sobre facturas
    ensename el correo msg_123

Email natural-language actions are read-only for now. Robe may search or show a specific message ID, but it does not send, delete, archive, label or unsubscribe from natural language.

Natural memory requests also work through the normal assistant flow. For example:

    recuerda que para garden quiero hablar en kilos, no en cajas
    ten en cuenta que para garden quiero hablar en kilos, no en cajas

The LLM may classify these as `create_memory`, but it only proposes the action. Robe Core validates explicit user intent, normalizes project/kind metadata, stores the memory in Postgres and confirms the saved record. The LLM never writes to Postgres directly.

If the proposal is invalid, Robe reports that the memory was not saved instead of silently persisting a malformed record.

Voice messages and audio files sent to Telegram are transcribed first, then processed like normal text. The STT command should print the transcript to stdout. Use `{audio}` in `STT_ARGS` where the downloaded audio path should be inserted; if omitted, Robe appends the audio path as the final argument. Robe filters common `whisper.cpp` log lines defensively, but a transcript-only wrapper is still preferred.

For `whisper.cpp`, make the wrapper print only the transcript on stdout. Keep logs on stderr and use `-np -nt`:

    /opt/ai/src/whisper.cpp/build/bin/whisper-cli -m /opt/ai/models/ggml-small.bin -f "$TMP_WAV" -l es -np -nt

If Robe replies that Calendar is not configured yet, calendar OAuth/config is not active. Set `CALENDAR_PROVIDER=google`, credentials/token paths and restart Robe.

If Robe replies that Email is not configured yet, Gmail OAuth/config is not active. Set `EMAIL_PROVIDER=gmail`, Gmail credentials/token paths and restart Robe.

Set `CONTACT_ENCRYPTION_KEY` before enabling the Postgres contact directory in a real mailbox. Robe stores a deterministic address hash for lookup and stores contact private fields such as `contacts.full_name`, `contact_addresses.email` and `contact_addresses.display_name_seen` encrypted with AES-GCM when this key is present. Without the key, new raw contact identity values are not persisted in those private columns.

For key rotation, set a new `CONTACT_ENCRYPTION_KEY` and put the old key in `CONTACT_ENCRYPTION_PREVIOUS_KEYS`. On startup, Robe can decrypt values with previous keys and re-encrypt them with the current key.

Email review scheduling is disabled by default. To run the multi-account review loop, set `EMAIL_REVIEW_ENABLED=true`; it bootstraps the Gmail account from `.env` into `email_accounts`, reads active accounts with `autoreview_enabled=true`, and keeps `EMAIL_REVIEW_DRY_RUN=true` by default.

## Quality checks

    make check

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 check

This runs:

- `gofmt`
- `go test ./...`
- `go vet ./...`

For substantial changes, keep tests and documentation in step with the code:

- update core tests when command behavior changes
- add focused tests for LLM cleanup and safety-sensitive logic
- update this README when setup, commands, architecture or roadmap changes
- update `AGENTS.md` when workflow, safety assumptions or architecture direction changes

## Server update checklist

On the Ubuntu server:

    cd /opt/ai/projects/robe
    git pull --ff-only
    make check
    make run

In another terminal, verify:

    curl http://localhost:8080/health

Expected smoke tests:

- `/ping` replies `pong`
- `/help` lists the available commands
- `/status` replies that Robe is online and shows env, LLM and access mode
- `/status` shows whether memory and embeddings are enabled
- `/remember the dentist prefers mornings` saves a memory when Postgres is configured
- `/memories dentist` lists matching memories
- `/askmem dentist | what should I know before booking?` answers using bounded retrieved memory IDs
- `recuerda que para garden quiero hablar en kilos, no en cajas` saves a project-scoped memory when `garden` is configured as a project or alias
- voice message `crea una cita mañana a las 12 con el dentista` returns a heard transcript and a proposal token
- `/calendar today` lists upcoming events with event IDs
- `/calendar create Test | 2026-06-07 10:00 | 2026-06-07 10:15` returns a proposal and token, not a created event
- `/calendar delete <event_id>` returns a proposal and token, not a deleted event
- `crea una cita mañana a las 12 con el dentista` returns a proposal and token, not a created event
- `/confirm <token>` executes the proposed create/delete
- `/email review dry-run` lists proposed labels/contact metadata without mutating Gmail
- `/ask responde solo OK` returns a final answer without thinking text
- an unauthorized Telegram account is ignored if `TELEGRAM_ALLOWED_USER_ID` is set

## Safety model

Robe should not allow the LLM to directly execute sensitive actions.

The intended policy is:

- read operations may be executed directly when authorized
- write operations require explicit confirmation
- destructive actions are disabled until specifically implemented
- email deletion, email sending, calendar modification and external posting require confirmation gates
- current email commands are read-only; label mutation is reserved for the Core-owned review workflow
- Core classifies side effects through a permission engine
- memory writes and calendar write proposals/executions are written to the audit log when Postgres is configured
- memory context injected into LLM prompts is deterministically redacted for common PII and secrets
- Core exposes `RedactExternalContentForPrompt` as the redaction contract for future email, web and RAG content before LLM prompt injection
- all future tool executions should use the same permission and audit model
- external content should pass through PII redaction before memory creation, prompt injection, RAG indexing or task generation when practical
- private project aliases, personal labels and secrets belong in `.env`, server config, secrets or database records, not in committed code

Current Governance policy:

- `internal/core` owns permission decisions and audit event shape
- memory creation is low risk and allowed only after explicit user intent validation
- memory archive/tag operations are medium risk local curation actions
- calendar create/delete are high risk external writes and require confirmation tokens
- Postgres stores audit events in `audit_events`
- audit metadata is intentionally compact and must not store raw secrets

Current Calendar policy:

- calendar reads execute directly for the authorized Telegram user
- calendar create requires `/confirm <token>`
- calendar delete requires `/confirm <token>`
- natural-language calendar create/delete also require `/confirm <token>`
- voice calendar create/delete also require `/confirm <token>`
- ambiguous confirmations such as "yes" are ignored

Current Email policy:

- Gmail access uses the modify OAuth scope so Robe can apply controlled review labels
- `/email search <query>` lists matching message IDs and snippets
- `/email show <message_id>` displays a safe view of a single message with sender/recipient aliases and redacted content
- `/email show raw <message_id>` is the explicit escape hatch for the raw message body and raw participants
- natural-language email search/show routes through the same read-only Core interface
- sender identity is shown as a safe alias; full sender names and email addresses are Core-private
- `ContactDirectory` persists raw contact identity locally in Postgres while exposing only safe aliases to Robe/LLM contexts
- contact private fields are encrypted at rest when `CONTACT_ENCRYPTION_KEY` is configured; previous keys can be supplied for rotation
- Postgres includes `email_accounts` as the durable foundation for multi-account scheduler configuration
- LLM-proposed contact relationship/category updates are validated by Core before persistence
- `EmailReviewService` can run unread/unreviewed review in dry-run mode and audit proposed labels before label execution
- the email scheduler is opt-in, reads `email_accounts`, and defaults to dry-run review
- Gmail messages include a web link so Telegram can open the message in an already-authenticated browser or mobile Gmail session
- no email sending, deletion, archiving or unsubscribe execution is implemented
- label mutation is limited to controlled `Robe/...` labels for the future review workflow and must remain Core-owned
- future email summarization must pass email content and sender identity through Core sanitization/redaction before LLM use

Current Memory policy:

- memories are saved only through explicit `/remember <text>` or explicit natural-language memory requests
- natural-language memory creation requires explicit intent such as `recuerda que`, `ten en cuenta que`, `from now on` or `de ahora en adelante`
- the LLM may request `create_memory`, but Robe Core validates and executes the write
- the LLM never writes directly to Postgres
- memory search is explicit through `/memories <query>`
- memory-assisted answers are explicit through `/askmem <memory query> | <question>`
- normal `/ask` and natural answers may receive compact relevant memory context before the LLM call
- embeddings are generated only for explicit memory records and explicit retrieval/context injection
- memories can be global or project-scoped
- supported memory kinds: `preference`, `fact`, `decision`, `constraint`, `task_context`, `contact_context`, `operational_note`
- project scopes are user-defined through `/project create` and optional private `MEMORY_PROJECT_ALIASES`
- supported metadata includes kind, project, tags, source, confidence, importance, status and timestamps
- no silent autonomous memory writes yet
- project context and RAG should build on top of this storage layer, not inside Telegram

Structured memory examples:

    /project create garden | Garden
    /project use garden
    /remember --kind decision --tags architecture,postgres Use Postgres as the source of truth.
    /remember --project garden --kind preference --tags orders,units Orders should be discussed in kilos.
    recuerda que para garden quiero hablar en kilos, no en cajas
    /memories --project garden --kind preference kilos
    /memories --tag dentist appointment
    /askmem postgres | what storage should Robe use?
    /forget 12
    /memory show 12
    /memory tag 12 architecture
    /memory archive 12

## Development roadmap

### v0.1

Local Telegram assistant using Ollama.

### v0.1.1

Thin Telegram adapter, core assistant command handling, safer `/status`, core tests and LLM thinking cleanup.

### v0.2

Google Calendar read support.

### v0.3

Calendar event creation and deletion with explicit confirmation tokens.

### v0.4

Gmail search/show, controlled review labels and thread summarization.

### v0.5

Web search adapter.

### v0.6

Voice input through local STT, TTS and mobile bridge.

### v0.7

Structured, project-aware local memory with Postgres, LLM-proposed memory actions validated by Robe Core, and optional Ollama embeddings for explicit context injection.

### v0.8

Governance foundation: central permission engine and PostgreSQL audit trail for memory writes and calendar write proposals/execution.

### Later

Ray-Ban / glasses bridge as an input-output adapter, not as the core of the assistant.
