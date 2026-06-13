package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresMemoryStore struct {
	db                   *sql.DB
	contactEncryptionKey []byte
}

type Options struct {
	ContactEncryptionKey string
}

func NewPostgresMemoryStore(ctx context.Context, databaseURL string) (*PostgresMemoryStore, error) {
	return NewPostgresMemoryStoreWithOptions(ctx, databaseURL, Options{})
}

func NewPostgresMemoryStoreWithOptions(ctx context.Context, databaseURL string, opts Options) (*PostgresMemoryStore, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &PostgresMemoryStore{
		db:                   db,
		contactEncryptionKey: deriveContactEncryptionKey(opts.ContactEncryptionKey),
	}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *PostgresMemoryStore) Close() error {
	return s.db.Close()
}

func (s *PostgresMemoryStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS projects (
	id BIGSERIAL PRIMARY KEY,
	slug TEXT UNIQUE NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memories (
	id BIGSERIAL PRIMARY KEY,
	project_id BIGINT NULL REFERENCES projects(id),
	kind TEXT NOT NULL DEFAULT 'note',
	text TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT 'manual',
	confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
	importance INTEGER NOT NULL DEFAULT 3,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	expires_at TIMESTAMPTZ NULL
);

ALTER TABLE memories ADD COLUMN IF NOT EXISTS project_id BIGINT NULL REFERENCES projects(id);
ALTER TABLE memories ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'note';
ALTER TABLE memories ADD COLUMN IF NOT EXISTS summary TEXT NOT NULL DEFAULT '';
ALTER TABLE memories ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0;
ALTER TABLE memories ADD COLUMN IF NOT EXISTS importance INTEGER NOT NULL DEFAULT 3;
ALTER TABLE memories ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE memories ADD COLUMN IF NOT EXISTS embedding JSONB NULL;
ALTER TABLE memories ADD COLUMN IF NOT EXISTS embedding_model TEXT NOT NULL DEFAULT '';
ALTER TABLE memories ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE memories ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS memory_tags (
	memory_id BIGINT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
	tag TEXT NOT NULL,
	PRIMARY KEY(memory_id, tag)
);

CREATE TABLE IF NOT EXISTS audit_events (
	id BIGSERIAL PRIMARY KEY,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL,
	source TEXT NOT NULL,
	action_type TEXT NOT NULL,
	risk_level TEXT NOT NULL,
	decision TEXT NOT NULL,
	resource_type TEXT NOT NULL DEFAULT '',
	resource_id TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	result TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS memories_created_at_idx ON memories (created_at DESC);
CREATE INDEX IF NOT EXISTS memories_text_lower_idx ON memories (lower(text));
CREATE INDEX IF NOT EXISTS memories_project_idx ON memories (project_id);
CREATE INDEX IF NOT EXISTS memories_kind_idx ON memories (kind);
CREATE INDEX IF NOT EXISTS memories_status_idx ON memories (status);
CREATE INDEX IF NOT EXISTS memory_tags_tag_idx ON memory_tags (tag);
CREATE INDEX IF NOT EXISTS audit_events_occurred_at_idx ON audit_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_action_type_idx ON audit_events (action_type);
CREATE INDEX IF NOT EXISTS audit_events_resource_idx ON audit_events (resource_type, resource_id);

CREATE TABLE IF NOT EXISTS contacts (
	id BIGSERIAL PRIMARY KEY,
	alias TEXT NOT NULL,
	full_name TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL DEFAULT 'unknown',
	relationship TEXT NOT NULL DEFAULT 'unknown',
	project_slug TEXT NOT NULL DEFAULT '',
	importance INTEGER NOT NULL DEFAULT 3,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contact_addresses (
	contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
	address_hash TEXT PRIMARY KEY,
	email TEXT NOT NULL DEFAULT '',
	display_name_seen TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS email_accounts (
	id BIGSERIAL PRIMARY KEY,
	provider TEXT NOT NULL,
	account_key TEXT UNIQUE NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	user_id TEXT NOT NULL DEFAULT '',
	credentials_file TEXT NOT NULL DEFAULT '',
	token_file TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	autoreview_enabled BOOLEAN NOT NULL DEFAULT false,
	notify_telegram_enabled BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS contacts_alias_idx ON contacts (alias);
CREATE INDEX IF NOT EXISTS contacts_relationship_idx ON contacts (relationship);
CREATE INDEX IF NOT EXISTS contact_addresses_contact_idx ON contact_addresses (contact_id);
CREATE INDEX IF NOT EXISTS email_accounts_provider_idx ON email_accounts (provider);
CREATE INDEX IF NOT EXISTS email_accounts_status_idx ON email_accounts (status);
`)
	return err
}

func (s *PostgresMemoryStore) RecordAuditEvent(ctx context.Context, event core.AuditEvent) error {
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO audit_events (occurred_at, actor, source, action_type, risk_level, decision, resource_type, resource_id, summary, metadata, result, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)
`, occurredAt, nonEmpty(event.Actor, core.ActorSystem), nonEmpty(event.Source, "core"), strings.TrimSpace(event.ActionType), nonEmpty(event.RiskLevel, core.RiskHigh), nonEmpty(event.Decision, core.DecisionDeny), strings.TrimSpace(event.ResourceType), strings.TrimSpace(event.ResourceID), strings.TrimSpace(event.Summary), string(metadataJSON), strings.TrimSpace(event.Result), strings.TrimSpace(event.Error))
	return err
}

func (s *PostgresMemoryStore) AddMemory(ctx context.Context, memory core.Memory) (core.Memory, error) {
	text := strings.TrimSpace(memory.Text)
	if text == "" {
		return core.Memory{}, errors.New("memory text is required")
	}

	source := strings.TrimSpace(memory.Source)
	if source == "" {
		source = "manual"
	}

	createdAt := memory.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	var expiresAt any
	if !memory.ExpiresAt.IsZero() {
		expiresAt = memory.ExpiresAt
	}

	projectID, err := s.projectIDBySlug(ctx, memory.Project.Slug)
	if err != nil {
		return core.Memory{}, err
	}

	var id int64
	embedding, err := encodeEmbedding(memory.Embedding)
	if err != nil {
		return core.Memory{}, err
	}

	err = s.db.QueryRowContext(ctx, `
INSERT INTO memories (project_id, kind, text, source, confidence, importance, status, embedding, embedding_model, created_at, updated_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)
RETURNING id
`, projectID, nonEmpty(memory.Kind, "note"), text, source, defaultFloat(memory.Confidence, 1.0), defaultInt(memory.Importance, 3), nonEmpty(memory.Status, "active"), embedding, strings.TrimSpace(memory.EmbeddingModel), createdAt, nonZeroTime(memory.UpdatedAt, createdAt), expiresAt).Scan(&id)
	if err != nil {
		return core.Memory{}, err
	}

	for _, tag := range memory.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO memory_tags (memory_id, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, tag); err != nil {
			return core.Memory{}, err
		}
	}

	memory.ID = fmt.Sprintf("%d", id)
	memory.Text = text
	memory.Source = source
	memory.CreatedAt = createdAt
	return memory, nil
}

func (s *PostgresMemoryStore) SearchMemories(ctx context.Context, filter core.MemoryFilter) ([]core.Memory, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Query == "" && !filter.Semantic {
		return nil, nil
	}
	if filter.Limit <= 0 || filter.Limit > 20 {
		filter.Limit = 5
	}

	if filter.Semantic && len(filter.Embedding) > 0 {
		memories, err := s.searchMemoriesSemantic(ctx, filter)
		if err != nil || len(memories) > 0 || filter.Query == "" {
			return memories, err
		}
		filter.Semantic = false
		filter.Embedding = nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT m.id, COALESCE(p.id::text, ''), COALESCE(p.slug, ''), COALESCE(p.name, ''),
       m.kind, m.text, m.source, m.confidence, m.importance, m.status, m.embedding, m.embedding_model, m.created_at, m.updated_at, m.expires_at
FROM memories m
LEFT JOIN projects p ON p.id = m.project_id
WHERE m.text ILIKE '%' || $1 || '%'
  AND ($2 = '' OR p.slug = $2 OR ($7 AND p.id IS NULL))
  AND ($3 = '' OR m.kind = $3)
  AND ($4 = '' OR m.status = $4)
  AND ($5 = '' OR EXISTS (SELECT 1 FROM memory_tags mt WHERE mt.memory_id = m.id AND mt.tag = $5))
  AND ($8 = false OR p.id IS NULL)
ORDER BY m.importance DESC, m.updated_at DESC
LIMIT $6
`, filter.Query, filter.ProjectSlug, filter.Kind, filter.Status, filter.Tag, filter.Limit, filter.IncludeGlobal, filter.GlobalOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []core.Memory
	for rows.Next() {
		var id int64
		var memory core.Memory
		var expiresAt sql.NullTime
		var embedding []byte
		if err := rows.Scan(&id, &memory.Project.ID, &memory.Project.Slug, &memory.Project.Name, &memory.Kind, &memory.Text, &memory.Source, &memory.Confidence, &memory.Importance, &memory.Status, &embedding, &memory.EmbeddingModel, &memory.CreatedAt, &memory.UpdatedAt, &expiresAt); err != nil {
			return nil, err
		}
		memory.Embedding, err = decodeEmbedding(embedding)
		if err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			memory.ExpiresAt = expiresAt.Time
		}
		memory.ID = fmt.Sprintf("%d", id)
		memory.Tags, err = s.tagsForMemory(ctx, id)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return memories, nil
}

func (s *PostgresMemoryStore) searchMemoriesSemantic(ctx context.Context, filter core.MemoryFilter) ([]core.Memory, error) {
	memories, err := s.selectMemories(ctx, `
WHERE ($1 = '' OR p.slug = $1 OR ($5 AND p.id IS NULL))
  AND ($2 = '' OR m.kind = $2)
  AND ($3 = '' OR m.status = $3)
  AND ($4 = '' OR EXISTS (SELECT 1 FROM memory_tags mt WHERE mt.memory_id = m.id AND mt.tag = $4))
  AND m.embedding IS NOT NULL
  AND ($6 = false OR p.id IS NULL)
`, filter.ProjectSlug, filter.Kind, filter.Status, filter.Tag, filter.IncludeGlobal, filter.GlobalOnly)
	if err != nil {
		return nil, err
	}

	type scoredMemory struct {
		memory core.Memory
		score  float64
	}
	scored := make([]scoredMemory, 0, len(memories))
	for _, memory := range memories {
		score := cosineSimilarity(filter.Embedding, memory.Embedding)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredMemory{memory: memory, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].memory.Importance > scored[j].memory.Importance
		}
		return scored[i].score > scored[j].score
	})

	if len(scored) > filter.Limit {
		scored = scored[:filter.Limit]
	}

	out := make([]core.Memory, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.memory)
	}
	return out, nil
}

func (s *PostgresMemoryStore) GetMemory(ctx context.Context, id string) (core.Memory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return core.Memory{}, errors.New("memory id is required")
	}

	memories, err := s.selectMemories(ctx, `
WHERE m.id = $1
`, id)
	if err != nil {
		return core.Memory{}, err
	}
	if len(memories) == 0 {
		return core.Memory{}, sql.ErrNoRows
	}
	return memories[0], nil
}

func (s *PostgresMemoryStore) ArchiveMemory(ctx context.Context, id string) (core.Memory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return core.Memory{}, errors.New("memory id is required")
	}

	if _, err := s.db.ExecContext(ctx, `
UPDATE memories
SET status = 'archived', updated_at = now()
WHERE id = $1
`, id); err != nil {
		return core.Memory{}, err
	}

	return s.GetMemory(ctx, id)
}

func (s *PostgresMemoryStore) AddMemoryTag(ctx context.Context, id string, tag string) (core.Memory, error) {
	id = strings.TrimSpace(id)
	tag = strings.TrimSpace(tag)
	if id == "" {
		return core.Memory{}, errors.New("memory id is required")
	}
	if tag == "" {
		return core.Memory{}, errors.New("memory tag is required")
	}

	if _, err := s.db.ExecContext(ctx, `
INSERT INTO memory_tags (memory_id, tag)
VALUES ($1, $2)
ON CONFLICT DO NOTHING
`, id, tag); err != nil {
		return core.Memory{}, err
	}

	if _, err := s.db.ExecContext(ctx, `UPDATE memories SET updated_at = now() WHERE id = $1`, id); err != nil {
		return core.Memory{}, err
	}

	return s.GetMemory(ctx, id)
}

func (s *PostgresMemoryStore) CreateProject(ctx context.Context, project core.Project) (core.Project, error) {
	project.Slug = strings.TrimSpace(project.Slug)
	project.Name = strings.TrimSpace(project.Name)
	if project.Slug == "" {
		return core.Project{}, errors.New("project slug is required")
	}
	if project.Name == "" {
		project.Name = project.Slug
	}

	now := time.Now()
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO projects (slug, name, description, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, updated_at = EXCLUDED.updated_at
RETURNING id, created_at, updated_at
`, project.Slug, project.Name, project.Description, nonEmpty(project.Status, "active"), now, now).Scan(&id, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return core.Project{}, err
	}

	project.ID = fmt.Sprintf("%d", id)
	project.Status = nonEmpty(project.Status, "active")
	return project, nil
}

func (s *PostgresMemoryStore) ListProjects(ctx context.Context) ([]core.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, slug, name, description, status, created_at, updated_at
FROM projects
WHERE status = 'active'
ORDER BY slug
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []core.Project
	for rows.Next() {
		var id int64
		var project core.Project
		if err := rows.Scan(&id, &project.Slug, &project.Name, &project.Description, &project.Status, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		project.ID = fmt.Sprintf("%d", id)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *PostgresMemoryStore) GetProject(ctx context.Context, slug string) (core.Project, error) {
	slug = strings.TrimSpace(slug)
	var id int64
	var project core.Project
	err := s.db.QueryRowContext(ctx, `
SELECT id, slug, name, description, status, created_at, updated_at
FROM projects
WHERE slug = $1 AND status = 'active'
`, slug).Scan(&id, &project.Slug, &project.Name, &project.Description, &project.Status, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return core.Project{}, err
	}
	project.ID = fmt.Sprintf("%d", id)
	return project, nil
}

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
	err := s.db.QueryRowContext(ctx, `
INSERT INTO contacts (alias, full_name, kind, relationship, project_slug, importance, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, $8)
RETURNING id
`, identity.Alias, identity.RawName, core.ContactKindUnknown, core.ContactRelationshipUnknown, normalizeProjectSlugStorage(identity.ProjectSlug), defaultInt(0, 3), now, now).Scan(&id)
	if err != nil {
		return core.Contact{}, err
	}

	if identity.RawEmail != "" {
		storedEmail, err := s.encryptContactEmail(identity.RawEmail)
		if err != nil {
			return core.Contact{}, err
		}
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO contact_addresses (contact_id, address_hash, email, display_name_seen, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (address_hash) DO UPDATE SET contact_id = EXCLUDED.contact_id, display_name_seen = EXCLUDED.display_name_seen, updated_at = EXCLUDED.updated_at
`, id, emailHash(identity.RawEmail), storedEmail, identity.RawName, now, now); err != nil {
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
	contact.Email = s.decryptContactEmail(contact.Email)
	contact.CreatedAt = createdAt
	contact.UpdatedAt = updatedAt
	return contact, nil
}

func (s *PostgresMemoryStore) projectIDBySlug(ctx context.Context, slug string) (any, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, nil
	}

	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE slug = $1 AND status = 'active'`, slug).Scan(&id); err != nil {
		return nil, err
	}
	return id, nil
}

func (s *PostgresMemoryStore) tagsForMemory(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag FROM memory_tags WHERE memory_id = $1 ORDER BY tag`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *PostgresMemoryStore) selectMemories(ctx context.Context, whereSQL string, args ...any) ([]core.Memory, error) {
	query := `
SELECT m.id, COALESCE(p.id::text, ''), COALESCE(p.slug, ''), COALESCE(p.name, ''),
       m.kind, m.text, m.source, m.confidence, m.importance, m.status, m.embedding, m.embedding_model, m.created_at, m.updated_at, m.expires_at
FROM memories m
LEFT JOIN projects p ON p.id = m.project_id
` + whereSQL + `
ORDER BY m.importance DESC, m.updated_at DESC
`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []core.Memory
	for rows.Next() {
		var id int64
		var memory core.Memory
		var expiresAt sql.NullTime
		var embedding []byte
		if err := rows.Scan(&id, &memory.Project.ID, &memory.Project.Slug, &memory.Project.Name, &memory.Kind, &memory.Text, &memory.Source, &memory.Confidence, &memory.Importance, &memory.Status, &embedding, &memory.EmbeddingModel, &memory.CreatedAt, &memory.UpdatedAt, &expiresAt); err != nil {
			return nil, err
		}
		memory.Embedding, err = decodeEmbedding(embedding)
		if err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			memory.ExpiresAt = expiresAt.Time
		}
		memory.ID = fmt.Sprintf("%d", id)
		memory.Tags, err = s.tagsForMemory(ctx, id)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

func nonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultFloat(value float64, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func nonZeroTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func encodeEmbedding(embedding []float64) (any, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(embedding)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func decodeEmbedding(data []byte) ([]float64, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var embedding []float64
	if err := json.Unmarshal(data, &embedding); err != nil {
		return nil, err
	}
	return embedding, nil
}

func cosineSimilarity(a []float64, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot float64
	var normA float64
	var normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func emailHash(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x", sum[:])
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

func deriveContactEncryptionKey(secret string) []byte {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func (s *PostgresMemoryStore) encryptContactEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", nil
	}
	if len(s.contactEncryptionKey) == 0 {
		return "", nil
	}

	block, err := aes.NewCipher(s.contactEncryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(email), nil)
	payload := append(nonce, ciphertext...)
	return "enc:v1:" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (s *PostgresMemoryStore) decryptContactEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "enc:v1:") || len(s.contactEncryptionKey) == 0 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "enc:v1:"))
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(s.contactEncryptionKey)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	if len(payload) < gcm.NonceSize() {
		return ""
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ""
	}
	return string(plaintext)
}
