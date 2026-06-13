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
	want += "\nCalendar: disabled\nEmail: disabled\nVoice: disabled\nMemory: disabled\nEmbeddings: disabled\nProject: global\nTimezone: Local"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
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

	want := "Robe v0.1 online.\nEnv: dev\nLLM: ollama/dolphin-mistral:latest\nAccess: setup-open\nCalendar: disabled\nEmail: disabled\nVoice: disabled\nMemory: disabled\nEmbeddings: disabled\nProject: global\nTimezone: Local"
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

	want := "Commands:\n/ping\n/status\n/ask <question>\n/askmem <memory query> | <question>\n/remember <text>\n/memories <query>\n/forget <memory_id>\n/memory show|archive|tag\n/project list|create|use|status\n/calendar today|tomorrow|week\n/calendar create <title> | <start> | <end> [| location] [| description]\n/calendar delete <event_id>\n/email search <query>\n/email show <message_id>\n/email show raw <message_id>\n/email review dry-run\n/pending\n/confirm <token>\n/cancel <token>"
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

func TestNewAssistantUsesLLMEmailClassifier(t *testing.T) {
	llm := &mockLLMWithEmailClassifier{}
	assistant := NewAssistant(llm, Status{})

	if assistant.emailClassifier == nil {
		t.Fatalf("expected email classifier from llm")
	}
}
