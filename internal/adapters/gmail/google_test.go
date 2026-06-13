package gmail

import (
	"encoding/base64"
	"testing"

	"github.com/N1074/robe/internal/core"
	gmailapi "google.golang.org/api/gmail/v1"
)

var _ core.EmailReviewStore = (*GoogleGmail)(nil)

func TestConvertMessage(t *testing.T) {
	body := base64.RawURLEncoding.EncodeToString([]byte("hello from gmail"))
	got := convertMessage(&gmailapi.Message{
		Id:       "msg_1",
		ThreadId: "thread_1",
		Snippet:  "snippet",
		LabelIds: []string{"UNREAD", "Label_1"},
		Payload: &gmailapi.MessagePart{
			Headers: []*gmailapi.MessagePartHeader{
				{Name: "From", Value: "sender@example.com"},
				{Name: "To", Value: `"Raul User" <me@example.com>`},
				{Name: "Cc", Value: `"John Smith" <john@example.com>`},
				{Name: "Subject", Value: "Hello"},
				{Name: "Date", Value: "Sat, 13 Jun 2026 10:00:00 +0200"},
			},
			Parts: []*gmailapi.MessagePart{
				{
					MimeType: "text/plain",
					Body:     &gmailapi.MessagePartBody{Data: body},
				},
			},
		},
	}, "me")

	if got.ID != "msg_1" || got.ThreadID != "thread_1" || got.From != "sender@example.com" || got.Subject != "Hello" {
		t.Fatalf("unexpected message: %#v", got)
	}
	if len(got.LabelIDs) != 2 || got.LabelIDs[0] != "UNREAD" {
		t.Fatalf("unexpected labels: %#v", got.LabelIDs)
	}
	if got.PlainText != "hello from gmail" {
		t.Fatalf("unexpected body: %q", got.PlainText)
	}
	if len(got.ToIdentities) != 1 || got.ToIdentities[0].Alias != "Raul U." {
		t.Fatalf("unexpected to identities: %#v", got.ToIdentities)
	}
	if len(got.CcIdentities) != 1 || got.CcIdentities[0].Alias != "John S." {
		t.Fatalf("unexpected cc identities: %#v", got.CcIdentities)
	}
	if got.Date.IsZero() {
		t.Fatalf("expected parsed date")
	}
	if got.WebURL != "https://mail.google.com/mail/u/0/#inbox/msg_1" {
		t.Fatalf("unexpected web url: %q", got.WebURL)
	}
}

func TestNormalizeGoogleConfigDefaultsUserID(t *testing.T) {
	got := normalizeGoogleConfig(GoogleConfig{})
	if got.UserID != "me" {
		t.Fatalf("expected default user id me, got %q", got.UserID)
	}
}

func TestBuildUnreadUnreviewedQuery(t *testing.T) {
	got := buildUnreadUnreviewedQuery("Robe/Reviewed")
	if got != `is:unread -label:"Robe/Reviewed"` {
		t.Fatalf("unexpected query: %q", got)
	}

	got = buildUnreadUnreviewedQuery("")
	if got != "is:unread" {
		t.Fatalf("unexpected fallback query: %q", got)
	}
}

func TestCleanLabelIDs(t *testing.T) {
	got := cleanLabelIDs([]string{" Label_1 ", "", "Label_1", "Label_2"})
	want := []string{"Label_1", "Label_2"}
	if len(got) != len(want) {
		t.Fatalf("expected %d labels, got %#v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("label %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestGmailMessageURLUsesConfiguredAccount(t *testing.T) {
	got := gmailMessageURL("person@example.com", "msg_1")
	want := "https://mail.google.com/mail/u/person@example.com/#inbox/msg_1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
