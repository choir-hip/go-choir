package agentcore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R1 on the completed path (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
// "R1 reached on the completed path", round 5c on staging): a
// Complete("completed") whose latest cited receipt observed the pre-edit tree
// was refused after the capsule had durably frozen, and every later fate,
// Complete("blocked") included, was refused at the doorway. Ways the fix can
// fail, written first: the refusal still freezes the capsule or stages a
// proposal; the doorway still refuses after a refusal; a corrected completed
// or a blocked report after the refusal does not record a terminal fate;
// the pre-check is skipped for blocked, so a foreign ref still strands; the
// race (the tree changes between the pre-check and the freeze) still leaves
// the assignment frozen and bound instead of ending it with its cause; the
// certified rule is relaxed for completed.

// r1FinalSubjectErr is what the capsule binder returns when the latest
// cited receipt did not observe the frozen tree.
var r1FinalSubjectErr = capsule.ErrFinalSubjectNotCertified

// r1Capsule is the saga's executor over one capsule whose worktree digest
// is a field. It applies the binder's rules (run, capsule, source per
// receipt; certified: the latest receipt observed the tree) and counts the
// freezes, so a test can see whether a refusal froze anything.
type r1Capsule struct {
	mu         sync.Mutex
	runID      string
	capsuleID  string
	subject    string
	tree       string
	frozen     bool
	freezes    int
	preflights int
	receipts   map[string]capsule.ExecutionReceipt
	// bindErr, when set, is what the post-freeze bind returns: a failure
	// that is not the final-subject refusal.
	bindErr error
	// atFreeze runs inside ExtractGranted before the capsule freezes: the
	// race, a writer that changed the tree after the pre-check.
	atFreeze func(*r1Capsule)
}

func (c *r1Capsule) AssignmentHandle(runID, capsuleID string) (string, error) {
	if runID != c.runID || capsuleID != c.capsuleID {
		return "", fmt.Errorf("capsule assignment capability unavailable")
	}
	return "handle-r1", nil
}

func (c *r1Capsule) ExtractGranted(context.Context, string, string) ([]capsule.FileChange, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.frozen && c.atFreeze != nil {
		c.atFreeze(c)
	}
	c.frozen = true
	c.freezes++
	return nil, nil
}

func (c *r1Capsule) InspectCapsuleRaw(id string) (*capsule.CapsuleDiagnostics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := capsule.StateActive
	if c.frozen {
		state = capsule.StateFrozen
	}
	return &capsule.CapsuleDiagnostics{ID: id, State: state}, nil
}

func (c *r1Capsule) ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.frozen {
		return "", fmt.Errorf("capsule granted worktree requires frozen capsule")
	}
	return c.tree, nil
}

func (c *r1Capsule) PersistGrantedFreezeReceipt(context.Context, string, string) (capsule.CapsuleFateReceipt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return capsule.CapsuleFateReceipt{
		ReceiptRef: "capsule-fate:" + objectgraph.SHA256([]byte("freeze\x00"+c.capsuleID+"\x00"+c.tree)),
		AgentRunID: c.runID, CapsuleID: c.capsuleID, Disposition: "frozen",
		SourceSubjectDigest: c.subject, FinalSubjectDigest: c.tree, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (c *r1Capsule) PersistGrantedCandidate(context.Context, string, string) (capsule.SourcePreflight, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return capsule.SourcePreflight{SubjectDigest: c.tree, ArtifactRef: "capsule-subject:" + c.tree}, nil
}

// bind applies the binder's rules against tree.
func (c *r1Capsule) bind(refs []string, binding capsule.SubjectBinding, tree string) ([]capsule.ExecutionReceipt, error) {
	out := make([]capsule.ExecutionReceipt, 0, len(refs))
	latestAt, latestTree := "", ""
	for _, ref := range refs {
		receipt, ok := c.receipts[ref]
		if !ok {
			return nil, fmt.Errorf("capsule execution evidence does not bind the exact run, handle, capsule, and frozen source: %s: unknown", ref)
		}
		if receipt.AgentRunID != c.runID || receipt.CapsuleID != c.capsuleID || receipt.SourceTreeDigest != c.subject {
			return nil, fmt.Errorf("capsule execution evidence does not bind the exact run, handle, capsule, and frozen source: %s", ref)
		}
		if receipt.OccurredAt > latestAt {
			latestAt, latestTree = receipt.OccurredAt, receipt.WorktreeDigest
		}
		out = append(out, receipt)
	}
	if binding == capsule.SubjectCertified && latestTree != tree {
		return nil, fmt.Errorf("%w: latest receipt observed %s, frozen subject is %s", r1FinalSubjectErr, latestTree, tree)
	}
	return out, nil
}

func (c *r1Capsule) ResolveGrantedExecutionReceiptsFor(_ context.Context, _, _ string, refs []string, binding capsule.SubjectBinding) ([]capsule.ExecutionReceipt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.frozen {
		return nil, fmt.Errorf("capsule execution evidence requires frozen capsule")
	}
	if c.bindErr != nil {
		return nil, c.bindErr
	}
	bound, err := c.bind(refs, binding, c.tree)
	if err != nil {
		return nil, err
	}
	for i := range bound {
		bound[i].GrantedReceiptRef = "capsule-granted-exec:" + objectgraph.SHA256([]byte(bound[i].ReceiptRef+"\x00"+string(binding)+"\x00"+c.tree))
	}
	return bound, nil
}

// PreflightGrantedExecutionReceipts is the pre-check against the live tree.
func (c *r1Capsule) PreflightGrantedExecutionReceipts(_ context.Context, _, _ string, refs []string, binding capsule.SubjectBinding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.preflights++
	_, err := c.bind(refs, binding, c.tree)
	return err
}

func (c *r1Capsule) receipt(command, observed string, at time.Time) capsule.ExecutionReceipt {
	ref := "capsule-exec:" + objectgraph.SHA256([]byte(command+"\x00"+observed+"\x00"+at.String()))
	receipt := capsule.ExecutionReceipt{
		ReceiptRef: ref, AgentRunID: c.runID, CapsuleID: c.capsuleID, Command: command,
		StdoutDigest: objectgraph.SHA256([]byte("out:" + command)), StderrDigest: objectgraph.SHA256([]byte("")),
		SourceTreeDigest: c.subject, WorktreeDigest: observed, OccurredAt: at.UTC().Format(time.RFC3339Nano),
	}
	c.receipts[ref] = receipt
	return receipt
}

type r1Fixture struct {
	rt   *Runtime
	s    *store.Store
	caps *r1Capsule
	run  types.RunRecord
	a    types.EngineeringAssignment
	// edit and check observed the edited tree; stale, the latest, observed
	// the assignment's source subject (round 5c's 77fc3b13).
	edit, check, stale capsule.ExecutionReceipt
}

func newR1Fixture(t *testing.T, owner string) *r1Fixture {
	t.Helper()
	rt, s, _ := c1eRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	edited := objectgraph.SHA256([]byte("tree after the App.svelte edit"))
	caps := &r1Capsule{runID: assignment.BoundRunID, capsuleID: assignment.Binding.CapsuleID,
		subject: assignment.Binding.SubjectDigest, tree: edited, receipts: map[string]capsule.ExecutionReceipt{}}
	rt.assignmentFateRuntime = caps
	base := time.Now().UTC().Add(-time.Minute)
	f := &r1Fixture{rt: rt, s: s, caps: caps, run: run, a: assignment}
	f.edit = caps.receipt("go_eval:edit App.svelte", edited, base)
	f.check = caps.receipt("npm run build", edited, base.Add(10*time.Second))
	f.stale = caps.receipt("go_eval:capture", assignment.Binding.SubjectDigest, base.Add(20*time.Second))
	return f
}

func (f *r1Fixture) complete(t *testing.T, result types.EngineeringAssignmentResultKind, call string, receipts ...capsule.ExecutionReceipt) (types.EngineeringAssignmentCommandResult, error) {
	t.Helper()
	report := types.EngineeringAssignmentReport{Result: result, Verdict: types.EngineeringVerdictNone,
		Summary: "reader sign-in state, " + string(result), Commands: recordedCommandsFromReceipts(receipts), Outputs: recordedOutputsFromReceipts(receipts)}
	return f.rt.recordAssignedEngineeringReport(context.Background(), &f.run, call, report)
}

func (f *r1Fixture) get(t *testing.T) types.EngineeringAssignment {
	t.Helper()
	return c1eGet(t, f.s, f.a)
}

// The staging shape: the refused Complete leaves the assignment active and
// its capsule unfrozen, the doorway open, and a corrected completed or a
// blocked Complete then records a terminal fate.
func TestCompleteRefusedForAStaleLatestReceiptLeavesTheAssignmentActive(t *testing.T) {
	for _, next := range []struct {
		name   string
		result types.EngineeringAssignmentResultKind
		cite   func(f *r1Fixture) []capsule.ExecutionReceipt
	}{
		{"corrected completed", types.EngineeringResultCompleted, func(f *r1Fixture) []capsule.ExecutionReceipt { return []capsule.ExecutionReceipt{f.edit, f.check} }},
		{"blocked", types.EngineeringResultBlocked, func(f *r1Fixture) []capsule.ExecutionReceipt {
			return []capsule.ExecutionReceipt{f.edit, f.check, f.stale}
		}},
	} {
		t.Run(next.name, func(t *testing.T) {
			f := newR1Fixture(t, "owner-r1-"+strings.ReplaceAll(next.name, " ", "-"))
			ctx := context.Background()

			_, err := f.complete(t, types.EngineeringResultCompleted, "rlm-complete:r1:stale", f.edit, f.check, f.stale)
			if err == nil || !strings.Contains(err.Error(), "does not bind the final frozen subject") {
				t.Fatalf("a completed report whose latest receipt observed the pre-edit tree = %v, want the final-subject refusal", err)
			}
			after := f.get(t)
			if after.Disposition != types.EngineeringAssignmentBound || after.CapsuleDisposition != types.EngineeringCapsuleActive || after.PendingProposal != nil {
				t.Fatalf("after the refusal the assignment is %s/%s (pending proposal %v); want bound/active with nothing staged",
					after.Disposition, after.CapsuleDisposition, after.PendingProposal != nil)
			}
			if f.caps.freezes != 0 {
				t.Fatalf("the refused Complete froze the capsule %d time(s)", f.caps.freezes)
			}
			if _, err := f.rt.assignedEngineeringObligation(ctx, &f.run); err != nil {
				t.Fatalf("the doorway refuses the next cell after the refusal: %v", err)
			}

			committed, err := f.complete(t, next.result, "rlm-complete:r1:next", next.cite(f)...)
			if err != nil {
				t.Fatalf("%s Complete after the refusal: %v", next.name, err)
			}
			if !committed.Assignment.Disposition.Terminal() || committed.Report == nil || committed.Report.Result != next.result {
				t.Fatalf("%s Complete recorded %s with report %+v; want a terminal %s fate", next.name, committed.Assignment.Disposition, committed.Report, next.result)
			}
			if f.caps.freezes != 1 {
				t.Fatalf("the recorded fate froze the capsule %d time(s), want once", f.caps.freezes)
			}
		})
	}
}

// A blocked or failed Complete that cites a ref this run did not mint is
// refused before the freeze too: it stranded the same way.
func TestBlockedCompleteCitingAForeignRefIsRefusedBeforeTheFreeze(t *testing.T) {
	f := newR1Fixture(t, "owner-r1-foreign")
	foreign := f.check
	foreign.ReceiptRef, foreign.AgentRunID = "capsule-exec:"+objectgraph.SHA256([]byte("foreign")), "run:someone-else"
	f.caps.receipts[foreign.ReceiptRef] = foreign

	if _, err := f.complete(t, types.EngineeringResultBlocked, "rlm-complete:r1:foreign", f.edit, foreign); err == nil {
		t.Fatal("a blocked report citing a foreign receipt recorded")
	}
	after := f.get(t)
	if after.CapsuleDisposition != types.EngineeringCapsuleActive || after.PendingProposal != nil || f.caps.freezes != 0 {
		t.Fatalf("the refused blocked Complete left %s/%s (pending %v, freezes %d); want active, nothing staged, unfrozen",
			after.Disposition, after.CapsuleDisposition, after.PendingProposal != nil, f.caps.freezes)
	}
	if committed, err := f.complete(t, types.EngineeringResultBlocked, "rlm-complete:r1:own", f.edit); err != nil || !committed.Assignment.Disposition.Terminal() {
		t.Fatalf("the blocked Complete citing its own receipt = %v, %v", committed.Assignment.Disposition, err)
	}
}

// The race: the pre-check passes, then a writer changes the tree before the
// freeze, so the authoritative post-freeze bind refuses. The assignment is
// not stranded frozen and bound: it ends, with the refusal as its cause.
func TestCompleteWhoseTreeChangesAfterThePreCheckEndsWithItsCause(t *testing.T) {
	f := newR1Fixture(t, "owner-r1-race")
	f.caps.atFreeze = func(c *r1Capsule) { c.tree = objectgraph.SHA256([]byte("a background writer's tree")) }

	_, err := f.complete(t, types.EngineeringResultCompleted, "rlm-complete:r1:race", f.edit, f.check)
	if err == nil || !strings.Contains(err.Error(), "does not bind the final frozen subject") {
		t.Fatalf("a completed report whose tree changed at the freeze = %v, want the final-subject refusal", err)
	}
	after := f.get(t)
	if !after.Disposition.Terminal() || after.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("after the post-freeze refusal the assignment is %s/%s; want terminal with its capsule revoked, not stranded",
			after.Disposition, after.CapsuleDisposition)
	}
	if !strings.Contains(after.DispositionReason, "does not bind the final frozen subject") {
		t.Fatalf("the assignment ended without its cause: %q", after.DispositionReason)
	}
	if assignedEngineeringFatePending(after) {
		t.Fatal("the ended assignment still owes a fate re-drive")
	}
	if len(after.ReportRefs) == 0 {
		t.Fatal("the ended assignment recorded no report")
	}
	report, err := f.s.GetEngineeringAssignmentReportByCanonicalID(context.Background(), after.ReportRefs[len(after.ReportRefs)-1])
	if err != nil {
		t.Fatal(err)
	}
	if report.Result != types.EngineeringResultFailed || !strings.Contains(report.Summary, "does not bind the final frozen subject") {
		t.Fatalf("the recorded fate is %s %q; want failed, naming the refusal", report.Result, report.Summary)
	}
}

// Only the final-subject refusal ends the assignment: any other post-freeze
// bind failure (an I/O error, a persist) may pass, so it keeps today's
// strand and its re-drive.
func TestOtherPostFreezeBindFailuresKeepTheRedrive(t *testing.T) {
	f := newR1Fixture(t, "owner-r1-transient")
	f.caps.bindErr = fmt.Errorf("persist executor receipt artifact: disk full")

	if _, err := f.complete(t, types.EngineeringResultCompleted, "rlm-complete:r1:transient", f.edit, f.check); err == nil {
		t.Fatal("a completed report whose post-freeze bind failed recorded")
	}
	after := f.get(t)
	if after.Disposition.Terminal() || !assignedEngineeringFatePending(after) {
		t.Fatalf("a transient post-freeze failure left %s/%s (fate pending %v); want the staged proposal kept for its re-drive",
			after.Disposition, after.CapsuleDisposition, assignedEngineeringFatePending(after))
	}
}

// A re-drive of a saga that already froze (the watchdog or planned-restart
// resume of the staged proposal) does not run the pre-check again: it starts
// past the freeze intent, and the post-freeze bind is its check (review 281,
// DeepSeek 5).
func TestResumedFrozenSagaSkipsThePreCheck(t *testing.T) {
	f := newR1Fixture(t, "owner-r1-resume")
	f.caps.bindErr = fmt.Errorf("persist executor receipt artifact: disk full")
	if _, err := f.complete(t, types.EngineeringResultCompleted, "rlm-complete:r1:resume", f.edit, f.check); err == nil {
		t.Fatal("fixture: the transient post-freeze failure recorded")
	}
	stranded := f.get(t)
	if !assignedEngineeringFatePending(stranded) || f.caps.preflights != 1 || f.caps.freezes != 1 {
		t.Fatalf("fixture: %s/%s pending %v, preflights %d, freezes %d; want a frozen strand after one pre-check",
			stranded.Disposition, stranded.CapsuleDisposition, assignedEngineeringFatePending(stranded), f.caps.preflights, f.caps.freezes)
	}

	f.caps.bindErr = nil
	if err := f.rt.resumeStrandedFrozenAssignmentCommit(context.Background(), stranded); err != nil {
		t.Fatalf("resume of the frozen saga: %v", err)
	}
	if f.caps.preflights != 1 {
		t.Fatalf("the resumed frozen saga ran the pre-check again (%d pre-checks)", f.caps.preflights)
	}
	if f.caps.freezes != 1 {
		t.Fatalf("the resumed frozen saga froze again (%d freezes)", f.caps.freezes)
	}
	if after := f.get(t); after.Disposition != types.EngineeringAssignmentCompleted {
		t.Fatalf("the resumed saga left the assignment %s/%s, want completed", after.Disposition, after.CapsuleDisposition)
	}
}
