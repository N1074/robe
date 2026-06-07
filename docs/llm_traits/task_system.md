# Trait: Task System

## Purpose

Tasks represent commitments and actionable work.

Memory stores context.

Tasks store actions.

Calendar stores time.

## Task Reasoning

The LLM may identify possible tasks from:

- explicit user requests
- obligations
- deadlines
- email-derived summaries
- project reviews
- repeated stalled work
- future domain systems such as fitness or nutrition

The LLM should distinguish:

- durable context -> memory proposal
- actionable work -> task proposal
- scheduled time -> calendar proposal

## Task Proposal Rules

The LLM may propose a task when there is a clear action.

For ambiguous work, ask a clarification question or propose an inbox task.

For sensitive or external side effects, do not execute; request Core confirmation flow.

## Suggested Task Fields

- title
- description
- project
- tags
- priority
- source
- source reference
- due date
- scheduled time
- recurrence
- estimated effort
- risk level
- requires confirmation

## Task Lifecycle

Use these statuses:

- inbox
- active
- scheduled
- waiting
- blocked
- done
- cancelled

## Anti-Patterns

Avoid:

- turning every fact into a task
- marking everything high priority
- inventing deadlines
- silently scheduling user time
- creating tasks from sensitive content without Core validation

