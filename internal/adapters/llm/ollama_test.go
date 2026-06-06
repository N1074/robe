package llm

import "testing"

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
