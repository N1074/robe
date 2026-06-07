package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripThinkingRemovesThinkBlock(t *testing.T) {
	got := stripThinking("<think>private reasoning</think>\nFinal answer.")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestStripThinkingRemovesMultipleThinkBlocks(t *testing.T) {
	got := stripThinking("<think>one</think>\nFinal <think>two</think>answer.")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestStripThinkingDropsUnclosedThinkBlock(t *testing.T) {
	got := stripThinking("Final answer.\n<think>unfinished")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestDecodeIntentCalendarCreate(t *testing.T) {
	got, err := decodeIntent(`{"action":"calendar_create","title":"Dentist","start":"2026-06-07T12:00:00+02:00","end":"2026-06-07T13:00:00+02:00","location":"Clinic","description":"Checkup"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Kind != "calendar_create" || got.CalendarDraft.Title != "Dentist" || got.CalendarDraft.Location != "Clinic" {
		t.Fatalf("unexpected intent: %#v", got)
	}
}

func TestDecodeIntentExtractsJSONFromText(t *testing.T) {
	got, err := decodeIntent("```json\n{\"action\":\"calendar_list\",\"period\":\"tomorrow\"}\n```")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Kind != "calendar_list" || got.CalendarPeriod != "tomorrow" {
		t.Fatalf("unexpected intent: %#v", got)
	}
}

func TestDecodeIntentCalendarDelete(t *testing.T) {
	got, err := decodeIntent(`{"action":"calendar_delete","event_id":"evt_1"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Kind != "calendar_delete" || got.CalendarEventID != "evt_1" {
		t.Fatalf("unexpected intent: %#v", got)
	}
}

func TestDecodeIntentMemoryCreate(t *testing.T) {
	got, err := decodeIntent(`{"action":"create_memory","text":"User prefers kilos, not boxes, for garden orders.","project":"garden","kind":"preference","tags":["garden","orders"],"importance":4,"confidence":0.9}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Kind != "create_memory" || got.MemoryDraft.Project.Slug != "garden" || got.MemoryDraft.Kind != "preference" {
		t.Fatalf("unexpected intent: %#v", got)
	}
	if len(got.MemoryDraft.Tags) != 2 || got.MemoryDraft.Importance != 4 || got.MemoryDraft.Confidence != 0.9 {
		t.Fatalf("unexpected memory draft: %#v", got.MemoryDraft)
	}
}

func TestOllamaEmbedderUsesEmbedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3]]}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-embed")
	got, err := embedder.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(got) != 3 || got[0] != 0.1 || embedder.Model() != "test-embed" {
		t.Fatalf("unexpected embedding: %#v", got)
	}
}

func TestOllamaEmbedderFallsBackToLegacyEndpoint(t *testing.T) {
	var legacyCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		legacyCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[0.4,0.5]}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-embed")
	got, err := embedder.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !legacyCalled || len(got) != 2 || got[1] != 0.5 {
		t.Fatalf("unexpected fallback result: called=%v embedding=%#v", legacyCalled, got)
	}
}
