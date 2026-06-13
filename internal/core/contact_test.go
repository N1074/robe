package core

import (
	"context"
	"strings"
	"testing"
)

func TestApplyContactProfileProposalValidatesAndAudits(t *testing.T) {
	directory := &mockContactDirectory{}
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithContactDirectory(directory), WithAuditLogger(audit))

	contact, err := assistant.applyContactProfileProposal(context.Background(), ContactProfileProposal{
		ContactID:    "contact_1",
		Alias:        "Maria S. B.",
		Kind:         ContactKindPerson,
		Relationship: ContactRelationshipAdmin,
		Importance:   4,
		Confidence:   0.8,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if contact.Relationship != ContactRelationshipAdmin || directory.proposal.Alias != "Maria S. B." {
		t.Fatalf("unexpected contact/proposal: %#v %#v", contact, directory.proposal)
	}
	if len(audit.events) != 1 || audit.events[0].ActionType != ActionContactProfile || audit.events[0].Result != AuditResultExecuted {
		t.Fatalf("expected contact audit event, got %#v", audit.events)
	}
}

func TestApplyContactProfileProposalRejectsUnsupportedRelationship(t *testing.T) {
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithContactDirectory(&mockContactDirectory{}), WithAuditLogger(audit))

	_, err := assistant.applyContactProfileProposal(context.Background(), ContactProfileProposal{
		Alias:        "Maria S. B.",
		Kind:         ContactKindPerson,
		Relationship: "best_friend_forever",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported contact relationship") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Result != AuditResultRejected {
		t.Fatalf("expected rejected audit event, got %#v", audit.events)
	}
}
