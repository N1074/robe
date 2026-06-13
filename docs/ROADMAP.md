# Robe Roadmap

This roadmap prioritizes professional foundations before broader agent autonomy.

## Phase 0: Baseline Assistant

Status: implemented.

- Go service
- `/health`
- Telegram adapter
- private Telegram access
- local Ollama LLM
- `/ask`, `/ping`, `/status`, `/help`
- Windows and Ubuntu development workflows

## Phase 1: Core Orchestration

Status: implemented.

- `internal/core`
- transport-agnostic command handling
- thin Telegram adapter
- LLM thinking cleanup
- focused core tests

## Phase 2: Calendar With Confirmation

Status: implemented.

- Google Calendar read
- calendar create proposal
- calendar delete proposal
- explicit `/confirm <token>`
- no ambiguous confirmation
- natural-language calendar intents
- voice-transcribed calendar intents

## Phase 3: Structured Memory Before RAG

Status: in progress.

- PostgreSQL memory store
- project-aware memory
- memory curation commands
- LLM-proposed `create_memory`
- Core validation before persistence
- optional Ollama embeddings
- compact memory context injection
- project aliases in private runtime config
- minimal audit trail for memory writes

Exit criteria:

- no personal project hardcodes in code or docs
- backfill command for existing memory embeddings
- integration test for Postgres migrations
- memory update workflow

## Phase 4: Governance Foundation

Status: in progress.

- central permission engine
- action risk classification
- audit log schema
- audit persistence in PostgreSQL
- audit events for memory writes
- audit events for calendar write proposals, confirmations and cancellations
- initial deterministic PII redaction before memory prompt injection
- Core redaction contract for external content before LLM prompt injection
- broader PII redaction for future external content
- policy tests for sensitive content

Exit criteria:

- every current side-effecting action is classified
- every current sensitive action creates an audit record
- redaction is applied before prompt injection for external content

## Phase 5: Email Review

Status: in progress.

- Gmail adapter with search/show and controlled label support
- Gmail search/show commands
- natural-language email search/show intents
- unread/unreviewed search and label primitives
- controlled Robe email review label taxonomy
- Core-private raw sender identity with Robe-facing safe aliases
- safe `/email show` with explicit raw escape hatch
- Postgres contact directory for raw identity and validated relationship metadata
- encrypted contact private fields when `CONTACT_ENCRYPTION_KEY` is configured
- contact encryption rotation through previous-key fallback
- Postgres `email_accounts` foundation for future multi-account review scheduling
- LLM-proposed contact profile updates validated by Core
- email review dry-run with audit records
- email classification
- initial review summaries
- task/memory proposals only
- unsubscribe proposal without execution
- no sending or deletion

Technical debt:

- implement the email scheduler only after manual dry-run behavior is reviewed
- scheduler should read `email_accounts`, remain dry-run by default and emit per-account audit/health records

Exit criteria:

- email content redacted before LLM use where practical
- sender email addresses and full surnames hidden from LLM context by default
- no email send/delete/archive/unsubscribe without explicit confirmation or later policy
- controlled label mutation is reversible, Core-owned and audited
- audit records for email-derived proposals
- scheduler remains unimplemented until manual dry-run review has been inspected

## Phase 6: RAG And Documents

Status: planned.

- document ingestion
- document metadata in PostgreSQL
- chunking/indexing pipeline
- retrieval through Core
- context assembly combining memory and documents

Rules:

- RAG does not replace memory
- LLM never queries raw storage directly
- project scoping is required before retrieval

## Phase 7: Task System

Status: planned.

- task model in PostgreSQL
- task extraction proposals
- due dates and reminders
- confirmation for external actions
- project-aware task retrieval
- task lifecycle: inbox, active, scheduled, waiting, blocked, done, cancelled
- task fields: project, title, description, tags, priority, source, source reference, due date, scheduled time, recurrence, risk level, estimated effort, confirmation requirement
- source pipelines from user messages, email, calendar, future fitness/nutrition systems and automation

Rules:

- memory stores context
- tasks store actions
- calendar stores time
- the LLM may propose task creation
- Core validates, persists and schedules
- ambiguous tasks should be proposed, not silently created

Exit criteria:

- Core task interface
- PostgreSQL task schema
- task creation and curation commands
- LLM-proposed task creation with validation
- permission classification for task side effects
- audit records for task creation and status changes

## Phase 8: Coaching System

Status: planned.

- coaching layer consumes memory, tasks, calendar, projects and future domain history
- coach proposes next actions, reframing, scheduling and review prompts
- Core validates and executes any resulting action
- user decides on side effects
- coaching traits are maintained as versioned Markdown context under `docs/llm_traits`

Rules:

- coach never owns data
- coach never invents facts
- coach may challenge behavior, not identity
- coach may propose task decomposition, scheduling and review
- coach must not manipulate, threaten, shame, coerce or create dependency

Exit criteria:

- coaching trait pack
- coaching prompt assembly through Core
- tests for allowed and forbidden coaching behaviors
- integration with task review without direct tool execution

## Phase 9: Fitness And Nutrition

Status: planned.

- domain schemas in PostgreSQL
- text/voice/image input convergence
- confidence-based clarification
- Core-owned progression and aggregation rules
- LLM explains recommendations but does not own calculations

## Phase 10: Broader Interfaces

Status: planned.

- HTTP/mobile adapter
- TTS
- device/glasses bridge
- notification routing

All new interfaces remain transports. Core remains the product.
