package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func TestMaintenanceHoldRefusesRunAdmission(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	rt := &Runtime{}
	// The hold gate precedes the pre-genesis store check, so a bare Runtime is
	// enough: while held, admission is refused before any store mutation.
	_, err := rt.createRunWithMetadata(context.Background(), "p", "owner", nil)
	if err == nil || !strings.Contains(err.Error(), "maintenance hold") {
		t.Fatalf("expected maintenance-hold admission refusal, got %v", err)
	}
}

func TestMaintenanceHoldRefusesSelfDevelopmentRun(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	h := &APIHandler{rt: &Runtime{}}
	_, err := h.ensureSelfDevelopmentRun(nil, selfdev.Operation{}, "owner", "prompt")
	if err == nil || !strings.Contains(err.Error(), "maintenance hold") {
		t.Fatalf("expected maintenance-hold self-development refusal, got %v", err)
	}
}
