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
CREATE TABLE IF NOT EXISTS memories (
	id BIGSERIAL PRIMARY KEY,
	text TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT 'manual',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS memories_created_at_idx ON memories (created_at DESC);
CREATE INDEX IF NOT EXISTS memories_text_lower_idx ON memories (lower(text));
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

	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO memories (text, source, created_at)
VALUES ($1, $2, $3)
RETURNING id
`, text, source, createdAt).Scan(&id)
	if err != nil {
		return core.Memory{}, err
	}

	memory.ID = fmt.Sprintf("%d", id)
	memory.Text = text
	memory.Source = source
	memory.CreatedAt = createdAt
	return memory, nil
}

func (s *PostgresMemoryStore) SearchMemories(ctx context.Context, query string, limit int) ([]core.Memory, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, text, source, created_at
FROM memories
WHERE text ILIKE '%' || $1 || '%'
ORDER BY created_at DESC
LIMIT $2
`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []core.Memory
	for rows.Next() {
		var id int64
		var memory core.Memory
		if err := rows.Scan(&id, &memory.Text, &memory.Source, &memory.CreatedAt); err != nil {
			return nil, err
		}
		memory.ID = fmt.Sprintf("%d", id)
		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return memories, nil
}
