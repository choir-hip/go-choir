package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"regexp"
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
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, "engineering", "implementation")
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
	handleRef, err := issuer.Issue("computer-choir", "engineering", 1, []BrokerAction{action}, time.Hour)
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

// TestChoirParityMessageRetired is corpus cell 5: the retired Message verb
// refuses on the symbol surface (the dead Assign verb is deleted).
func TestChoirParityMessageRetired(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	// Message is retired (protocol P2c): the symbol still resolves and
	// refuses, naming the acts to use.
	if _, err := scope.Message("activation-choir", "note", "hi"); err == nil || !strings.Contains(err.Error(), "choir.ReportPacket") {
		t.Fatalf("retired message = %v, want a refusal naming ReportPacket", err)
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
	narrow := &ChoirScope{broker: broker, computerID: "computer-choir", epoch: 1, activationID: "activation-narrow"}
	narrow.handleRef.Store(&readOnlyRef)
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

// TestChoirPredeclaredInSession pins the desk contract: the choir binding is
// installed at session construction, so a model's first cell may call
// choir.* without `import "choir"` — and a model-authored re-import is
// still deduped rather than fatal. Staging receipt 2026-09-28: two whole
// iterations per activation died on `undefined: choir`.
func TestChoirPredeclaredInSession(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	allowlist := NewAllowlist("choir", "fmt")
	sess, err := NewSession(allowlist, scope.ChoirExports())
	if err != nil {
		t.Fatalf("session with choir symbols: %v", err)
	}
	defer sess.Close()
	ctx := context.Background()
	if _, err := sess.Eval(ctx, `choir.WriteFile("predeclared.txt", "no-import")`); err != nil {
		t.Fatalf("choir call without import: %v", err)
	}
	if _, err := sess.Eval(ctx, "import \"choir\""); err != nil {
		t.Fatalf("model-authored re-import must dedupe, got: %v", err)
	}
	res, err := sess.Eval(ctx, `choir.ReadFile("predeclared.txt")`)
	if err != nil {
		t.Fatalf("read after deduped import: %v", err)
	}
	if !res.Value.IsValid() || res.Value.Interface() != "no-import" {
		t.Fatalf("read cell value = %v", res.Value)
	}
}

// TestChoirResearchScopeIsReadOnly guards the read-only-world contract: a
// researcher-bound scope observes files but cannot write, execute, or assign.
// Mission R2 deliberately grants research full message authority, so staged
// messaging verbs export while file/exec mutation stays absent from the table
// and denied at the method level.
func TestChoirResearchScopeIsReadOnly(t *testing.T) {
	t.Parallel()
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
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, SessionRoleResearch, "")
	if err != nil {
		t.Fatal(err)
	}
	exports := scope.ChoirExports()["choir/choir"]
	// World mutation is absent: research cannot write or exec.
	for _, sym := range []string{"WriteFile", "Exec"} {
		if _, ok := exports[sym]; ok {
			t.Fatalf("researcher exports world-mutation %s", sym)
		}
	}
	for _, sym := range []string{"ReadFile", "ListDir", "Context"} {
		if _, ok := exports[sym]; !ok {
			t.Fatalf("researcher missing %s", sym)
		}
	}
	// Message authority is present (R2): staged verbs export for research.
	// The retired Message and Emit transports and their Inbox and Emits
	// mail are gone (Cut 1 L6): research converses on records.
	for _, sym := range []string{"Message", "Emit", "Inbox", "Emits", "Updates"} {
		if _, ok := exports[sym]; ok {
			t.Fatalf("researcher exports retired transport %s", sym)
		}
	}
	for _, sym := range []string{"Records", "Cast", "Report", "Escalate"} {
		if _, ok := exports[sym]; !ok {
			t.Fatalf("researcher missing message authority %s", sym)
		}
	}
	// Emit + the host-mediated egress verbs export: research's signal plane and
	// world evidence surface ride the broker, not filesystem mutation (M-SUB /
	// M0a). read-only denies WriteFile/Exec only — messaging stays open.
	for _, sym := range []string{"SearchStart", "Searches", "SearchWait", "Read", "ReadCapture",
		"SearchWireCorpus", "SaveEvidence", "ReadEvidence",
		"ListEvidence", "RunMemoryEntry"} {
		if _, ok := exports[sym]; !ok {
			t.Fatalf("researcher missing egress/message verb %s", sym)
		}
	}
	// Cut 2 unit 2a: Read (fetch into a capture) replaced the fetch, import
	// and ContentItem selector verbs on research and its children.
	child, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, SessionRoleResearchChild, "")
	if err != nil {
		t.Fatal(err)
	}
	childExports := child.ChoirExports()["choir/choir"]
	if _, ok := childExports["Read"]; !ok {
		t.Fatal("research child missing Read")
	}
	for _, sym := range []string{"FetchURL", "ImportDocument", "ImportURL", "ReadContentItem", "ListContentSelectors", "ReadContentSelector"} {
		if _, ok := exports[sym]; ok {
			t.Fatalf("researcher still exports replaced verb %s", sym)
		}
		if _, ok := childExports[sym]; ok {
			t.Fatalf("research child still exports replaced verb %s", sym)
		}
	}
	// Method-level: mutation calls still deny even though they never export.
	if _, err := scope.WriteFile("x.txt", "x"); err == nil {
		t.Fatal("researcher WriteFile allowed")
	}
	if _, err := scope.Exec("echo", nil); err == nil {
		t.Fatal("researcher Exec allowed")
	}
	// Messaging is NOT denied for research: Emit must reach the broker (failing
	// on the nil host handler), never the read-only gate — the gap that
	// silently blocked research findings on the M-SUB plane.
	if _, err := scope.Emit("texture", "evidence", "found"); err == nil {
		t.Fatal("Emit unexpectedly succeeded with nil host handler")
	} else if strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Emit denied for read-only role — messaging authority broken: %v", err)
	}
	if _, err := scope.ReadFile("missing.txt"); err == nil {
		t.Fatal("researcher ReadFile unexpectedly succeeded on missing file")
	}
}

// TestChoirProductAPIIsManagementOnly pins the SMG contract: the deleted
// product_api_request tool's capability now lives behind choir.ProductAPI,
// exported only on the management desk — every other desk's module set
// excludes it AND its session handle never authorizes product_api.
func TestChoirProductAPIIsManagementOnly(t *testing.T) {
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
	mgmt, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, "management", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mgmt.ChoirExports()["choir/choir"]["ProductAPI"]; !ok {
		t.Fatal("management scope must export choir.ProductAPI")
	}
	for _, role := range []string{SessionRoleResearch, "engineering", "texture", "co-super"} {
		scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, role, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := scope.ChoirExports()["choir/choir"]["ProductAPI"]; ok {
			t.Fatalf("%s scope must not export choir.ProductAPI", role)
		}
		// Even if a cell bypassed the export map, the handle refuses the
		// broker action — the authority check is the durable gate.
		if _, err := scope.ProductAPI("GET", "/api/trajectories", nil); err == nil || !strings.Contains(err.Error(), "product_api") {
			t.Fatalf("%s ProductAPI call should fail handle verify, got %v", role, err)
		}
	}
}

// TestPromptTaughtVerbsMatchExports cross-checks every `choir.<Name>`
// identifier the deployed desk prompts teach against the kernel's actual
// export map — the 2026-10-06 defect was prompts teaching
// ListContentItemSelectors/ReadContentItemSelector while the kernel exports
// ListContentSelectors/ReadContentSelector (cells written from the prompt
// fail to compile). This is the bidirectional check: any taught name missing
// from exports fails the test.
func TestPromptTaughtVerbsMatchExports(t *testing.T) {
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

	promptSources := map[string]string{}
	for _, path := range []string{
		"../promptstore/defaults/research.yaml",
		"../promptstore/defaults/engineering.yaml",
		"../promptstore/defaults/management.yaml",
		"../promptstore/defaults/core.yaml",
		"../runtimeprompts/overlays/rlm_research_runtime.yaml",
		"../runtimeprompts/overlays/rlm_engineering_runtime.yaml",
		"../runtimeprompts/overlays/rlm_management_runtime.yaml",
		// Texture's prompt is its core alone (Cut 2 2a): the per-run
		// overlays were deleted with ApplyTexture.
		"../textureprompts/texture.yaml",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("prompt source %s not present: %v", path, err)
			continue
		}
		promptSources[path] = string(raw)
	}
	verbRe := regexp.MustCompile(`choir\.([A-Z][A-Za-z0-9_]*)`)

	// Map prompt files to the desk role they compose for.
	roles := map[string]string{
		"research":    SessionRoleResearch,
		"engineering": "engineering",
		"management":  "management",
		"texture":     "texture",
		// core.yaml is shared by every desk — check it against the most
		// restrictive desk surfaces.
		"core": SessionRoleResearch,
	}
	for deskRole, sessionRole := range roles {
		scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, sessionRole, "")
		if err != nil {
			t.Fatalf("scope %s: %v", deskRole, err)
		}
		exports := scope.ChoirExports()["choir/choir"]
		// Slot-conditional exports exist only when the activation carries
		// slot=verifier. Verify is management's since C1e step 4.
		slotConditional := map[string]bool{"InspectBundle": true}
		for path, text := range promptSources {
			// core.yaml composes into every desk's prompt — check it against
			// each desk's surface; desk-specific files match on role name.
			if !strings.Contains(path, deskRole) && !strings.Contains(path, "core.yaml") {
				continue
			}
			for _, m := range verbRe.FindAllStringSubmatch(text, -1) {
				name := m[1]
				if _, ok := exports[name]; !ok && !slotConditional[name] {
					t.Errorf("%s teaches choir.%s but the %s scope does not export it", path, name, deskRole)
				}
			}
		}
		// The verifier slot actually delivers the conditional exports.
		vscope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, sessionRole, "verifier")
		if err == nil {
			vexports := vscope.ChoirExports()["choir/choir"]
			for name := range slotConditional {
				if deskRole == "engineering" {
					if _, ok := vexports[name]; !ok {
						t.Errorf("verifier-slot scope missing conditional export %s", name)
					}
				}
			}
		}
	}
}

// TestNoDeskJudgesSupport pins that support judgement is off every desk
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md): the runtime
// asks Jev about a Texture commit's citation pairs after the commit, so no
// module set exports Judge and the surface does not document it. Ways it
// can fail: management keeps choir.Judge (its serial turn is back in the
// path), or a desk can still stage verdicts.
func TestNoDeskJudgesSupport(t *testing.T) {
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
	for _, role := range []string{"management", SessionRoleResearch, SessionRoleResearchChild, "engineering", "texture"} {
		scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, role, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := scope.ChoirExports()["choir/choir"]["Judge"]; ok {
			t.Fatalf("%s scope must not export choir.Judge", role)
		}
	}
	if _, ok := choirFunctionDocs["Judge"]; ok {
		t.Fatal("the choir surface still documents Judge")
	}
}
