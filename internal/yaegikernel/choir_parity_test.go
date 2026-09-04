package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testChoirFixture(t *testing.T) (*Broker, *HandleIssuer, *ChoirScope, string) {
	t.Helper()
	root := t.TempDir()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: root}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1)
	if err != nil {
		t.Fatalf("choir scope: %v", err)
	}
	return broker, issuer, scope, root
}

func dtoCall(t *testing.T, broker *Broker, issuer *HandleIssuer, action BrokerAction, payload any) *BrokerResponse {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	handleRef, err := issuer.Issue("computer-choir", "cosuper", 1, []BrokerAction{action}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return broker.HandleRequest(context.Background(), &BrokerRequest{
		ProtocolVersion: ProtocolVersion, RequestID: newReceiptID(),
		HandleRef: handleRef, Epoch: 1, Action: action, Payload: raw,
	})
}

// TestChoirParityWriteReadRoundTrip is Def 2 parity corpus cell 1-2: file
// writes and reads behave identically through choir symbols and JSON DTOs.
func TestChoirParityWriteReadRoundTrip(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	if n, err := scope.WriteFile("note.txt", "parity"); err != nil || n != 6 {
		t.Fatalf("symbol write = %d, %v", n, err)
	}
	resp := dtoCall(t, broker, issuer, ActionReadFile, ReadFilePayload{Path: "note.txt"})
	if !resp.Success {
		t.Fatalf("dto read: %s", resp.Error)
	}
	var got ReadFileResult
	if err := json.Unmarshal(resp.Result, &got); err != nil || got.Content != "parity" {
		t.Fatalf("dto read = %+v, %v", got, err)
	}
	content, err := scope.ReadFile("note.txt")
	if err != nil || content != "parity" {
		t.Fatalf("symbol read = %q, %v", content, err)
	}
}

// TestChoirParityListDir is corpus cell 3: directory listing parity.
func TestChoirParityListDir(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := scope.WriteFile(name, "x"); err != nil {
			t.Fatal(err)
		}
	}
	resp := dtoCall(t, broker, issuer, ActionListDir, ListDirPayload{Path: "."})
	if !resp.Success {
		t.Fatalf("dto list: %s", resp.Error)
	}
	var dtoResult ListDirResult
	if err := json.Unmarshal(resp.Result, &dtoResult); err != nil {
		t.Fatal(err)
	}
	symEntries, err := scope.ListDir(".")
	if err != nil {
		t.Fatalf("symbol list: %v", err)
	}
	if len(symEntries) != 2 || len(dtoResult.Entries) != 2 {
		t.Fatalf("entries symbol=%v dto=%v", symEntries, dtoResult.Entries)
	}
}

// TestChoirParityJailbreakRefused is corpus cell 4: path escapes fail
// identically on both surfaces (same jailing, same refusal class).
func TestChoirParityJailbreakRefused(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	if _, err := scope.ReadFile("../escape.txt"); err == nil || !strings.Contains(err.Error(), "escapes allowed root") {
		t.Fatalf("symbol jailbreak = %v, want escapes refusal", err)
	}
	resp := dtoCall(t, broker, issuer, ActionReadFile, ReadFilePayload{Path: "../escape.txt"})
	if resp.Success || !strings.Contains(resp.Error, "escapes allowed root") {
		t.Fatalf("dto jailbreak success=%v err=%q, want escapes refusal", resp.Success, resp.Error)
	}
}

// TestChoirParityAssignMessageReceipts is corpus cell 5: assignment and
// messaging record with receipts on both surfaces.
func TestChoirParityAssignMessageReceipts(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	assigned, err := scope.Assign("task-1", "researcher", "do it")
	if err != nil || assigned.AssignmentID == "" || assigned.Status != "dispatched" {
		t.Fatalf("symbol assign = %+v, %v", assigned, err)
	}
	resp := dtoCall(t, broker, issuer, ActionAssign, AssignPayload{TaskID: "task-2", ActorProfile: "researcher", Instruction: "do it"})
	if !resp.Success {
		t.Fatalf("dto assign: %s", resp.Error)
	}
	messaged, err := scope.Message("activation-choir", "note", "hi")
	if err != nil || messaged.MessageID == "" {
		t.Fatalf("symbol message = %+v, %v", messaged, err)
	}
	outcome, err := scope.Outcome("arc complete")
	if err != nil || outcome.MessageID == "" {
		t.Fatalf("symbol outcome = %+v, %v", outcome, err)
	}
}

// TestChoirSymbolsEnforceScopes proves a handle without a scope refuses the
// matching symbol call (fail closed per action).
func TestChoirSymbolsEnforceScopes(t *testing.T) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: root}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	readOnlyRef, err := issuer.Issue("computer-choir", "choir-session", 1, []BrokerAction{ActionReadFile}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	narrow := &ChoirScope{broker: broker, handleRef: readOnlyRef, computerID: "computer-choir", epoch: 1, activationID: "activation-narrow"}
	if _, err := narrow.WriteFile("x.txt", "x"); err == nil {
		t.Fatal("write with read-only handle must fail")
	}
}

// TestChoirSymbolsInSession proves the exports bind into a persistent Session:
// import choir once, write in one cell, read in the next.
func TestChoirSymbolsInSession(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	allowlist := NewAllowlist("choir", "fmt")
	sess, err := NewSession(allowlist, scope.ChoirExports())
	if err != nil {
		t.Fatalf("session with choir symbols: %v", err)
	}
	defer sess.Close()
	ctx := context.Background()
	if _, err := sess.Eval(ctx, "import \"choir\""); err != nil {
		t.Fatalf("import choir: %v", err)
	}
	if _, err := sess.Eval(ctx, `choir.WriteFile("sess.txt", "session-data")`); err != nil {
		t.Fatalf("write cell: %v", err)
	}
	res, err := sess.Eval(ctx, `choir.ReadFile("sess.txt")`)
	if err != nil {
		t.Fatalf("read cell: %v", err)
	}
	if !res.Value.IsValid() || res.Value.Interface() != "session-data" {
		t.Fatalf("read cell value = %v", res.Value)
	}
}
