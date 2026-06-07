# LLM Traits

This directory contains versioned trait packs that may later be injected into LLM context by Robe Core.

Traits are not runtime prompts yet. They are design artifacts for controlled prompt assembly.

Rules:

- traits must remain project-agnostic unless explicitly scoped to a non-private product domain
- no personal project names, private labels, secrets or user-specific aliases
- traits describe behavior boundaries, not storage or execution permissions
- Core remains responsible for permission checks, validation, persistence and tool execution
- traits should be concise enough to be composed into prompts without context pollution

## Available Traits

- `core_agent.md`: baseline Robe agent behavior
- `memory_system.md`: memory proposal and retrieval behavior
- `safety_and_permissions.md`: safety boundaries and confirmation behavior
- `task_system.md`: task reasoning and task proposal behavior
- `ethical_coach.md`: coaching behavior, allowed pressure and forbidden manipulation
