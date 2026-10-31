package agentcore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func round3Runtime(t *testing.T, computerID string) *Runtime {
	t.Helper()
	rt := telemetryRuntime(t, computerID)
	rt.bus = events.NewEventBus()
	rt.cfg.StorePath = filepath.Join(t.TempDir(), "store.db")
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: time.Hour}); err != nil {
		t.Fatal(err)
	}
	rt.store.SetTapeGroupCommit(true)
	return rt
}

func readMarker(t *testing.T, rt *Runtime) PlannedRestart {
	t.Helper()
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil {
		t.Fatal(err)
	}
	var m PlannedRestart
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Round 3 finding 1: the restore restart marker drains, stamps, and holds
// synchronous mode like the other writers.
func TestRestoreRestartMarkerDrainsAndStamps(t *testing.T) {
	rt := round3Runtime(t, "computer-restore-marker")
	if err := rt.writeRestoreRestartMarker("ckpt"); err != nil {
		t.Fatal(err)
	}
	m := readMarker(t, rt)
	if m.RunMemoryWindowOpen == nil || !*m.RunMemoryWindowOpen {
		t.Fatalf("restore marker window = %v, want stamped open", m.RunMemoryWindowOpen)
	}
	rt.hostRestartTelemetryMu.Lock()
	held := rt.hostRestartTelemetryRestore != nil
	rt.hostRestartTelemetryMu.Unlock()
	if !held {
		t.Fatal("the restore marker did not hold synchronous telemetry until the restart")
	}
	rt.clearRestoreRestartMarker()
	rt.hostRestartTelemetryMu.Lock()
	held = rt.hostRestartTelemetryRestore != nil
	rt.hostRestartTelemetryMu.Unlock()
	if held {
		t.Fatal("clearing the restore marker did not release the hold")
	}
}

// Round 3 finding 2: the owner flips the setting off during the drain; the
// stamp is the state before the drain.
func TestMarkerStampSurvivesAFlipDuringTheDrain(t *testing.T) {
	for _, writer := range []string{"mark", "host"} {
		t.Run(writer, func(t *testing.T) {
			rt := round3Runtime(t, "computer-flip-"+writer)
			rt.restartDrainHook = func() { rt.store.SetTapeGroupCommit(false) }
			if writer == "mark" {
				if _, err := rt.markPlannedRestart("platform update", "t"); err != nil {
					t.Fatal(err)
				}
			} else if _, err := rt.writeHostPlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: "platform update"}); err != nil {
				t.Fatal(err)
			}
			if rt.store.TapeGroupCommitOn() {
				t.Fatal("the hook did not flip the setting")
			}
			m := readMarker(t, rt)
			if m.RunMemoryWindowOpen == nil || !*m.RunMemoryWindowOpen {
				t.Fatalf("marker window after an on-to-off flip during the drain = %v, want open", m.RunMemoryWindowOpen)
			}
		})
	}
}

// Round 3 finding 4: a planned restart with a damaged run in the process
// writes a failed drain with the window open, so the boot closes runs.
func TestMarkerWithADamagedRunClosesAtBoot(t *testing.T) {
	rt := round3Runtime(t, "computer-damaged-marker")
	rt.store.SetTapeGroupCommit(false)
	rt.store.MarkRunMemoryDamaged("run-x")
	if _, err := rt.writeHostPlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: "platform update"}); err != nil {
		t.Fatal(err)
	}
	m := readMarker(t, rt)
	if !m.TelemetryDrainFailed || m.RunMemoryWindowOpen == nil || !*m.RunMemoryWindowOpen {
		t.Fatalf("marker with a damaged run = %+v, want drain failed and window open", m)
	}
}

// Round 3 finding 4: the provider path does not complete a damaged run.
func TestExecuteWithProviderFailsADamagedRun(t *testing.T) {
	rt := round3Runtime(t, "computer-provider-damaged")
	rt.provider = provider.NewStubProvider(0)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{RunID: "run-prov-damaged", AgentID: "research:prov", AgentProfile: agentprofile.Research,
		AgentRole: agentprofile.Research, OwnerID: "owner-telemetry", ComputerID: rt.cfg.ComputerID, State: types.RunRunning,
		Prompt: "work", CreatedAt: now, UpdatedAt: now}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rt.store.MarkRunMemoryDamaged(rec.RunID)
	rt.executeWithProvider(ctx, &rec, func(types.EventKind, string, json.RawMessage) {})
	got, err := rt.store.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != types.RunFailed {
		t.Fatalf("damaged provider-path run = %s, want failed", got.State)
	}
}
