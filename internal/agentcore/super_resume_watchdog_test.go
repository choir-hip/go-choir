package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Resume watchdog: a reactivated persistent Super run that never dispatches its
// recovery occurrence must be terminalized past the dispatch deadline, or it
// occupies the I26 singleton slot forever while doing no work (2026-09-03
// fe92ea2b hang). These tests pin the hang predicate and the fail-closed
// sweep, including the scope guard that keeps Researcher reactivations out.

func watchdogSuperRun(ownerID, runID string, state types.RunState, updatedAt time.Time, flagged bool) types.RunRecord {
	md := map[string]any{
		runMetadataAgentProfile: "super",
		runMetadataAgentRole:    "super",
	}
	if flagged {
		md["actor_reactivated_from_passivated"] = true
	}
	return types.RunRecord{
		RunID: runID, OwnerID: ownerID, AgentID: persistentSuperAgentID(ownerID),
		AgentProfile: "super", AgentRole: "super", State: state,
		CreatedAt: updatedAt, UpdatedAt: updatedAt, Metadata: md,
	}
}

func TestReactivatedSuperResumeExpired(t *testing.T) {
	ownerID := "owner-resume-watchdog"
	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	queued := []struct {
		name string
		rec  *types.RunRecord
		want bool
	}{
		{"expired flagged pending super run", ptr(watchdogSuperRun(ownerID, "r-expired", types.RunPending, old, true)), true},
		{"fresh flagged pending super run", ptr(watchdogSuperRun(ownerID, "r-fresh", types.RunPending, now, true)), false},
		{"running flagged super run", ptr(watchdogSuperRun(ownerID, "r-running", types.RunRunning, old, true)), false},
		{"pending unflagged super run", ptr(watchdogSuperRun(ownerID, "r-unflagged", types.RunPending, old, false)), false},
		{"completed flagged super run", ptr(watchdogSuperRun(ownerID, "r-done", types.RunCompleted, old, true)), false},
		{"zero updatedAt flagged pending super run", ptr(watchdogSuperRun(ownerID, "r-zero", types.RunPending, time.Time{}, true)), true},
		{"nil run", nil, false},
	}
	for _, tc := range queued {
		if got := reactivatedSuperResumeExpired(tc.rec, now); got != tc.want {
			t.Errorf("%s: expired=%t, want %t", tc.name, got, tc.want)
		}
	}

	// Scope guard: the Researcher injection-recovery path shares the flag.
	tex := watchdogSuperRun(ownerID, "r-texture", types.RunPending, old, true)
	tex.AgentID = "texture:doc-1"
	tex.AgentProfile = "texture"
	tex.AgentRole = "texture"
	if reactivatedSuperResumeExpired(&tex, now) {
		t.Error("flagged pending texture run must never expire under the Super watchdog")
	}
}

func ptr[T any](v T) *T { return &v }

func TestFailExpiredReactivatedSuperResume(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-resume-watchdog-fail"
	if _, err := rt.EnsurePersistentSuperAgent(ctx, ownerID); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-time.Hour)

	stuck := watchdogSuperRun(ownerID, "run-watchdog-stuck", types.RunPending, old, true)
	stuck.ComputerID = rt.TextureComputerID()
	if err := s.CreateRun(ctx, stuck); err != nil {
		t.Fatalf("create stuck run: %v", err)
	}
	failed, err := rt.failExpiredReactivatedSuperResume(ctx, ownerID, stuck.RunID, time.Now().UTC())
	if err != nil || !failed {
		t.Fatalf("expected stuck run failed: failed=%t err=%v", failed, err)
	}
	reloaded, err := s.GetRunByOwner(ctx, ownerID, stuck.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.State != types.RunFailed {
		t.Errorf("stuck run state=%s, want failed", reloaded.State)
	}
	if reloaded.FinishedAt == nil {
		t.Error("stuck run missing FinishedAt")
	}
	if !strings.Contains(reloaded.Error, "slot released") {
		t.Errorf("stuck run error missing slot-release reason: %q", reloaded.Error)
	}
	if !metadataBoolValue(reloaded.Metadata, resumeWatchdogFiredMetadata) {
		t.Error("stuck run missing resume watchdog metadata mark")
	}

	// A freshly reactivated run is spared.
	fresh := watchdogSuperRun(ownerID, "run-watchdog-fresh", types.RunPending, time.Now().UTC(), true)
	fresh.ComputerID = rt.TextureComputerID()
	if err := s.CreateRun(ctx, fresh); err != nil {
		t.Fatalf("create fresh run: %v", err)
	}
	failed, err = rt.failExpiredReactivatedSuperResume(ctx, ownerID, fresh.RunID, time.Now().UTC())
	if err != nil || failed {
		t.Fatalf("fresh run must be spared: failed=%t err=%v", failed, err)
	}

	// Missing run is a no-op, not an error.
	failed, err = rt.failExpiredReactivatedSuperResume(ctx, ownerID, "run-does-not-exist", time.Now().UTC())
	if err != nil || failed {
		t.Fatalf("missing run must be a no-op: failed=%t err=%v", failed, err)
	}
}
