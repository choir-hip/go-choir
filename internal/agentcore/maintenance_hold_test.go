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

func TestMaintenanceHoldSkipsRuntimeStartRewake(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	rt := &Runtime{}
	// Held Start short-circuits before the passivate/rewake sweeps; it must not
	// panic and must not reach the started log's autoputer access.
	rt.Start(context.Background())
}

func TestMaintenanceHoldRefusesSelfDevelopmentRun(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	h := &APIHandler{rt: &Runtime{}}
	_, err := h.ensureSelfDevelopmentRun(nil, selfdev.Operation{}, "owner", "prompt")
	if err == nil || !strings.Contains(err.Error(), "maintenance hold") {
		t.Fatalf("expected maintenance-hold self-development refusal, got %v", err)
	}
}

func TestMaintenanceHoldOffWhenEnvUnset(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "0")
	rt := &Runtime{}
	if rt.maintenanceHeld() {
		t.Fatal("maintenanceHeld should be false when RUNTIME_MAINTENANCE_HOLD != 1")
	}
}
