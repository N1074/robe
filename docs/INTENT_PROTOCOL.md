# Intent Protocol

The intent protocol is the contract between the LLM reasoning layer and Robe Core.

The LLM emits structured proposals. Core validates and executes.

## Current Actions

- `ask`
- `none`
- `calendar_list`
- `calendar_create`
- `calendar_delete`
- `create_memory`
- `email_search`
- `email_show`

## General Rules

- Return compact JSON only.
- Do not execute actions.
- Do not claim persistence or external side effects.
- Use Core-provided project hints only.
- Use `global` when project scope is unclear.
- Never invent IDs for destructive actions.
- Never include secrets in proposed actions.

## Calendar Actions

Calendar reads may execute directly for an authorized user.

Calendar writes are proposals and require Core confirmation.

Calendar delete requires an explicit event ID.

## Memory Actions

`create_memory` is allowed only when the user explicitly asks Robe to remember durable context.

Core validates:

- explicit user intent
- project scope
- memory kind
- sensitivity
- persistence availability

If validation fails, Core reports that the memory was not saved.

## Email Actions

Email natural-language actions are read-only in the current protocol.

`email_search` may execute directly for an authorized user. The LLM should fill `query` with a concise Gmail-compatible search query or plain search terms.

`email_show` may execute only when the user provides an explicit message ID. The LLM must not invent message IDs; if no explicit message ID is present, use `email_search` instead.

Email natural-language actions must not send, delete, archive, label or unsubscribe. Future automatic label application belongs to a Core-owned review workflow, not to natural-language intent execution.

Raw sender email addresses and full display names are Core-private. When email content is later summarized or classified by the LLM, Core should provide sanitized sender aliases and redacted content rather than raw identities.

Contact relationship/category proposals may be derived from sanitized aliases and redacted email context. Core must validate the proposal before writing to `ContactDirectory`.

## Future Actions

Future actions should follow the same shape:

- action name
- normalized fields
- confidence where useful
- source reference where useful
- no direct execution

Before adding a new action:

- add Core validation
- add permission classification
- add tests for valid, invalid and ambiguous inputs
- update `docs/ARCHITECTURE_GOVERNANCE.md`
- update `docs/ROADMAP.md`
- update relevant LLM traits
