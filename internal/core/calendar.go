package core

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Calendar interface {
	ListEvents(ctx context.Context, query CalendarQuery) ([]CalendarEvent, error)
	CreateEvent(ctx context.Context, draft CalendarEventDraft) (CalendarEvent, error)
	DeleteEvent(ctx context.Context, eventID string) error
}

type CalendarQuery struct {
	Start time.Time
	End   time.Time
}

type CalendarEvent struct {
	ID          string
	Title       string
	Start       time.Time
	End         time.Time
	Location    string
	Description string
	AllDay      bool
}

type CalendarEventDraft struct {
	Title       string
	Start       time.Time
	End         time.Time
	Location    string
	Description string
}

type pendingCalendarAction struct {
	Token     string
	Kind      string
	Draft     CalendarEventDraft
	EventID   string
	ExpiresAt time.Time
}

type pendingCalendarStore struct {
	mu      sync.Mutex
	actions map[string]pendingCalendarAction
}

func newPendingCalendarStore() *pendingCalendarStore {
	return &pendingCalendarStore{
		actions: make(map[string]pendingCalendarAction),
	}
}

func (s *pendingCalendarStore) put(action pendingCalendarAction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actions[action.Token] = action
}

func (s *pendingCalendarStore) get(token string, now time.Time) (pendingCalendarAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action, ok := s.actions[token]
	if !ok {
		return pendingCalendarAction{}, false
	}

	if now.After(action.ExpiresAt) {
		delete(s.actions, token)
		return pendingCalendarAction{}, false
	}

	return action, true
}

func (s *pendingCalendarStore) delete(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.actions[token]; !ok {
		return false
	}

	delete(s.actions, token)
	return true
}

func (s *pendingCalendarStore) list(now time.Time) []pendingCalendarAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := make([]pendingCalendarAction, 0, len(s.actions))
	for token, action := range s.actions {
		if now.After(action.ExpiresAt) {
			delete(s.actions, token)
			continue
		}

		actions = append(actions, action)
	}

	return actions
}

func (a *Assistant) handleCalendar(ctx context.Context, text string) (string, error) {
	if a.calendar == nil {
		return "Calendar is not configured.", nil
	}

	arg := strings.TrimSpace(strings.TrimPrefix(text, "/calendar"))
	if arg == "" {
		return calendarUsage(), nil
	}

	switch {
	case arg == "today":
		return a.listCalendar(ctx, "today", startOfDay(a.now(), a.location), 1)

	case arg == "tomorrow":
		start := startOfDay(a.now().AddDate(0, 0, 1), a.location)
		return a.listCalendar(ctx, "tomorrow", start, 1)

	case arg == "week":
		return a.listCalendar(ctx, "next 7 days", startOfDay(a.now(), a.location), 7)

	case strings.HasPrefix(arg, "create "):
		return a.proposeCalendarCreate(strings.TrimSpace(strings.TrimPrefix(arg, "create ")))

	case strings.HasPrefix(arg, "delete "):
		return a.proposeCalendarDelete(strings.TrimSpace(strings.TrimPrefix(arg, "delete ")))

	default:
		return calendarUsage(), nil
	}
}

func (a *Assistant) listCalendar(ctx context.Context, label string, start time.Time, days int) (string, error) {
	end := start.AddDate(0, 0, days)

	events, err := a.calendar.ListEvents(ctx, CalendarQuery{Start: start, End: end})
	if err != nil {
		return "", err
	}

	if len(events) == 0 {
		return "Calendar " + label + ":\nNo events.", nil
	}

	var b strings.Builder
	b.WriteString("Calendar ")
	b.WriteString(label)
	b.WriteString(":\n")

	for _, event := range events {
		b.WriteString("- ")
		b.WriteString(formatEventTime(event))
		b.WriteString(" ")
		b.WriteString(nonEmpty(event.Title, "(untitled)"))
		if event.Location != "" {
			b.WriteString(" @ ")
			b.WriteString(event.Location)
		}
		if event.ID != "" {
			b.WriteString(" [id: ")
			b.WriteString(event.ID)
			b.WriteString("]")
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) proposeCalendarCreate(input string) (string, error) {
	draft, err := parseCalendarCreate(input, a.location)
	if err != nil {
		return err.Error(), nil
	}

	return a.proposeCalendarCreateDraft(draft)
}

func (a *Assistant) proposeCalendarCreateDraft(draft CalendarEventDraft) (string, error) {
	if a.calendar == nil {
		return "Calendar is not configured.", nil
	}

	draft.Title = strings.TrimSpace(draft.Title)
	if draft.Title == "" {
		return "Calendar event title is required.", nil
	}
	if draft.Start.IsZero() {
		return "Calendar event start time is required.", nil
	}
	if draft.End.IsZero() {
		draft.End = draft.Start.Add(time.Hour)
	}
	if !draft.End.After(draft.Start) {
		return "Calendar event end time must be after start time.", nil
	}

	token, err := a.newCalendarToken()
	if err != nil {
		return "", err
	}

	action := pendingCalendarAction{
		Token:     token,
		Kind:      "create",
		Draft:     draft,
		ExpiresAt: a.now().Add(a.pendingTTL),
	}
	a.pendingCalendar.put(action)

	return formatCreateProposal(action), nil
}

func (a *Assistant) proposeCalendarDelete(eventID string) (string, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "Usage: /calendar delete <event_id>", nil
	}

	token, err := a.newCalendarToken()
	if err != nil {
		return "", err
	}

	action := pendingCalendarAction{
		Token:     token,
		Kind:      "delete",
		EventID:   eventID,
		ExpiresAt: a.now().Add(a.pendingTTL),
	}
	a.pendingCalendar.put(action)

	return formatDeleteProposal(action), nil
}

func (a *Assistant) handleConfirm(ctx context.Context, token string) (string, error) {
	if a.calendar == nil {
		return "Calendar is not configured.", nil
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "Usage: /confirm <token>", nil
	}

	action, ok := a.pendingCalendar.get(token, a.now())
	if !ok {
		return "No pending action found for token " + token + ".", nil
	}

	switch action.Kind {
	case "create":
		event, err := a.calendar.CreateEvent(ctx, action.Draft)
		if err != nil {
			return "", err
		}
		a.pendingCalendar.delete(token)
		return "Calendar event created:\n" + formatEventDetail(event), nil

	case "delete":
		if err := a.calendar.DeleteEvent(ctx, action.EventID); err != nil {
			return "", err
		}
		a.pendingCalendar.delete(token)
		return "Calendar event deleted:\nID: " + action.EventID, nil

	default:
		return "", errors.New("unknown pending calendar action kind: " + action.Kind)
	}
}

func (a *Assistant) handleCancel(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "Usage: /cancel <token>", nil
	}

	if !a.pendingCalendar.delete(token) {
		return "No pending action found for token " + token + ".", nil
	}

	return "Cancelled pending action " + token + ".", nil
}

func (a *Assistant) handlePending() (string, error) {
	actions := a.pendingCalendar.list(a.now())
	if len(actions) == 0 {
		return "No pending actions.", nil
	}

	var b strings.Builder
	b.WriteString("Pending actions:\n")

	for _, action := range actions {
		b.WriteString("- ")
		b.WriteString(action.Token)
		b.WriteString(" ")
		b.WriteString(action.Kind)
		if action.Kind == "create" {
			b.WriteString(" ")
			b.WriteString(action.Draft.Title)
			b.WriteString(" ")
			b.WriteString(formatTime(action.Draft.Start))
		}
		if action.Kind == "delete" {
			b.WriteString(" event ")
			b.WriteString(action.EventID)
		}
		b.WriteString(" expires ")
		b.WriteString(formatTime(action.ExpiresAt))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func parseCalendarCreate(input string, loc *time.Location) (CalendarEventDraft, error) {
	parts := splitPipe(input)
	if len(parts) < 3 {
		return CalendarEventDraft{}, errors.New("Usage: /calendar create <title> | <start> | <end> [| location] [| description]")
	}

	title := strings.TrimSpace(parts[0])
	if title == "" {
		return CalendarEventDraft{}, errors.New("Calendar event title is required.")
	}

	start, err := parseUserTime(parts[1], loc)
	if err != nil {
		return CalendarEventDraft{}, fmt.Errorf("Invalid start time: %w", err)
	}

	end, err := parseUserTime(parts[2], loc)
	if err != nil {
		return CalendarEventDraft{}, fmt.Errorf("Invalid end time: %w", err)
	}

	if !end.After(start) {
		return CalendarEventDraft{}, errors.New("Calendar event end time must be after start time.")
	}

	draft := CalendarEventDraft{
		Title: strings.TrimSpace(title),
		Start: start,
		End:   end,
	}

	if len(parts) > 3 {
		draft.Location = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		draft.Description = strings.TrimSpace(parts[4])
	}

	return draft, nil
}

func splitPipe(input string) []string {
	raw := strings.Split(input, "|")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

func parseUserTime(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}

	var lastErr error
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				return parsed, nil
			}
			lastErr = err
			continue
		}

		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}

func (a *Assistant) newCalendarToken() (string, error) {
	return a.tokenGenerator("cal_")
}

func randomToken(prefix string) (string, error) {
	var buf [5]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	token := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:])
	return prefix + strings.ToUpper(token), nil
}

func calendarUsage() string {
	return "Calendar commands:\n/calendar today\n/calendar tomorrow\n/calendar week\n/calendar create <title> | <start> | <end> [| location] [| description]\n/calendar delete <event_id>\n/pending\n/confirm <token>\n/cancel <token>"
}

func formatCreateProposal(action pendingCalendarAction) string {
	var b strings.Builder
	b.WriteString("Proposed calendar action:\nCreate event\nToken: ")
	b.WriteString(action.Token)
	b.WriteString("\nTitle: ")
	b.WriteString(action.Draft.Title)
	b.WriteString("\nStart: ")
	b.WriteString(formatTime(action.Draft.Start))
	b.WriteString("\nEnd: ")
	b.WriteString(formatTime(action.Draft.End))
	if action.Draft.Location != "" {
		b.WriteString("\nLocation: ")
		b.WriteString(action.Draft.Location)
	}
	if action.Draft.Description != "" {
		b.WriteString("\nDescription: ")
		b.WriteString(action.Draft.Description)
	}
	b.WriteString("\n\nConfirm with:\n/confirm ")
	b.WriteString(action.Token)
	return b.String()
}

func formatDeleteProposal(action pendingCalendarAction) string {
	return "Proposed calendar action:\nDelete event\nToken: " + action.Token + "\nEvent ID: " + action.EventID + "\n\nConfirm with:\n/confirm " + action.Token
}

func formatEventDetail(event CalendarEvent) string {
	var b strings.Builder
	b.WriteString("ID: ")
	b.WriteString(event.ID)
	b.WriteString("\nTitle: ")
	b.WriteString(nonEmpty(event.Title, "(untitled)"))
	b.WriteString("\nStart: ")
	b.WriteString(formatTime(event.Start))
	b.WriteString("\nEnd: ")
	b.WriteString(formatTime(event.End))
	if event.Location != "" {
		b.WriteString("\nLocation: ")
		b.WriteString(event.Location)
	}
	return b.String()
}

func formatEventTime(event CalendarEvent) string {
	if event.AllDay {
		return event.Start.Format("2006-01-02")
	}

	return event.Start.Format("2006-01-02 15:04") + "-" + event.End.Format("15:04")
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
