package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ContactKindPerson       = "person"
	ContactKindOrganization = "organization"
	ContactKindService      = "service"
	ContactKindUnknown      = "unknown"

	ContactRelationshipAdmin          = "admin"
	ContactRelationshipSupplier       = "supplier"
	ContactRelationshipClient         = "client"
	ContactRelationshipPersonal       = "personal"
	ContactRelationshipProject        = "project"
	ContactRelationshipNewsletter     = "newsletter"
	ContactRelationshipOnlinePurchase = "online_purchase"
	ContactRelationshipUnknown        = "unknown"
)

type ContactDirectory interface {
	UpsertEmailContact(ctx context.Context, identity EmailIdentity) (Contact, error)
	ApplyContactProfileProposal(ctx context.Context, proposal ContactProfileProposal) (Contact, error)
}

type Contact struct {
	ID           string
	Alias        string
	FullName     string
	Email        string
	Kind         string
	Relationship string
	ProjectSlug  string
	Importance   int
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ContactProfileProposal struct {
	ContactID    string
	Alias        string
	Kind         string
	Relationship string
	ProjectSlug  string
	Importance   int
	Confidence   float64
	Reason       string
}

func (a *Assistant) ApplyContactProfileProposal(ctx context.Context, proposal ContactProfileProposal) (Contact, error) {
	if a.contactDirectory == nil {
		return Contact{}, errors.New("contact directory is not configured")
	}
	if err := validateContactProfileProposal(proposal); err != nil {
		action := contactProfileAction(proposal.ContactID, proposal)
		a.recordAudit(ctx, action, PermissionDecision{RiskLevel: RiskMedium, Decision: DecisionDeny, Reason: err.Error()}, AuditResultRejected, err)
		return Contact{}, err
	}

	action := contactProfileAction(proposal.ContactID, proposal)
	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		err := fmt.Errorf("contact profile denied: %s", decision.Reason)
		a.recordAudit(ctx, action, decision, AuditResultRejected, err)
		return Contact{}, err
	}

	contact, err := a.contactDirectory.ApplyContactProfileProposal(ctx, proposal)
	action.ResourceID = contact.ID
	a.recordAudit(ctx, action, decision, AuditResultExecuted, err)
	return contact, err
}

func (a *Assistant) applyContactProfileProposal(ctx context.Context, proposal ContactProfileProposal) (Contact, error) {
	return a.ApplyContactProfileProposal(ctx, proposal)
}

func validateContactProfileProposal(proposal ContactProfileProposal) error {
	if strings.TrimSpace(proposal.ContactID) == "" && strings.TrimSpace(proposal.Alias) == "" {
		return errors.New("contact id or alias is required")
	}
	if !isAllowedContactKind(normalizeContactKind(proposal.Kind)) {
		return fmt.Errorf("unsupported contact kind: %s", proposal.Kind)
	}
	if !isAllowedContactRelationship(normalizeContactRelationship(proposal.Relationship)) {
		return fmt.Errorf("unsupported contact relationship: %s", proposal.Relationship)
	}
	if proposal.Confidence < 0 || proposal.Confidence > 1 {
		return errors.New("contact proposal confidence must be between 0 and 1")
	}
	if proposal.Importance < 0 || proposal.Importance > 5 {
		return errors.New("contact proposal importance must be between 0 and 5")
	}
	return nil
}

func normalizeContactKind(value string) string {
	switch strings.TrimSpace(value) {
	case "", "person_or_org":
		return ContactKindUnknown
	case ContactKindPerson, ContactKindOrganization, ContactKindService, ContactKindUnknown:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(value)
	}
}

func NormalizeContactKindForStorage(value string) string {
	return normalizeContactKind(value)
}

func normalizeContactRelationship(value string) string {
	switch strings.TrimSpace(value) {
	case "":
		return ContactRelationshipUnknown
	default:
		return strings.TrimSpace(value)
	}
}

func NormalizeContactRelationshipForStorage(value string) string {
	return normalizeContactRelationship(value)
}

func isAllowedContactKind(kind string) bool {
	switch kind {
	case ContactKindPerson, ContactKindOrganization, ContactKindService, ContactKindUnknown:
		return true
	default:
		return false
	}
}

func isAllowedContactRelationship(relationship string) bool {
	switch relationship {
	case ContactRelationshipAdmin, ContactRelationshipSupplier, ContactRelationshipClient, ContactRelationshipPersonal, ContactRelationshipProject, ContactRelationshipNewsletter, ContactRelationshipOnlinePurchase, ContactRelationshipUnknown:
		return true
	default:
		return false
	}
}
