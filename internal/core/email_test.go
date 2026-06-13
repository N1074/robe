package core

import (
	"context"
	"strings"
	"testing"
)

func TestHandleTextEmailRequiresEmailAdapter(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/email search invoice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != emailNotConfiguredMessage() {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextEmailSearch(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				From:         `"Maria Sanchez Barroso" <maria.sanchez@example.com>`,
				FromIdentity: ParseEmailIdentity(`"Maria Sanchez Barroso" <maria.sanchez@example.com>`),
				Subject:      "Invoice",
				Snippet:      "June invoice",
				WebURL:       "https://mail.google.com/mail/u/0/#inbox/msg_1",
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email))

	got, err := assistant.HandleText(context.Background(), "/email search invoice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Email:") || !strings.Contains(got, "[msg_1] Invoice") || !strings.Contains(got, "https://mail.google.com/mail/u/0/#inbox/msg_1") {
		t.Fatalf("unexpected response: %q", got)
	}
	if strings.Contains(got, "maria.sanchez@example.com") || strings.Contains(got, "Sanchez") || strings.Contains(got, "Barroso") {
		t.Fatalf("email response leaked raw sender identity: %q", got)
	}
	if !strings.Contains(got, "From: Maria S. B.") {
		t.Fatalf("email response did not include safe alias: %q", got)
	}
	if email.lastQuery.Query != "invoice" || email.lastQuery.Limit != 5 {
		t.Fatalf("unexpected query: %#v", email.lastQuery)
	}
}

func TestHandleTextEmailShowIsSafeByDefault(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				ThreadID:     "thread_1",
				From:         `"Maria Sanchez Barroso" <maria.sanchez@example.com>`,
				FromIdentity: ParseEmailIdentity(`"Maria Sanchez Barroso" <maria.sanchez@example.com>`),
				To:           `"Raul User" <me@example.com>`,
				ToIdentities: ParseEmailIdentities(`"Raul User" <me@example.com>`),
				Cc:           `"John Smith" <john@example.com>`,
				CcIdentities: ParseEmailIdentities(`"John Smith" <john@example.com>`),
				Subject:      "Hello",
				PlainText:    "Body with token=secret123 and maria.sanchez@example.com",
				WebURL:       "https://mail.google.com/mail/u/0/#inbox/msg_1",
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email))

	got, err := assistant.HandleText(context.Background(), "/email show msg_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "ID: msg_1") || !strings.Contains(got, "Link: https://mail.google.com/mail/u/0/#inbox/msg_1") {
		t.Fatalf("unexpected response: %q", got)
	}
	for _, leaked := range []string{"maria.sanchez@example.com", "secret123", "Sanchez", "Barroso", "me@example.com", "john@example.com", "Smith"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("email detail leaked raw identity/content %q: %q", leaked, got)
		}
	}
	if !strings.Contains(got, "To: Raul U.") || !strings.Contains(got, "Cc: John S.") || !strings.Contains(got, "[REDACTED_SECRET]") {
		t.Fatalf("email detail did not include safe content: %q", got)
	}
	if email.lastID != "msg_1" {
		t.Fatalf("unexpected message id: %q", email.lastID)
	}
}

func TestHandleTextEmailShowRaw(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:        "msg_1",
				From:      `"Maria Sanchez Barroso" <maria.sanchez@example.com>`,
				To:        "me@example.com",
				Subject:   "Hello",
				PlainText: "Raw Body",
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email))

	got, err := assistant.HandleText(context.Background(), "/email show raw msg_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "maria.sanchez@example.com") || !strings.Contains(got, "Raw Body") {
		t.Fatalf("expected raw response, got %q", got)
	}
}

func TestHandleTextEmailShowRawUsage(t *testing.T) {
	assistant := NewAssistant(nil, Status{}, WithEmail(&mockEmail{}))

	got, err := assistant.HandleText(context.Background(), "/email show raw")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Usage: /email show raw <message_id>" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestEmailInterfaceIsReadOnly(t *testing.T) {
	var _ Email = (*mockEmail)(nil)
}

func TestExternalContentForPromptRedactsThroughCoreContract(t *testing.T) {
	got := externalContentForPrompt("Email body", "Contact user@example.com with token=secret123.")

	if strings.Contains(got, "user@example.com") || strings.Contains(got, "secret123") {
		t.Fatalf("external prompt content leaked sensitive data: %q", got)
	}
	if !strings.Contains(got, "Email body:") || !strings.Contains(got, "[REDACTED_EMAIL]") || !strings.Contains(got, "[REDACTED_SECRET]") {
		t.Fatalf("unexpected redacted content: %q", got)
	}
}

func TestDefaultEmailReviewLabelsAreControlled(t *testing.T) {
	labels := DefaultEmailReviewLabels()
	if len(labels) != 10 {
		t.Fatalf("unexpected label count: %#v", labels)
	}
	for _, label := range labels {
		if !strings.HasPrefix(label, "Robe/") {
			t.Fatalf("expected Robe-owned label, got %q", label)
		}
	}
}
