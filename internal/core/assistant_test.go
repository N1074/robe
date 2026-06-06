package core

import (
	"context"
	"errors"
	"testing"
)

func TestHandleTextEmptyMessage(t *testing.T) {
	assistant := NewAssistant(nil, "")

	got, err := assistant.HandleText(context.Background(), "   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextUnknownCommand(t *testing.T) {
	assistant := NewAssistant(nil, "")

	got, err := assistant.HandleText(context.Background(), "/nope")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextPing(t *testing.T) {
	assistant := NewAssistant(nil, "")

	got, err := assistant.HandleText(context.Background(), "/ping")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "pong" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextStatus(t *testing.T) {
	assistant := NewAssistant(nil, "Robe test status.")

	got, err := assistant.HandleText(context.Background(), "/status")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Robe test status." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextHelp(t *testing.T) {
	assistant := NewAssistant(nil, "")

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
	assistant := NewAssistant(mockLLM{}, "")

	got, err := assistant.HandleText(context.Background(), "/ask   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Usage: /ask <question>" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskWithMockLLMSuccess(t *testing.T) {
	assistant := NewAssistant(mockLLM{answer: "local answer"}, "")

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
	assistant := NewAssistant(mockLLM{err: wantErr}, "")

	_, err := assistant.HandleText(context.Background(), "/ask hello")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

type mockLLM struct {
	answer string
	err    error
}

func (m mockLLM) Ask(ctx context.Context, prompt string) (string, error) {
	return m.answer, m.err
}
