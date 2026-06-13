package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
)

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
