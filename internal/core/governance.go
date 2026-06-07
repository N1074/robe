package core

import (
	"context"
	"strings"
	"time"
)

const (
	ActionMemoryCreate   = "memory.create"
	ActionMemoryArchive  = "memory.archive"
	ActionMemoryTag      = "memory.tag"
	ActionCalendarCreate = "calendar.create"
	ActionCalendarDelete = "calendar.delete"

	ResourceMemory   = "memory"
	ResourceCalendar = "calendar"

	ActorUser       = "user"
	ActorLLM        = "llm"
	ActorSystem     = "system"
	ActorAutomation = "automation"

	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"

	DecisionAllow   = "allow"
	DecisionConfirm = "confirm"
	DecisionDeny    = "deny"

	AuditResultProposed  = "proposed"
	AuditResultExecuted  = "executed"
	AuditResultCancelled = "cancelled"
	AuditResultRejected  = "rejected"
	AuditResultFailed    = "failed"
)

type Action struct {
	Type         string
	Actor        string
	Source       string
	ResourceType string
	ResourceID   string
	Summary      string
	Metadata     map[string]string
}

type PermissionDecision struct {
	RiskLevel string
	Decision  string
	Reason    string
}

type PermissionEngine interface {
	Decide(action Action) PermissionDecision
}

type AuditLogger interface {
	RecordAuditEvent(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	OccurredAt   time.Time
	Actor        string
	Source       string
	ActionType   string
	RiskLevel    string
	Decision     string
	ResourceType string
	ResourceID   string
	Summary      string
	Metadata     map[string]string
	Result       string
	Error        string
}

type DefaultPermissionEngine struct{}

func (DefaultPermissionEngine) Decide(action Action) PermissionDecision {
	switch strings.TrimSpace(action.Type) {
	case ActionMemoryCreate:
		return PermissionDecision{RiskLevel: RiskLow, Decision: DecisionAllow, Reason: "explicit memory creation is a local reversible write"}
	case ActionMemoryArchive, ActionMemoryTag:
		return PermissionDecision{RiskLevel: RiskMedium, Decision: DecisionAllow, Reason: "memory curation is a local reversible write"}
	case ActionCalendarCreate, ActionCalendarDelete:
		return PermissionDecision{RiskLevel: RiskHigh, Decision: DecisionConfirm, Reason: "calendar writes are external side effects"}
	default:
		return PermissionDecision{RiskLevel: RiskHigh, Decision: DecisionDeny, Reason: "unknown action type"}
	}
}

func WithPermissionEngine(engine PermissionEngine) AssistantOption {
	return func(a *Assistant) {
		if engine != nil {
			a.permissions = engine
		}
	}
}

func WithAuditLogger(logger AuditLogger) AssistantOption {
	return func(a *Assistant) {
		a.audit = logger
	}
}

func (a *Assistant) decide(action Action) PermissionDecision {
	engine := a.permissions
	if engine == nil {
		engine = DefaultPermissionEngine{}
	}
	return engine.Decide(action)
}

func (a *Assistant) recordAudit(ctx context.Context, action Action, decision PermissionDecision, result string, err error) {
	if a.audit == nil {
		return
	}

	event := AuditEvent{
		OccurredAt:   a.now(),
		Actor:        nonEmpty(action.Actor, ActorUser),
		Source:       nonEmpty(action.Source, "telegram"),
		ActionType:   strings.TrimSpace(action.Type),
		RiskLevel:    nonEmpty(decision.RiskLevel, RiskHigh),
		Decision:     nonEmpty(decision.Decision, DecisionDeny),
		ResourceType: strings.TrimSpace(action.ResourceType),
		ResourceID:   strings.TrimSpace(action.ResourceID),
		Summary:      strings.TrimSpace(action.Summary),
		Metadata:     action.Metadata,
		Result:       strings.TrimSpace(result),
	}
	if err != nil {
		event.Error = err.Error()
		if event.Result == "" {
			event.Result = AuditResultFailed
		}
	}
	_ = a.audit.RecordAuditEvent(ctx, event)
}

func memoryAction(actionType string, memory Memory) Action {
	actor := ActorUser
	if strings.Contains(memory.Source, "/llm") {
		actor = ActorLLM
	}
	return Action{
		Type:         actionType,
		Actor:        actor,
		Source:       nonEmpty(memory.Source, "telegram"),
		ResourceType: ResourceMemory,
		ResourceID:   strings.TrimSpace(memory.ID),
		Summary:      "memory " + strings.TrimPrefix(actionType, "memory.") + " " + normalizeMemoryKind(memory.Kind),
		Metadata: map[string]string{
			"kind":       normalizeMemoryKind(memory.Kind),
			"project":    nonEmpty(memory.Project.Slug, "global"),
			"importance": importanceLabel(memory.Importance),
		},
	}
}

func calendarAction(actionType string, token string, eventID string) Action {
	metadata := map[string]string{
		"token": strings.TrimSpace(token),
	}
	if eventID != "" {
		metadata["event_id"] = strings.TrimSpace(eventID)
	}
	return Action{
		Type:         actionType,
		Actor:        ActorUser,
		Source:       "telegram",
		ResourceType: ResourceCalendar,
		ResourceID:   strings.TrimSpace(eventID),
		Summary:      "calendar " + strings.TrimPrefix(actionType, "calendar."),
		Metadata:     metadata,
	}
}
