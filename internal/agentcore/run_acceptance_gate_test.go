package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestFailedCapsuleCheckpointBlocksAcceptance pins the P3-acceptance floor: a
// completed implementation/verification run without its freeze/verify evidence
// must not report an accepted level, even when every other checkpoint passes.
func TestFailedCapsuleCheckpointBlocksAcceptance(t *testing.T) {
	passing := []types.RunAcceptanceCheckpoint{
		{Kind: "submitted", State: "passed"},
		{Kind: runAcceptanceCheckpointTextureOpened, State: "passed"},
		{Kind: "super_direction_opened", State: "passed"},
	}
	_, state := acceptanceLevelAndState(passing)
	if state != types.RunAcceptanceAccepted {
		t.Fatalf("all-passing checkpoints must accept, got %s", state)
	}
	for _, kind := range []string{"capsule_effect_frozen", "capsule_verification_recorded"} {
		checkpoints := append(append([]types.RunAcceptanceCheckpoint{}, passing...),
			types.RunAcceptanceCheckpoint{Kind: kind, State: "failed"})
		_, state := acceptanceLevelAndState(checkpoints)
		if state == types.RunAcceptanceAccepted {
			t.Errorf("failed %s checkpoint must block acceptance", kind)
		}
	}
}

// TestFreezeCheckpointSurvivesPostFrozenStates proves the freeze evidence is
// the durable bundle digest, not a state allowlist: a verify-fail trajectory
// (state failed) or a materialized one (state applied) still reports the
// freeze fact. It drives the real selfdev store through the canonical
// transition chain and reads the checkpoint the builder emits.
func TestFreezeCheckpointSurvivesPostFrozenStates(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-acceptance"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{SchemaVersion: 1, EventID: genesisID, ComputerID: computerID, EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64), ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevOperations: operations}

	// One trajectory per state; each gets its own operation and impl run.
	for _, target := range []struct {
		state string
		path  []string
	}{
		{selfdev.StateFrozen, []string{selfdev.StateExecuting, selfdev.StateFrozen}},
		{selfdev.StateVerified, []string{selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified}},
		{selfdev.StateFailed, []string{selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateFailed}},
		{selfdev.StateApplied, []string{selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied}},
	} {
		trajectoryID := "trajectory-" + target.state
		operation, err := operations.Start(ctx, selfdev.StartRequest{
			ComputerID: computerID, IdempotencyKey: "op-" + target.state,
			PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
			TrajectoryID:      trajectoryID,
		})
		if err != nil {
			t.Fatalf("%s: start: %v", target.state, err)
		}
		for _, next := range target.path {
			next := next
			operation, err = operations.Transition(ctx, computerID, operation.OperationID, operation.State, next, func(op *selfdev.Operation) error {
				if next == selfdev.StateFrozen {
					op.BundleDigest = strings.Repeat("d", 64)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("%s: transition to %s: %v", target.state, next, err)
			}
		}
		run := types.RunRecord{
			RunID: "run-" + target.state, ComputerID: computerID, State: types.RunCompleted,
			Metadata: map[string]any{
				runMetadataAgentProfile: agentprofile.CoSuper, "assignment_kind": string(types.CoSuperAssignmentImplementation),
				runMetadataTrajectoryID: trajectoryID,
			},
		}
		builder := &acceptanceBuilder{evidenceSet: map[string]bool{}}
		addAcceptanceDurableAgentCapsuleCheckpoints(ctx, rt, builder, []types.RunRecord{run}, nil)
		var freeze *types.RunAcceptanceCheckpoint
		for i := range builder.record.Checkpoints {
			if builder.record.Checkpoints[i].Kind == "capsule_effect_frozen" {
				freeze = &builder.record.Checkpoints[i]
			}
		}
		if freeze == nil {
			t.Fatalf("%s: no capsule_effect_frozen checkpoint", target.state)
		}
		if freeze.State != "passed" {
			t.Errorf("%s: freeze checkpoint = %s, want passed (digest bound)", target.state, freeze.State)
		}
	}
}
