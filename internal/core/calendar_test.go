package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestHandleTextCalendarDisabled(t *testing.T) {
	assistant := NewAssistant(nil, Status{})

	got, err := assistant.HandleText(context.Background(), "/calendar today")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got != calendarNotConfiguredMessage() {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestCalendarNotConfiguredMessageIsActionable(t *testing.T) {
	got := calendarNotConfiguredMessage()
	if !strings.Contains(got, "CALENDAR_PROVIDER=google") || !strings.Contains(got, "Google OAuth") {
		t.Fatalf("message is not actionable: %q", got)
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

func TestHandleTextCalendarCreateAuditsProposalAndExecution(t *testing.T) {
	calendar := &mockCalendar{}
	audit := &mockAuditLogger{}
	assistant := testCalendarAssistant(t, calendar, WithAuditLogger(audit))

	if _, err := assistant.HandleText(context.Background(), "/calendar create Dentist | 2026-06-07 10:00 | 2026-06-07 11:00"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected proposal audit event, got %d", len(audit.events))
	}
	if audit.events[0].ActionType != ActionCalendarCreate || audit.events[0].Decision != DecisionConfirm || audit.events[0].Result != AuditResultProposed {
		t.Fatalf("unexpected proposal audit event: %#v", audit.events[0])
	}

	if _, err := assistant.HandleText(context.Background(), "/confirm cal_TEST"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(audit.events) != 2 {
		t.Fatalf("expected execution audit event, got %d", len(audit.events))
	}
	if audit.events[1].ActionType != ActionCalendarCreate || audit.events[1].Result != AuditResultExecuted || audit.events[1].ResourceID != "created_1" {
		t.Fatalf("unexpected execution audit event: %#v", audit.events[1])
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
	audit := &mockAuditLogger{}
	assistant := testCalendarAssistant(t, calendar, WithAuditLogger(audit))

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
	if len(audit.events) != 2 || audit.events[1].Result != AuditResultCancelled {
		t.Fatalf("expected cancellation audit event, got %#v", audit.events)
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
