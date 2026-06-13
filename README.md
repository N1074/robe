# Robe

Robe is a local-first personal assistant service written in Go. It is meant to be a small, explicit and auditable orchestration layer for private assistant workflows, not an opaque autonomous agent.

Current status: **active development, governance/email phase**.

## What Works Today

- HTTP server with `/health`
- Telegram bot adapter with optional private access through `TELEGRAM_ALLOWED_USER_ID`
- Core-owned command handling in `internal/core`
- Local LLM integration through Ollama
- `/ask <question>`
- Google Calendar read/create/delete, with confirmation gates for writes
- Natural-language calendar intents through the local LLM
- Telegram voice/audio input through a configurable local STT command
- Structured memory in Postgres
- Project-aware memories and private project aliases
- Optional Ollama embeddings for memory-assisted context
- Central Core permission engine
- Postgres audit events for memory writes and calendar write lifecycle
- Gmail search/show
- Gmail unread/unreviewed dry-run review through `/email review dry-run`
- Controlled Gmail review labels under `Robe/...`
- Safe email display by default, with explicit raw view
- Core-owned email identity sanitization before LLM use
- Contact directory foundation in Postgres
- Optional encryption for contact private fields
- Email account schema foundation for future multi-account scheduling

Not implemented yet:

- email scheduler
- Telegram email notifications
- email sending/deletion/archive/unsubscribe
- web search
- RAG/document ingestion
- task system
- TTS/mobile/glasses bridge

## Governance

Primary references:

- [Architecture and Governance](docs/ARCHITECTURE_GOVERNANCE.md)
- [Roadmap](docs/ROADMAP.md)
- [Intent Protocol](docs/INTENT_PROTOCOL.md)
- [LLM Traits](docs/llm_traits/README.md)

Short version:

- Core owns orchestration, validation, permissions, persistence and execution.
- The LLM proposes actions and transformations.
- Adapters perform I/O.
- PostgreSQL is the source of truth for durable state.
- Sensitive external writes require explicit confirmation.
- Telegram is a transport, not a business-logic layer.

## Runtime Flow

Text:

```text
Telegram -> Telegram adapter -> Core assistant -> tools/LLM/storage -> Core response -> Telegram
```

Voice:

```text
Telegram audio -> Telegram adapter -> STT adapter -> Core assistant -> tools/LLM/storage -> Telegram
```

Email review:

```text
Gmail adapter -> Core redaction/identity sanitization -> optional LLM classification -> Core label/contact proposal -> audit
```

## Requirements

- Go 1.25.8+
- Ollama
- A local model, currently tested with `qwen3:14b`
- Telegram bot token for Telegram runtime
- Optional Postgres for memory/audit/contact features
- Optional Ollama embedding model, recommended `nomic-embed-text`
- Optional Google OAuth credentials for Calendar and Gmail
- Optional local STT command for Telegram voice/audio

The current server deployment commonly uses:

```text
LLM_BASE_URL=http://172.17.0.1:11434
LLM_MODEL=qwen3:14b
LLM_NUM_PREDICT=1024
```

`LLM_NUM_PREDICT=1024` is recommended for `qwen3:14b` because low values can be consumed by internal thinking and produce empty final content.

## Configuration

Create a local `.env` from the example:

```bash
cp .env.example .env
chmod 600 .env
```

Important settings:

```env
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
CALENDAR_CREDENTIALS_FILE=secrets/google-calendar-credentials.json
CALENDAR_TOKEN_FILE=secrets/google-calendar-token.json
CALENDAR_TIMEZONE=Europe/Madrid

EMAIL_PROVIDER=gmail
GMAIL_CREDENTIALS_FILE=secrets/google-gmail-credentials.json
GMAIL_TOKEN_FILE=secrets/google-gmail-token.json
GMAIL_USER_ID=me

MEMORY_PROVIDER=postgres
DATABASE_URL=postgres://robe:robe_dev_password@localhost:5432/robe?sslmode=disable
MEMORY_PROJECT_ALIASES=
CONTACT_ENCRYPTION_KEY=
CONTACT_ENCRYPTION_PREVIOUS_KEYS=

EMBEDDING_PROVIDER=ollama
EMBEDDING_BASE_URL=http://172.17.0.1:11434
EMBEDDING_MODEL=nomic-embed-text

STT_PROVIDER=command
STT_COMMAND=
STT_ARGS={audio}
STT_TIMEOUT_SECONDS=120
```

Never commit `.env`, OAuth tokens, bot tokens, local secrets or database files.

## Running

Ubuntu/server workflow:

```bash
make run
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 run
```

Health check:

```bash
curl http://localhost:8080/health
```

## Database

Postgres is used for:

- structured memory
- projects
- audit events
- contact directory
- future email account configuration

Start local Postgres:

```bash
make db-up
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 db-up
```

Useful commands:

```bash
make db-logs
make db-psql
make db-down
```

Contact private fields are encrypted at rest when `CONTACT_ENCRYPTION_KEY` is configured. This covers fields such as `contacts.full_name`, `contact_addresses.email` and `contact_addresses.display_name_seen`. `CONTACT_ENCRYPTION_PREVIOUS_KEYS` lets Robe decrypt old values during rotation and re-encrypt them with the current key.

## Google Calendar

Calendar uses Google OAuth credentials and a local token file.

Generate or refresh the token:

```bash
make google-auth
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 google-auth
```

Calendar writes are never executed directly from the LLM. Create/delete requests produce a pending confirmation token and require `/confirm <token>`.

## Gmail

Gmail currently supports:

- `/email search <query>`
- `/email show <message_id>`
- `/email show raw <message_id>`
- `/email review dry-run`
- natural-language email search/show when the LLM can parse the intent

Gmail does not support sending, deleting, archiving or unsubscribe execution.

Generate or refresh the Gmail token:

```bash
GOOGLE_AUTH_TARGET=gmail make google-auth
```

Windows PowerShell:

```powershell
$env:GOOGLE_AUTH_TARGET="gmail"; powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 google-auth
```

Robe uses Gmail modify scope because controlled review labels may be applied by Core-owned review flows. Existing read-only tokens must be regenerated.

Controlled labels:

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

Email identity handling:

- `/email show` is safe by default and uses sender/recipient aliases.
- `/email show raw <message_id>` is the explicit raw escape hatch.
- LLM email classification uses `EmailMessageForPrompt`, not raw email addresses.
- Full names and addresses are Core/storage-private.
- The scheduler is intentionally deferred. Use `/email review dry-run` manually until behavior has been reviewed.

## Telegram Commands

- `/start`
- `/help`
- `/ping`
- `/status`
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

Natural language currently covers:

- calendar list/create/delete intents
- email search/show intents
- explicit memory creation intents
- normal assistant questions

Examples:

```text
crea una cita manana a las 12 con el dentista
que tengo manana en el calendario
busca correos sobre facturas
ensename el correo msg_123
recuerda que para garden quiero hablar en kilos, no en cajas
```

## Memory

Memory is explicit and structured. The LLM may propose `create_memory`, but Core validates and persists it. There are no silent autonomous memory writes.

Examples:

```text
/project create garden | Garden
/project use garden
/remember --kind decision --tags architecture,postgres Use Postgres as the source of truth.
/remember --project garden --kind preference --tags orders,units Orders should be discussed in kilos.
/memories --project garden --kind preference kilos
/askmem postgres | what storage should Robe use?
/memory show 12
/memory tag 12 architecture
/memory archive 12
```

Embeddings are optional retrieval metadata for explicit memories. They are not RAG and do not create memory by themselves.

## Voice

Telegram voice/audio messages are transcribed through a configured local command and then handled like text.

The STT command should print only the transcript on stdout. Logs belong on stderr. Use `{audio}` in `STT_ARGS` where the downloaded audio path should be inserted.

Example wrapper target for `whisper.cpp`:

```bash
/opt/ai/src/whisper.cpp/build/bin/whisper-cli -m /opt/ai/models/ggml-small.bin -f "$TMP_WAV" -l es -np -nt
```

## Safety Model

- Read operations may execute directly for the authorized user.
- Calendar create/delete require confirmation.
- Email send/delete/archive/unsubscribe are not implemented.
- Email label mutation is limited to controlled `Robe/...` labels through Core review flows.
- Memory writes require explicit user intent and Core validation.
- New side-effecting tools must use the Core permission engine and audit model.
- External content should pass through Core redaction before LLM use when practical.
- Audit metadata must stay compact and must not store secrets.

## Quality Checks

```bash
make check
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 check
```

This runs:

- `gofmt`
- `go test ./...`
- `go vet ./...`

## Server Update Checklist

```bash
cd /opt/ai/projects/robe
git pull --ff-only
make check
make run
```

Smoke tests:

- `curl http://localhost:8080/health`
- Telegram `/ping`
- Telegram `/help`
- Telegram `/status`
- Telegram `/calendar today`
- Telegram `/calendar create Test | 2026-06-07 10:00 | 2026-06-07 10:15`
- Telegram `/pending`, `/confirm <token>`, `/cancel <token>`
- Telegram `/remember test memory`
- Telegram `/memories test`
- Telegram `/ask responde solo OK`
- Telegram `/email search newer_than:7d` when Gmail is configured
- Telegram `/email review dry-run` when Gmail is configured

## Roadmap

The detailed roadmap lives in [docs/ROADMAP.md](docs/ROADMAP.md).

Near-term focus:

- keep Phase 3/4 memory, governance and audit foundations tight
- complete Phase 5 email review manually before any scheduler
- split/refactor Postgres storage by domain for auditability
- add stronger Postgres migration/integration coverage
- keep future scheduler, tasks, RAG and notifications behind explicit design gates

