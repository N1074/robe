package core

import (
	"context"
	"strings"
)

type LLM interface {
	Ask(ctx context.Context, prompt string) (string, error)
}

type Assistant struct {
	llm    LLM
	status string
}

func NewAssistant(llm LLM, status string) *Assistant {
	return &Assistant{
		llm:    llm,
		status: strings.TrimSpace(status),
	}
}

func (a *Assistant) HandleText(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)

	switch {
	case text == "/ping":
		return "pong", nil

	case text == "/start":
		return "Robe v0.1 online. Try /ping or /ask <question>.", nil

	case text == "/help":
		return "Commands:\n/ping\n/status\n/ask <question>", nil

	case text == "/status":
		if a.status != "" {
			return a.status, nil
		}
		return "Robe v0.1 online.", nil

	case text == "/ask" || strings.HasPrefix(text, "/ask "):
		return a.handleAsk(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/ask")))

	default:
		return "Unknown command. Try /help.", nil
	}
}

func (a *Assistant) handleAsk(ctx context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "Usage: /ask <question>", nil
	}

	if a.llm == nil {
		return "LLM is not configured.", nil
	}

	return a.llm.Ask(ctx, prompt)
}
