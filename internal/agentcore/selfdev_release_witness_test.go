package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// S6-0 release witness. The failure modes these tests cover are written in
// docs/problems/s6-activation-trusts-not-compares-2026-10-20.md ("The release
// witness and how it could fail"): wrong join, stale read, a private-data
// leak, a compare claimed that no gate makes, a write, a partial row.

func witnessGet(t *testing.T, h *APIHandler, computerID, operationID string) (ReleaseWitness, *http.Response, string) {
	t.Helper()
	w := releaseRouterCall(t, h, http.MethodGet, "/api/computers/"+computerID+"/self-development/operations/"+operationID+"/release-witness", "owner-a", computerID, nil)
	var witness ReleaseWitness
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &witness); err != nil {
			t.Fatalf("witness does not decode: %v\n%s", err, w.Body.String())
		}
	}
	return witness, w.Result(), w.Body.String()
}

func witnessBinding(t *testing.T, witness ReleaseWitness, pair string) WitnessBinding {
	t.Helper()
	for _, binding := range witness.Bindings {
		if binding.Pair == pair {
			return binding
		}
	}
	t.Fatalf("witness has no binding %q", pair)
	return WitnessBinding{}
}

func requireBound(t *testing.T, witness ReleaseWitness, pair string, want *bool) {
	t.Helper()
	got := witnessBinding(t, witness, pair)
	switch {
	case want == nil && got.Bound != nil:
		t.Fatalf("%s: bound = %v (%s), want unknown", pair, *got.Bound, got.Reason)
	case want != nil && got.Bound == nil:
		t.Fatalf("%s: bound unknown (%s), want %v", pair, got.Reason, *want)
	case want != nil && *got.Bound != *want:
		t.Fatalf("%s: bound = %v (%s), want %v", pair, *got.Bound, got.Reason, *want)
	}
}

var (
	witnessTrue  = witnessBoolPtr(true)
	witnessFalse = witnessBoolPtr(false)
)

// witnessCountIn counts needle in the file under the package directory.
func witnessCountIn(t *testing.T, file, needle string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(file))
	if err != nil {
		t.Fatalf("gate source %s: %v", file, err)
	}
	return strings.Count(string(raw), needle)
}

// Failure mode 4: a compare claimed that no gate makes. The table's
// compared_at_gate is tied to the gate source: a compared row names text that
// must be present; a recorded-only row names text that must stay absent (or
// under its count). A change to the gate that adds or removes a compare fails
// here until selfdevGatePairs is updated with it.
func TestReleaseWitnessGateTableMatchesTheGateCode(t *testing.T) {
	for _, row := range selfdevGatePairs {
		if len(row.Anchors) == 0 {
			t.Errorf("%s: a row with no anchor in the gate source is a hand-asserted claim", row.Pair)
		}
		for _, anchor := range row.Anchors {
			count := witnessCountIn(t, anchor.File, anchor.Needle)
			switch {
			case anchor.Min > 0 && count < anchor.Min:
				t.Errorf("%s (compared=%v): %q appears %d times in %s, want at least %d; the gate changed, update selfdevGatePairs and the problem doc",
					row.Pair, row.Compared, anchor.Needle, count, anchor.File, anchor.Min)
			case anchor.Min == 0 && count > anchor.Max:
				t.Errorf("%s (compared=%v): %q appears %d times in %s, want at most %d; a compare was added to the gate, update selfdevGatePairs and the problem doc",
					row.Pair, row.Compared, anchor.Needle, count, anchor.File, anchor.Max)
			}
		}
		if row.Compared && row.ComparedAt == witnessAtNowhere {
			t.Errorf("%s: compared at nowhere", row.Pair)
		}
		if !row.Compared && row.ComparedAt != witnessAtNowhere && row.ComparedAt != witnessAtFreeze {
			t.Errorf("%s: not compared at the gate but ComparedAt = %q", row.Pair, row.ComparedAt)
		}
	}
}

func TestReleaseWitnessTableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range selfdevGatePairs {
		if row.Pair == "" || seen[row.Pair] || row.Left == "" || row.Right == "" || row.GateRef == "" {
			t.Fatalf("malformed or duplicate row: %+v", row)
		}
		seen[row.Pair] = true
	}
	// An observation cannot name a pair outside the table, and compared_at_gate
	// is not settable by an observation.
	defer func() {
		if recover() == nil {
			t.Fatal("an observation for an unknown pair was accepted")
		}
	}()
	b := newWitnessBindings()
	b.observe("test_receipts_exit", witnessTrue, "ok")
	list, _ := b.list()
	for _, binding := range list {
		for _, row := range selfdevGatePairs {
			if row.Pair == binding.Pair && binding.ComparedAtGate != row.Compared {
				t.Fatalf("%s: compared_at_gate %v differs from the table's %v", row.Pair, binding.ComparedAtGate, row.Compared)
			}
		}
	}
	b.observe("not_a_pair", witnessTrue, "x")
}

// The gate rows the table says refuse really refuse, and the row it says does
// not compare really has no refusal at the gate: the behavioral half of the
// anchors above.
func TestReleaseWitnessGateRowsBehaveAsTheTableSays(t *testing.T) {
	// test_receipts_exit: refused at Freeze (the table's only compare).
	failing := capsule.ExecutionReceipt{ReceiptRef: "capsule-exec:sha256:" + strings.Repeat("7", 64), ExitCode: 1}
	if err := freezeReceiptOutcomesError("capsule-exec:sha256:"+strings.Repeat("5", 64), []string{failing.ReceiptRef}, nil, []capsule.ExecutionReceipt{failing}); err == nil {
		t.Fatal("Freeze accepted a failing test receipt; the table's freeze row is wrong")
	}
	// decision_operation: the gate's verifier refuses a decision bound to a
	// different bundle.
	fx := newDerivableSelfDevFixture(t, "computer-witness-gate")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-witness-gate")
	fx.appendDecision(t, operation, bundleDigest, "decision-witness-gate")
	transition, found, err := fx.store.FinalizedDecisionForOperation(context.Background(), fx.computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil || !found {
		t.Fatalf("decision unavailable: %v %v", found, err)
	}
	if _, err := verifyFinalizedSelfDevelopmentDecision(operation, transition); err != nil {
		t.Fatalf("control decision refused: %v", err)
	}
	other := operation
	other.BundleDigest = strings.Repeat("1", 64)
	if _, err := verifyFinalizedSelfDevelopmentDecision(other, transition); err == nil {
		t.Fatal("the decision verifier accepted a different bundle digest; the decision_operation row is wrong")
	}
}

// A partial row, an unknown operation, the auth boundary and the method.
func TestReleaseWitnessRouteBoundariesAndPartialRows(t *testing.T) {
	const computerID = "computer-witness-route"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	h := NewAPIHandler(rt)
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-bare", computerID: computerID, state: selfdev.StateRequested})
	// A requested operation has no decision, checkpoint or route yet.
	if _, err := productStore.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET decision_event = '', checkpoint_ref = '', route_certificate = '' WHERE operation_id = 'op-bare'`); err != nil {
		t.Fatal(err)
	}
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-other", computerID: "computer-other", state: selfdev.StateRequested})
	_ = updaterRoot
	path := "/api/computers/" + computerID + "/self-development/operations/op-bare/release-witness"

	if w := releaseRouterCall(t, h, http.MethodGet, path, "", computerID, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no owner = %d, want 401", w.Code)
	}
	if w := releaseRouterCall(t, h, http.MethodGet, path, "owner-a", "computer-other", nil); w.Code != http.StatusForbidden {
		t.Fatalf("another computer's binding = %d, want 403", w.Code)
	}
	if w := releaseRouterCall(t, h, http.MethodPost, path, "owner-a", computerID, []byte("{}")); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", w.Code)
	}
	if _, resp, _ := witnessGet(t, h, computerID, "op-missing"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown operation = %d, want 404", resp.StatusCode)
	}
	// Failure mode 1(d): another computer's operation is not visible here.
	if _, resp, _ := witnessGet(t, h, computerID, "op-other"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("another computer's operation = %d, want 404", resp.StatusCode)
	}

	witness, resp, _ := witnessGet(t, h, computerID, "op-bare")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d cache-control %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if witness.Schema != releaseWitnessSchema || witness.OperationID != "op-bare" || witness.Operation.State != selfdev.StateRequested || !witness.ReadConsistent {
		t.Fatalf("witness header = %+v", witness)
	}
	// Failure mode 6: every pair is present, none is bound, each says why.
	if len(witness.Bindings) != len(selfdevGatePairs) {
		t.Fatalf("%d bindings, want %d", len(witness.Bindings), len(selfdevGatePairs))
	}
	for index, binding := range witness.Bindings {
		row := selfdevGatePairs[index]
		if binding.Pair != row.Pair || binding.ComparedAtGate != row.Compared || binding.ComparedAt != row.ComparedAt || binding.GateRef != row.GateRef {
			t.Fatalf("binding %d = %+v, table row %+v", index, binding, row)
		}
		if binding.Bound != nil && binding.Pair != "serving_current" {
			t.Errorf("%s bound on a bare operation: %v (%s)", binding.Pair, *binding.Bound, binding.Reason)
		}
		if strings.TrimSpace(binding.Reason) == "" {
			t.Errorf("%s has no reason", binding.Pair)
		}
	}
	if witness.Summary.Pairs != len(selfdevGatePairs) || witness.Summary.ComparedAtGate+witness.Summary.RecordedOnly != witness.Summary.Pairs {
		t.Fatalf("summary = %+v", witness.Summary)
	}
}

// witnessFrozenBundle stages a valid final bundle citing the given test
// receipts, bound to the operation row insertReleaseOperation writes.
func witnessFrozenBundle(t *testing.T, updaterRoot, computerID, operationID string, patch []byte, testReceipts []string) string {
	t.Helper()
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: "trajectory-" + operationID, CapsuleIdentity: "capsule-" + operationID,
		CapabilityPolicyDigest: strings.Repeat("2", 64), SourceTreeRef: "source-tree:sha256:" + strings.Repeat("3", 64),
		OrderedFileEffects:    []transaction.ChangeRecord{{Path: "frontend/index.html", Kind: "added", Mode: 0o444}},
		GeneratedArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("4", 64)},
		BuildRecipeRef:        "capsule-exec:sha256:" + strings.Repeat("5", 64), RuntimeArtifactRef: "runtime-artifact:sha256:" + strings.Repeat("6", 64),
		TestReceipts: testReceipts, VerifierReceipts: []string{strings.Repeat("f", 64)},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + strings.Repeat("8", 64)},
		ResourceReceipts:        []string{"artifact:sha256:" + strings.Repeat("9", 64)},
		ClassifierV:             "witness-test", ClassifierDigest: strings.Repeat("0", 64), Groups: map[string][]transaction.ChangeRecord{},
		RuntimeFiles: []capsule.FrozenReleaseFile{{Path: "frontend/index.html", SHA256: strings.Repeat("a", 64), Mode: 0o444}},
	}
	if len(patch) > 0 {
		bundle.SourcePatchSHA256, bundle.SourcePatchBaseCommit = computerevent.DigestBytes(patch), strings.Repeat("c", 40)
	}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	digest := computerevent.DigestBytes(raw)
	dir := filepath.Join(updaterRoot, "incoming", digest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	if len(patch) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "source.patch"), patch, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	return digest
}

func withWitnessReceipts(t *testing.T, fn func(ref string) (capsule.ExecutionReceipt, error)) *atomic.Int32 {
	t.Helper()
	calls := &atomic.Int32{}
	previous := witnessOpenExecutionReceipt
	witnessOpenExecutionReceipt = func(_ *Runtime, ref string) (capsule.ExecutionReceipt, error) {
		calls.Add(1)
		return fn(ref)
	}
	t.Cleanup(func() { witnessOpenExecutionReceipt = previous })
	return calls
}

// The declared test's exit status is compared at Freeze only: the witness
// shows a failing receipt as unbound AND not compared at the gate, and a
// passing one as bound but still not compared at the gate.
func TestReleaseWitnessShowsTestExitStatusAsRecordedNotCompared(t *testing.T) {
	const computerID = "computer-witness-test"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	h := NewAPIHandler(rt)
	passRef, failRef := "capsule-exec:sha256:"+strings.Repeat("7", 64), "capsule-exec:sha256:"+strings.Repeat("b", 64)
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-pass", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: witnessFrozenBundle(t, updaterRoot, computerID, "op-pass", nil, []string{passRef})})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-fail", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: witnessFrozenBundle(t, updaterRoot, computerID, "op-fail", nil, []string{failRef})})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-gone", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: witnessFrozenBundle(t, updaterRoot, computerID, "op-gone", nil, []string{"capsule-exec:sha256:" + strings.Repeat("d", 64)})})
	secretCommand := "go test ./... # SECRET-COMMAND-TEXT"
	withWitnessReceipts(t, func(ref string) (capsule.ExecutionReceipt, error) {
		switch ref {
		case passRef:
			return capsule.ExecutionReceipt{ReceiptRef: ref, Command: secretCommand, ExitCode: 0}, nil
		case failRef:
			return capsule.ExecutionReceipt{ReceiptRef: ref, Command: secretCommand, ExitCode: 2}, nil
		}
		return capsule.ExecutionReceipt{}, errWitnessNoExecutor
	})

	pass, _, passBody := witnessGet(t, h, computerID, "op-pass")
	requireBound(t, pass, "test_receipts_exit", witnessTrue)
	if b := witnessBinding(t, pass, "test_receipts_exit"); b.ComparedAtGate || b.ComparedAt != witnessAtFreeze {
		t.Fatalf("test_receipts_exit = %+v, want not compared at the gate, compared at freeze", b)
	}
	requireBound(t, pass, "bundle_operation", witnessTrue)

	fail, _, failBody := witnessGet(t, h, computerID, "op-fail")
	requireBound(t, fail, "test_receipts_exit", witnessFalse)
	if len(fail.TestResult.Receipts) != 1 || fail.TestResult.Receipts[0].ExitCode == nil || *fail.TestResult.Receipts[0].ExitCode != 2 ||
		fail.TestResult.Receipts[0].OutcomeRefusal == "" || fail.TestResult.AllOK == nil || *fail.TestResult.AllOK {
		t.Fatalf("failing test result = %+v", fail.TestResult)
	}
	if fail.Summary.RecordedOnlyUnbound < 1 {
		t.Fatalf("a failing, uncompared test must count as recorded-only and unbound: %+v", fail.Summary)
	}

	gone, _, _ := witnessGet(t, h, computerID, "op-gone")
	requireBound(t, gone, "test_receipts_exit", nil)

	// Failure mode 3: the command is its digest, never its text.
	for name, body := range map[string]string{"pass": passBody, "fail": failBody} {
		if strings.Contains(body, "SECRET-COMMAND-TEXT") {
			t.Fatalf("%s witness leaks the receipt's command text", name)
		}
	}
	if pass.TestResult.Receipts[0].CommandSHA256 != computerevent.DigestBytes([]byte(secretCommand)) {
		t.Fatalf("command digest = %q", pass.TestResult.Receipts[0].CommandSHA256)
	}
}

// Failure modes 1 and 3: a mis-joined bundle is unbound, never joined by
// guesswork, and the witness carries no patch bytes or error detail.
func TestReleaseWitnessWrongJoinAndNoLeak(t *testing.T) {
	const computerID = "computer-witness-join"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	h := NewAPIHandler(rt)
	patch := []byte("diff --git a/frontend/x b/frontend/x\n--- /dev/null\n+++ b/frontend/x\n@@ -0,0 +1 @@\n+PATCH-LINE-NOT-FOR-THE-WITNESS ghp_" + strings.Repeat("Z", 36) + "\n")
	good := witnessFrozenBundle(t, updaterRoot, computerID, "op-join", patch, []string{"capsule-exec:sha256:" + strings.Repeat("7", 64)})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-join", computerID: computerID, state: selfdev.StateAwaitingApproval, bundleDigest: good})
	// The same bundle bound to an operation with another trajectory.
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-mismatch", computerID: computerID, state: selfdev.StateAwaitingApproval, bundleDigest: good, trajectoryID: "trajectory-other"})
	// A source.patch whose bytes were changed after the freeze.
	changed := witnessFrozenBundle(t, updaterRoot, computerID, "op-changed", patch, []string{"capsule-exec:sha256:" + strings.Repeat("7", 64)})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-changed", computerID: computerID, state: selfdev.StateAwaitingApproval, bundleDigest: changed})
	patchPath := filepath.Join(updaterRoot, "incoming", changed, "source.patch")
	if err := os.Chmod(patchPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patchPath, []byte("a different patch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withWitnessReceipts(t, func(ref string) (capsule.ExecutionReceipt, error) {
		return capsule.ExecutionReceipt{ReceiptRef: ref}, nil
	})

	ok, _, okBody := witnessGet(t, h, computerID, "op-join")
	requireBound(t, ok, "bundle_operation", witnessTrue)
	requireBound(t, ok, "bundle_source_patch", witnessTrue)
	for _, secret := range []string{"PATCH-LINE-NOT-FOR-THE-WITNESS", "ghp_", "diff --git"} {
		if strings.Contains(okBody, secret) {
			t.Fatalf("the witness carries patch content %q", secret)
		}
	}
	if ok.Bundle.SourcePatchSHA256 != computerevent.DigestBytes(patch) {
		t.Fatalf("bundle patch digest = %q", ok.Bundle.SourcePatchSHA256)
	}

	mismatch, _, _ := witnessGet(t, h, computerID, "op-mismatch")
	requireBound(t, mismatch, "bundle_operation", witnessFalse)
	if mismatch.Bundle.Bound == nil || *mismatch.Bundle.Bound {
		t.Fatalf("mismatched bundle reported bound: %+v", mismatch.Bundle)
	}

	mutated, _, _ := witnessGet(t, h, computerID, "op-changed")
	requireBound(t, mutated, "bundle_operation", witnessTrue)
	requireBound(t, mutated, "bundle_source_patch", witnessFalse)
}

// Failure mode 5: a read changes nothing, including the event head.
func TestReleaseWitnessIsReadOnly(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-witness-readonly")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-witness-readonly")
	fx.appendDecision(t, operation, bundleDigest, "decision-witness-readonly")
	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	head := fx.waitForStableHead(t)
	events := len(fx.cas.snapshot())

	h := NewAPIHandler(fx.rt)
	for i := 0; i < 3; i++ {
		if _, resp, _ := witnessGet(t, h, fx.computerID, operation.OperationID); resp.StatusCode != http.StatusOK {
			t.Fatalf("witness = %d", resp.StatusCode)
		}
	}
	after, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(applied)
	now, _ := json.Marshal(after)
	if string(before) != string(now) {
		t.Fatalf("a read changed the operation:\n%s\n%s", before, now)
	}
	headAfter := fx.waitForStableHead(t)
	if *head != *headAfter || len(fx.cas.snapshot()) != events {
		t.Fatalf("a read moved the tape: head %+v -> %+v, events %d -> %d", head, headAfter, events, len(fx.cas.snapshot()))
	}
}

// Failure mode 2: a read that races an operation change repeats; one that
// never settles says so.
func TestReleaseWitnessRepeatsAStaleReadAndSaysWhenItNeverSettles(t *testing.T) {
	const computerID = "computer-witness-stale"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	h := NewAPIHandler(rt)
	ref := "capsule-exec:sha256:" + strings.Repeat("7", 64)
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-stale", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: witnessFrozenBundle(t, updaterRoot, computerID, "op-stale", nil, []string{ref})})
	touch := func() {
		if _, err := productStore.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET updated_at = ? WHERE operation_id = ?`,
			time.Now().UTC().Add(time.Duration(touches.Add(1))*time.Second).Truncate(time.Microsecond), "op-stale"); err != nil {
			t.Fatal(err)
		}
	}

	// Moves once during the first read: the second read is stable.
	var moved atomic.Bool
	withWitnessReceipts(t, func(string) (capsule.ExecutionReceipt, error) {
		if moved.CompareAndSwap(false, true) {
			touch()
		}
		return capsule.ExecutionReceipt{ReceiptRef: ref}, nil
	})
	settled, _, _ := witnessGet(t, h, computerID, "op-stale")
	if !settled.ReadConsistent {
		t.Fatal("a read that raced one change did not settle on the repeat")
	}

	// Moves on every read: three tries, then the witness reports it.
	withWitnessReceipts(t, func(string) (capsule.ExecutionReceipt, error) {
		touch()
		return capsule.ExecutionReceipt{ReceiptRef: ref}, nil
	})
	unsettled, _, _ := witnessGet(t, h, computerID, "op-stale")
	if unsettled.ReadConsistent {
		t.Fatal("a read that never settled claims to be consistent")
	}
}

var touches atomic.Int32

// An applied frontend-only operation on the full production materializer:
// every join the witness can derive is bound, and the compared flags are the
// table's.
func TestReleaseWitnessJoinsAnAppliedOperation(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-witness-applied")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-witness-applied")
	h := NewAPIHandler(fx.rt)

	awaiting, _, _ := witnessGet(t, h, fx.computerID, operation.OperationID)
	requireBound(t, awaiting, "decision_operation", nil)
	requireBound(t, awaiting, "bundle_operation", witnessTrue)
	if awaiting.Decision.Present {
		t.Fatalf("decision present before the decision: %+v", awaiting.Decision)
	}

	fx.appendDecision(t, operation, bundleDigest, "decision-witness-applied")
	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	withWitnessReceipts(t, func(ref string) (capsule.ExecutionReceipt, error) {
		return capsule.ExecutionReceipt{ReceiptRef: ref}, nil
	})
	witness, _, _ := witnessGet(t, h, fx.computerID, operation.OperationID)

	if !witness.Decision.Present || witness.Decision.EventDigest != applied.DecisionEvent || witness.Decision.AuthorityKind != "external-owner" ||
		witness.Decision.GateVerified == nil || !*witness.Decision.GateVerified {
		t.Fatalf("decision = %+v", witness.Decision)
	}
	for _, pair := range []string{"decision_operation", "decision_expected_heads", "bundle_operation", "test_receipts_exit", "manifest_decision",
		"decision_pending_transition", "checkpoint_event", "route_event"} {
		requireBound(t, witness, pair, witnessTrue)
	}
	for _, pair := range []string{"bundle_source_patch", "bundle_host_build", "builder_receipt_patch", "manifest_closure_commit",
		"manifest_builder_receipt", "frozen_surface_decision_heads", "divergence_derived"} {
		requireBound(t, witness, pair, nil)
	}
	if !witness.Applied.ManifestPresent || witness.Applied.ReleaseDigest != applied.ReleaseDigest || witness.Applied.AcceptedEventHead != applied.DecisionEvent ||
		!witness.Applied.AppliedEventPresent || witness.Checkpoint.Ref != applied.CheckpointRef || !witness.Checkpoint.EventPresent ||
		witness.Route.Certificate != applied.RouteCertificate || !witness.Route.EventPresent || witness.Heads.Live == nil {
		t.Fatalf("witness sections = %+v", witness)
	}
	if witness.EffectiveRelease == nil || witness.EffectiveRelease.ReleaseDigest != applied.ReleaseDigest {
		t.Fatalf("effective release = %+v, want %s", witness.EffectiveRelease, applied.ReleaseDigest)
	}
	requireBound(t, witness, "effective_release_operation", witnessTrue)
	for _, binding := range witness.Bindings {
		for _, row := range selfdevGatePairs {
			if row.Pair == binding.Pair && (binding.ComparedAtGate != row.Compared || binding.ComparedAt != row.ComparedAt) {
				t.Fatalf("%s: compared flag %v/%s differs from the table", row.Pair, binding.ComparedAtGate, binding.ComparedAt)
			}
		}
	}
	if !witness.ReadConsistent || witness.Summary.Unbound != 0 {
		t.Fatalf("applied witness: consistent=%v summary=%+v", witness.ReadConsistent, witness.Summary)
	}

	// A mutated join is shown unbound: the row's recorded decision event no
	// longer names the decision on the tape.
	if _, err := fx.store.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET decision_event = ? WHERE operation_id = ?`,
		strings.Repeat("9", 64), operation.OperationID); err != nil {
		t.Fatal(err)
	}
	mutated, _, _ := witnessGet(t, h, fx.computerID, operation.OperationID)
	requireBound(t, mutated, "decision_operation", witnessFalse)
	requireBound(t, mutated, "manifest_decision", witnessFalse)
	if mutated.Summary.Unbound < 2 {
		t.Fatalf("mutated summary = %+v", mutated.Summary)
	}
}

// A host-built layered release: the host-build joins are bound, a builder
// receipt that names another patch is unbound and NOT refused by any gate, and
// the gate's own staged-build check refuses a tampered job.
func TestReleaseWitnessJoinsAHostBuiltRelease(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-witness-host", "done")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-witness-host")
	fx.appendDecision(t, operation, bundleDigest, "decision-witness-host")
	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	withWitnessReceipts(t, func(ref string) (capsule.ExecutionReceipt, error) {
		return capsule.ExecutionReceipt{ReceiptRef: ref}, nil
	})
	h := NewAPIHandler(fx.rt)

	witness, _, _ := witnessGet(t, h, fx.computerID, operation.OperationID)
	if !witness.HostBuild.Required || !witness.HostBuild.Found || witness.HostBuild.Candidates != 1 || witness.HostBuild.GateVerified == nil || !*witness.HostBuild.GateVerified {
		t.Fatalf("host build = %+v", witness.HostBuild)
	}
	for _, pair := range []string{"bundle_source_patch", "bundle_host_build", "builder_receipt_patch", "manifest_closure_commit", "manifest_builder_receipt", "manifest_decision"} {
		requireBound(t, witness, pair, witnessTrue)
	}
	if !witness.Applied.Layered || witness.Applied.ClosureDigest != witness.HostBuild.ClosureDigest || witness.Applied.ReleaseDigest != applied.ReleaseDigest {
		t.Fatalf("applied = %+v host = %+v", witness.Applied, witness.HostBuild)
	}
	if b := witnessBinding(t, witness, "builder_receipt_patch"); b.ComparedAtGate {
		t.Fatalf("builder_receipt_patch claims a gate compares it: %+v", b)
	}

	// The staged builder receipt is rewritten to name another patch. The
	// gate's file-hash check now refuses the staged build too, so both rows
	// are unbound; no other gate looks at the receipt's content.
	stagedDir := filepath.Join(fx.updaterRoot, "incoming", "host-build-"+witness.HostBuild.JobID)
	receiptPath := filepath.Join(stagedDir, "builder-receipt.json")
	if err := os.Chmod(receiptPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, []byte(`{"code_commit":"`+witness.HostBuild.CodeCommit+`","source_patch_sha256":"`+strings.Repeat("0", 64)+`","source_patch_base_commit":"`+witness.HostBuild.BaseCommit+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tampered, _, _ := witnessGet(t, h, fx.computerID, operation.OperationID)
	requireBound(t, tampered, "builder_receipt_patch", witnessFalse)
	requireBound(t, tampered, "bundle_host_build", witnessFalse)

	// The gate refuses a host result that binds another patch (the table's
	// bundle_host_build row).
	request, err := fx.rt.selfDevelopmentHostBuildRequest(applied, mustReadWitnessBundle(t, fx, applied))
	if err != nil {
		t.Fatal(err)
	}
	job := hostbuild.Job{}
	raw, err := os.ReadFile(filepath.Join(stagedDir, "host-build.json"))
	if err != nil || json.Unmarshal(raw, &job) != nil {
		t.Fatalf("staged job: %v", err)
	}
	if err := hostbuild.VerifyJob(job, request, fx.rt.selfdevControl.PublicKey()); err != nil {
		t.Fatalf("control job refused: %v", err)
	}
	request.SourcePatchSHA256 = strings.Repeat("0", 64)
	if err := hostbuild.VerifyJob(job, request, fx.rt.selfdevControl.PublicKey()); err == nil {
		t.Fatal("VerifyJob accepted a job for another patch; the bundle_host_build row is wrong")
	}
}

func mustReadWitnessBundle(t *testing.T, fx *derivableSelfDevFixture, operation selfdev.Operation) transaction.CapsuleEffectBundle {
	t.Helper()
	bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(fx.updaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
