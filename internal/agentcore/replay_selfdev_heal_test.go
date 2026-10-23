package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The operator heal for historic intent-error stamps
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md,
// "Residual: operator heal for historic stamps" and the section "Operator heal
// and a corrupt vocabulary sidecar"). A computer that stamped before the
// RecordIntentError fix holds a last_intent_error and an updated_at no tape
// event carries, so it replays not_equivalent.
//
// Ways it can fail, written before the code:
//
//  1. The heal does not make the replay equivalent (wrong columns, wrong
//     format of updated_at, a snapshot the projector reads differently).
//  2. The dry run appends something.
//  3. A second heal appends again (not idempotent), or selects rows that
//     already agree.
//  4. The heal selects an equivalent row (it must select only rows whose
//     last_intent_error or updated_at differ from the replayed row).
//  5. A row whose state moved between the plan and the apply is overwritten
//     with the old snapshot instead of being refused by ExpectedState, or a
//     row whose stamp moved is overwritten by the older stamp.
//  6. Two heals run at once and both append.
//  7. A store with no projection sink bound (a test seam) appends nothing yet
//     claims it healed.

// preFixStamp writes the stamp the way the old RecordIntentError did: raw SQL,
// no tape event.
func preFixStamp(t *testing.T, rt *Runtime, operationID, reason string) {
	t.Helper()
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET last_intent_error=?, updated_at=? WHERE operation_id=?`,
		reason, "2026-10-23T05:06:07.123456Z", operationID); err != nil {
		t.Fatal(err)
	}
}

func headSequence(t *testing.T, ctx context.Context, rt *Runtime, computerID string) uint64 {
	t.Helper()
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head = %+v, %v", head, err)
	}
	return head.Sequence
}

func TestSelfDevelopmentIntentStampHealMakesAPreFixStampReplayEquivalent(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal"
	rt, operations := selfdevReplayRuntime(t, computerID)
	rt.selfdevOperations = operations
	// An earlier operation that agrees with its replay (a terminal row): it
	// must be examined and not selected.
	earlier := openExecutingOperation(t, ctx, operations, computerID)
	if _, err := operations.Transition(ctx, computerID, earlier.OperationID, selfdev.StateExecuting, selfdev.StateFailed, nil); err != nil {
		t.Fatal(err)
	}
	opened, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: "idem-second",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operations.Transition(ctx, computerID, opened.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
		t.Fatal(err)
	}

	// Control: the tape-only history is equivalent and a heal selects nothing.
	clean, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !clean.Probe.Equivalent || len(clean.Rows) != 0 || len(clean.RefusedDiverged) != 0 || clean.RowsExamined != 2 {
		t.Fatalf("control: an equivalent computer selected rows: %+v", clean)
	}

	preFixStamp(t, rt, opened.OperationID, "freeze: refused (no verifier refs)")
	broken, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if broken.Result.Equivalent() {
		t.Fatal("control: a stamp written outside the tape replayed equivalent")
	}

	before := headSequence(t, ctx, rt, computerID)
	dry, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Mode != "dry_run" || dry.Probe.Equivalent || dry.Appended != 0 {
		t.Fatalf("dry run = %+v", dry)
	}
	if len(dry.Rows) != 1 || dry.Rows[0].OperationID != opened.OperationID || dry.Rows[0].State != selfdev.StateExecuting ||
		dry.Rows[0].LiveLastIntentError != "freeze: refused (no verifier refs)" || dry.Rows[0].ReplayLastIntentError != "" ||
		strings.Join(dry.Rows[0].Differs, ",") != "last_intent_error,updated_at" {
		t.Fatalf("dry run rows = %+v", dry.Rows)
	}
	if got := headSequence(t, ctx, rt, computerID); got != before {
		t.Fatalf("the dry run appended events: sequence %d -> %d", before, got)
	}

	applied, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Mode != "applied" || applied.Appended != 1 || len(applied.Rows) != 1 {
		t.Fatalf("apply = %+v", applied)
	}
	if got := headSequence(t, ctx, rt, computerID); got != before+1 {
		t.Fatalf("the heal is one batch: sequence %d -> %d, want +1", before, got)
	}
	if applied.After == nil || !applied.After.Equivalent || !applied.After.Eligible {
		t.Fatalf("after the heal the report says %+v", applied.After)
	}
	healed, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !healed.Result.Equivalent() || !healed.Eligibility.Eligible {
		t.Fatalf("after the heal the replay is not equivalent: %#v %+v", healed.Result.Differences, healed.Eligibility)
	}

	// Idempotent: nothing to select, nothing to append.
	sequence := headSequence(t, ctx, rt, computerID)
	again, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Rows) != 0 || again.Appended != 0 {
		t.Fatalf("a second heal selected %+v", again)
	}
	if got := headSequence(t, ctx, rt, computerID); got != sequence {
		t.Fatalf("a second heal appended events: sequence %d -> %d", sequence, got)
	}
	// The live stamp survived.
	live, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || live.LastIntentError != "freeze: refused (no verifier refs)" || live.UpdatedAt != "2026-10-23T05:06:07.123456Z" {
		t.Fatalf("live row after the heal = %+v, %v", live, err)
	}
}

func TestSelfDevelopmentIntentStampHealRefusesARowWhoseStateMovedAfterThePlan(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal-state-moved"
	rt, operations := selfdevReplayRuntime(t, computerID)
	rt.selfdevOperations = operations
	opened := openExecutingOperation(t, ctx, operations, computerID)
	preFixStamp(t, rt, opened.OperationID, "verify: bundle digest mismatch")

	rt.selfdevStampHealAfterPlan = func() {
		if _, err := operations.Transition(ctx, computerID, opened.OperationID, selfdev.StateExecuting, selfdev.StateFailed, nil); err != nil {
			t.Errorf("transition between plan and apply: %v", err)
		}
	}
	before := headSequence(t, ctx, rt, computerID)
	_, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false)
	if !errors.Is(err, selfdev.ErrConflict) || !errors.Is(err, computerevent.ErrProjectionMismatch) {
		t.Fatalf("a row whose state moved was not refused by ExpectedState: %v", err)
	}
	// The Transition itself is one event; the heal added none.
	if got := headSequence(t, ctx, rt, computerID); got != before+1 {
		t.Fatalf("sequence %d -> %d: only the transition may have appended", before, got)
	}
	live, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || live.State != selfdev.StateFailed {
		t.Fatalf("live row = %+v, %v: the old snapshot overwrote the new state", live, err)
	}
}

func TestSelfDevelopmentIntentStampHealRefusesARowWhoseStampMovedAfterThePlan(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal-stamp-moved"
	rt, operations := selfdevReplayRuntime(t, computerID)
	rt.selfdevOperations = operations
	opened := openExecutingOperation(t, ctx, operations, computerID)
	preFixStamp(t, rt, opened.OperationID, "freeze: refused")

	rt.selfdevStampHealAfterPlan = func() {
		if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, "verify: a newer refusal"); err != nil {
			t.Errorf("stamp between plan and apply: %v", err)
		}
	}
	if _, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false); !errors.Is(err, selfdev.ErrConflict) {
		t.Fatalf("a row whose stamp moved was not refused: %v", err)
	}
	live, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || live.LastIntentError != "verify: a newer refusal" {
		t.Fatalf("live row = %+v, %v: an older stamp overwrote the newer one", live, err)
	}
	// After the newer stamp is on the tape the heal converges on the next run.
	rt.selfdevStampHealAfterPlan = nil
	if _, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false); err != nil {
		t.Fatal(err)
	}
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil || !report.Result.Equivalent() {
		t.Fatalf("after the retried heal: %v %#v", err, report.Result.Differences)
	}
}

func TestSelfDevelopmentIntentStampHealRefusesAnUnboundStoreAndAConcurrentHeal(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal-guards"
	rt, operations := selfdevReplayRuntime(t, computerID)
	unbound, err := selfdev.NewStore(rt.store, rt.store)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = unbound
	opened := openExecutingOperation(t, ctx, operations, computerID)
	preFixStamp(t, rt, opened.OperationID, "freeze: refused")
	if _, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false); !errors.Is(err, selfdev.ErrStampHealUnbound) {
		t.Fatalf("a store with no projection sink healed or failed otherwise: %v", err)
	}

	rt.selfdevOperations = operations
	entered, release := make(chan struct{}), make(chan struct{})
	rt.selfdevStampHealAfterPlan = func() { close(entered); <-release }
	done := make(chan error, 1)
	go func() {
		_, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false)
		done <- err
	}()
	<-entered
	if _, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false); !errors.Is(err, ErrStampHealRunning) {
		t.Fatalf("a concurrent heal was not refused: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// A row whose state differs from the replayed row (changed outside the tape)
// must never be healed: the heal event would pass ExpectedState against the
// live table and fail it against the replayed one, and an event the replay
// cannot apply on the canonical tape is a boot loop. The plan reports it with
// the replay's values and appends nothing for it; a healthy row beside it is
// still healed, and the tape still reconstructs.
func TestSelfDevelopmentIntentStampHealRefusesARowWhoseStateDivergedFromTheReplay(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal-state-diverged"
	rt, operations := selfdevReplayRuntime(t, computerID)
	rt.selfdevOperations = operations
	earlier := openExecutingOperation(t, ctx, operations, computerID)
	if _, err := operations.Transition(ctx, computerID, earlier.OperationID, selfdev.StateExecuting, selfdev.StateFailed, nil); err != nil {
		t.Fatal(err)
	}
	healthy, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: "idem-healthy",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operations.Transition(ctx, computerID, healthy.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
		t.Fatal(err)
	}
	preFixStamp(t, rt, healthy.OperationID, "freeze: refused")
	// The earlier row: state moved outside the tape, with a stamp on top.
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state=?, last_intent_error=?, updated_at=? WHERE operation_id=?`,
		selfdev.StateExecuting, "stale stamp", "2026-10-23T06:00:00.000001Z", earlier.OperationID); err != nil {
		t.Fatal(err)
	}

	before := headSequence(t, ctx, rt, computerID)
	report, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != 1 || report.Rows[0].OperationID != healthy.OperationID || report.Appended != 1 {
		t.Fatalf("the heal selected %+v appended %d, want only the healthy row", report.Rows, report.Appended)
	}
	if len(report.RefusedDiverged) != 1 {
		t.Fatalf("refused = %+v, want the diverged row", report.RefusedDiverged)
	}
	refused := report.RefusedDiverged[0]
	if refused.OperationID != earlier.OperationID || refused.Reason != "state_diverged" ||
		refused.LiveState != selfdev.StateExecuting || refused.ReplayState != selfdev.StateFailed ||
		refused.ReplayValues["state"] != selfdev.StateFailed {
		t.Fatalf("refusal = %+v", refused)
	}
	if got := headSequence(t, ctx, rt, computerID); got != before+1 {
		t.Fatalf("sequence %d -> %d: exactly the healthy row's batch may append", before, got)
	}
	// The replay still reconstructs (it is not equivalent: the diverged row
	// is the operator's to repair), and a second heal appends nothing.
	if _, err := rt.ReplayCompleteness(ctx, computerID); err != nil {
		t.Fatalf("the tape no longer reconstructs after the heal: %v", err)
	}
	again, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false)
	if err != nil || again.Appended != 0 || len(again.Rows) != 0 || len(again.RefusedDiverged) != 1 {
		t.Fatalf("second heal = %+v, %v", again, err)
	}
	// The refused row keeps the live values.
	live, err := operations.Get(ctx, computerID, earlier.OperationID)
	if err != nil || live.State != selfdev.StateExecuting || live.LastIntentError != "stale stamp" {
		t.Fatalf("refused row changed: %+v, %v", live, err)
	}
}

// A stamp that went A, B, A between the plan and the apply has the plan's text
// and a later updated_at; comparing the text alone would let the older
// snapshot roll updated_at back.
func TestSelfDevelopmentIntentStampHealRefusesAStampThatReturnedToItsPlannedTextWithALaterUpdatedAt(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-stamp-heal-aba"
	rt, operations := selfdevReplayRuntime(t, computerID)
	rt.selfdevOperations = operations
	opened := openExecutingOperation(t, ctx, operations, computerID)
	preFixStamp(t, rt, opened.OperationID, "freeze: refused")

	rt.selfdevStampHealAfterPlan = func() {
		for _, reason := range []string{"verify: B", "freeze: refused"} {
			if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, reason); err != nil {
				t.Errorf("stamp %q between plan and apply: %v", reason, err)
			}
		}
	}
	before := headSequence(t, ctx, rt, computerID)
	if _, err := rt.HealSelfDevelopmentIntentStamps(ctx, computerID, true, false); !errors.Is(err, selfdev.ErrConflict) {
		t.Fatalf("an A, B, A stamp was not refused: %v", err)
	}
	if got := headSequence(t, ctx, rt, computerID); got != before+2 {
		t.Fatalf("sequence %d -> %d: only the two stamps may have appended", before, got)
	}
}
