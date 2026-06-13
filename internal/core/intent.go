package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	IntentNone           = "none"
	IntentAsk            = "ask"
	IntentCalendarList   = "calendar_list"
	IntentCalendarCreate = "calendar_create"
	IntentCalendarDelete = "calendar_delete"
	IntentMemoryCreate   = "create_memory"
	IntentEmailSearch    = "email_search"
	IntentEmailShow      = "email_show"
)

type IntentParser interface {
	ParseIntent(ctx context.Context, req IntentRequest) (Intent, error)
}

type IntentRequest struct {
	Text         string
	Now          time.Time
	Timezone     string
	ProjectHints []string
}

type Intent struct {
	Kind            string
	AskPrompt       string
	CalendarPeriod  string
	CalendarDraft   CalendarEventDraft
	CalendarEventID string
	MemoryDraft     Memory
	EmailQuery      string
	EmailMessageID  string
}

func (a *Assistant) handleNaturalText(ctx context.Context, text string) (string, error) {
	if strings.HasPrefix(text, "/") {
		return "Unknown command. Try /help.", nil
	}

	if a.intentParser == nil {
		return a.handleAsk(ctx, text)
	}

	intent, err := a.intentParser.ParseIntent(ctx, IntentRequest{
		Text:         text,
		Now:          a.now(),
		Timezone:     a.location.String(),
		ProjectHints: projectHintList(a.projectAliases),
	})
	if err != nil {
		return a.handleAsk(ctx, text)
	}

	return a.handleIntent(ctx, intent, text)
}

func (a *Assistant) handleIntent(ctx context.Context, intent Intent, originalText string) (string, error) {
	switch strings.TrimSpace(intent.Kind) {
	case IntentCalendarList:
		return a.handleCalendarListIntent(ctx, intent.CalendarPeriod)

	case IntentCalendarCreate:
		return a.proposeCalendarCreateDraft(ctx, intent.CalendarDraft)

	case IntentCalendarDelete:
		return a.proposeCalendarDelete(ctx, intent.CalendarEventID)

	case IntentMemoryCreate:
		return a.handleMemoryCreateIntent(ctx, intent.MemoryDraft, originalText)

	case IntentEmailSearch:
		query := strings.TrimSpace(intent.EmailQuery)
		if query == "" {
			query = originalText
		}
		return a.searchEmail(ctx, query)

	case IntentEmailShow:
		return a.showEmail(ctx, intent.EmailMessageID)

	case IntentAsk, IntentNone, "":
		prompt := strings.TrimSpace(intent.AskPrompt)
		if prompt == "" {
			prompt = originalText
		}
		return a.handleAsk(ctx, prompt)

	default:
		return a.handleAsk(ctx, originalText)
	}
}

func (a *Assistant) handleMemoryCreateIntent(ctx context.Context, draft Memory, originalText string) (string, error) {
	if err := a.validateMemoryProposal(originalText, draft); err != nil {
		if hasExplicitMemorySignal(originalText) {
			action := memoryAction(ActionMemoryCreate, draft)
			action.Actor = ActorLLM
			action.Source = "telegram/llm"
			a.recordAudit(ctx, action, PermissionDecision{RiskLevel: RiskLow, Decision: DecisionDeny, Reason: err.Error()}, AuditResultRejected, err)
			return "Memory was not saved: " + err.Error(), nil
		}
		return a.handleAsk(ctx, originalText)
	}

	draft.Source = "telegram/llm"
	if draft.Project.Slug == "" {
		draft.Project.Slug = a.projectForText(originalText)
	}

	memory, err := a.createMemory(ctx, draft)
	if err != nil {
		return "", err
	}

	return "Memory saved:\n" + formatMemory(memory), nil
}

func (a *Assistant) handleCalendarListIntent(ctx context.Context, period string) (string, error) {
	if a.calendar == nil {
		return calendarNotConfiguredMessage(), nil
	}

	switch strings.TrimSpace(period) {
	case "today":
		return a.listCalendar(ctx, "today", startOfDay(a.now(), a.location), 1)
	case "tomorrow":
		start := startOfDay(a.now().AddDate(0, 0, 1), a.location)
		return a.listCalendar(ctx, "tomorrow", start, 1)
	case "week":
		return a.listCalendar(ctx, "next 7 days", startOfDay(a.now(), a.location), 7)
	default:
		return "", errors.New("unsupported calendar list period: " + period)
	}
}
