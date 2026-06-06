package core

import (
	"context"
	"errors"
	"testing"
)

func TestHandleTextEmptyMessage(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextUnknownCommand(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/nope")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextPing(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/ping")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "pong" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextStatus(t *testing.T) {
	assistant := NewAssistant(nil, Status{
		Env:              "test",
		LLMProvider:      "ollama",
		LLMModel:         "qwen3:14b",
		AccessRestricted: true,
	})

	got, err := assistant.HandleText(context.Background(), "/status")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Robe v0.1 online.\nEnv: test\nLLM: ollama/qwen3:14b\nAccess: restricted"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextHelp(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Commands:\n/ping\n/status\n/ask <question>"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskEmptyPrompt(t *testing.T) {
	assistant := NewAssistant(mockLLM{}, Status{})

	got, err := assistant.HandleText(context.Background(), "/ask   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Usage: /ask <question>" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskWithMockLLMSuccess(t *testing.T) {
	assistant := NewAssistant(mockLLM{answer: "local answer"}, Status{})

	got, err := assistant.HandleText(context.Background(), "/ask hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "local answer" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskWithMockLLMError(t *testing.T) {
	wantErr := errors.New("llm unavailable")
	assistant := NewAssistant(mockLLM{err: wantErr}, Status{})

	_, err := assistant.HandleText(context.Background(), "/ask hello")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestHandleTextStatusShowsSetupOpenAccess(t *testing.T) {
	assistant := NewAssistant(nil, Status{
		Env:         "dev",
		LLMProvider: "ollama",
		LLMModel:    "dolphin-mistral:latest",
	})

	got, err := assistant.HandleText(context.Background(), "/status")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Robe v0.1 online.\nEnv: dev\nLLM: ollama/dolphin-mistral:latest\nAccess: setup-open"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

type mockLLM struct {
	answer string
	err    error
}

func (m mockLLM) Ask(ctx context.Context, prompt string) (string, error) {
	return m.answer, m.err
}
