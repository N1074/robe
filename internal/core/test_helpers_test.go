package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockLLM struct {
	answer     string
	err        error
	lastPrompt *string
}

func (m mockLLM) Ask(ctx context.Context, prompt string) (string, error) {
	if m.lastPrompt != nil {
		*m.lastPrompt = prompt
	}
	return m.answer, m.err
}

type mockCalendar struct {
	events       []CalendarEvent
	lastQuery    CalendarQuery
	createdCount int
	deletedID    string
}

func (m *mockCalendar) ListEvents(ctx context.Context, query CalendarQuery) ([]CalendarEvent, error) {
	m.lastQuery = query
	return m.events, nil
}

func (m *mockCalendar) CreateEvent(ctx context.Context, draft CalendarEventDraft) (CalendarEvent, error) {
	m.createdCount++
	return CalendarEvent{
		ID:       "created_1",
		Title:    draft.Title,
		Start:    draft.Start,
		End:      draft.End,
		Location: draft.Location,
	}, nil
}

func (m *mockCalendar) DeleteEvent(ctx context.Context, eventID string) error {
	m.deletedID = eventID
	return nil
}

type mockMemoryStore struct {
	memories   []Memory
	projects   map[string]Project
	lastFilter MemoryFilter
	filters    []MemoryFilter
}

func (m *mockMemoryStore) AddMemory(ctx context.Context, memory Memory) (Memory, error) {
	memory.ID = "1"
	m.memories = append(m.memories, memory)
	return memory, nil
}

func (m *mockMemoryStore) SearchMemories(ctx context.Context, filter MemoryFilter) ([]Memory, error) {
	m.lastFilter = filter
	m.filters = append(m.filters, filter)

	var out []Memory
	for _, memory := range m.memories {
		if filter.Status != "" && memory.Status != "" && memory.Status != filter.Status {
			continue
		}
		if filter.GlobalOnly && memory.Project.Slug != "" {
			continue
		}
		if filter.ProjectSlug != "" && memory.Project.Slug != filter.ProjectSlug {
			if !(filter.IncludeGlobal && memory.Project.Slug == "") {
				continue
			}
		}
		if filter.Kind != "" && memory.Kind != filter.Kind {
			continue
		}
		if filter.Tag != "" && !containsString(memory.Tags, filter.Tag) {
			continue
		}
		if !filter.Semantic && filter.Query != "" && !strings.Contains(strings.ToLower(memory.Text), strings.ToLower(filter.Query)) {
			continue
		}
		out = append(out, memory)
	}
	return out, nil
}

func (m *mockMemoryStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	for _, memory := range m.memories {
		if memory.ID == id {
			return memory, nil
		}
	}
	return Memory{}, errors.New("memory not found")
}

func (m *mockMemoryStore) ArchiveMemory(ctx context.Context, id string) (Memory, error) {
	for i := range m.memories {
		if m.memories[i].ID == id {
			m.memories[i].Status = "archived"
			return m.memories[i], nil
		}
	}
	return Memory{}, errors.New("memory not found")
}

func (m *mockMemoryStore) AddMemoryTag(ctx context.Context, id string, tag string) (Memory, error) {
	for i := range m.memories {
		if m.memories[i].ID == id {
			m.memories[i].Tags = append(m.memories[i].Tags, tag)
			return m.memories[i], nil
		}
	}
	return Memory{}, errors.New("memory not found")
}

func (m *mockMemoryStore) CreateProject(ctx context.Context, project Project) (Project, error) {
	if m.projects == nil {
		m.projects = make(map[string]Project)
	}
	project.ID = "1"
	project.Status = nonEmpty(project.Status, "active")
	m.projects[project.Slug] = project
	return project, nil
}

func (m *mockMemoryStore) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	for _, project := range m.projects {
		projects = append(projects, project)
	}
	return projects, nil
}

func (m *mockMemoryStore) GetProject(ctx context.Context, slug string) (Project, error) {
	return m.projects[slug], nil
}

type mockAuditLogger struct {
	events []AuditEvent
}

func (m *mockAuditLogger) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	m.events = append(m.events, event)
	return nil
}

type mockIntentParser struct {
	intent Intent
	err    error
}

func (m mockIntentParser) ParseIntent(ctx context.Context, req IntentRequest) (Intent, error) {
	return m.intent, m.err
}

type recordingIntentParser struct {
	intent      Intent
	err         error
	lastRequest IntentRequest
}

func (m *recordingIntentParser) ParseIntent(ctx context.Context, req IntentRequest) (Intent, error) {
	m.lastRequest = req
	return m.intent, m.err
}

type mockEmbedder struct {
	embedding []float64
	model     string
	err       error
}

func (m mockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	return m.embedding, m.err
}

func (m mockEmbedder) Model() string {
	return m.model
}

func testCalendarAssistant(t *testing.T, calendar Calendar, opts ...AssistantOption) *Assistant {
	t.Helper()

	now := fixedTime(t, "2026-06-06T12:00:00+02:00")
	options := []AssistantOption{
		WithCalendar(calendar),
		WithNow(func() time.Time { return now }),
		WithTokenGenerator(func(prefix string) (string, error) { return prefix + "TEST", nil }),
	}
	options = append(options, opts...)

	return NewAssistant(
		nil,
		Status{Timezone: "Europe/Madrid"},
		options...,
	)
}

func fixedTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
