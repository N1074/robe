package core

import "testing"

func TestDefaultPermissionEngineClassifiesActions(t *testing.T) {
	engine := DefaultPermissionEngine{}

	if got := engine.Decide(Action{Type: ActionMemoryCreate}); got.Decision != DecisionAllow || got.RiskLevel != RiskLow {
		t.Fatalf("unexpected memory create decision: %#v", got)
	}
	if got := engine.Decide(Action{Type: ActionCalendarCreate}); got.Decision != DecisionConfirm || got.RiskLevel != RiskHigh {
		t.Fatalf("unexpected calendar create decision: %#v", got)
	}
	if got := engine.Decide(Action{Type: "unknown"}); got.Decision != DecisionDeny || got.RiskLevel != RiskHigh {
		t.Fatalf("unexpected unknown action decision: %#v", got)
	}
}
