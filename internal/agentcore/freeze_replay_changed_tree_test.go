package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R9-1 (docs/problems/freeze-replays-a-changed-tree-2026-10-26.md): on
// staging an engineer froze bundle 359777e1, removed 17 stray
// frontend/test-results files in the capsule and called Freeze again; the
// second Freeze answered the frozen operation as if it had frozen the new
// tree. Ways the fix can fail, written first: an unchanged tree refuses (a
// retry after a committed Freeze breaks); a changed tree still replays; a
// content-only change replays; /tmp churn refuses; the check cannot read
// the tree and replays anyway; the path list floods the cell; the refusal
// strands the assignment so it cannot Complete.

// replayTree is the executor read the replay check takes: the receipts the
// frozen bundle cites and the capsule's live tree.
type replayTree struct {
	receipts map[string]capsule.ExecutionReceipt
	live     capsule.GrantedWorktree
	err      error
	reads    int
}

func (r *replayTree) ResolveExecutionReceipts(refs []string) ([]capsule.ExecutionReceipt, error) {
	out := make([]capsule.ExecutionReceipt, 0, len(refs))
	for _, ref := range refs {
		receipt, ok := r.receipts[ref]
		if !ok {
			return nil, fmt.Errorf("capsule execution receipt %s unavailable", ref)
		}
		out = append(out, receipt)
	}
	return out, nil
}

func (r *replayTree) InspectGrantedWorktree(context.Context, string, string) (capsule.GrantedWorktree, error) {
	r.reads++
	if r.err != nil {
		return capsule.GrantedWorktree{}, r.err
	}
	return r.live, nil
}

// replayOperations opens a self-development store with a genesis head for
// computerID, so an operation can start there.
func replayOperations(t *testing.T, computerID string) (*choirstore.Store, *selfdev.Store) {
	t.Helper()
	ctx := context.Background()
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { productStore.Close() })
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
	return productStore, operations
}

// frozenReplay is one assignment whose operation froze a bundle over
// frozenChanges, citing three receipts the latest of which observed
// frozenTree.
type frozenReplay struct {
	toolCtx                 *CapsuleToolCtx
	rec                     *types.RunRecord
	tree                    *replayTree
	operation               selfdev.Operation
	frozenChanges           []capsule.FileChange
	frozenTree              string
	recipe, test, toolchain string
}

func newFrozenReplay(t *testing.T, computerID, assignmentID, runID, capsuleID string, receipts map[string]capsule.ExecutionReceipt, recipe, test, toolchain string) *frozenReplay {
	t.Helper()
	ctx := context.Background()
	productStore, operations := replayOperations(t, computerID)
	operation, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: choirstore.EngineeringFreezeIdempotencyKey(assignmentID),
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
		t.Fatal(err)
	}
	f := &frozenReplay{recipe: recipe, test: test, toolchain: toolchain}
	// The round-2 candidate: the window manager edit plus the stray
	// Playwright output the engineer removed after the freeze.
	f.frozenChanges = []capsule.FileChange{
		{Path: "/workspace/platform/frontend/src/lib/WindowManager.svelte", Kind: capsule.ChangeModified, Mode: 0o644},
	}
	for i := 0; i < 17; i++ {
		f.frozenChanges = append(f.frozenChanges, capsule.FileChange{
			Path: fmt.Sprintf("/workspace/platform/frontend/test-results/run-%02d/trace.zip", i), Kind: capsule.ChangeAdded, Mode: 0o644,
		})
	}
	builder := transaction.NewTransactionBuilder(transaction.NewClassifier())
	record, err := builder.BuildBundleFromDiff(capsuleID, f.frozenChanges)
	if err != nil || record.Rejected {
		t.Fatalf("frozen bundle = %+v, %v", record, err)
	}
	record.ComputerID, record.BaseEventHead, record.TrajectoryRef = computerID, operation.BaseHead, operation.TrajectoryID
	record.BuildRecipeRef, record.TestReceipts, record.DependencyToolchainRefs = recipe, []string{test}, []string{toolchain}
	record.VerifierReceipts = []string{}
	record.ContentDigest, err = record.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := computerevent.CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}
	updaterRoot := t.TempDir()
	bundleRoot := filepath.Join(updaterRoot, "incoming", record.ContentDigest)
	if err := os.MkdirAll(bundleRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleRoot, "bundle.draft.json"), draft, 0o400); err != nil {
		t.Fatal(err)
	}
	operation, err = operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.CapsuleID, next.BundleDigest = capsuleID, record.ContentDigest
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.operation = operation
	f.frozenTree = receipts[test].WorktreeDigest
	f.tree = &replayTree{receipts: receipts, live: capsule.GrantedWorktree{Digest: f.frozenTree, Changes: f.frozenChanges}}
	f.rec = &types.RunRecord{RunID: runID, TrajectoryID: operation.TrajectoryID, Metadata: map[string]any{"assignment_id": assignmentID}}
	f.toolCtx = &CapsuleToolCtx{
		ComputerID: computerID, AgentRunID: runID, UpdaterRoot: updaterRoot,
		Executor:           capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0),
		OperationStore:     operations,
		EventProjection:    freezeTestProjection{store: productStore},
		EventAppender:      new(computerevent.ComputerEventAppender),
		TransactionBuilder: builder,
		frozenTree:         f.tree,
	}
	return f
}

func (f *frozenReplay) freeze() (map[string]any, error) {
	return freezeCapsuleEffectBundle(context.Background(), f.toolCtx, f.rec, "handle-r9", f.recipe, []string{f.test}, []string{f.toolchain})
}

// replayReceipts mints the three receipts the first Freeze cited: the
// build and toolchain earlier, the test last, on the frozen tree.
func replayReceipts(runID, capsuleID, source, frozenTree string) (map[string]capsule.ExecutionReceipt, string, string, string) {
	base := time.Now().UTC().Add(-time.Hour)
	mint := func(command, tree string, at time.Time) capsule.ExecutionReceipt {
		return capsule.ExecutionReceipt{
			ReceiptRef: "capsule-exec:" + objectgraph.SHA256([]byte(command)), AgentRunID: runID, CapsuleID: capsuleID,
			Command: command, SourceTreeDigest: source, WorktreeDigest: tree, OccurredAt: at.Format(time.RFC3339Nano),
		}
	}
	recipe := mint("npm run build", objectgraph.SHA256([]byte("tree before the test run")), base)
	toolchain := mint("node --version", objectgraph.SHA256([]byte("tree before the test run")), base.Add(time.Second))
	test := mint("npx playwright test", frozenTree, base.Add(time.Minute))
	receipts := map[string]capsule.ExecutionReceipt{recipe.ReceiptRef: recipe, toolchain.ReceiptRef: toolchain, test.ReceiptRef: test}
	return receipts, recipe.ReceiptRef, test.ReceiptRef, toolchain.ReceiptRef
}

func newStagingReplay(t *testing.T) *frozenReplay {
	t.Helper()
	frozenTree := objectgraph.SHA256([]byte("tree with the stray test-results"))
	receipts, recipe, test, toolchain := replayReceipts("run-r9", "capsule-r9", strings.Repeat("5", 64), frozenTree)
	return newFrozenReplay(t, "computer-r9", "assignment-r9", "run-r9", "capsule-r9", receipts, recipe, test, toolchain)
}

func (f *frozenReplay) replayAnswer() map[string]any {
	return map[string]any{"handle": "handle-r9", "bundle_digest": f.operation.BundleDigest, "operation_id": f.operation.OperationID, "state": f.operation.State}
}

// A Freeze retried over the tree it froze answers exactly what it answers
// today, and so does one whose only new paths are ignored (/tmp).
func TestSecondFreezeOverTheFrozenTreeReplays(t *testing.T) {
	f := newStagingReplay(t)
	for _, step := range []struct {
		name    string
		changes []capsule.FileChange
	}{
		{"unchanged", f.frozenChanges},
		{"tmp churn", append(append([]capsule.FileChange(nil), f.frozenChanges...),
			capsule.FileChange{Path: "/tmp/playwright-run/results.json", Kind: capsule.ChangeAdded, Mode: 0o644})},
	} {
		f.tree.live = capsule.GrantedWorktree{Digest: f.frozenTree, Changes: step.changes}
		out, err := f.freeze()
		if err != nil {
			t.Fatalf("%s: a second Freeze over the frozen tree refused: %v", step.name, err)
		}
		if !reflect.DeepEqual(out, f.replayAnswer()) {
			t.Fatalf("%s: a second Freeze over the frozen tree answered %+v, want the replay %+v", step.name, out, f.replayAnswer())
		}
	}
	if f.tree.reads != 2 {
		t.Fatalf("the replay check read the live tree %d time(s), want once per Freeze", f.tree.reads)
	}
}

// The staging shape: the stray files removed after the freeze. The second
// Freeze refuses, names the bundle and the removed paths (at most 20), and
// says the frozen candidate is what will be reviewed.
func TestSecondFreezeOverAChangedTreeNamesTheChange(t *testing.T) {
	f := newStagingReplay(t)
	f.tree.live = capsule.GrantedWorktree{Digest: objectgraph.SHA256([]byte("tree after rm -rf test-results")), Changes: f.frozenChanges[:1]}
	out, err := f.freeze()
	if err == nil {
		t.Fatalf("a second Freeze over a changed tree answered %+v; want a refusal naming the change", out)
	}
	msg := err.Error()
	for _, want := range []string{
		"already frozen as bundle " + f.operation.BundleDigest,
		"removed /workspace/platform/frontend/test-results/run-16/trace.zip",
		"frozen candidate is what will be reviewed",
		"new round",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal %q does not say %q", msg, want)
		}
	}
	if got := strings.Count(msg, "/workspace/platform/frontend/test-results/"); got != 17 {
		t.Fatalf("refusal lists %d of the 17 removed test-results paths: %q", got, msg)
	}

	// More than 20 changed paths: the first 20 and a count.
	flood := append([]capsule.FileChange(nil), f.frozenChanges...)
	for i := 0; i < 30; i++ {
		flood = append(flood, capsule.FileChange{Path: fmt.Sprintf("/workspace/platform/frontend/node_modules/.vite/dep-%02d.js", i), Kind: capsule.ChangeAdded, Mode: 0o644})
	}
	f.tree.live = capsule.GrantedWorktree{Digest: objectgraph.SHA256([]byte("tree after npm install")), Changes: flood}
	if _, err := f.freeze(); err == nil || strings.Count(err.Error(), "added /workspace/platform/") != 20 || !strings.Contains(err.Error(), "and 10 more") {
		t.Fatalf("a Freeze after 30 new paths = %v; want 20 listed and 10 counted", err)
	}

	// An edit after the freeze to a file already in the candidate keeps the
	// path set; the digests name it.
	changed := objectgraph.SHA256([]byte("WindowManager.svelte edited again"))
	f.tree.live = capsule.GrantedWorktree{Digest: changed, Changes: f.frozenChanges}
	if _, err := f.freeze(); err == nil || !strings.Contains(err.Error(), f.frozenTree) || !strings.Contains(err.Error(), changed) {
		t.Fatalf("a content-only change after the freeze = %v; want a refusal naming both tree digests", err)
	}

	// A tree the check cannot read is not an unchanged tree.
	f.tree.err = fmt.Errorf("capsule granted worktree unavailable")
	if _, err := f.freeze(); err == nil || !strings.Contains(err.Error(), "already frozen as bundle "+f.operation.BundleDigest) {
		t.Fatalf("a Freeze whose tree could not be read = %v; want a refusal naming the frozen bundle", err)
	}
	loaded, err := f.toolCtx.OperationStore.Get(context.Background(), f.toolCtx.ComputerID, f.operation.OperationID)
	if err != nil || loaded.State != selfdev.StateFrozen || loaded.BundleDigest != f.operation.BundleDigest {
		t.Fatalf("the refusals moved the operation: %+v, %v", loaded, err)
	}
}

// The refusal strands nothing: the assignment Completes after it, and the
// Complete names the frozen bundle as its candidate.
func TestCompleteAfterAChangedTreeFreezeRefusal(t *testing.T) {
	r := newR1Fixture(t, "owner-r9-complete")
	edited := r.caps.tree
	receipts := map[string]capsule.ExecutionReceipt{}
	for _, receipt := range []capsule.ExecutionReceipt{r.edit, r.check} {
		receipts[receipt.ReceiptRef] = receipt
	}
	toolchain := r.caps.receipt("node --version", edited, time.Now().UTC().Add(-50*time.Second))
	receipts[toolchain.ReceiptRef] = toolchain
	f := newFrozenReplay(t, r.a.Binding.ComputerID, r.a.AssignmentID, r.a.BoundRunID, r.a.Binding.CapsuleID, receipts, r.edit.ReceiptRef, r.check.ReceiptRef, toolchain.ReceiptRef)
	r.rt.selfdevOperations = f.toolCtx.OperationStore

	// After the freeze the engineer removes files: the tree moves, and the
	// rm's own receipt observed the new tree.
	removed := objectgraph.SHA256([]byte("tree after rm -rf test-results"))
	r.caps.tree = removed
	rm := r.caps.receipt("rm -rf frontend/test-results", removed, time.Now().UTC())
	f.tree.live = capsule.GrantedWorktree{Digest: removed, Changes: f.frozenChanges[:1]}
	if _, err := f.freeze(); err == nil {
		t.Fatal("a second Freeze over a changed tree replayed")
	}

	committed, err := r.complete(t, types.EngineeringResultCompleted, "rlm-complete:r9:after-refusal", r.check, rm)
	if err != nil {
		t.Fatalf("Complete after the changed-tree refusal: %v", err)
	}
	if !committed.Assignment.Disposition.Terminal() || committed.Report == nil || committed.Report.Result != types.EngineeringResultCompleted {
		t.Fatalf("Complete after the refusal recorded %s with report %+v; want a terminal completed fate", committed.Assignment.Disposition, committed.Report)
	}
	refs := r.rt.assignmentOutcomeRefs(context.Background(), committed.Assignment, *committed.Report)
	if !selfDevelopmentContainsString(refs, "bundle:"+f.operation.BundleDigest) {
		t.Fatalf("the Complete's outcome refs %v do not name the frozen bundle %s", refs, f.operation.BundleDigest)
	}
}
