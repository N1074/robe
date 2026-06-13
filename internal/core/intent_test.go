package core

import (
	"context"
	"strings"
	"testing"
)

func TestHandleTextNaturalCalendarCreateIntentRequiresConfirmation(t *testing.T) {
	calendar := &mockCalendar{}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind: IntentCalendarCreate,
		},
	}
	intentParser.intent.CalendarDraft = CalendarEventDraft{
		Title: "Dentist",
		Start: fixedTime(t, "2026-06-07T12:00:00+02:00"),
		End:   fixedTime(t, "2026-06-07T13:00:00+02:00"),
	}
	assistant := testCalendarAssistant(t, calendar, WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "crea una cita de calendario para manana a las 12 con el dentista")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Proposed calendar action:\nCreate event") || !strings.Contains(got, "Title: Dentist") || !strings.Contains(got, "/confirm cal_TEST") {
		t.Fatalf("unexpected proposal: %q", got)
	}
	if calendar.createdCount != 0 {
		t.Fatalf("event was created before confirmation")
	}
}

func TestHandleTextNaturalCalendarListIntent(t *testing.T) {
	calendar := &mockCalendar{
		events: []CalendarEvent{
			{
				ID:    "evt_1",
				Title: "Dentist",
				Start: fixedTime(t, "2026-06-07T12:00:00+02:00"),
				End:   fixedTime(t, "2026-06-07T13:00:00+02:00"),
			},
		},
	}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind:           IntentCalendarList,
			CalendarPeriod: "tomorrow",
		},
	}
	assistant := testCalendarAssistant(t, calendar, WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "que tengo manana")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Calendar tomorrow:") || !strings.Contains(got, "Dentist") {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextNaturalAskIntentFallsBackToLLM(t *testing.T) {
	llm := mockLLM{answer: "answer"}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind:      IntentAsk,
			AskPrompt: "hello",
		},
	}
	assistant := NewAssistant(llm, Status{}, WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "answer" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextNaturalEmailSearchIntent(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{
			{ID: "msg_1", From: "sender@example.com", Subject: "Invoice", Snippet: "June invoice"},
		},
	}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind:       IntentEmailSearch,
			EmailQuery: "from:sender@example.com invoice",
		},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "busca correos de sender sobre invoice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Email:") || !strings.Contains(got, "[msg_1] Invoice") {
		t.Fatalf("unexpected response: %q", got)
	}
	if email.lastQuery.Query != "from:sender@example.com invoice" {
		t.Fatalf("unexpected email query: %#v", email.lastQuery)
	}
}

func TestHandleTextNaturalEmailSearchFallsBackToOriginalTextWhenQueryMissing(t *testing.T) {
	email := &mockEmail{}
	intentParser := mockIntentParser{
		intent: Intent{Kind: IntentEmailSearch},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithIntentParser(intentParser))

	if _, err := assistant.HandleText(context.Background(), "busca correos sobre facturas"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if email.lastQuery.Query != "busca correos sobre facturas" {
		t.Fatalf("expected original text fallback, got %#v", email.lastQuery)
	}
}

func TestHandleTextNaturalEmailShowIntentRequiresExplicitID(t *testing.T) {
	email := &mockEmail{
		messages: []EmailMessage{{ID: "msg_1", Subject: "Hello", PlainText: "Body"}},
	}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind:           IntentEmailShow,
			EmailMessageID: "msg_1",
		},
	}
	assistant := NewAssistant(nil, Status{}, WithEmail(email), WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "abre el correo msg_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "ID: msg_1") || !strings.Contains(got, "Body") {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextPassesConfiguredProjectHintsToIntentParser(t *testing.T) {
	intentParser := &recordingIntentParser{
		intent: Intent{Kind: IntentAsk, AskPrompt: "hello"},
	}
	assistant := NewAssistant(
		mockLLM{answer: "answer"},
		Status{},
		WithIntentParser(intentParser),
		WithProjectAliases(map[string]string{"demo": "demo", "veg": "demo"}),
	)

	if _, err := assistant.HandleText(context.Background(), "hello"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(intentParser.lastRequest.ProjectHints) != 1 || intentParser.lastRequest.ProjectHints[0] != "demo" {
		t.Fatalf("unexpected project hints: %#v", intentParser.lastRequest.ProjectHints)
	}
}

func TestHandleTextExplicitMemoryRequestCreatesMemory(t *testing.T) {
	memory := &mockMemoryStore{}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind: IntentMemoryCreate,
			MemoryDraft: Memory{
				Project:    ProjectRef{Slug: "demo"},
				Kind:       MemoryKindPreference,
				Text:       "User prefers kilos, not boxes, for demo orders.",
				Tags:       []string{"demo", "orders"},
				Importance: 4,
				Confidence: 0.9,
			},
		},
	}
	assistant := NewAssistant(nil, Status{}, WithMemory(memory), WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "recuerda que para el demo quiero hablar en kilos, no en cajas")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Memory saved:") || len(memory.memories) != 1 {
		t.Fatalf("unexpected memory response: %q memories=%d", got, len(memory.memories))
	}
	if memory.memories[0].Project.Slug != "demo" || memory.memories[0].Kind != MemoryKindPreference || memory.memories[0].Source != "telegram/llm" {
		t.Fatalf("unexpected stored memory: %#v", memory.memories[0])
	}
}

func TestHandleTextNormalConversationDoesNotCreateMemory(t *testing.T) {
	llm := mockLLM{answer: "Madrid is the capital of Spain."}
	memory := &mockMemoryStore{}
	intentParser := mockIntentParser{
		intent: Intent{Kind: IntentAsk, AskPrompt: "what is the capital of Spain?"},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithIntentParser(intentParser))

	got, err := assistant.HandleText(context.Background(), "cual es la capital de espana")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Madrid is the capital of Spain." || len(memory.memories) != 0 {
		t.Fatalf("unexpected result: %q memories=%d", got, len(memory.memories))
	}
}

func TestHandleTextSensitiveMemoryIsNotStoredWithoutExplicitIntent(t *testing.T) {
	llm := mockLLM{answer: "I will not store that."}
	memory := &mockMemoryStore{}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind: IntentMemoryCreate,
			MemoryDraft: Memory{
				Kind: MemoryKindFact,
				Text: "User password is hunter2.",
			},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithIntentParser(intentParser))

	if _, err := assistant.HandleText(context.Background(), "mi password es hunter2"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(memory.memories) != 0 {
		t.Fatalf("expected no memory to be stored, got %#v", memory.memories)
	}
}

func TestHandleTextMemoryToolRequestIsValidatedBeforePersistence(t *testing.T) {
	llm := mockLLM{answer: "No memory stored."}
	memory := &mockMemoryStore{}
	audit := &mockAuditLogger{}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind: IntentMemoryCreate,
			MemoryDraft: Memory{
				Kind: "unsupported",
				Text: "This should not persist.",
			},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithIntentParser(intentParser), WithAuditLogger(audit))

	got, err := assistant.HandleText(context.Background(), "recuerda que esto deberia fallar")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "Memory was not saved: unsupported memory kind") {
		t.Fatalf("unexpected validation response: %q", got)
	}
	if len(memory.memories) != 0 {
		t.Fatalf("expected validation to block memory, got %#v", memory.memories)
	}
	if len(audit.events) != 1 || audit.events[0].Decision != DecisionDeny || audit.events[0].Result != AuditResultRejected {
		t.Fatalf("expected rejected audit event, got %#v", audit.events)
	}
}

func TestHandleTextMemoryToolRequestIsValidatedLengthLimit(t *testing.T) {
	llm := mockLLM{answer: "No memory stored."}
	memory := &mockMemoryStore{}
	audit := &mockAuditLogger{}
	intentParser := mockIntentParser{
		intent: Intent{
			Kind: IntentMemoryCreate,
			MemoryDraft: Memory{
				Kind: MemoryKindPreference,
				Text: strings.Repeat("a", 1001),
			},
		},
	}
	assistant := NewAssistant(llm, Status{}, WithMemory(memory), WithIntentParser(intentParser), WithAuditLogger(audit))

	got, err := assistant.HandleText(context.Background(), "recuerda que esto deberia fallar")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "Memory was not saved: memory text exceeds maximum length of 1000 characters") {
		t.Fatalf("unexpected validation response: %q", got)
	}
	if len(memory.memories) != 0 {
		t.Fatalf("expected validation to block memory, got %#v", memory.memories)
	}
	if len(audit.events) != 1 || audit.events[0].Decision != DecisionDeny || audit.events[0].Result != AuditResultRejected {
		t.Fatalf("expected rejected audit event, got %#v", audit.events)
	}
}
