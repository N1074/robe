# Robe

Robe is a local-first personal assistant service written in Go.

The project is designed as a small orchestration layer for private assistant workflows: Telegram input, local LLM inference through Ollama, and future adapters for calendar, email, search, voice and visual context.

Current status: **v0.6 development**.

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
- command handling through `internal/core`
- `/ping`, `/start`, `/help`, `/status`
- `/ask <question>` using a local Ollama model
- Google Calendar read/create/delete behind confirmation gates
- natural-language intent routing through the local LLM
- Telegram voice/audio input through configurable local STT
- manual local memory backed by Postgres
- `.env` based configuration
- basic project quality commands through `make`
- config, core command and LLM response cleanup tests

Planned:

- Gmail read-only search and summarization
- web search adapter
- audit log
- TTS
- mobile / glasses bridge
- memory, project context and RAG
- project context and RAG

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
- audit logging

Tool adapters:

- local LLM via Ollama
- Google Calendar
- Gmail read-only
- web search
- local storage
- Postgres memory store

## Requirements

- Go 1.25.8+
- Ollama running locally
- Telegram bot token
- Optional local STT command for voice/audio input
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
    LLM_NUM_PREDICT=1024
    LLM_TEMPERATURE=0.2

    CALENDAR_PROVIDER=google
    CALENDAR_ID=primary
    CALENDAR_CREDENTIALS_FILE=/opt/ai/projects/robe/secrets/google-calendar-credentials.json
    CALENDAR_TOKEN_FILE=/opt/ai/projects/robe/secrets/google-calendar-token.json
    CALENDAR_TIMEZONE=Europe/Madrid

    STT_PROVIDER=command
    STT_COMMAND=/opt/ai/bin/transcribe-audio
    STT_ARGS={audio}
    STT_TIMEOUT_SECONDS=120

    MEMORY_PROVIDER=postgres
    DATABASE_URL=postgres://robe:robe_dev_password@localhost:5432/robe?sslmode=disable

`.env` must not be committed.

## Local database

Robe uses Postgres for local memory when `MEMORY_PROVIDER=postgres`.

Start the database:

    make db-up

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 db-up

Useful database commands:

    make db-logs
    make db-psql
    make db-down

## Google Calendar setup

Calendar integration uses Google OAuth credentials and a local token file.

Generate the token on the server after configuring `CALENDAR_CREDENTIALS_FILE` and `CALENDAR_TOKEN_FILE`:

    make google-auth

On Windows PowerShell:

    powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1 google-auth

Open the printed URL, approve Calendar access, paste the authorization code, then keep the generated token file under `secrets/` or another non-committed path.

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
- `/remember <text>`
- `/memories <query>`
- `/project list`
- `/project create <slug> | <name>`
- `/project use <slug>`
- `/project status`
- `/calendar today`
- `/calendar tomorrow`
- `/calendar week`
- `/calendar create <title> | <start> | <end> [| location] [| description]`
- `/calendar delete <event_id>`
- `/pending`
- `/confirm <token>`
- `/cancel <token>`

Natural language also works for supported calendar intents. For example:

    crea una cita de calendario para mañana a las 12 con el dentista
    que tengo mañana en el calendario

Calendar create/delete requests made in natural language still return a proposal and require `/confirm <token>`.

Voice messages and audio files sent to Telegram are transcribed first, then processed like normal text. The STT command should print the transcript to stdout. Use `{audio}` in `STT_ARGS` where the downloaded audio path should be inserted; if omitted, Robe appends the audio path as the final argument. Robe filters common `whisper.cpp` log lines defensively, but a transcript-only wrapper is still preferred.

For `whisper.cpp`, make the wrapper print only the transcript on stdout. Keep logs on stderr and use `-np -nt`:

    /opt/ai/src/whisper.cpp/build/bin/whisper-cli -m /opt/ai/models/ggml-small.bin -f "$TMP_WAV" -l es -np -nt

If Robe replies that Calendar is not configured yet, calendar OAuth/config is not active. Set `CALENDAR_PROVIDER=google`, credentials/token paths and restart Robe.

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
- `/remember the dentist prefers mornings` saves a memory when Postgres is configured
- `/memories dentist` lists matching memories
- voice message `crea una cita mañana a las 12 con el dentista` returns a heard transcript and a proposal token
- `/calendar today` lists upcoming events with event IDs
- `/calendar create Test | 2026-06-07 10:00 | 2026-06-07 10:15` returns a proposal and token, not a created event
- `/calendar delete <event_id>` returns a proposal and token, not a deleted event
- `crea una cita mañana a las 12 con el dentista` returns a proposal and token, not a created event
- `/confirm <token>` executes the proposed create/delete
- `/ask responde solo OK` returns a final answer without thinking text
- an unauthorized Telegram account is ignored if `TELEGRAM_ALLOWED_USER_ID` is set

## Safety model

Robe should not allow the LLM to directly execute sensitive actions.

The intended policy is:

- read operations may be executed directly when authorized
- write operations require explicit confirmation
- destructive actions are disabled until specifically implemented
- email deletion, email sending, calendar modification and external posting require confirmation gates
- all future tool executions should be auditable

Current Calendar policy:

- calendar reads execute directly for the authorized Telegram user
- calendar create requires `/confirm <token>`
- calendar delete requires `/confirm <token>`
- natural-language calendar create/delete also require `/confirm <token>`
- voice calendar create/delete also require `/confirm <token>`
- ambiguous confirmations such as "yes" are ignored

Current Memory policy:

- memories are saved only through explicit `/remember <text>`
- memory search is explicit through `/memories <query>`
- memories can be global or project-scoped
- supported memory kinds: `note`, `preference`, `fact`, `task`, `decision`, `project_knowledge`, `contact`, `operational`
- supported metadata includes kind, project, tags, source, confidence, importance, status and timestamps
- no automatic memory writes yet
- project context and RAG should build on top of this storage layer, not inside Telegram

Structured memory examples:

    /project create robe | Robe
    /project use robe
    /remember --kind decision --tags architecture,postgres Use Postgres as Robe's source of truth.
    /remember --project home --kind preference --tags calendar,dentist Dentist appointments should be in the morning.
    /memories --project robe --kind decision postgres
    /memories --tag dentist appointment

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

Gmail read-only search and thread summarization.

### v0.5

Web search adapter.

### v0.6

Voice input through local STT, TTS and mobile bridge.

### v0.7

Structured, project-aware local memory with Postgres, followed by retrieval-augmented context behind explicit storage/tool boundaries.

### Later

Ray-Ban / glasses bridge as an input-output adapter, not as the core of the assistant.
