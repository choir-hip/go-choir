package yaegikernel

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestTrayStagesWithoutBlocking proves in-cell orchestration never touches
// the network: Message/Spawn/Complete return cell-local IDs in microseconds
// with no broker roundtrip.
func TestTrayStagesWithoutBlocking(t *testing.T) {
	var tray Tray
	start := time.Now()
	msgID, err := tray.Message("research", "look at x")
	if err != nil {
		t.Fatal(err)
	}
	spawnID, err := tray.Spawn("research", "survey the tree")
	if err != nil {
		t.Fatal(err)
	}
	if err := tray.Complete(CompleteCompleted, "none", "done", []string{"ref-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("tray staging took %v, want microseconds", elapsed)
	}
	staged := tray.Drain()
	if len(staged) != 3 {
		t.Fatalf("drained %d intents, want 3", len(staged))
	}
	if staged[0].LocalID != msgID || staged[1].LocalID != spawnID {
		t.Fatalf("local IDs not preserved: %+v", staged)
	}
	if len(tray.Drain()) != 0 {
		t.Fatal("second drain must be empty")
	}
}

func TestTrayQuotas(t *testing.T) {
	var tray Tray
	for range MaxIntentsPerCell {
		if _, err := tray.Message("peer", "x"); err != nil {
			t.Fatalf("quota fill: %v", err)
		}
	}
	if _, err := tray.Message("peer", "one too many"); err == nil {
		t.Fatal("17th intent must exceed quota")
	}
	var big Tray
	if _, err := big.Message("peer", strings.Repeat("b", MaxIntentBody+1)); err == nil {
		t.Fatal("oversize body must be rejected")
	}
	var twice Tray
	if err := twice.Complete(CompleteCompleted, "none", "s", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := twice.Complete(CompleteFailed, "v", "s", nil, nil); err == nil {
		t.Fatal("second complete must be rejected")
	}
	var bad Tray
	if err := bad.Complete("shipped", "v", "s", nil, nil); err == nil {
		t.Fatal("unknown result must be rejected")
	}
}

// TestCellBindingInboxAndStaging proves the cell contract: Begin installs an
// isolated inbox snapshot, staged calls return local IDs, End drains and
// unbinds, and failed cells drop their tray (only successful cells reduce).
func TestCellBindingInboxAndStaging(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	hooks := scope.BindCell()
	frame := SessionFrame{ID: "cell-1", Inbox: []IncomingMessage{
		{ID: "m-1", FromDesk: "management", ToDesk: "engineering", Kind: "directive", Body: "build it"},
	}}
	hooks.Begin(frame)
	frame.Inbox[0].Body = "mutated after inject"
	if got := scope.Inbox(); len(got) != 1 || got[0].Body != "build it" {
		t.Fatalf("inbox snapshot not isolated: %+v", got)
	}
	res, err := scope.Message("management", "evidence_update", "built")
	if err != nil || res.MessageID == "" {
		t.Fatalf("bound message = %+v, %v", res, err)
	}
	if _, err := scope.Spawn("research", "verify"); err != nil {
		t.Fatalf("bound spawn: %v", err)
	}
	staged := hooks.End()
	if len(staged) != 2 || staged[0].Kind != IntentMessage || staged[1].Kind != IntentSpawn {
		t.Fatalf("drained = %+v", staged)
	}
	if len(scope.Inbox()) != 0 {
		t.Fatal("inbox must clear at cell end")
	}
	if _, err := scope.Spawn("research", "late"); err == nil {
		t.Fatal("spawn outside a cell must fail")
	}
	if err := scope.Complete(CompleteCompleted, "v", "s", nil, nil); err == nil {
		t.Fatal("complete outside a cell must fail")
	}
}

// TestCellBindingUpdates proves the RLM prompt-as-variable contract:
// Begin installs the pending-update records as an isolated cell snapshot,
// scope.Updates() exposes them inside the cell, and End clears them so a
// post-cell call never replays a stale payload.
func TestCellBindingUpdates(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	hooks := scope.BindCell()
	frame := SessionFrame{ID: "cell-2", Updates: []PendingUpdate{
		{UpdateID: "upd-1", FromAgentID: "texture:doc-1", FromRole: "texture", HumanProjection: "write the intro"},
	}}
	hooks.Begin(frame)
	frame.Updates[0].HumanProjection = "mutated after inject"
	got := scope.Updates()
	if len(got) != 1 || got[0].UpdateID != "upd-1" || got[0].HumanProjection != "write the intro" {
		t.Fatalf("updates snapshot not bound/isolated: %+v", got)
	}
	hooks.End()
	if len(scope.Updates()) != 0 {
		t.Fatal("updates must clear at cell end")
	}
}

// TestCellBindingEmits proves the same prompt-as-variable contract for the
// boundary-drained emission plane: Begin installs addressed emissions as an
// isolated cell snapshot, scope.Emits() exposes them inside the cell, and
// End clears them so a post-cell call never replays a stale body.
func TestCellBindingEmits(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	hooks := scope.BindCell()
	frame := SessionFrame{ID: "cell-3", Emits: []PendingEmit{
		{ChannelID: "ch-sender", MessageSeq: 7, FromAgentID: "research:doc-1", Kind: "emit", Body: "typed signal body"},
	}}
	hooks.Begin(frame)
	frame.Emits[0].Body = "mutated after inject"
	got := scope.Emits()
	if len(got) != 1 || got[0].MessageSeq != 7 || got[0].Body != "typed signal body" {
		t.Fatalf("emits snapshot not bound/isolated: %+v", got)
	}
	hooks.End()
	if len(scope.Emits()) != 0 {
		t.Fatal("emits must clear at cell end")
	}
}

// TestServeCellFailedDropsTray proves the two-phase ack gate at the cell
// level: a poisoned cell ships no staged intents, so the reducer never sees
// them and the inbox cursor cannot advance.
func TestServeCellFailedDropsTray(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist("choir"), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if _, err := sess.Eval(context.Background(), `import "choir"`); err != nil {
		t.Fatalf("import choir: %v", err)
	}
	hooks := scope.BindCell()
	good, err := serveCell(sess, SessionFrame{ID: "c-ok", Source: `choir.Message("management", "k", "hi")`}, nil, &hooks)
	if err != nil {
		t.Fatalf("good cell: %v", err)
	}
	if len(good.Intents) != 1 {
		t.Fatalf("good cell intents = %+v", good.Intents)
	}
	// Runtime failure (not a compile rejection): the cell stages then panics
	// at execution, so it poisons and ships nothing.
	bad, err := serveCell(sess, SessionFrame{ID: "c-bad", Source: `choir.Message("management", "k", "x"); panic("boom")`}, nil, &hooks)
	if err == nil {
		t.Fatal("bad cell must poison")
	}
	if len(bad.Intents) != 0 {
		t.Fatalf("failed cell shipped intents: %+v", bad.Intents)
	}
}

// TestFreezeVerifyStaging proves the settlement intents stage into the tray
// with their fields intact and enforce the one-per-cell rule.
func TestFreezeVerifyStaging(t *testing.T) {
	var tray Tray
	if err := tray.Freeze("capsule-exec:sha256:aa", []string{"capsule-exec:sha256:bb"}, []string{"capsule-exec:sha256:cc"}); err != nil {
		t.Fatal(err)
	}
	if err := tray.Freeze("x", []string{"y"}, []string{"z"}); err == nil {
		t.Fatal("second freeze must be rejected")
	}
	if err := tray.Verify("pass", []string{"ref-1"}, "sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if err := tray.Verify("fail", []string{"ref-2"}, "sha256:abc"); err == nil {
		t.Fatal("second verify must be rejected")
	}
	staged := tray.Drain()
	if len(staged) != 2 || staged[0].Kind != IntentFreeze || staged[1].Kind != IntentVerify {
		t.Fatalf("staged = %+v", staged)
	}
	if staged[0].BuildRecipeRef != "capsule-exec:sha256:aa" || staged[1].Decision != "pass" || staged[1].BundleDigest != "sha256:abc" {
		t.Fatalf("staged fields = %+v", staged)
	}
	var bad Tray
	if err := bad.Verify("maybe", nil, "sha256:abc"); err == nil {
		t.Fatal("invalid decision must be rejected at staging")
	}
	if err := bad.Verify("pass", []string{"ref"}, ""); err == nil {
		t.Fatal("missing bundle digest must be rejected at staging")
	}
}

// TestTypedCommitmentVerbsStageShape exercises the desk-cell surface: the
// typed acts are accepted as structured values, remain separate staged intents,
// and carry no free-text fallback fields.
func TestTypedCommitmentVerbsStageShape(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	if _, err := scope.Precommit(map[string]any{
		"question":     "will the retained tape replay?",
		"distribution": map[string]float64{"yes": 0.8, "no": 0.2},
		"resolver":     "management",
	}); err != nil {
		t.Fatalf("typed precommit: %v", err)
	}
	if _, err := scope.Resolve("commitment-1", map[string]any{
		"verdict":       "confirmed",
		"evidence_refs": []string{"evidence://replay"},
	}); err != nil {
		t.Fatalf("typed resolve: %v", err)
	}
	if _, err := scope.Disagreement(map[string]any{
		"commitment_id":    "commitment-1",
		"scorer_verdict":   "contradicted",
		"resolver_verdict": "confirmed",
		"evidence_refs":    []string{"evidence://counterexample"},
	}); err != nil {
		t.Fatalf("typed disagreement: %v", err)
	}
	staged := hooks.End()
	if len(staged) != 3 || staged[0].Kind != IntentPrecommit ||
		staged[1].Kind != IntentResolve || staged[2].Kind != IntentDisagreement {
		t.Fatalf("typed commitment intents = %+v", staged)
	}
	if staged[0].Precommit == "" || staged[1].Resolve == "" || staged[2].Disagreement == "" {
		t.Fatalf("typed commitment bodies missing: %+v", staged)
	}
}

// TestVerifierSlotExports proves Verify/InspectBundle export only for the
// verifier slot: implementation and researcher scopes never see them.
func TestVerifierSlotExports(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	exports := scope.ChoirExports()["choir/choir"]
	if _, ok := exports["Freeze"]; !ok {
		t.Error("implementation exports missing Freeze")
	}
	for _, name := range []string{"Verify", "InspectBundle"} {
		if _, ok := exports[name]; ok {
			t.Errorf("implementation exports must not carry %q", name)
		}
	}
	verifier, err := NewChoirScope(broker, issuer, "computer-choir", "activation-v", 1, "engineering", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	vexp := verifier.ChoirExports()["choir/choir"]
	for _, name := range []string{"Verify", "InspectBundle", "Freeze"} {
		if _, ok := vexp[name]; !ok {
			t.Errorf("verifier exports missing %q", name)
		}
	}
	if err := scope.Verify("pass", []string{"ref"}, "sha256:abc"); err == nil {
		t.Error("implementation Verify must be denied")
	}
	if _, err := scope.InspectBundle(); err == nil {
		t.Error("implementation InspectBundle must be denied")
	}
}
