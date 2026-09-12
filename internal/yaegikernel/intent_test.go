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
	spawnID, err := tray.Spawn("researcher", "survey the tree")
	if err != nil {
		t.Fatal(err)
	}
	if err := tray.Complete(CompleteCompleted, "ok", "done", []string{"ref-1"}); err != nil {
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
	if err := twice.Complete(CompleteCompleted, "v", "s", nil); err != nil {
		t.Fatal(err)
	}
	if err := twice.Complete(CompleteFailed, "v", "s", nil); err == nil {
		t.Fatal("second complete must be rejected")
	}
	var bad Tray
	if err := bad.Complete("shipped", "v", "s", nil); err == nil {
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
		{ID: "m-1", FromDesk: "super", ToDesk: "cosuper", Kind: "directive", Body: "build it"},
	}}
	hooks.Begin(frame)
	frame.Inbox[0].Body = "mutated after inject"
	if got := scope.Inbox(); len(got) != 1 || got[0].Body != "build it" {
		t.Fatalf("inbox snapshot not isolated: %+v", got)
	}
	res, err := scope.Message("super", "evidence_update", "built")
	if err != nil || res.MessageID == "" {
		t.Fatalf("bound message = %+v, %v", res, err)
	}
	if _, err := scope.Spawn("researcher", "verify"); err != nil {
		t.Fatalf("bound spawn: %v", err)
	}
	staged := hooks.End()
	if len(staged) != 2 || staged[0].Kind != IntentMessage || staged[1].Kind != IntentSpawn {
		t.Fatalf("drained = %+v", staged)
	}
	if len(scope.Inbox()) != 0 {
		t.Fatal("inbox must clear at cell end")
	}
	if _, err := scope.Spawn("researcher", "late"); err == nil {
		t.Fatal("spawn outside a cell must fail")
	}
	if err := scope.Complete(CompleteCompleted, "v", "s", nil); err == nil {
		t.Fatal("complete outside a cell must fail")
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
	good, err := serveCell(sess, SessionFrame{ID: "c-ok", Source: `choir.Message("super", "k", "hi")`}, nil, &hooks)
	if err != nil {
		t.Fatalf("good cell: %v", err)
	}
	if len(good.Intents) != 1 {
		t.Fatalf("good cell intents = %+v", good.Intents)
	}
	bad, err := serveCell(sess, SessionFrame{ID: "c-bad", Source: `choir.Message("super", "k", "x"); undefined.Symbol()`}, nil, &hooks)
	if err == nil {
		t.Fatal("bad cell must poison")
	}
	if len(bad.Intents) != 0 {
		t.Fatalf("failed cell shipped intents: %+v", bad.Intents)
	}
}

// TestChoirExportsCarryOrchestrationSurface proves the model-facing surface:
// Spawn/Complete/Inbox exist for CoSuper, Inbox alone for researchers.
func TestChoirExportsCarryOrchestrationSurface(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	exports := scope.ChoirExports()["choir/choir"]
	for _, name := range []string{"Spawn", "Complete", "Inbox", "Message", "Exec", "ReadFile"} {
		if _, ok := exports[name]; !ok {
			t.Errorf("cosuper exports missing %q", name)
		}
	}
	researcher, err := NewChoirScope(broker, issuer, "computer-choir", "activation-r", 1, SessionRoleResearcher)
	if err != nil {
		t.Fatal(err)
	}
	rexp := researcher.ChoirExports()["choir/choir"]
	if _, ok := rexp["Inbox"]; !ok {
		t.Error("researcher exports missing Inbox")
	}
	for _, name := range []string{"Spawn", "Complete", "Message", "Exec", "WriteFile"} {
		if _, ok := rexp[name]; ok {
			t.Errorf("researcher exports must not carry %q", name)
		}
	}
	if _, err := researcher.Spawn("researcher", "x"); err == nil {
		t.Error("researcher spawn must be denied")
	}
}
