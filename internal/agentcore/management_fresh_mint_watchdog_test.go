package agentcore

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Fresh-mint dispatch watchdog: a persistent Management run minted pending whose
// initial_dispatch never executes strands forever — the live-occurrence bind
// branch codifies "locked mint already dispatched" and the reactivation resume
// watchdog covers only runs carrying the reactivation flag (2026-09-13 run
// 2454bdd4: minted during the boot rewarm window, pending 45 minutes with no
// dispatch log and no inference). These tests pin the strand predicate and the
// recovery-occurrence re-drive.

func TestFreshMintManagementResumeStranded(t *testing.T) {
	ownerID := "owner-fresh-mint-watchdog"
	now := time.Now().UTC()
	old := now.Add(-time.Hour)

	stranded := watchdogManagementRun(ownerID, "run-fresh-mint-old", types.RunPending, old, false)
	if !freshMintManagementResumeStranded(&stranded, now) {
		t.Error("pending fresh mint past the deadline must count as stranded")
	}

	// Within the dispatch window: not stranded yet.
	fresh := watchdogManagementRun(ownerID, "run-fresh-mint-new", types.RunPending, now.Add(-time.Minute), false)
	if freshMintManagementResumeStranded(&fresh, now) {
		t.Error("pending fresh mint inside the window must not count as stranded")
	}

	// Reactivated runs belong to the reactivation resume watchdog.
	flagged := watchdogManagementRun(ownerID, "run-fresh-mint-flagged", types.RunPending, old, true)
	if freshMintManagementResumeStranded(&flagged, now) {
		t.Error("reactivated-flagged run must be owned by the resume watchdog, not the fresh-mint watchdog")
	}

	// Terminal and running runs are never stranded.
	running := watchdogManagementRun(ownerID, "run-fresh-mint-running", types.RunRunning, old, false)
	if freshMintManagementResumeStranded(&running, now) {
		t.Error("running run must not count as stranded")
	}
	terminal := watchdogManagementRun(ownerID, "run-fresh-mint-terminal", types.RunFailed, old, false)
	if freshMintManagementResumeStranded(&terminal, now) {
		t.Error("terminal run must not count as stranded")
	}

	// Non-Management agents are out of scope, flagged or not.
	tex := watchdogManagementRun(ownerID, "run-fresh-mint-texture", types.RunPending, old, false)
	tex.AgentID = agentprofile.Texture + ":doc-1"
	tex.AgentProfile = agentprofile.Texture
	tex.AgentRole = agentprofile.Texture
	if freshMintManagementResumeStranded(&tex, now) {
		t.Error("non-Management run must never strand under the fresh-mint watchdog")
	}

	// Zero UpdatedAt counts as stranded (fail-closed slot release).
	zero := watchdogManagementRun(ownerID, "run-fresh-mint-zero", types.RunPending, time.Time{}, false)
	if !freshMintManagementResumeStranded(&zero, now) {
		t.Error("zero UpdatedAt pending fresh mint must count as stranded")
	}
}

func TestRedriveStrandedFreshMintManagement(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-fresh-mint-redrive"
	if _, err := rt.EnsurePersistentManagementAgent(ctx, ownerID); err != nil {
		t.Fatal(err)
	}
	managementAgent := persistentManagementAgentID(ownerID)

	fixture := seedTextureLifecycleControl(t, s, ownerID, "freshmint", managementAgent, agentprofile.Management)

	old := time.Now().UTC().Add(-time.Hour)
	stranded := types.RunRecord{
		RunID: "run-fresh-mint-stranded", OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		AgentID: managementAgent, AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		ChannelID: "doc-control-freshmint", State: types.RunPending,
		Metadata: map[string]any{
			runMetadataAgentProfile:    agentprofile.Management,
			runMetadataAgentRole:       agentprofile.Management,
			"assignment_trajectory_id": fixture.trajectoryID,
			"lifecycle_work_item_id":   fixture.workID,
			"work_item_ids":            []string{fixture.workID},
			"request_source":           "lifecycle_texture_control",
		},
		CreatedAt: old, UpdatedAt: old,
	}
	if err := s.CreateRun(ctx, stranded); err != nil {
		t.Fatalf("create stranded run: %v", err)
	}
	if _, err := rt.bindLifecycleControlsToRun(ctx, &stranded, []types.CoagentSourcePacket{fixture.control}); err != nil {
		t.Fatalf("bind control to stranded run: %v", err)
	}
	// The bind refreshes UpdatedAt; re-age to the stranded deadline window.
	stranded.UpdatedAt = old
	if err := s.UpdateRun(ctx, stranded); err != nil {
		t.Fatalf("re-age stranded run: %v", err)
	}

	var mu sync.Mutex
	var dispatched []string
	rt.dispatchActor = func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		dispatched = append(dispatched, kind+"\x00"+content)
		return nil
	}

	redriven, err := rt.redriveStrandedFreshMintManagement(ctx, ownerID, stranded.RunID)
	if err != nil || !redriven {
		t.Fatalf("expected stranded run re-driven: redriven=%t err=%v", redriven, err)
	}
	mu.Lock()
	if len(dispatched) != 1 {
		t.Fatalf("recovery dispatch count=%d, want 1", len(dispatched))
	}
	sent := dispatched[0]
	mu.Unlock()
	if !strings.HasPrefix(sent, "coagent_result\x00"+PersistentManagementRecoveryPrefix) {
		t.Fatalf("recovery dispatch content is not a recovery occurrence: %q", sent[:min(80, len(sent))])
	}
	decoded, decodeErr := DecodePersistentManagementRecovery(strings.TrimPrefix(sent, "coagent_result\x00"))
	if decodeErr != nil {
		t.Fatalf("decode recovery occurrence: %v", decodeErr)
	}
	if decoded.TrajectoryID != fixture.trajectoryID || decoded.RunID != stranded.RunID || decoded.AgentID != managementAgent {
		t.Fatalf("recovery occurrence scope mismatch: trajectory=%q run=%q agent=%q", decoded.TrajectoryID, decoded.RunID, decoded.AgentID)
	}
	if len(decoded.Controls) == 0 || decoded.Controls[0].UpdateID != fixture.control.UpdateID {
		t.Fatalf("recovery occurrence missing the bound control %q", fixture.control.UpdateID)
	}

	// A run inside its dispatch window is spared.
	recent := types.RunRecord{
		RunID: "run-fresh-mint-inflight", OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		AgentID: managementAgent, AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		State: types.RunPending, Metadata: map[string]any{runMetadataAgentProfile: agentprofile.Management, runMetadataAgentRole: agentprofile.Management},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateRun(ctx, recent); err != nil {
		t.Fatalf("create in-flight run: %v", err)
	}
	redriven, err = rt.redriveStrandedFreshMintManagement(ctx, ownerID, recent.RunID)
	if err != nil || redriven {
		t.Fatalf("in-flight run must not be re-driven: redriven=%t err=%v", redriven, err)
	}

	// A run whose dispatch already executed (state running) is not re-driven.
	executed := stranded
	executed.RunID = "run-fresh-mint-executed"
	executed.State = types.RunRunning
	if err := s.CreateRun(ctx, executed); err != nil {
		t.Fatalf("create executed run: %v", err)
	}
	redriven, err = rt.redriveStrandedFreshMintManagement(ctx, ownerID, executed.RunID)
	if err != nil || redriven {
		t.Fatalf("executing run must not be re-driven: redriven=%t err=%v", redriven, err)
	}
}

func TestArmFreshMintManagementResumeWatchdogFilters(t *testing.T) {
	ownerID := "owner-fresh-mint-arm"
	now := time.Now().UTC()

	nonManagement := watchdogManagementRun(ownerID, "run-arm-texture", types.RunPending, now, false)
	nonManagement.AgentID = agentprofile.Texture + ":doc-1"
	nonManagement.AgentProfile = agentprofile.Texture
	nonManagement.AgentRole = agentprofile.Texture
	if freshMintManagementResumeArming(&nonManagement) {
		t.Error("non-Management run must not arm the fresh-mint watchdog")
	}

	flagged := watchdogManagementRun(ownerID, "run-arm-flagged", types.RunPending, now, true)
	if freshMintManagementResumeArming(&flagged) {
		t.Error("reactivated-flagged run must not arm the fresh-mint watchdog (resume watchdog owns it)")
	}

	running := watchdogManagementRun(ownerID, "run-arm-running", types.RunRunning, now, false)
	if freshMintManagementResumeArming(&running) {
		t.Error("running run must not arm the fresh-mint watchdog")
	}

	stranded := watchdogManagementRun(ownerID, "run-arm-stranded", types.RunPending, now, false)
	if !freshMintManagementResumeArming(&stranded) {
		t.Error("pending Management fresh mint must arm the fresh-mint watchdog")
	}
}
