package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendarapi "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GoogleConfig struct {
	CredentialsFile string
	TokenFile       string
	CalendarID      string
}

type GoogleCalendar struct {
	service    *calendarapi.Service
	calendarID string
}

func NewGoogleCalendar(ctx context.Context, cfg GoogleConfig) (*GoogleCalendar, error) {
	cfg = normalizeGoogleConfig(cfg)
	if cfg.CredentialsFile == "" {
		return nil, errors.New("calendar credentials file is required")
	}
	if cfg.TokenFile == "" {
		return nil, errors.New("calendar token file is required")
	}

	oauthConfig, err := loadOAuthConfig(cfg.CredentialsFile)
	if err != nil {
		return nil, err
	}

	token, err := loadToken(cfg.TokenFile)
	if err != nil {
		return nil, err
	}

	client := oauthConfig.Client(ctx, token)
	service, err := calendarapi.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &GoogleCalendar{
		service:    service,
		calendarID: cfg.CalendarID,
	}, nil
}

func AuthURL(credentialsFile string) (string, error) {
	oauthConfig, err := loadOAuthConfig(credentialsFile)
	if err != nil {
		return "", err
	}

	return oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func ExchangeCode(ctx context.Context, credentialsFile string, tokenFile string, code string) error {
	oauthConfig, err := loadOAuthConfig(credentialsFile)
	if err != nil {
		return err
	}

	token, err := oauthConfig.Exchange(ctx, strings.TrimSpace(code))
	if err != nil {
		return err
	}

	return saveToken(tokenFile, token)
}

func (g *GoogleCalendar) ListEvents(ctx context.Context, query core.CalendarQuery) ([]core.CalendarEvent, error) {
	events, err := g.service.Events.List(g.calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(query.Start.Format(time.RFC3339)).
		TimeMax(query.End.Format(time.RFC3339)).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	out := make([]core.CalendarEvent, 0, len(events.Items))
	for _, item := range events.Items {
		event, err := convertGoogleEvent(item)
		if err != nil {
			continue
		}
		out = append(out, event)
	}

	return out, nil
}

func (g *GoogleCalendar) CreateEvent(ctx context.Context, draft core.CalendarEventDraft) (core.CalendarEvent, error) {
	item := &calendarapi.Event{
		Summary:     draft.Title,
		Location:    draft.Location,
		Description: draft.Description,
		Start: &calendarapi.EventDateTime{
			DateTime: draft.Start.Format(time.RFC3339),
		},
		End: &calendarapi.EventDateTime{
			DateTime: draft.End.Format(time.RFC3339),
		},
	}

	created, err := g.service.Events.Insert(g.calendarID, item).Context(ctx).Do()
	if err != nil {
		return core.CalendarEvent{}, err
	}

	return convertGoogleEvent(created)
}

func (g *GoogleCalendar) DeleteEvent(ctx context.Context, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return errors.New("event id is required")
	}

	return g.service.Events.Delete(g.calendarID, eventID).Context(ctx).Do()
}

func normalizeGoogleConfig(cfg GoogleConfig) GoogleConfig {
	cfg.CredentialsFile = strings.TrimSpace(cfg.CredentialsFile)
	cfg.TokenFile = strings.TrimSpace(cfg.TokenFile)
	cfg.CalendarID = strings.TrimSpace(cfg.CalendarID)
	if cfg.CalendarID == "" {
		cfg.CalendarID = "primary"
	}
	return cfg
}

func loadOAuthConfig(credentialsFile string) (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	return google.ConfigFromJSON(data, calendarapi.CalendarScope)
}

func loadToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

func convertGoogleEvent(item *calendarapi.Event) (core.CalendarEvent, error) {
	if item == nil {
		return core.CalendarEvent{}, errors.New("google calendar event is nil")
	}

	start, allDay, err := parseGoogleEventTime(item.Start)
	if err != nil {
		return core.CalendarEvent{}, fmt.Errorf("event %q start: %w", item.Id, err)
	}

	end, _, err := parseGoogleEventTime(item.End)
	if err != nil {
		return core.CalendarEvent{}, fmt.Errorf("event %q end: %w", item.Id, err)
	}

	return core.CalendarEvent{
		ID:          item.Id,
		Title:       item.Summary,
		Start:       start,
		End:         end,
		Location:    item.Location,
		Description: item.Description,
		AllDay:      allDay,
	}, nil
}

func parseGoogleEventTime(value *calendarapi.EventDateTime) (time.Time, bool, error) {
	if value == nil {
		return time.Time{}, false, errors.New("missing event time")
	}

	if value.DateTime != "" {
		parsed, err := time.Parse(time.RFC3339, value.DateTime)
		return parsed, false, err
	}

	if value.Date != "" {
		parsed, err := time.Parse("2006-01-02", value.Date)
		return parsed, true, err
	}

	return time.Time{}, false, errors.New("missing event date")
}
