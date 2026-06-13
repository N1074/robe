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

func TestRedactForPromptRedactsProviderSecrets(t *testing.T) {
	jwt := "ey" + "JhbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjMifQ" + "." + "signature"
	awsKey := "AK" + "IAIOSFODNN7EXAMPLE"
	githubToken := "gh" + "p_abcdefghijklmnopqrstuvwxyzABCDEFGHIJ1234"
	slackToken := "xo" + "xb-123456789012-abcdefghijklmno"
	googleAPIKey := "AI" + "zaSyD-abcdefghijklmnopqrstuvwxyz1234567"

	input := strings.Join([]string{
		"Authorization: Bearer " + jwt,
		"access_token=secret-access-token",
		awsKey,
		githubToken,
		slackToken,
		googleAPIKey,
	}, "\n")

	got := redactForPrompt(input)

	for _, leaked := range []string{
		jwt,
		"secret-access-token",
		awsKey,
		githubToken,
		slackToken,
		googleAPIKey,
	} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	for _, marker := range []string{
		"[REDACTED_AUTH_HEADER]",
		"[REDACTED_SECRET]",
		"[REDACTED_AWS_ACCESS_KEY]",
		"[REDACTED_GITHUB_TOKEN]",
		"[REDACTED_SLACK_TOKEN]",
		"[REDACTED_GOOGLE_API_KEY]",
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("expected marker %q in %q", marker, got)
		}
	}
}

func TestRedactForPromptRedactsSensitiveURLsAndKeys(t *testing.T) {
	input := strings.Join([]string{
		"postgres://robe:secretpass@localhost:5432/robe",
		"https://example.com/download?X-Amz-Signature=abc123&expires=999",
		"https://newsletter.example/unsubscribe?u=user@example.com",
		"-----BEGIN PRIVATE KEY-----\nabc123\n-----END PRIVATE KEY-----",
	}, "\n")

	got := redactForPrompt(input)

	for _, leaked := range []string{"secretpass", "X-Amz-Signature", "unsubscribe", "BEGIN PRIVATE KEY", "user@example.com"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	for _, marker := range []string{"[REDACTED_CREDENTIALS]", "[REDACTED_SIGNED_URL]", "[REDACTED_UNSUBSCRIBE_URL]", "[REDACTED_PRIVATE_KEY]"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("expected marker %q in %q", marker, got)
		}
	}
}

func TestRedactForPromptRedactsGovernmentIDs(t *testing.T) {
	got := redactForPrompt("SSN 123-45-6789, DNI 12345678Z, NIE X1234567L.")

	for _, leaked := range []string{"123-45-6789", "12345678Z", "X1234567L"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	if strings.Count(got, "[REDACTED_GOVERNMENT_ID]") != 3 {
		t.Fatalf("expected three government id markers, got %q", got)
	}
}

func TestRedactForPromptKeepsNormalURLsAndTechnicalIDs(t *testing.T) {
	input := "Docs at https://example.com/path?query=ok, commit abcdef1234567890, model qwen3:14b, date 2026-06-13."
	got := redactForPrompt(input)

	if got != input {
		t.Fatalf("expected ordinary technical text unchanged, got %q", got)
	}
}
