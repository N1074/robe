package core

import (
	"strings"
	"testing"
)

func TestParseEmailIdentityAliasesFullName(t *testing.T) {
	got := ParseEmailIdentity(`"María Sánchez Barroso" <maria.sanchez@example.com>`)

	if got.RawName != "María Sánchez Barroso" || got.RawEmail != "maria.sanchez@example.com" {
		t.Fatalf("expected raw identity retained for Core, got %#v", got)
	}
	if got.Alias != "Maria S. B." {
		t.Fatalf("expected alias Maria S. B., got %q", got.Alias)
	}
	if strings.Contains(got.Alias, "Sanchez") || strings.Contains(got.Alias, "Barroso") || strings.Contains(got.Alias, "@") {
		t.Fatalf("alias leaked full identity: %#v", got)
	}
}

func TestParseEmailIdentityAliasesOrganization(t *testing.T) {
	got := ParseEmailIdentity(`Administración Agencia Tributaria <notificaciones@example.gob>`)

	if got.Alias != "Administracion A. T." {
		t.Fatalf("unexpected organization alias: %q", got.Alias)
	}
	if got.Kind != "organization" {
		t.Fatalf("expected organization kind, got %q", got.Kind)
	}
}

func TestParseEmailIdentityWithoutDisplayNameDoesNotExposeAddress(t *testing.T) {
	got := ParseEmailIdentity(`sender@example.com`)

	if got.Alias != "Unknown sender" {
		t.Fatalf("expected unknown sender alias, got %q", got.Alias)
	}
	if got.RawEmail != "sender@example.com" {
		t.Fatalf("expected raw email retained for Core, got %#v", got)
	}
}

func TestSanitizedEmailSenderForPromptDoesNotExposeRawEmail(t *testing.T) {
	message := EmailMessage{
		From:         `"María Sánchez Barroso" <maria.sanchez@example.com>`,
		FromIdentity: ParseEmailIdentity(`"María Sánchez Barroso" <maria.sanchez@example.com>`),
	}

	got := SanitizedEmailSenderForPrompt(message)

	for _, leaked := range []string{"maria.sanchez@example.com", "Sanchez", "Barroso"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("sanitized sender leaked %q in %q", leaked, got)
		}
	}
	if !strings.Contains(got, "Maria S. B.") {
		t.Fatalf("expected alias in sanitized sender, got %q", got)
	}
}

func TestEmailMessageForPromptRedactsSenderAndContent(t *testing.T) {
	message := EmailMessage{
		From:         `"María Sánchez Barroso" <maria.sanchez@example.com>`,
		FromIdentity: ParseEmailIdentity(`"María Sánchez Barroso" <maria.sanchez@example.com>`),
		Subject:      "Write to maria.sanchez@example.com",
		PlainText:    "Token=secret123 from María Sánchez Barroso.",
	}

	got := EmailMessageForPrompt(message)

	for _, leaked := range []string{"maria.sanchez@example.com", "secret123", "Sanchez", "Barroso"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("prompt email leaked %q in %q", leaked, got)
		}
	}
	if !strings.Contains(got, "From: Maria S. B.") || !strings.Contains(got, "[REDACTED_EMAIL]") || !strings.Contains(got, "[REDACTED_SECRET]") {
		t.Fatalf("unexpected prompt email: %q", got)
	}
}
