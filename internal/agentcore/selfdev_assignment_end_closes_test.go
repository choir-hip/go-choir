package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A failed freeze left its operation executing for 17 hours
// (docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md):
// the busy probe counted it, vmctl deferred every refresh, and no owner
// decision could close it. A freeze refusal is retryable while the
// assignment lives (TestFreezeRefusalStampsOperationIntentError pins
// that), so the operation closes when its driver, the assignment, ends.
//
// Ways it can fail (written before the code):
//  1. An assignment reports terminal (completed or failed) before its
//     operation froze, and the operation stays executing: busy forever.
//     Only cancellation closed the operation before this change.
//  2. The close carries no visible fate: the terminal error must name the
//     assignment's end and the last intent error (the freeze refusal).
//  3. The close kills a frozen candidate of a completed assignment, which
//     waits for management's Verify.
//  4. A re-freeze of the same assignment revives the failed operation
//     instead of being refused.
//  5. The busy probe (selfdev_active_operations) still counts it.
//  6. The crash-boot close regresses
//     (TestCrashBootClosesPreDecisionSelfDevelopmentOperations pins it).

func assignmentEndFixture(t *testing.T, owner string) (*Runtime, *store.Store, types.EngineeringAssignment) {
	t.Helper()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	return rt, s, assignment
}

// pinnedAssignmentReport is a terminal report shaped as the entry point
// derives it: the pinned subject, its proposition digest and slot report id.
func pinnedAssignmentReport(assignment types.EngineeringAssignment, result types.EngineeringAssignmentResultKind) types.EngineeringAssignmentReport {
	report := terminalAssignmentReport(assignment, result)
	report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
	report.CandidateArtifactRef = ""
	report.Mutations[0].AfterDigest = assignment.Binding.SubjectDigest
	report.Mutations[0].SubjectBytesChanged = false
	report.PropositionDigest, _ = store.ComputeTerminalPropositionDigest(assignment.Binding.SubjectDigest, report.Result, report.Verdict, report.Commands, report.Outputs, report.EvidenceRefs)
	report.ReportID = store.TerminalReportID(assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt, report.PropositionDigest)
	return report
}

func seedAssignmentOperation(t *testing.T, rt *Runtime, assignment types.EngineeringAssignment, state, intentError string) string {
	t.Helper()
	operationID := store.EngineeringFreezeOperationID(assignment.Binding.ComputerID, assignment.AssignmentID)
	now := time.Now().UTC()
	if _, err := rt.store.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, bundle_digest, desired_head, effective_head, state, last_intent_error, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', '[]', ?, '', '', ?, ?, ?, ?)`,
		operationID, assignment.Binding.ComputerID, store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID), assignment.Binding.TrajectoryID,
		map[bool]string{true: strings.Repeat("f", 64), false: ""}[state == selfdev.StateFrozen], state, intentError, now, now); err != nil {
		t.Fatal(err)
	}
	return operationID
}

func terminalAssignmentReport(assignment types.EngineeringAssignment, result types.EngineeringAssignmentResultKind) types.EngineeringAssignmentReport {
	after := objectgraph.SHA256([]byte("changed subject " + assignment.AssignmentID))
	return types.EngineeringAssignmentReport{
		Schema: types.EngineeringAssignmentSchemaV1, ReportID: "report-" + string(result) + "-" + assignment.AssignmentID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt, OwnerID: assignment.Binding.OwnerID,
		ComputerID: assignment.Binding.ComputerID, TrajectoryID: assignment.Binding.TrajectoryID,
		RunID: assignment.BoundRunID, AssignedAgentID: assignment.Binding.AssignedAgentID,
		Result: result, Verdict: types.EngineeringVerdictNone, ObservedSubjectDigest: after,
		CandidateArtifactRef: "capsule-subject:" + after,
		Mutations: []types.EngineeringRecordedMutation{{
			MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: assignment.Binding.SubjectDigest,
			AfterDigest: after, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
		}},
		Summary: "the freeze was refused; reporting " + string(result), CreatedAt: time.Now().UTC(),
	}
}

func TestAssignmentEndClosesItsUnfrozenOperation(t *testing.T) {
	const refusal = "freeze: capsule release must contain executable bin/autoputer: the running binary was built from 219535ef"
	for _, result := range []types.EngineeringAssignmentResultKind{types.EngineeringResultCompleted, types.EngineeringResultFailed} {
		t.Run(string(result), func(t *testing.T) {
			ctx := context.Background()
			rt, s, assignment := assignmentEndFixture(t, "owner-end-"+string(result))
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, refusal)
			if got := rt.activeSelfdevOperations(ctx); got != 1 {
				t.Fatalf("busy probe before the report = %d, want 1", got)
			}

			committed, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, result))
			if err != nil {
				t.Fatalf("terminal report: %v", err)
			}
			if !committed.Assignment.Disposition.Terminal() {
				t.Fatalf("assignment after the terminal report = %s", committed.Assignment.Disposition)
			}

			operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
			if err != nil {
				t.Fatal(err)
			}
			if operation.State != selfdev.StateFailed {
				t.Fatalf("operation after its assignment ended = %s, want failed (item 1)", operation.State)
			}
			if !strings.Contains(operation.TerminalError, "before its candidate froze") || !strings.Contains(operation.TerminalError, refusal) {
				t.Fatalf("terminal error = %q; want the assignment's end and the freeze refusal (item 2)", operation.TerminalError)
			}
			if got := rt.activeSelfdevOperations(ctx); got != 0 {
				t.Fatalf("busy probe after the close = %d, want 0 (item 5)", got)
			}

			// Item 4: the same assignment's candidate is never revived.
			reopened, err := openFreezeCandidate(ctx, rt.selfdevOperations, func(context.Context) error { return nil },
				assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, assignment.AssignmentID, "retry")
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if reopened.OperationID != operationID || reopened.State != selfdev.StateFailed {
				t.Fatalf("a re-freeze revived the failed operation: %+v", reopened)
			}

			// A replay of the same report writes nothing new to the closed
			// operation. The outer replay path that must also close it is
			// TestReplayedReportClosesTheOperationAfterACrashBeforeTheClose.
			if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, result)); err != nil {
				t.Fatalf("replayed report: %v", err)
			}
			again, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
			if err != nil || again.State != selfdev.StateFailed || again.TerminalError != operation.TerminalError {
				t.Fatalf("a replayed report changed the closed operation: %+v, %v", again, err)
			}
		})
	}
}

func TestCompletedAssignmentKeepsItsFrozenCandidate(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-end-frozen")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultCompleted)); err != nil {
		t.Fatalf("terminal report: %v", err)
	}
	operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.State != selfdev.StateFrozen {
		t.Fatalf("a completed assignment's frozen candidate = %s, want frozen for management's Verify (item 3)", operation.State)
	}
}

func TestFailedAssignmentClosesItsFrozenCandidate(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-end-failed-frozen")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultFailed)); err != nil {
		t.Fatalf("terminal report: %v", err)
	}
	operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	// Management verifies only a completed assignment's candidate, so a
	// failed assignment's frozen candidate can never advance.
	if operation.State != selfdev.StateFailed || !strings.Contains(operation.TerminalError, "assignment failed") {
		t.Fatalf("a failed assignment's frozen candidate = %s %q, want failed", operation.State, operation.TerminalError)
	}
}

// Review bunch 15, F1/F4: a crash between the report's commit and the
// close leaves a terminal assignment with a stored report and an executing
// operation. The re-delivered report takes the outer replay path
// (recordAssignedEngineeringReportOnce returns the stored receipt early),
// which must still close the operation: nothing else re-drives a terminal
// assignment on a planned boot.
func TestReplayedReportClosesTheOperationAfterACrashBeforeTheClose(t *testing.T) {
	for _, result := range []types.EngineeringAssignmentResultKind{types.EngineeringResultCompleted, types.EngineeringResultFailed} {
		t.Run(string(result), func(t *testing.T) {
			ctx := context.Background()
			rt, s, _ := c1eRuntime(t)
			operations, err := selfdev.NewStore(s, s)
			if err != nil {
				t.Fatal(err)
			}
			rt.selfdevOperations = operations
			seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-replay-"+string(result), rt.TextureComputerID(), 1)
			if err != nil {
				t.Fatal(err)
			}
			assignment, run := c1eOpenBound(t, s, seed, 0)
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "freeze: refused")

			report := pinnedAssignmentReport(assignment, result)

			// The crash window: the store commits the terminal report; the
			// close never runs.
			req := types.RecordEngineeringAssignmentReportRequest{
				CommandID: "engineering-report:" + assignment.AssignmentID + ":" + report.ReportID,
				OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
				AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
			}
			req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
			committed, err := s.RecordEngineeringAssignmentReport(ctx, req)
			if err != nil || !committed.Assignment.Disposition.Terminal() {
				t.Fatalf("store report = %+v, %v", committed.Assignment.Disposition, err)
			}
			if before, _ := operations.Get(ctx, assignment.Binding.ComputerID, operationID); before.State != selfdev.StateExecuting {
				t.Fatalf("setup: operation = %s before the replay", before.State)
			}

			if _, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:replay", report); err != nil {
				t.Fatalf("replayed report: %v", err)
			}
			operation, err := operations.Get(ctx, assignment.Binding.ComputerID, operationID)
			if err != nil {
				t.Fatal(err)
			}
			if operation.State != selfdev.StateFailed || !strings.Contains(operation.TerminalError, "before its candidate froze") {
				t.Fatalf("operation after the replayed report = %s %q, want failed", operation.State, operation.TerminalError)
			}
			if got := rt.activeSelfdevOperations(ctx); got != 0 {
				t.Fatalf("busy probe after the replay = %d, want 0", got)
			}
		})
	}
}

// endedAssignment is the terminal assignment value the close reads: its
// disposition, id and computer. The close never reads anything else.
func endedAssignment(computerID, assignmentID string, disposition types.EngineeringAssignmentDisposition) types.EngineeringAssignment {
	return types.EngineeringAssignment{
		AssignmentID: assignmentID, Disposition: disposition,
		Binding: types.EngineeringAssignmentBinding{ComputerID: computerID, TrajectoryID: "trajectory-" + assignmentID},
	}
}

// tapeBoundStore is a product store whose projection tape is bound, as in
// production: self-development transitions write only through the tape.
func tapeBoundStore(t *testing.T, computerID string) *store.Store {
	t.Helper()
	ctx := context.Background()
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = productStore.Close() })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "close-tape"}, PrivateKey: privateKey}
	// The derivable fixture's pinner and CAS: they pin the non-private
	// projection batches a bound tape appends.
	appender, err := computerevent.NewComputerEventAppender(computerID, &derivableTestPinner{key: signingKey}, productStore, &derivableChainCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	commitment := strings.Repeat("a", 64)
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: commitment, ProposedEffectRef: strings.Repeat("b", 64),
		ResultingEffectiveCommitment: commitment, ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		t.Fatal(err)
	}
	return productStore
}

// DeepSeek review of bunch 15: the production store binds the projection
// tape, where Transition writes only through the event projection. The
// close must work there, not only on the direct-SQL test seam.
func TestAssignmentEndCloseWritesThroughTheProjectionTape(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-close-tape"
	productStore := tapeBoundStore(t, computerID)
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	operations.BindProjectionSink(productStore)
	opened, err := openFreezeCandidate(ctx, operations, func(context.Context) error { return nil }, computerID, "trajectory-close-tape", "assignment-close-tape", "objective")
	if err != nil || opened.State != selfdev.StateExecuting {
		t.Fatalf("open = %+v, %v", opened, err)
	}
	if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, "freeze: refused on the tape path"); err != nil {
		t.Fatal(err)
	}
	before, err := productStore.Head(ctx, computerID)
	if err != nil || before == nil {
		t.Fatalf("head = %+v, %v", before, err)
	}

	rt := &Runtime{selfdevOperations: operations}
	rt.closeEndedAssignmentSelfdevOperation(ctx, endedAssignment(computerID, "assignment-close-tape", types.EngineeringAssignmentCompleted))

	closed, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || closed.State != selfdev.StateFailed || !strings.Contains(closed.TerminalError, "refused on the tape path") {
		t.Fatalf("operation on the tape path = %+v, %v; want failed with the refusal", closed, err)
	}
	after, err := productStore.Head(ctx, computerID)
	if err != nil || after == nil || after.CanonicalEventHead == before.CanonicalEventHead {
		t.Fatalf("the close appended nothing to the tape: head before %+v after %+v (%v)", before, after, err)
	}
}

// DeepSeek review of bunch 15: a late freeze racing the close. Either may
// win, but the operation never stays executing; a failed assignment's
// candidate closes even when the freeze lands first (the close retries on
// the state conflict).
func TestAssignmentEndCloseRacesALateFreeze(t *testing.T) {
	for _, disposition := range []types.EngineeringAssignmentDisposition{types.EngineeringAssignmentCompleted, types.EngineeringAssignmentFailed} {
		t.Run(string(disposition), func(t *testing.T) {
			ctx := context.Background()
			rt, _ := testRuntime(t)
			computerID := rt.TextureComputerID()
			for i := 0; i < 20; i++ {
				assignment := endedAssignment(computerID, fmt.Sprintf("assignment-race-%s-%d", disposition, i), disposition)
				operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "freeze: refused")
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, _ = rt.selfdevOperations.Transition(ctx, computerID, operationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
						next.BundleDigest = strings.Repeat("f", 64)
						return nil
					})
				}()
				go func() {
					defer wg.Done()
					rt.closeEndedAssignmentSelfdevOperation(ctx, assignment)
				}()
				wg.Wait()
				operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
				if err != nil {
					t.Fatal(err)
				}
				switch {
				case operation.State == selfdev.StateExecuting:
					t.Fatalf("race %d left the operation executing", i)
				case disposition == types.EngineeringAssignmentFailed && operation.State != selfdev.StateFailed:
					t.Fatalf("race %d: a failed assignment's operation = %s, want failed", i, operation.State)
				case disposition == types.EngineeringAssignmentCompleted && operation.State != selfdev.StateFailed && operation.State != selfdev.StateFrozen:
					t.Fatalf("race %d: a completed assignment's operation = %s, want failed or frozen", i, operation.State)
				}
			}
			if got := rt.activeSelfdevOperations(ctx); got != 0 {
				t.Fatalf("busy probe after the races = %d, want 0", got)
			}
		})
	}
}

// DeepSeek review of bunch 15: an owner decision's states are never closed
// by an assignment's end, whatever its disposition.
func TestAssignmentEndNeverClosesADecidedOperation(t *testing.T) {
	ctx := context.Background()
	rt, _ := testRuntime(t)
	computerID := rt.TextureComputerID()
	decided := []string{selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied, selfdev.StateRollbackPending, selfdev.StateDegraded}
	for _, disposition := range []types.EngineeringAssignmentDisposition{types.EngineeringAssignmentCompleted, types.EngineeringAssignmentFailed, types.EngineeringAssignmentCancelled} {
		for _, state := range decided {
			assignment := endedAssignment(computerID, "assignment-decided-"+string(disposition)+"-"+state, disposition)
			operationID := seedAssignmentOperation(t, rt, assignment, state, "")
			rt.closeEndedAssignmentSelfdevOperation(ctx, assignment)
			operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
			if err != nil {
				t.Fatal(err)
			}
			if operation.State != state || operation.TerminalError != "" {
				t.Errorf("%s assignment moved a %s operation to %s %q", disposition, state, operation.State, operation.TerminalError)
			}
		}
	}
}

// DeepSeek review of bunch 15: on the tape, a competing writer that does
// not share this store's write mutex (another process, or another Store
// over the same tape) loses its CAS as ErrProjectionMismatch, not
// selfdev.ErrConflict. The close must retry that too, or a failed
// assignment's candidate frozen by the competing writer stays open.
func TestAssignmentEndCloseRetriesATapeConflict(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-close-tape-race"
	productStore := tapeBoundStore(t, computerID)
	closer, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	closer.BindProjectionSink(productStore)
	freezer, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	freezer.BindProjectionSink(productStore)
	rt := &Runtime{selfdevOperations: closer}
	authorized := func(context.Context) error { return nil }
	for i := 0; i < tapeRaceIterations; i++ {
		assignmentID := fmt.Sprintf("assignment-tape-race-%d", i)
		opened, err := openFreezeCandidate(ctx, closer, authorized, computerID, "trajectory-"+assignmentID, assignmentID, "objective")
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = freezer.Transition(ctx, computerID, opened.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
				next.BundleDigest = strings.Repeat("f", 64)
				return nil
			})
		}()
		go func() {
			defer wg.Done()
			rt.closeEndedAssignmentSelfdevOperation(ctx, endedAssignment(computerID, assignmentID, types.EngineeringAssignmentFailed))
		}()
		wg.Wait()
		operation, err := closer.Get(ctx, computerID, opened.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		if operation.State != selfdev.StateFailed {
			t.Fatalf("race %d on the tape: a failed assignment's operation = %s, want failed", i, operation.State)
		}
	}
}

// Bunch 19 (DeepSeek, Devin, flash): the cancellation close,
// failSelfdevOperation, stopped on ErrProjectionMismatch. Two Stores over
// one tape do not share a write mutex; production has that shape in the
// rematerialize rebind window (rematerialize.go swaps rt.selfdevOperations
// while the old Store may still be writing). A cancelled assignment's
// operation must end failed whichever writer wins.
func TestCancellationCloseRetriesATapeConflict(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-cancel-tape-race"
	productStore := tapeBoundStore(t, computerID)
	closer, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	closer.BindProjectionSink(productStore)
	freezer, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	freezer.BindProjectionSink(productStore)
	rt := &Runtime{selfdevOperations: closer}
	authorized := func(context.Context) error { return nil }
	for i := 0; i < tapeRaceIterations; i++ {
		assignmentID := fmt.Sprintf("assignment-cancel-race-%d", i)
		opened, err := openFreezeCandidate(ctx, closer, authorized, computerID, "trajectory-"+assignmentID, assignmentID, "objective")
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = freezer.Transition(ctx, computerID, opened.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
				next.BundleDigest = strings.Repeat("f", 64)
				return nil
			})
		}()
		go func() {
			defer wg.Done()
			rt.failBoundSelfdevOperation(ctx, computerID, assignmentID, "management cancelled the assignment")
		}()
		wg.Wait()
		operation, err := closer.Get(ctx, computerID, opened.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		if operation.State != selfdev.StateFailed || operation.TerminalError != "management cancelled the assignment" {
			t.Fatalf("race %d on the tape: a cancelled assignment's operation = %s %q, want failed", i, operation.State, operation.TerminalError)
		}
	}
}

// tapeRaceIterations is how many two-Store races a tape-conflict test runs.
// Measured 2026-10-22 under -race (GOMAXPROCS=2): the losing CAS was a
// projection mismatch on 28 to 30 of 30 iterations per run, so eight
// iterations miss the race with probability about 0.07^8.
const tapeRaceIterations = 8

// shortenSelfdevCloseTimeout lets a wedged-append test hit the close's own
// deadline in a fraction of a second instead of the production 15s. The
// tests' 20s no-deadline guard is unchanged: a close with no deadline still
// fails them.
func shortenSelfdevCloseTimeout(t *testing.T) {
	t.Helper()
	previous := selfdevCloseTimeout
	selfdevCloseTimeout = 300 * time.Millisecond
	t.Cleanup(func() { selfdevCloseTimeout = previous })
}

// blockingProjectionSink never commits: it waits for its context to end.
type blockingProjectionSink struct{}

func (blockingProjectionSink) AppendProjectionOps(ctx context.Context, _ []computerevent.ProjectionOp) error {
	<-ctx.Done()
	return ctx.Err()
}

// Bunch 19 item 2: the close detaches from the request's cancellation but
// must keep a deadline of its own, as settleRulePrecommits does; a wedged
// tape append must not hold the report path forever.
func TestAssignmentEndCloseIsBounded(t *testing.T) {
	shortenSelfdevCloseTimeout(t)
	rt, _ := testRuntime(t)
	computerID := rt.TextureComputerID()
	assignment := endedAssignment(computerID, "assignment-close-bounded", types.EngineeringAssignmentCompleted)
	seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "freeze: refused")
	rt.selfdevOperations.BindProjectionSink(blockingProjectionSink{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.closeEndedAssignmentSelfdevOperation(context.Background(), assignment)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the close did not return within 20s of a wedged tape append; it has no deadline")
	}
	assertOperationUntouched(t, rt, computerID, store.EngineeringFreezeOperationID(computerID, assignment.AssignmentID), selfdev.StateExecuting)
}

// assertOperationUntouched checks a close that gave up left the operation
// in its prior state, whole: still open, so the obligations surface shows
// it stalled, and never half-written.
func assertOperationUntouched(t *testing.T, rt *Runtime, computerID, operationID, state string) {
	t.Helper()
	operation, err := rt.selfdevOperations.Get(context.Background(), computerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.State != state || operation.TerminalError != "" {
		t.Fatalf("a close that timed out changed the operation: %s %q, want %s with no terminal error", operation.State, operation.TerminalError, state)
	}
}

// Bunch 24 item 2: the cancellation close (failSelfdevOperation, behind
// every assignment and trajectory cancellation) holds selfdev.Store's write
// mutex, and on the reconcile paths engineeringAssignmentOpenMu, across the
// tape append. A wedged append must not hold them forever.
func TestCancellationCloseIsBounded(t *testing.T) {
	shortenSelfdevCloseTimeout(t)
	rt, _ := testRuntime(t)
	computerID := rt.TextureComputerID()
	assignment := endedAssignment(computerID, "assignment-cancel-bounded", types.EngineeringAssignmentCancelled)
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "")
	rt.selfdevOperations.BindProjectionSink(blockingProjectionSink{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.failBoundSelfdevOperation(context.Background(), computerID, assignment.AssignmentID, "management cancelled the assignment")
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the cancellation close did not return within 20s of a wedged tape append; it has no deadline")
	}
	assertOperationUntouched(t, rt, computerID, operationID, selfdev.StateExecuting)
}

// Bunch 24 item 1: the same-digest slot replay return in
// recordAssignedEngineeringReportOnce. A stored terminal report with the
// same proposition under another report id is a row the store still
// builds: its report_id check is gated on a non-empty proposition digest in
// the request, and it keeps the caller's id. The re-delivered report's
// derived id is not stored, so the slot scan matches and returns the
// recorded receipt early; the entry-point close must still close.
func TestSlotReplayClosesTheOperation(t *testing.T) {
	ctx := context.Background()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-slot-replay", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "freeze: refused")

	derived := pinnedAssignmentReport(assignment, types.EngineeringResultCompleted)
	legacy := derived
	legacy.ReportID = "report-legacy-" + assignment.AssignmentID
	legacy.PropositionDigest = ""
	req := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "engineering-report:" + assignment.AssignmentID + ":" + legacy.ReportID,
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: legacy,
	}
	req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
	committed, err := s.RecordEngineeringAssignmentReport(ctx, req)
	if err != nil || !committed.Assignment.Disposition.Terminal() {
		t.Fatalf("legacy-id report = %+v, %v", committed.Assignment.Disposition, err)
	}
	if _, err := s.GetEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, derived.ReportID); err == nil {
		t.Fatal("setup: the derived report id is stored; the replay would not take the slot path")
	}
	matchID, _, _, err := s.SlotTerminalReport(ctx, c1eGet(t, s, assignment), derived.PropositionDigest)
	if err != nil || matchID != legacy.ReportID {
		t.Fatalf("setup: slot match = %q, %v; want the legacy-id report", matchID, err)
	}

	if _, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:slot-replay", derived); err != nil {
		t.Fatalf("slot replay: %v", err)
	}
	operation, err := operations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.State != selfdev.StateFailed || !strings.Contains(operation.TerminalError, "before its candidate froze") {
		t.Fatalf("operation after the slot replay = %s %q, want failed", operation.State, operation.TerminalError)
	}
}
