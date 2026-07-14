package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestAcceptanceServingCommitPrefersCompiledArtifactIdentity(t *testing.T) {
	t.Parallel()

	build := buildinfo.Info{
		Commit:         "compiled-serving-sha",
		DeployedCommit: "mutable-release-target-sha",
	}
	if got := acceptanceServingCommit(build); got != "compiled-serving-sha" {
		t.Fatalf("acceptance serving commit = %q, want compiled-serving-sha", got)
	}
}

func TestAcceptanceServingCommitFallsBackForLegacyHealth(t *testing.T) {
	t.Parallel()

	build := buildinfo.Info{DeployedCommit: "legacy-release-sha"}
	if got := acceptanceServingCommit(build); got != "legacy-release-sha" {
		t.Fatalf("acceptance serving commit = %q, want legacy-release-sha", got)
	}
}

func TestAppPromotionCheckpointsTolerateCompletedVerificationAndRollForward(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	kinds := []types.EventKind{
		types.EventAppChangePackagePublished,
		types.EventAppAdoptionVerificationStarted,
		types.EventAppAdoptionVerified,
		types.EventAppAdoptionPromoted,
		types.EventAppAdoptionRolledBack,
		types.EventAppAdoptionPromoted,
	}
	events := make([]types.EventRecord, 0, len(kinds))
	for i, kind := range kinds {
		events = append(events, types.EventRecord{
			EventID:      string(rune('a' + i)),
			OwnerID:      "user-alice",
			TrajectoryID: "traj-adoption-cycle",
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			StreamSeq:    int64(i + 1),
			Kind:         kind,
			Payload:      json.RawMessage(`{"adoption_id":"adoption-cycle","package_id":"pkg-cycle","rollback_source_ref":"refs/computers/computer-b/active-before-adoption"}`),
		})
	}
	builder := acceptanceBuilder{
		record:      types.RunAcceptanceRecord{Checkpoints: []types.RunAcceptanceCheckpoint{}},
		evidenceSet: map[string]bool{},
	}
	addAcceptanceAppPromotionCheckpoints(&builder, events)

	checkpointSeq := map[string]int64{}
	for _, checkpoint := range builder.record.Checkpoints {
		checkpointSeq[checkpoint.Kind] = checkpoint.StreamSeq
		if checkpoint.Kind == "app_adoption_verifying" && checkpoint.State == "pending" {
			t.Errorf("terminal verification left a stale pending checkpoint: %+v", checkpoint)
		}
	}
	for _, kind := range []string{"app_adoption_promoted", "rollback_available"} {
		if seq, exists := checkpointSeq[kind]; !exists || seq != 4 {
			t.Errorf("%s checkpoint stream_seq = %d, exists=%t, want first-promotion boundary 4", kind, seq, exists)
		}
	}
	causalOrderFound := false
	for _, check := range buildAcceptanceInvariantChecks(builder.record) {
		if check.Name == "checkpoint_causal_order" {
			causalOrderFound = true
			if check.State != "passed" {
				t.Errorf("promotion rollback and roll-forward violated causal order: checkpoints=%+v", builder.record.Checkpoints)
			}
		}
	}
	if !causalOrderFound {
		t.Error("checkpoint_causal_order invariant missing")
	}
}

func TestAppPromotionCheckpointsCorrelateLegacyVerificationWithoutAdoptionID(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	events := []types.EventRecord{
		{
			EventID:   "legacy-start",
			Timestamp: now,
			StreamSeq: 1,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"package_id":"pkg-legacy","target_computer_id":"computer-b"}`),
		},
		{
			EventID:   "legacy-identified-retry",
			Timestamp: now.Add(time.Second),
			StreamSeq: 2,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-legacy","package_id":"pkg-legacy","target_computer_id":"computer-b"}`),
		},
		{
			EventID:   "legacy-distinct-adoption-start",
			Timestamp: now.Add(2 * time.Second),
			StreamSeq: 3,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-legacy-b","package_id":"pkg-legacy","target_computer_id":"computer-b"}`),
		},
		{
			EventID:   "legacy-verified",
			Timestamp: now.Add(3 * time.Second),
			StreamSeq: 4,
			Kind:      types.EventAppAdoptionVerified,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-legacy","package_id":"pkg-legacy","target_computer_id":"computer-b"}`),
		},
		{
			EventID:   "identified-start",
			Timestamp: now.Add(4 * time.Second),
			StreamSeq: 5,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-identified","package_id":"pkg-identified","target_computer_id":"computer-d"}`),
		},
		{
			EventID:   "identified-verified",
			Timestamp: now.Add(5 * time.Second),
			StreamSeq: 6,
			Kind:      types.EventAppAdoptionVerified,
			Payload:   json.RawMessage(`{"package_id":"pkg-identified","target_computer_id":"computer-d"}`),
		},
		{
			EventID:   "concurrent-a-start",
			Timestamp: now.Add(6 * time.Second),
			StreamSeq: 7,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-a","package_id":"pkg-shared","target_computer_id":"computer-e"}`),
		},
		{
			EventID:   "concurrent-b-start",
			Timestamp: now.Add(7 * time.Second),
			StreamSeq: 8,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-b","package_id":"pkg-shared","target_computer_id":"computer-e"}`),
		},
		{
			EventID:   "concurrent-a-verified",
			Timestamp: now.Add(8 * time.Second),
			StreamSeq: 9,
			Kind:      types.EventAppAdoptionVerified,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-a","package_id":"pkg-shared","target_computer_id":"computer-e"}`),
		},
		{
			EventID:   "unknown-adoption-terminal",
			Timestamp: now.Add(9 * time.Second),
			StreamSeq: 10,
			Kind:      types.EventAppAdoptionVerified,
			Payload:   json.RawMessage(`{"adoption_id":"adoption-unknown","package_id":"pkg-shared","target_computer_id":"computer-e"}`),
		},
		{
			EventID:   "other-start",
			Timestamp: now.Add(10 * time.Second),
			StreamSeq: 11,
			Kind:      types.EventAppAdoptionVerificationStarted,
			Payload:   json.RawMessage(`{"package_id":"pkg-other","target_computer_id":"computer-c"}`),
		},
	}
	builder := acceptanceBuilder{
		record:      types.RunAcceptanceRecord{Checkpoints: []types.RunAcceptanceCheckpoint{}},
		evidenceSet: map[string]bool{},
	}
	addAcceptanceAppPromotionCheckpoints(&builder, events)

	pending := 0
	for _, checkpoint := range builder.record.Checkpoints {
		if checkpoint.Kind != "app_adoption_verifying" || checkpoint.State != "pending" {
			continue
		}
		pending++
		if checkpoint.StreamSeq != 3 {
			t.Errorf("pending verification stream_seq = %d, want distinct legacy adoption B at 3", checkpoint.StreamSeq)
		}
		if count := checkpoint.Details["verifying_event_count"]; count != 3 {
			t.Errorf("pending verification count = %v, want both adoption B verifications plus unrelated verification", count)
		}
	}
	if pending != 1 {
		t.Errorf("pending verification checkpoints = %d, want one unrelated outstanding adoption", pending)
	}
}
