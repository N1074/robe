package core

import (
	"strings"
	"testing"
)

func TestRedactForPromptRedactsCommonPII(t *testing.T) {
	got := redactForPrompt("email me at user@example.com or +34 612 345 678, token=abc123secret, card 4111 1111 1111 1111")

	for _, leaked := range []string{"user@example.com", "+34 612 345 678", "abc123secret", "4111 1111 1111 1111"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	for _, marker := range []string{"[REDACTED_EMAIL]", "[REDACTED_PHONE]", "[REDACTED_SECRET]", "[REDACTED_CARD]"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("expected marker %q in %q", marker, got)
		}
	}
}

func TestRedactForPromptKeepsNonCardNumbers(t *testing.T) {
	got := redactForPrompt("Use model qwen3:14b with LLM_NUM_PREDICT=1024, event 1234567890123, and date 2026-06-13 10:00.")

	if strings.Contains(got, "[REDACTED_CARD]") || strings.Contains(got, "[REDACTED_PHONE]") {
		t.Fatalf("unexpected redaction: %q", got)
	}
}
