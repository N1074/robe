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
	status Status
}

type Status struct {
	Env              string
	LLMProvider      string
	LLMModel         string
	AccessRestricted bool
}

func NewAssistant(llm LLM, status Status) *Assistant {
	return &Assistant{
		llm:    llm,
		status: status,
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
		return a.renderStatus(), nil

	case text == "/ask" || strings.HasPrefix(text, "/ask "):
		return a.handleAsk(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/ask")))

	default:
		return "Unknown command. Try /help.", nil
	}
}

func (a *Assistant) renderStatus() string {
	env := strings.TrimSpace(a.status.Env)
	if env == "" {
		env = "unknown"
	}

	provider := strings.TrimSpace(a.status.LLMProvider)
	if provider == "" {
		provider = "unknown"
	}

	model := strings.TrimSpace(a.status.LLMModel)
	if model == "" {
		model = "unknown"
	}

	access := "restricted"
	if !a.status.AccessRestricted {
		access = "setup-open"
	}

	return "Robe v0.1 online.\nEnv: " + env + "\nLLM: " + provider + "/" + model + "\nAccess: " + access
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
