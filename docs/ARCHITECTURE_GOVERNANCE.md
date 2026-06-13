# Robe Architecture and Governance

Robe is not a chatbot. Robe is a local-first personal agent platform where the Core orchestrates state, permissions, tools and execution. The LLM is a replaceable reasoning component.

## Principles

### Core First

Core owns orchestration.

The LLM never:

- accesses databases directly
- executes actions directly
- accesses secrets directly
- decides permissions
- persists state by itself

The LLM proposes. Core validates. Core executes.

### PostgreSQL As Source Of Truth

Durable state belongs in PostgreSQL or a Core-owned persistence adapter backed by PostgreSQL.

Examples:

- memories
- projects
- future tasks
- future email classifications
- audit logs
- future document indexes metadata
- future fitness and nutrition history

The LLM is never persistent storage.

### Transport Isolation

Telegram, HTTP, mobile, voice and future device bridges are transports. They should authenticate, normalize input and deliver output, but they must not contain business logic.

### Privacy By Default

Do not hardcode user projects, personal domains, private labels, contacts, addresses or aliases in Go code or committed docs.

Private runtime configuration belongs in:

- `.env`
- server-side secrets
- database records created by user action

Examples:

- project aliases: `MEMORY_PROJECT_ALIASES`
- OAuth tokens: `secrets/`
- local credentials: never committed

## Governance Model

## Compliance Checklist For New Features

Before merging a feature that touches user data, external content or side effects, verify:

- No secrets, tokens, private project names, personal aliases or private identifiers are hardcoded.
- Durable state is stored through a Core-owned persistence interface.
- The LLM does not directly execute tools or write to storage.
- Side effects are classified by risk.
- Sensitive or external side effects require confirmation.
- External content is redacted before prompt injection or indexing when practical.
- Tests cover allow, deny and confirmation paths for safety-sensitive behavior.
- README, AGENTS and roadmap/governance docs are updated when architecture or policy changes.
- The feature works when optional AI capabilities fail, or the failure mode is documented and user-visible.

### Permission Engine

A centralized permission engine exists in Core and must remain the single place where side-effecting actions are classified.

Every action should be classified:

- `low`: read-only or local non-sensitive operation
- `medium`: reversible local mutation or curation action
- `high`: external side effect, destructive action, financial action, email sending, deletion or external posting

Core decides whether execution is:

- allowed directly
- allowed with confirmation
- denied

Current MVP policy:

- memory creation: `low` risk, allowed only after explicit memory intent validation
- memory archive/tag: `medium` risk, allowed as local reversible curation
- calendar create/delete: `high` risk, requires confirmation
- unknown action types: denied

New side-effecting features must add action types, permission tests and audit tests before being treated as stable.

### Auditability

Sensitive actions should generate audit records before being treated as stable. Current memory writes and calendar write proposal/confirm/cancel flows emit audit events when the Postgres store is configured.

Minimum audit fields:

- timestamp
- actor: `user`, `llm`, `system`, `automation`
- transport
- action
- normalized parameters
- decision: `allowed`, `confirmed`, `denied`, `failed`
- result

Audit logs must not store raw secrets.

Audit events are governance records, not memory. They should contain compact summaries and metadata only.

## PII Protection

All external content should pass through a redaction layer before:

- memory creation
- prompt injection
- RAG indexing
- task generation
- email-derived actions

Examples of sensitive content:

- government identifiers
- passport numbers
- tax identifiers
- bank account numbers
- card numbers
- personal addresses
- phone numbers
- email addresses
- tokens
- signed URLs
- unsubscribe links
- OAuth codes
- API keys

The redaction layer should be deterministic and Core-owned. The LLM may help classify content, but Core owns the redaction decision.

Current implementation starts with deterministic redaction of memory context before it is injected into LLM prompts. This protects common emails, phone numbers, card numbers, bank account numbers, signed URLs, OAuth codes and token-like secrets while preserving the original stored memory for user review and curation.

## Memory Governance

Memory exists for the agent. It is not primarily a note-taking system.

Memory rules:

- memory writes require explicit user intent
- the LLM may propose `create_memory`
- Core validates and persists
- Core may reject malformed or sensitive proposals (including text exceeding 1000 characters to prevent context bloat)
- no silent autonomous memory writes
- project inference uses private runtime aliases, never hardcoded project names

Memory and RAG are separate:

- memory stores concise facts, preferences, constraints and decisions
- RAG stores long-form documents and retrieved passages

## Future Domain Systems

Future systems should follow the same pattern:

Input -> Core validation -> optional LLM proposal -> permission decision -> Core execution -> audit record

Planned domains:

- email
- tasks
- coaching
- RAG/documents
- fitness
- nutrition
- web search

Each domain should expose Core interfaces first, then adapters.

## LLM Trait Packs

Reusable LLM behavior traits live in `docs/llm_traits`.

Traits are design artifacts for future prompt assembly. They do not grant permissions.

The LLM/Core action contract is documented in `docs/INTENT_PROTOCOL.md`.

Rules:

- Core decides which traits are included in context.
- Traits must not contain secrets or private project aliases.
- Traits must not bypass confirmation, permission or audit rules.
- Traits should be small, composable and testable through behavior-level tests.
