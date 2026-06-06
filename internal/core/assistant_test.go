package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHandleTextEmptyMessage(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextUnknownCommand(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/nope")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Unknown command. Try /help." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextPing(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/ping")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "pong" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextStatus(t *testing.T) {
	assistant := NewAssistant(nil, Status{
		Env:              "test",
		LLMProvider:      "ollama",
		LLMModel:         "qwen3:14b",
		AccessRestricted: true,
	})

	got, err := assistant.HandleText(context.Background(), "/status")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Robe v0.1 online.\nEnv: test\nLLM: ollama/qwen3:14b\nAccess: restricted"
	want += "\nCalendar: disabled\nTimezone: Local"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextHelp(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Commands:\n/ping\n/status\n/ask <question>\n/calendar today|tomorrow|week\n/calendar create <title> | <start> | <end> [| location] [| description]\n/calendar delete <event_id>\n/pending\n/confirm <token>\n/cancel <token>"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskEmptyPrompt(t *testing.T) {
	assistant := NewAssistant(mockLLM{}, Status{})

	got, err := assistant.HandleText(context.Background(), "/ask   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Usage: /ask <question>" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskWithMockLLMSuccess(t *testing.T) {
	assistant := NewAssistant(mockLLM{answer: "local answer"}, Status{})

	got, err := assistant.HandleText(context.Background(), "/ask hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "local answer" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextAskWithMockLLMError(t *testing.T) {
	wantErr := errors.New("llm unavailable")
	assistant := NewAssistant(mockLLM{err: wantErr}, Status{})

	_, err := assistant.HandleText(context.Background(), "/ask hello")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestHandleTextStatusShowsSetupOpenAccess(t *testing.T) {
	assistant := NewAssistant(nil, Status{
		Env:         "dev",
		LLMProvider: "ollama",
		LLMModel:    "dolphin-mistral:latest",
	})

	got, err := assistant.HandleText(context.Background(), "/status")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Robe v0.1 online.\nEnv: dev\nLLM: ollama/dolphin-mistral:latest\nAccess: setup-open\nCalendar: disabled\nTimezone: Local"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextCalendarDisabled(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/calendar today")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Calendar is not configured." {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestHandleTextCalendarTodayListsEvents(t *testing.T) {
	now := fixedTime(t, "2026-06-06T12:00:00+02:00")
	calendar := &mockCalendar{
		events: []CalendarEvent{
			{
				ID:       "evt_1",
				Title:    "Standup",
				Start:    fixedTime(t, "2026-06-06T09:00:00+02:00"),
				End:      fixedTime(t, "2026-06-06T09:30:00+02:00"),
				Location: "Office",
			},
		},
	}
	assistant := NewAssistant(nil, Status{Timezone: "Europe/Madrid"}, WithCalendar(calendar), WithNow(func() time.Time { return now }))

	got, err := assistant.HandleText(context.Background(), "/calendar today")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "Calendar today:\n- 2026-06-06 09:00-09:30 Standup @ Office [id: evt_1]"
	if got != want {
		t.Fatalf("unexpected response: %q", got)
	}

	if calendar.lastQuery.Start.Format("2006-01-02 15:04") != "2026-06-06 00:00" {
		t.Fatalf("unexpected query start: %v", calendar.lastQuery.Start)
	}
}

func TestHandleTextCalendarCreateRequiresConfirmation(t *testing.T) {
	calendar := &mockCalendar{}
	assistant := testCalendarAssistant(t, calendar)

	got, err := assistant.HandleText(context.Background(), "/calendar create Dentist | 2026-06-07 10:00 | 2026-06-07 11:00 | Clinic | Checkup")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Proposed calendar action:\nCreate event") || !strings.Contains(got, "/confirm cal_TEST") {
		t.Fatalf("unexpected proposal: %q", got)
	}
	if calendar.createdCount != 0 {
		t.Fatalf("event was created before confirmation")
	}

	got, err = assistant.HandleText(context.Background(), "/confirm cal_TEST")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Calendar event created:") || calendar.createdCount != 1 {
		t.Fatalf("unexpected confirm result %q, created count %d", got, calendar.createdCount)
	}
}

func TestHandleTextCalendarDeleteRequiresConfirmation(t *testing.T) {
	calendar := &mockCalendar{}
	assistant := testCalendarAssistant(t, calendar)

	got, err := assistant.HandleText(context.Background(), "/calendar delete evt_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, "Proposed calendar action:\nDelete event") || !strings.Contains(got, "/confirm cal_TEST") {
		t.Fatalf("unexpected proposal: %q", got)
	}
	if calendar.deletedID != "" {
		t.Fatalf("event was deleted before confirmation")
	}

	got, err = assistant.HandleText(context.Background(), "/confirm cal_TEST")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != "Calendar event deleted:\nID: evt_1" || calendar.deletedID != "evt_1" {
		t.Fatalf("unexpected confirm result %q, deleted id %q", got, calendar.deletedID)
	}
}

func TestHandleTextCancelPendingCalendarAction(t *testing.T) {
	calendar := &mockCalendar{}
	assistant := testCalendarAssistant(t, calendar)

	if _, err := assistant.HandleText(context.Background(), "/calendar delete evt_1"); err != nil {
		t.Fatalf("expected proposal, got %v", err)
	}

	got, err := assistant.HandleText(context.Background(), "/cancel cal_TEST")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Cancelled pending action cal_TEST." {
		t.Fatalf("unexpected cancel response: %q", got)
	}

	got, err = assistant.HandleText(context.Background(), "/confirm cal_TEST")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "No pending action found for token cal_TEST." {
		t.Fatalf("unexpected confirm response: %q", got)
	}
}

func TestHandleTextPendingCalendarActions(t *testing.T) {
	calendar := &mockCalendar{}
	assistant := testCalendarAssistant(t, calendar)

	if _, err := assistant.HandleText(context.Background(), "/calendar delete evt_1"); err != nil {
		t.Fatalf("expected proposal, got %v", err)
	}

	got, err := assistant.HandleText(context.Background(), "/pending")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(got, "cal_TEST delete event evt_1") {
		t.Fatalf("unexpected pending response: %q", got)
	}
}

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

type mockLLM struct {
	answer string
	err    error
}

func (m mockLLM) Ask(ctx context.Context, prompt string) (string, error) {
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

type mockIntentParser struct {
	intent Intent
	err    error
}

func (m mockIntentParser) ParseIntent(ctx context.Context, req IntentRequest) (Intent, error) {
	return m.intent, m.err
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
