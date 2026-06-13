package storage

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
)

func (s *PostgresMemoryStore) RecordAuditEvent(ctx context.Context, event core.AuditEvent) error {
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO audit_events (occurred_at, actor, source, action_type, risk_level, decision, resource_type, resource_id, summary, metadata, result, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)
`, occurredAt, nonEmpty(event.Actor, core.ActorSystem), nonEmpty(event.Source, "core"), strings.TrimSpace(event.ActionType), nonEmpty(event.RiskLevel, core.RiskHigh), nonEmpty(event.Decision, core.DecisionDeny), strings.TrimSpace(event.ResourceType), strings.TrimSpace(event.ResourceID), strings.TrimSpace(event.Summary), string(metadataJSON), strings.TrimSpace(event.Result), strings.TrimSpace(event.Error))
	return err
}
