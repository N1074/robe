package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
)

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
