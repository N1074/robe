package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
)

func (s *PostgresMemoryStore) UpsertEmailContact(ctx context.Context, identity core.EmailIdentity) (core.Contact, error) {
	identity.RawEmail = strings.ToLower(strings.TrimSpace(identity.RawEmail))
	identity.RawName = strings.TrimSpace(identity.RawName)
	identity.Alias = strings.TrimSpace(identity.Alias)
	if identity.Alias == "" {
		identity.Alias = "Unknown sender"
	}

	if identity.RawEmail != "" {
		existing, err := s.contactByEmailHash(ctx, emailHash(identity.RawEmail))
		if err == nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return core.Contact{}, err
		}
	}

	now := time.Now()
	var id int64
	storedFullName, err := s.encryptContactPrivateValue(identity.RawName)
	if err != nil {
		return core.Contact{}, err
	}
	err = s.db.QueryRowContext(ctx, `
INSERT INTO contacts (alias, full_name, kind, relationship, project_slug, importance, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, $8)
RETURNING id
`, identity.Alias, storedFullName, core.ContactKindUnknown, core.ContactRelationshipUnknown, normalizeProjectSlugStorage(identity.ProjectSlug), defaultInt(0, 3), now, now).Scan(&id)
	if err != nil {
		return core.Contact{}, err
	}

	if identity.RawEmail != "" {
		storedEmail, err := s.encryptContactPrivateValue(identity.RawEmail)
		if err != nil {
			return core.Contact{}, err
		}
		storedDisplayName, err := s.encryptContactPrivateValue(identity.RawName)
		if err != nil {
			return core.Contact{}, err
		}
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO contact_addresses (contact_id, address_hash, email, display_name_seen, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (address_hash) DO UPDATE SET contact_id = EXCLUDED.contact_id, display_name_seen = EXCLUDED.display_name_seen, updated_at = EXCLUDED.updated_at
`, id, emailHash(identity.RawEmail), storedEmail, storedDisplayName, now, now); err != nil {
			return core.Contact{}, err
		}
	}

	return s.contactByID(ctx, id)
}

func (s *PostgresMemoryStore) ApplyContactProfileProposal(ctx context.Context, proposal core.ContactProfileProposal) (core.Contact, error) {
	if err := validateStorageContactProposal(proposal); err != nil {
		return core.Contact{}, err
	}

	now := time.Now()
	if strings.TrimSpace(proposal.ContactID) != "" {
		id := strings.TrimSpace(proposal.ContactID)
		_, err := s.db.ExecContext(ctx, `
UPDATE contacts
SET alias = COALESCE(NULLIF($2, ''), alias),
    kind = $3,
    relationship = $4,
    project_slug = $5,
    importance = $6,
    updated_at = $7
WHERE id = $1
`, id, strings.TrimSpace(proposal.Alias), core.NormalizeContactKindForStorage(proposal.Kind), core.NormalizeContactRelationshipForStorage(proposal.Relationship), normalizeProjectSlugStorage(proposal.ProjectSlug), defaultInt(proposal.Importance, 3), now)
		if err != nil {
			return core.Contact{}, err
		}
		var numericID int64
		if _, err := fmt.Sscanf(id, "%d", &numericID); err != nil {
			return core.Contact{}, err
		}
		return s.contactByID(ctx, numericID)
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO contacts (alias, kind, relationship, project_slug, importance, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
RETURNING id
`, strings.TrimSpace(proposal.Alias), core.NormalizeContactKindForStorage(proposal.Kind), core.NormalizeContactRelationshipForStorage(proposal.Relationship), normalizeProjectSlugStorage(proposal.ProjectSlug), defaultInt(proposal.Importance, 3), now, now).Scan(&id)
	if err != nil {
		return core.Contact{}, err
	}
	return s.contactByID(ctx, id)
}

func (s *PostgresMemoryStore) contactByEmailHash(ctx context.Context, hash string) (core.Contact, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT contact_id FROM contact_addresses WHERE address_hash = $1`, hash).Scan(&id)
	if err != nil {
		return core.Contact{}, err
	}
	return s.contactByID(ctx, id)
}

func (s *PostgresMemoryStore) contactByID(ctx context.Context, id int64) (core.Contact, error) {
	var contact core.Contact
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT c.id::text, c.alias, c.full_name, COALESCE(ca.email, ''), c.kind, c.relationship, c.project_slug, c.importance, c.status, c.created_at, c.updated_at
FROM contacts c
LEFT JOIN contact_addresses ca ON ca.contact_id = c.id
WHERE c.id = $1
ORDER BY ca.created_at ASC
LIMIT 1
`, id).Scan(&contact.ID, &contact.Alias, &contact.FullName, &contact.Email, &contact.Kind, &contact.Relationship, &contact.ProjectSlug, &contact.Importance, &contact.Status, &createdAt, &updatedAt)
	if err != nil {
		return core.Contact{}, err
	}
	contact.FullName = s.decryptContactPrivateValue(contact.FullName)
	contact.Email = s.decryptContactPrivateValue(contact.Email)
	contact.CreatedAt = createdAt
	contact.UpdatedAt = updatedAt
	return contact, nil
}

func validateStorageContactProposal(proposal core.ContactProfileProposal) error {
	if strings.TrimSpace(proposal.ContactID) == "" && strings.TrimSpace(proposal.Alias) == "" {
		return errors.New("contact id or alias is required")
	}
	return nil
}

func normalizeProjectSlugStorage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	if value == "global" {
		return ""
	}
	return value
}
