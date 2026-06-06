package core

import (
	"context"
	"strings"
	"time"
)

type LLM interface {
	Ask(ctx context.Context, prompt string) (string, error)
}

type Assistant struct {
	llm             LLM
	intentParser    IntentParser
	calendar        Calendar
	memory          MemoryStore
	activeProject   string
	status          Status
	location        *time.Location
	pendingTTL      time.Duration
	pendingCalendar *pendingCalendarStore
	now             func() time.Time
	tokenGenerator  func(prefix string) (string, error)
}

type Status struct {
	Env              string
	LLMProvider      string
	LLMModel         string
	AccessRestricted bool
	CalendarEnabled  bool
	VoiceEnabled     bool
	MemoryEnabled    bool
	Timezone         string
}

type AssistantOption func(*Assistant)

func NewAssistant(llm LLM, status Status, opts ...AssistantOption) *Assistant {
	location := time.Local
	if status.Timezone != "" {
		if loaded, err := time.LoadLocation(status.Timezone); err == nil {
			location = loaded
		} else {
			status.Timezone = location.String()
		}
	} else {
		status.Timezone = location.String()
	}

	assistant := &Assistant{
		llm:             llm,
		status:          status,
		location:        location,
		pendingTTL:      10 * time.Minute,
		pendingCalendar: newPendingCalendarStore(),
		now:             func() time.Time { return time.Now().In(location) },
		tokenGenerator:  randomToken,
	}

	if parser, ok := llm.(IntentParser); ok {
		assistant.intentParser = parser
	}

	for _, opt := range opts {
		opt(assistant)
	}

	return assistant
}

func WithCalendar(calendar Calendar) AssistantOption {
	return func(a *Assistant) {
		a.calendar = calendar
		a.status.CalendarEnabled = calendar != nil
	}
}

func WithMemory(memory MemoryStore) AssistantOption {
	return func(a *Assistant) {
		a.memory = memory
		a.status.MemoryEnabled = memory != nil
	}
}

func WithIntentParser(parser IntentParser) AssistantOption {
	return func(a *Assistant) {
		a.intentParser = parser
	}
}

func WithNow(now func() time.Time) AssistantOption {
	return func(a *Assistant) {
		if now != nil {
			a.now = now
		}
	}
}

func WithTokenGenerator(generator func(prefix string) (string, error)) AssistantOption {
	return func(a *Assistant) {
		if generator != nil {
			a.tokenGenerator = generator
		}
	}
}

func (a *Assistant) HandleText(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)

	switch {
	case text == "":
		return "Unknown command. Try /help.", nil

	case text == "/ping":
		return "pong", nil

	case text == "/start":
		return "Robe v0.1 online. Try /ping or /ask <question>.", nil

	case text == "/help":
		return "Commands:\n/ping\n/status\n/ask <question>\n/remember <text>\n/memories <query>\n/project list|create|use|status\n/calendar today|tomorrow|week\n/calendar create <title> | <start> | <end> [| location] [| description]\n/calendar delete <event_id>\n/pending\n/confirm <token>\n/cancel <token>", nil

	case text == "/status":
		return a.renderStatus(), nil

	case text == "/ask" || strings.HasPrefix(text, "/ask "):
		return a.handleAsk(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/ask")))

	case text == "/remember" || strings.HasPrefix(text, "/remember "):
		return a.handleRemember(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/remember")))

	case text == "/memories" || strings.HasPrefix(text, "/memories "):
		return a.handleMemories(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/memories")))

	case text == "/project" || strings.HasPrefix(text, "/project "):
		return a.handleProject(ctx, text)

	case text == "/calendar" || strings.HasPrefix(text, "/calendar "):
		return a.handleCalendar(ctx, text)

	case text == "/pending":
		return a.handlePending()

	case text == "/confirm" || strings.HasPrefix(text, "/confirm "):
		return a.handleConfirm(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/confirm")))

	case text == "/cancel" || strings.HasPrefix(text, "/cancel "):
		return a.handleCancel(strings.TrimSpace(strings.TrimPrefix(text, "/cancel")))

	default:
		return a.handleNaturalText(ctx, text)
	}
}

func (a *Assistant) renderStatus() string {
	env := strings.TrimSpace(a.status.Env)
	if env == "" {
		env = "unknown"
	}

	provider := strings.TrimSpace(a.status.LLMProvider)
	if provider == "" {
		provider = "unknown"
	}

	model := strings.TrimSpace(a.status.LLMModel)
	if model == "" {
		model = "unknown"
	}

	access := "restricted"
	if !a.status.AccessRestricted {
		access = "setup-open"
	}

	calendar := "disabled"
	if a.status.CalendarEnabled {
		calendar = "enabled"
	}

	voice := "disabled"
	if a.status.VoiceEnabled {
		voice = "enabled"
	}

	memory := "disabled"
	if a.status.MemoryEnabled {
		memory = "enabled"
	}

	timezone := strings.TrimSpace(a.status.Timezone)
	if timezone == "" {
		timezone = a.location.String()
	}

	project := a.activeProject
	if project == "" {
		project = "global"
	}

	return "Robe v0.1 online.\nEnv: " + env + "\nLLM: " + provider + "/" + model + "\nAccess: " + access + "\nCalendar: " + calendar + "\nVoice: " + voice + "\nMemory: " + memory + "\nProject: " + project + "\nTimezone: " + timezone
}

func (a *Assistant) handleAsk(ctx context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "Usage: /ask <question>", nil
	}

	if a.llm == nil {
		return "LLM is not configured.", nil
	}

	return a.llm.Ask(ctx, prompt)
}
