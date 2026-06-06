package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MemoryStore interface {
	AddMemory(ctx context.Context, memory Memory) (Memory, error)
	SearchMemories(ctx context.Context, filter MemoryFilter) ([]Memory, error)
	CreateProject(ctx context.Context, project Project) (Project, error)
	ListProjects(ctx context.Context) ([]Project, error)
	GetProject(ctx context.Context, slug string) (Project, error)
}

type Memory struct {
	ID         string
	Project    ProjectRef
	Kind       string
	Text       string
	Source     string
	Tags       []string
	Confidence float64
	Importance int
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
}

type Project struct {
	ID          string
	Slug        string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectRef struct {
	ID   string
	Slug string
	Name string
}

type MemoryFilter struct {
	Query       string
	ProjectSlug string
	Kind        string
	Tag         string
	Status      string
	Limit       int
}

func (a *Assistant) handleRemember(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return rememberUsage(), nil
	}

	draft, err := parseRememberArgs(text)
	if err != nil {
		return err.Error(), nil
	}

	if draft.Project.Slug == "" {
		draft.Project.Slug = a.activeProject
	}

	memory, err := a.memory.AddMemory(ctx, Memory{
		Project:    draft.Project,
		Kind:       nonEmpty(draft.Kind, "note"),
		Text:       draft.Text,
		Source:     "telegram",
		Tags:       draft.Tags,
		Confidence: defaultConfidence(draft.Confidence),
		Importance: defaultImportance(draft.Importance),
		Status:     "active",
		CreatedAt:  a.now(),
		UpdatedAt:  a.now(),
	})
	if err != nil {
		return "", err
	}

	return "Memory saved:\n" + formatMemory(memory), nil
}

func (a *Assistant) handleMemories(ctx context.Context, query string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return memoriesUsage(), nil
	}

	filter, err := parseMemoriesArgs(query)
	if err != nil {
		return err.Error(), nil
	}
	if filter.ProjectSlug == "" {
		filter.ProjectSlug = a.activeProject
	}
	if filter.Status == "" {
		filter.Status = "active"
	}
	if filter.Limit == 0 {
		filter.Limit = 5
	}

	memories, err := a.memory.SearchMemories(ctx, filter)
	if err != nil {
		return "", err
	}

	if len(memories) == 0 {
		return "No memories found.", nil
	}

	var b strings.Builder
	b.WriteString("Memories:\n")
	for _, memory := range memories {
		b.WriteString("- ")
		b.WriteString(formatMemory(memory))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func formatMemory(memory Memory) string {
	var b strings.Builder
	if strings.TrimSpace(memory.ID) != "" {
		b.WriteString("[")
		b.WriteString(memory.ID)
		b.WriteString("] ")
	}
	b.WriteString("[")
	b.WriteString(nonEmpty(memory.Kind, "note"))
	if memory.Project.Slug != "" {
		b.WriteString("/")
		b.WriteString(memory.Project.Slug)
	}
	b.WriteString("] ")
	b.WriteString(strings.TrimSpace(memory.Text))
	if len(memory.Tags) > 0 {
		b.WriteString(" #")
		b.WriteString(strings.Join(memory.Tags, " #"))
	}
	if !memory.CreatedAt.IsZero() {
		b.WriteString(" (")
		b.WriteString(formatTime(memory.CreatedAt))
		b.WriteString(")")
	}
	return b.String()
}

func memoryNotConfiguredMessage() string {
	return "Memory is not configured yet. Set MEMORY_PROVIDER=postgres and DATABASE_URL, then restart Robe."
}

func (a *Assistant) handleProject(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	arg := strings.TrimSpace(strings.TrimPrefix(text, "/project"))
	if arg == "" || arg == "status" {
		return a.projectStatus(), nil
	}

	switch {
	case arg == "list":
		return a.listProjects(ctx)

	case strings.HasPrefix(arg, "use "):
		return a.useProject(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "use ")))

	case strings.HasPrefix(arg, "create "):
		return a.createProject(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "create ")))

	default:
		return projectUsage(), nil
	}
}

func (a *Assistant) createProject(ctx context.Context, input string) (string, error) {
	slug, name, ok := strings.Cut(input, "|")
	if !ok {
		slug = input
		name = input
	}

	project, err := a.memory.CreateProject(ctx, Project{
		Slug:   strings.TrimSpace(slug),
		Name:   strings.TrimSpace(name),
		Status: "active",
	})
	if err != nil {
		return "", err
	}

	return "Project created:\n" + formatProject(project), nil
}

func (a *Assistant) useProject(ctx context.Context, slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "Usage: /project use <slug>", nil
	}

	project, err := a.memory.GetProject(ctx, slug)
	if err != nil {
		return "", err
	}

	a.activeProject = project.Slug
	return "Active project: " + formatProject(project), nil
}

func (a *Assistant) listProjects(ctx context.Context) (string, error) {
	projects, err := a.memory.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "No projects found.", nil
	}

	var b strings.Builder
	b.WriteString("Projects:\n")
	for _, project := range projects {
		b.WriteString("- ")
		b.WriteString(formatProject(project))
		if project.Slug == a.activeProject {
			b.WriteString(" (active)")
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) projectStatus() string {
	if a.activeProject == "" {
		return "Active project: global"
	}
	return "Active project: " + a.activeProject
}

func formatProject(project Project) string {
	return project.Slug + " | " + project.Name
}

func parseRememberArgs(input string) (Memory, error) {
	var memory Memory
	memory.Kind = "note"
	memory.Status = "active"
	memory.Importance = 3
	memory.Confidence = 1.0

	rest := strings.TrimSpace(input)
	for strings.HasPrefix(rest, "--") {
		flag, tail, ok := strings.Cut(rest, " ")
		if !ok {
			return Memory{}, fmt.Errorf("missing value for %s", flag)
		}
		value, remaining, _ := strings.Cut(strings.TrimSpace(tail), " ")
		if strings.TrimSpace(value) == "" {
			return Memory{}, fmt.Errorf("missing value for %s", flag)
		}

		switch flag {
		case "--project":
			memory.Project.Slug = strings.TrimSpace(value)
		case "--kind":
			memory.Kind = strings.TrimSpace(value)
		case "--tags":
			memory.Tags = splitCSV(value)
		case "--importance":
			parsed, err := parseInt(value)
			if err != nil {
				return Memory{}, fmt.Errorf("invalid importance: %w", err)
			}
			memory.Importance = parsed
		default:
			return Memory{}, fmt.Errorf("unknown remember flag: %s", flag)
		}

		rest = strings.TrimSpace(remaining)
	}

	memory.Text = rest
	if memory.Text == "" {
		return Memory{}, fmt.Errorf("memory text is required")
	}

	return memory, nil
}

func parseMemoriesArgs(input string) (MemoryFilter, error) {
	var filter MemoryFilter

	rest := strings.TrimSpace(input)
	for strings.HasPrefix(rest, "--") {
		flag, tail, ok := strings.Cut(rest, " ")
		if !ok {
			return MemoryFilter{}, fmt.Errorf("missing value for %s", flag)
		}
		value, remaining, _ := strings.Cut(strings.TrimSpace(tail), " ")
		if strings.TrimSpace(value) == "" {
			return MemoryFilter{}, fmt.Errorf("missing value for %s", flag)
		}

		switch flag {
		case "--project":
			filter.ProjectSlug = strings.TrimSpace(value)
		case "--kind":
			filter.Kind = strings.TrimSpace(value)
		case "--tag":
			filter.Tag = strings.TrimSpace(value)
		default:
			return MemoryFilter{}, fmt.Errorf("unknown memories flag: %s", flag)
		}

		rest = strings.TrimSpace(remaining)
	}

	filter.Query = rest
	if filter.Query == "" {
		return MemoryFilter{}, fmt.Errorf("memory query is required")
	}

	return filter, nil
}

func rememberUsage() string {
	return "Usage: /remember [--project slug] [--kind note|preference|fact|task|decision|project_knowledge|contact|operational] [--tags a,b] <text>"
}

func memoriesUsage() string {
	return "Usage: /memories [--project slug] [--kind kind] [--tag tag] <query>"
}

func projectUsage() string {
	return "Project commands:\n/project list\n/project create <slug> | <name>\n/project use <slug>\n/project status"
}

func splitCSV(value string) []string {
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseInt(value string) (int, error) {
	var out int
	_, err := fmt.Sscanf(value, "%d", &out)
	return out, err
}

func defaultImportance(value int) int {
	if value <= 0 {
		return 3
	}
	return value
}

func defaultConfidence(value float64) float64 {
	if value <= 0 {
		return 1.0
	}
	return value
}
