package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHandleTextAskWithMemory(t *testing.T) {
	var lastPrompt string
	llm := mockLLM{answer: "Use Postgres.", lastPrompt: &lastPrompt}
	memory := &mockMemoryStore{
		memories: []Memory{
			{
				ID:        "1",
				Kind:      "decision",
				Text:      "Use Postgres as Robe's source of truth.",
				Source:    "telegram",
				Status:    "active",
				CreatedAt: fixedTime(t, "2026-06-06T12:00:00+02:00"),
			},
		},
	}
	embedder := mockEmbedder{embedding: []float64{1, 0, 0}, model: "test-embed"}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithEmbedder(embedder))

	got, err := assistant.HandleText(context.Background(), "/askmem postgres | what database should Robe use?")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Used memories: 1") || !strings.Contains(got, "Use Postgres.") {
		t.Fatalf("unexpected response: %q", got)
	}
	if !strings.Contains(lastPrompt, "Relevant memory:") || !strings.Contains(lastPrompt, "Use Postgres as Robe's source of truth.") {
		t.Fatalf("prompt did not include bounded memory context: %q", lastPrompt)
	}
	if !memory.lastFilter.Semantic || len(memory.lastFilter.Embedding) != 3 || !memory.lastFilter.IncludeGlobal {
		t.Fatalf("expected semantic memory filter, got %#v", memory.lastFilter)
	}
}

func TestHandleTextAskWithMemoryRequiresSeparator(t *testing.T) {
	assistant := NewAssistant(mockLLM{}, Status{}, WithMemory(&mockMemoryStore{}))

	got, err := assistant.HandleText(context.Background(), "/askmem postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Usage: /askmem <memory query> | <question>" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextRememberRequiresMemoryStore(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/remember the dentist prefers mornings")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != memoryNotConfiguredMessage() {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextRememberStoresMemory(t *testing.T) {
	memory := &mockMemoryStore{}
	now := fixedTime(t, "2026-06-06T12:00:00+02:00")
	assistant := NewAssistant(nil, Status{}, WithMemory(memory), WithEmbedder(mockEmbedder{embedding: []float64{0.1, 0.2}, model: "test-embed"}), WithNow(func() time.Time { return now }))

	got, err := assistant.HandleText(context.Background(), "/remember the dentist prefers mornings")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Memory saved:") || !strings.Contains(got, "the dentist prefers mornings") {
		t.Fatalf("unexpected response: %q", got)
	}
	if len(memory.memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memory.memories))
	}
	if len(memory.memories[0].Embedding) != 2 || memory.memories[0].EmbeddingModel != "test-embed" {
		t.Fatalf("expected embedded memory, got %#v", memory.memories[0])
	}
}

func TestHandleTextRememberCappingMemoryText(t *testing.T) {
	memory := &mockMemoryStore{}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	longText := strings.Repeat("a", 1001)
	got, err := assistant.HandleText(context.Background(), "/remember "+longText)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "memory text exceeds maximum length of 1000 characters") {
		t.Fatalf("expected error message for text exceeding limit, got %q", got)
	}
	if len(memory.memories) != 0 {
		t.Fatalf("expected no memory to be stored, got %d", len(memory.memories))
	}
}

func TestHandleTextRememberAuditsMemoryCreate(t *testing.T) {
	memory := &mockMemoryStore{}
	audit := &mockAuditLogger{}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory), WithAuditLogger(audit))

	if _, err := assistant.HandleText(context.Background(), "/remember --kind preference concise answers"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	event := audit.events[0]
	if event.ActionType != ActionMemoryCreate || event.Decision != DecisionAllow || event.Result != AuditResultExecuted || event.ResourceID != "1" {
		t.Fatalf("unexpected audit event: %#v", event)
	}
	if event.Metadata["kind"] != MemoryKindPreference || event.Metadata["project"] != "global" {
		t.Fatalf("unexpected audit metadata: %#v", event.Metadata)
	}
}

func TestHandleTextRememberStoresMemoryWhenEmbeddingFails(t *testing.T) {
	memory := &mockMemoryStore{}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory), WithEmbedder(mockEmbedder{err: errors.New("embed unavailable")}))

	got, err := assistant.HandleText(context.Background(), "/remember the dentist prefers mornings")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Memory saved:") || len(memory.memories) != 1 {
		t.Fatalf("unexpected response: %q memories=%d", got, len(memory.memories))
	}
	if len(memory.memories[0].Embedding) != 0 || memory.memories[0].EmbeddingModel != "" {
		t.Fatalf("expected memory without embedding, got %#v", memory.memories[0])
	}
}

func TestHandleTextRememberStoresStructuredMemory(t *testing.T) {
	memory := &mockMemoryStore{}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	got, err := assistant.HandleText(context.Background(), "/remember --project robe --kind decision --tags architecture,postgres use Postgres as source of truth")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "[decision/robe]") || !strings.Contains(got, "#architecture #postgres") {
		t.Fatalf("unexpected response: %q", got)
	}
	if memory.memories[0].Project.Slug != "robe" || memory.memories[0].Kind != "decision" {
		t.Fatalf("unexpected memory: %#v", memory.memories[0])
	}
}

func TestHandleTextMemoriesSearchesMemory(t *testing.T) {
	memory := &mockMemoryStore{
		memories: []Memory{
			{
				ID:        "1",
				Text:      "the dentist prefers mornings",
				Source:    "telegram",
				CreatedAt: fixedTime(t, "2026-06-06T12:00:00+02:00"),
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	got, err := assistant.HandleText(context.Background(), "/memories dentist")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Memories:") || !strings.Contains(got, "dentist") {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextMemoryShowArchiveAndTag(t *testing.T) {
	memory := &mockMemoryStore{
		memories: []Memory{
			{
				ID:         "1",
				Kind:       "preference",
				Text:       "the dentist prefers mornings",
				Source:     "telegram",
				Status:     "active",
				Confidence: 1,
				Importance: 3,
				CreatedAt:  fixedTime(t, "2026-06-06T12:00:00+02:00"),
				UpdatedAt:  fixedTime(t, "2026-06-06T12:00:00+02:00"),
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	got, err := assistant.HandleText(context.Background(), "/memory show 1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "Memory:") || !strings.Contains(got, "Status: active") {
		t.Fatalf("unexpected show response: %q", got)
	}

	got, err = assistant.HandleText(context.Background(), "/memory tag 1 dentist")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "#dentist") {
		t.Fatalf("unexpected tag response: %q", got)
	}

	got, err = assistant.HandleText(context.Background(), "/memory archive 1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "Memory archived:") || memory.memories[0].Status != "archived" {
		t.Fatalf("unexpected archive response: %q", got)
	}
}

func TestHandleTextForgetArchivesMemory(t *testing.T) {
	memory := &mockMemoryStore{
		memories: []Memory{
			{ID: "1", Text: "old context", Status: "active"},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	got, err := assistant.HandleText(context.Background(), "/forget 1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Memory forgotten:") || memory.memories[0].Status != "archived" {
		t.Fatalf("unexpected forget response: %q", got)
	}
}

func TestHandleTextProjectCreateUseAndRemember(t *testing.T) {
	memory := &mockMemoryStore{}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory))

	got, err := assistant.HandleText(context.Background(), "/project create robe | Robe")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "Project created:") {
		t.Fatalf("unexpected create response: %q", got)
	}

	got, err = assistant.HandleText(context.Background(), "/project use robe")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Active project: robe | Robe" {
		t.Fatalf("unexpected use response: %q", got)
	}

	if _, err = assistant.HandleText(context.Background(), "/remember project memory"); err != nil {
		t.Fatalf("expected remember, got %v", err)
	}
	if memory.memories[0].Project.Slug != "robe" {
		t.Fatalf("expected active project robe, got %#v", memory.memories[0].Project)
	}
}

func TestProjectForTextUsesOnlyConfiguredAliases(t *testing.T) {
	assistant := NewAssistant(nil, Status{})
	if got := assistant.projectForText("question about garden"); got != "" {
		t.Fatalf("expected no project without aliases, got %q", got)
	}

	assistant = NewAssistant(nil, Status{}, WithProjectAliases(map[string]string{"garden": "garden"}))
	if got := assistant.projectForText("question about garden"); got != "garden" {
		t.Fatalf("expected configured project, got %q", got)
	}
}

func TestHandleTextProjectMemoryIsRetrievedOnlyWhenRelevant(t *testing.T) {
	var lastPrompt string
	llm := mockLLM{answer: "Use kilos.", lastPrompt: &lastPrompt}
	memory := &mockMemoryStore{
		memories: []Memory{
			{ID: "1", Project: ProjectRef{Slug: "garden"}, Kind: MemoryKindConstraint, Text: "User prefers kilos, not boxes, for garden orders.", Status: "active", Importance: 4},
			{ID: "2", Project: ProjectRef{Slug: "writing"}, Kind: MemoryKindFact, Text: "Writing project uses a dark fantasy tone.", Status: "active", Importance: 4},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithEmbedder(mockEmbedder{embedding: []float64{1, 0}, model: "test"}), WithProjectAliases(map[string]string{"garden": "garden"}))

	if _, err := assistant.HandleText(context.Background(), "/ask que unidad usamos para garden"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(lastPrompt, "kilos") || strings.Contains(lastPrompt, "dark fantasy") {
		t.Fatalf("unexpected memory prompt: %q", lastPrompt)
	}
}

func TestHandleTextGlobalMemoryIsRetrievedBroadly(t *testing.T) {
	var lastPrompt string
	llm := mockLLM{answer: "Concise.", lastPrompt: &lastPrompt}
	memory := &mockMemoryStore{
		memories: []Memory{
			{ID: "1", Kind: MemoryKindPreference, Text: "User prefers concise technical answers.", Status: "active", Importance: 4},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithEmbedder(mockEmbedder{embedding: []float64{1, 0}, model: "test"}))

	if _, err := assistant.HandleText(context.Background(), "/ask explain robe architecture"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(lastPrompt, "[global/preference/high] User prefers concise technical answers.") {
		t.Fatalf("expected global memory in prompt, got %q", lastPrompt)
	}
}

func TestHandleTextAskFallsBackWhenEmbeddingRetrievalFails(t *testing.T) {
	var lastPrompt string
	llm := mockLLM{answer: "Plain answer.", lastPrompt: &lastPrompt}
	memory := &mockMemoryStore{
		memories: []Memory{
			{ID: "1", Kind: MemoryKindPreference, Text: "User prefers concise answers.", Status: "active", Importance: 4},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithEmbedder(mockEmbedder{err: errors.New("embed unavailable")}))

	got, err := assistant.HandleText(context.Background(), "/ask explain robe")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Plain answer." || strings.Contains(lastPrompt, "Relevant memory:") {
		t.Fatalf("unexpected answer/prompt: %q / %q", got, lastPrompt)
	}
}
