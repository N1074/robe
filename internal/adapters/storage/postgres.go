package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresMemoryStore struct {
	db *sql.DB
}

func NewPostgresMemoryStore(ctx context.Context, databaseURL string) (*PostgresMemoryStore, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	store := &PostgresMemoryStore{db: db}
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
ALTER TABLE memories ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE memories ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS memory_tags (
	memory_id BIGINT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
	tag TEXT NOT NULL,
	PRIMARY KEY(memory_id, tag)
);

CREATE INDEX IF NOT EXISTS memories_created_at_idx ON memories (created_at DESC);
CREATE INDEX IF NOT EXISTS memories_text_lower_idx ON memories (lower(text));
CREATE INDEX IF NOT EXISTS memories_project_idx ON memories (project_id);
CREATE INDEX IF NOT EXISTS memories_kind_idx ON memories (kind);
CREATE INDEX IF NOT EXISTS memories_status_idx ON memories (status);
CREATE INDEX IF NOT EXISTS memory_tags_tag_idx ON memory_tags (tag);
`)
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
	err = s.db.QueryRowContext(ctx, `
INSERT INTO memories (project_id, kind, text, source, confidence, importance, status, created_at, updated_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id
`, projectID, nonEmpty(memory.Kind, "note"), text, source, defaultFloat(memory.Confidence, 1.0), defaultInt(memory.Importance, 3), nonEmpty(memory.Status, "active"), createdAt, nonZeroTime(memory.UpdatedAt, createdAt), expiresAt).Scan(&id)
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
	if filter.Query == "" {
		return nil, nil
	}
	if filter.Limit <= 0 || filter.Limit > 20 {
		filter.Limit = 5
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT m.id, COALESCE(p.id::text, ''), COALESCE(p.slug, ''), COALESCE(p.name, ''),
       m.kind, m.text, m.source, m.confidence, m.importance, m.status, m.created_at, m.updated_at, m.expires_at
FROM memories m
LEFT JOIN projects p ON p.id = m.project_id
WHERE m.text ILIKE '%' || $1 || '%'
  AND ($2 = '' OR p.slug = $2)
  AND ($3 = '' OR m.kind = $3)
  AND ($4 = '' OR m.status = $4)
  AND ($5 = '' OR EXISTS (SELECT 1 FROM memory_tags mt WHERE mt.memory_id = m.id AND mt.tag = $5))
ORDER BY m.importance DESC, m.updated_at DESC
LIMIT $6
`, filter.Query, filter.ProjectSlug, filter.Kind, filter.Status, filter.Tag, filter.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []core.Memory
	for rows.Next() {
		var id int64
		var memory core.Memory
		var expiresAt sql.NullTime
		if err := rows.Scan(&id, &memory.Project.ID, &memory.Project.Slug, &memory.Project.Name, &memory.Kind, &memory.Text, &memory.Source, &memory.Confidence, &memory.Importance, &memory.Status, &memory.CreatedAt, &memory.UpdatedAt, &expiresAt); err != nil {
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
