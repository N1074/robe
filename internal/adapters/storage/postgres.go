package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresMemoryStore struct {
	db                            *sql.DB
	contactEncryptionKey          []byte
	previousContactEncryptionKeys [][]byte
}

type Options struct {
	ContactEncryptionKey          string
	PreviousContactEncryptionKeys []string
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
		db:                            db,
		contactEncryptionKey:          deriveContactEncryptionKey(opts.ContactEncryptionKey),
		previousContactEncryptionKeys: deriveContactEncryptionKeys(opts.PreviousContactEncryptionKeys),
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
