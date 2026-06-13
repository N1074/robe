package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReviewUnreadEmailsDryRunAuditsWithoutApplyingLabels(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				From:         `"Agencia Tributaria" <notice@example.gob>`,
				FromIdentity: ParseEmailIdentity(`"Agencia Tributaria" <notice@example.gob>`),
				Subject:      "Administracion: requerimiento",
				Snippet:      "Tienes una notificacion importante.",
				WebURL:       "https://mail.google.com/mail/u/0/#inbox/msg_1",
			},
		},
	}
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithAuditLogger(audit))

	results, err := assistant.ReviewUnreadEmails(context.Background(), EmailReviewOptions{DryRun: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 || !results[0].Important || !containsString(results[0].Labels, EmailLabelAdmin) || !containsString(results[0].Labels, EmailLabelReviewed) {
		t.Fatalf("unexpected review results: %#v", results)
	}
	if len(email.appliedLabels) != 0 {
		t.Fatalf("dry run applied labels: %#v", email.appliedLabels)
	}
	if len(audit.events) != 1 || audit.events[0].ActionType != ActionEmailLabel || audit.events[0].Result != AuditResultProposed {
		t.Fatalf("expected proposed audit event, got %#v", audit.events)
	}
}

func TestReviewUnreadEmailsAppliesControlledLabels(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{{ID: "msg_1", Subject: "Invoice for order"}},
	}
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithAuditLogger(audit))

	results, err := assistant.ReviewUnreadEmails(context.Background(), EmailReviewOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 || !containsString(results[0].Labels, EmailLabelOnlinePurchases) {
		t.Fatalf("unexpected results: %#v", results)
	}
	if len(email.appliedLabels["msg_1"]) == 0 {
		t.Fatalf("expected labels applied, got %#v", email.appliedLabels)
	}
	if len(audit.events) != 1 || audit.events[0].Result != AuditResultExecuted {
		t.Fatalf("expected executed audit event, got %#v", audit.events)
	}
}

func TestHandleTextEmailReviewDryRun(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{{ID: "msg_1", Subject: "Invoice for order", WebURL: "https://mail.google.com/mail/u/0/#inbox/msg_1"}},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithAuditLogger(&mockAuditLogger{}))

	got, err := assistant.HandleText(context.Background(), "/email review dry-run")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Email review dry-run:") || !strings.Contains(got, EmailLabelOnlinePurchases) || !strings.Contains(got, "msg_1") {
		t.Fatalf("unexpected dry-run response: %q", got)
	}
	if len(email.appliedLabels) != 0 {
		t.Fatalf("dry-run applied labels: %#v", email.appliedLabels)
	}
}

func TestReviewUnreadEmailsDryRunAuditsClassifierContactProposal(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				From:         `"Agencia Tributaria" <notice@example.gob>`,
				FromIdentity: ParseEmailIdentity(`"Agencia Tributaria" <notice@example.gob>`),
				Subject:      "Official notice",
			},
		},
	}
	classifier := &mockEmailClassifier{
		classification: EmailClassification{
			Labels:    []string{EmailLabelReviewed, EmailLabelAdmin, "Bad/Label"},
			Important: true,
			Summary:   "Official notice.",
			ContactProposal: ContactProfileProposal{
				Kind:         ContactKindOrganization,
				Relationship: ContactRelationshipAdmin,
				Importance:   4,
				Confidence:   0.8,
				Reason:       "official notice",
			},
		},
	}
	directory := &mockContactDirectory{}
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithEmailClassifier(classifier), WithContactDirectory(directory), WithAuditLogger(audit))

	results, err := assistant.ReviewUnreadEmails(context.Background(), EmailReviewOptions{DryRun: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 || !containsString(results[0].Labels, EmailLabelAdmin) || containsString(results[0].Labels, "Bad/Label") {
		t.Fatalf("unexpected classifier labels: %#v", results)
	}
	if !strings.Contains(classifier.lastPrompt, "From: Agencia T.") || strings.Contains(classifier.lastPrompt, "notice@example.gob") {
		t.Fatalf("classifier prompt was not sanitized: %q", classifier.lastPrompt)
	}
	if len(directory.contacts) != 0 || directory.proposal.Relationship != "" {
		t.Fatalf("dry-run persisted contact proposal, got contacts=%#v proposal=%#v", directory.contacts, directory.proposal)
	}
	if len(audit.events) < 2 {
		t.Fatalf("expected email and contact audit events, got %#v", audit.events)
	}
	if audit.events[1].ActionType != ActionContactProfile || audit.events[1].Result != AuditResultProposed {
		t.Fatalf("expected proposed contact audit event, got %#v", audit.events)
	}
}

func TestReviewUnreadEmailsApplyPersistsValidatedContactProposal(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				From:         `"Agencia Tributaria" <notice@example.gob>`,
				FromIdentity: ParseEmailIdentity(`"Agencia Tributaria" <notice@example.gob>`),
				Subject:      "Official notice",
			},
		},
	}
	classifier := &mockEmailClassifier{
		classification: EmailClassification{
			Labels: []string{EmailLabelReviewed, EmailLabelAdmin},
			ContactProposal: ContactProfileProposal{
				Kind:         ContactKindOrganization,
				Relationship: ContactRelationshipAdmin,
				Importance:   4,
				Confidence:   0.8,
				Reason:       "official notice",
			},
		},
	}
	directory := &mockContactDirectory{}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithEmailClassifier(classifier), WithContactDirectory(directory), WithAuditLogger(&mockAuditLogger{}))

	if _, err := assistant.ReviewUnreadEmails(context.Background(), EmailReviewOptions{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(directory.contacts) == 0 || directory.proposal.Relationship != ContactRelationshipAdmin {
		t.Fatalf("expected contact proposal persisted, got contacts=%#v proposal=%#v", directory.contacts, directory.proposal)
	}
}

func TestReviewUnreadEmailsReportsContactProposalError(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{
				ID:           "msg_1",
				FromIdentity: ParseEmailIdentity(`"Agencia Tributaria" <notice@example.gob>`),
				Subject:      "Official notice",
			},
		},
	}
	classifier := &mockEmailClassifier{
		classification: EmailClassification{
			Labels: []string{EmailLabelReviewed, EmailLabelAdmin},
			ContactProposal: ContactProfileProposal{
				Kind:         ContactKindOrganization,
				Relationship: ContactRelationshipAdmin,
				Importance:   4,
				Confidence:   0.8,
				Reason:       "official notice",
			},
		},
	}
	directory := &mockContactDirectory{applyErr: errors.New("contact store unavailable")}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithEmailClassifier(classifier), WithContactDirectory(directory), WithAuditLogger(&mockAuditLogger{}))

	results, err := assistant.ReviewUnreadEmails(context.Background(), EmailReviewOptions{})
	if err != nil {
		t.Fatalf("expected no review error, got %v", err)
	}
	if len(results) != 1 || !strings.Contains(results[0].ContactError, "contact store unavailable") {
		t.Fatalf("expected contact error in result, got %#v", results)
	}
}
