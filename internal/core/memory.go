package core

import (
	"context"
	"strings"
	"time"
)

type MemoryStore interface {
	AddMemory(ctx context.Context, memory Memory) (Memory, error)
	SearchMemories(ctx context.Context, query string, limit int) ([]Memory, error)
}

type Memory struct {
	ID        string
	Text      string
	Source    string
	CreatedAt time.Time
}

func (a *Assistant) handleRemember(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "Usage: /remember <text>", nil
	}

	memory, err := a.memory.AddMemory(ctx, Memory{
		Text:      text,
		Source:    "telegram",
		CreatedAt: a.now(),
	})
	if err != nil {
		return "", err
	}

	return "Memory saved:\n" + formatMemory(memory), nil
}

func (a *Assistant) handleMemories(ctx context.Context, query string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "Usage: /memories <query>", nil
	}

	memories, err := a.memory.SearchMemories(ctx, query, 5)
	if err != nil {
		return "", err
	}

	if len(memories) == 0 {
		return "No memories found.", nil
	}

	var b strings.Builder
	b.WriteString("Memories:\n")
	for _, memory := range memories {
		b.WriteString("- ")
		b.WriteString(formatMemory(memory))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func formatMemory(memory Memory) string {
	var b strings.Builder
	if strings.TrimSpace(memory.ID) != "" {
		b.WriteString("[")
		b.WriteString(memory.ID)
		b.WriteString("] ")
	}
	b.WriteString(strings.TrimSpace(memory.Text))
	if !memory.CreatedAt.IsZero() {
		b.WriteString(" (")
		b.WriteString(formatTime(memory.CreatedAt))
		b.WriteString(")")
	}
	return b.String()
}

func memoryNotConfiguredMessage() string {
	return "Memory is not configured yet. Set MEMORY_PROVIDER=postgres and DATABASE_URL, then restart Robe."
}
