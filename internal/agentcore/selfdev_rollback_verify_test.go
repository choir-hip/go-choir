package agentcore

// A rollback operation must be able to record its own success
// (docs/problems/selfdev-rollback-cannot-record-success-2026-10-26.md). Ways
// this could fail, written first:
//   - the rollback applies in the updater and then sticks in rollback_pending,
//     because the verification payload it carries names the TARGET's
//     operation, not its own (the defect);
//   - the guest side is loosened so any operation may cite any payload: an
//     apply whose payload names another operation is accepted, or a rollback
//     whose target is not the operation the payload names is accepted;
//   - the certificate is minted for the rollback's own id, so the checkpoint
//     authority's verbatim join (payload operation == certificate operation)
//     refuses it (the fixture's fake authority enforces the same join);
//   - a rollback to a target that is itself a rollback (its verifier ref was
//     copied from the operation before it) is not followed to the operation
//     the payload names;
//   - the route ledger is not rolled to the target's version, or the pending
//     transition is not discharged.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func (fx *derivableSelfDevFixture) requireRolledBackTo(t *testing.T, rollback, target selfdev.Operation) selfdev.Operation {
	t.Helper()
	ctx := context.Background()
	got := fx.waitForState(t, rollback.OperationID, selfdev.StateRolledBack)
	if got.CheckpointRef == "" || got.RouteReceipt == "" || got.MaterializationReceipt == "" {
		t.Fatalf("rolled-back operation lacks its receipts: %+v", got)
	}
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	slot, _, err := fx.ledger.Resolve(ctx, slotID)
	if err != nil {
		t.Fatal(err)
	}
	if string(slot.Current.CodeRef) != target.CodeRef || string(slot.Current.ArtifactProgramRef) != target.ArtifactProgramRef {
		t.Fatalf("route current = %+v, want the rollback target's version %s / %s", slot.Current, target.CodeRef, target.ArtifactProgramRef)
	}
	if head, err := fx.store.Head(ctx, fx.computerID); err != nil || head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("head after the rollback = %+v err=%v; the pending transition must be discharged", head, err)
	}
	return got
}

// A real rollback, end to end with no test shim: two applied releases, a
// rollback to the first through the owner API, recorded as rolled_back with a
// checkpoint and a rollback route transition.
func TestRollbackMaterializesEndToEnd(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-e2e")
	rollback := fx.startRollbackTo(t, first, "rollback-e2e")
	fx.requireRolledBackTo(t, rollback, first)
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRollbackApplied] != 1 {
		t.Fatalf("events = %v; want one rollback_applied", kinds)
	}
	if open, found, err := fx.operations.OpenOperation(context.Background(), fx.computerID); err != nil || found {
		t.Fatalf("open operation after a rollback = %+v found=%v err=%v", open, found, err)
	}
}

// A rollback whose target is itself a rollback: the second rollback's verifier
// ref was copied from the first's, which was copied from the original, so the
// payload names an operation two hops back.
func TestRollbackToARolledBackTargetMaterializes(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-chain")
	firstRollback := fx.requireRolledBackTo(t, fx.startRollbackTo(t, first, "rollback-chain-1"), first)
	// A new candidate is frozen on the base the computer is booted on.
	fx.setBootedBuildCommit(t, joinTestBaseA)
	fx.applyPlain(t, "op-rb-chain-3")
	second := fx.startRollbackTo(t, firstRollback, "rollback-chain-2")
	fx.requireRolledBackTo(t, second, firstRollback)
}

// The guest side stays strict: only a rollback may cite a payload that names
// its target, and only when the target is the operation the payload names and
// shares the verifier ref and bundle.
func TestVerificationEvidenceStaysStrict(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-strict")
	ctx := context.Background()
	asRollback := func(target selfdev.Operation) selfdev.Operation {
		op := target
		op.OperationID, op.State, op.BaseHead = "rollback-strict", selfdev.StateRollbackPending, target.EffectiveHead
		return op
	}
	if _, err := fx.rt.verificationEvidence(ctx, asRollback(first)); err != nil {
		t.Fatalf("a rollback citing its target's payload: %v", err)
	}

	apply := first
	apply.OperationID, apply.State = "selfdev-another", selfdev.StateMaterializing
	if _, err := fx.rt.verificationEvidence(ctx, apply); err == nil {
		t.Fatal("an apply citing a payload that names another operation was accepted")
	}

	garbageBase := asRollback(first)
	garbageBase.BaseHead = strings.Repeat("9", 64)
	if _, err := fx.rt.verificationEvidence(ctx, garbageBase); err == nil {
		t.Fatal("a rollback whose base head resolves to no target was accepted")
	}

	wrongTarget := asRollback(first)
	otherRefs, err := fx.operations.ListByStates(ctx, fx.computerID, selfdev.StateApplied)
	if err != nil || len(otherRefs) < 2 {
		t.Fatalf("applied operations = %d err=%v", len(otherRefs), err)
	}
	for _, other := range otherRefs {
		if other.OperationID != first.OperationID {
			wrongTarget.VerifierRefs = other.VerifierRefs // its payload names `other`, the base head names `first`
		}
	}
	if _, err := fx.rt.verificationEvidence(ctx, wrongTarget); err == nil {
		t.Fatal("a rollback whose payload names an operation other than its target was accepted")
	}
}

// The guest refuses a recorded pass payload whose schema_version is not 1, as
// the checkpoint authority does (internal/platform/checkpoints.go), so a
// wrong-schema payload is refused at the guest instead of minting a
// certificate the authority then refuses.
func TestVerificationEvidenceRefusesWrongSchemaVersion(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-schema")
	ctx := context.Background()
	recordPayload := func(key string, schema any) selfdev.Operation {
		fields := map[string]any{
			"operation_id": first.OperationID, "bundle_digest": first.BundleDigest,
			"decision": "pass", "verifier_refs": []string{"capsule-go-eval:sha256:" + strings.Repeat("a", 64)},
			"verifier_run_id": "run:verifier-" + key,
		}
		if schema != nil {
			fields["schema_version"] = schema
		}
		payload, err := computerevent.CanonicalJSON(fields)
		if err != nil {
			t.Fatal(err)
		}
		id, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: fx.computerID,
			EventKind: computerevent.EventVerificationRecorded, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: "test-verification-" + key, TrajectoryID: first.TrajectoryID, CapsuleID: first.CapsuleID,
			ActorProfile: agentprofile.Engineering, AuthorityRef: "guest-core:self-development-verifier",
			PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, err := fx.appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.self-development-verification+json", "public"); err != nil {
			t.Fatal(err)
		}
		recorded, found, err := fx.store.EventByIdempotency(ctx, fx.computerID, "test-verification-"+key)
		if err != nil || !found {
			t.Fatalf("verification event unavailable: %v", err)
		}
		digest, err := recorded.Digest()
		if err != nil {
			t.Fatal(err)
		}
		op := first
		op.State, op.VerifierRefs = selfdev.StateMaterializing, []string{digest}
		return op
	}
	if _, err := fx.rt.verificationEvidence(ctx, recordPayload("schema-1", 1)); err != nil {
		t.Fatalf("a schema_version 1 payload was refused: %v", err)
	}
	for name, schema := range map[string]any{"absent": nil, "two": 2, "zero": 0} {
		if _, err := fx.rt.verificationEvidence(ctx, recordPayload("schema-"+name, schema)); err == nil {
			t.Fatalf("a payload with schema_version %s was accepted; the authority refuses it", name)
		}
	}
}
