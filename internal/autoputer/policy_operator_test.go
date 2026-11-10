package autoputer

// The platform operator's policy write on the guest
// (docs/design/platform-operator-account-2026-11-06.md, "Failure list
// (build)": R1 to R6, E1, E2). Ways it could fail:
//   - the write lands with no receipt, or the receipt is written after it;
//   - a receipt that cannot be recorded still lets the write through;
//   - the marker is believed from an in-guest caller or an in-process desk
//     call, so a desk could forge a receipt naming an operator;
//   - a marked request reaches any file but the policy and its revisions, or
//     any verb but the policy read and the validated write;
//   - the receipt carries an email, or lacks the revision or the digests;
//   - an ordinary owner write grows a receipt (the operator path leaks onto
//     the owner path);
//   - a delete then a move lands an unvalidated policy, or a write through a
//     symlink alias skips validation.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const testOperator = "1111aaaa-2222-3333-4444-55555555beef"

func operatorRequest(method, path, body string) *http.Request {
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	var req *http.Request
	if rd != nil {
		req = authBodyRequest(method, "/api/files/"+path, rd)
	} else {
		req = authRequest(method, "/api/files/"+path)
	}
	req.Header.Set(server.PlatformOperatorHeader, testOperator)
	return req
}

func serve(fh *FilesHandler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)
	return w
}

// R1 R4: the receipt is recorded before the write, names the operator, the
// revision and both digests.
func TestOperatorPolicyWriteIsRecordedBeforeItLands(t *testing.T) {
	fh, root := setupFileTest(t)
	p := seedPolicy(t, root, goodPolicy)
	next := strings.Replace(goodPolicy, "low", "high", 1)
	var acts []PolicyAct
	fh.SetPolicyActRecorder(func(r *http.Request, act *PolicyAct) error {
		// R1: at the moment of recording, the working file is still the old one.
		if got, _ := os.ReadFile(p); string(got) != goodPolicy {
			t.Errorf("the receipt was recorded after the write: %q", got)
		}
		act.ReceiptID = "evt-1"
		acts = append(acts, *act)
		return nil
	})
	w := serve(fh, operatorRequest(http.MethodPut, "System/model-policy.toml", next))
	if w.Code != http.StatusOK {
		t.Fatalf("operator PUT = %d %s", w.Code, w.Body)
	}
	if got, _ := os.ReadFile(p); string(got) != next {
		t.Fatal("the validated write did not land")
	}
	if len(acts) != 1 {
		t.Fatalf("receipts = %d", len(acts))
	}
	a := acts[0]
	names := revisionNames(t, root)
	if a.OperatorAccountID != testOperator || a.Act != "model_policy.write" || a.Path != PolicyFilePath ||
		len(names) != 1 || a.Revision != names[0] || a.OldSHA256 != sha256Hex([]byte(goodPolicy)) || a.NewSHA256 != sha256Hex([]byte(next)) || a.Bytes != len(next) {
		t.Fatalf("receipt = %+v revisions %v", a, names)
	}
	if !strings.Contains(w.Body.String(), a.Revision) || !strings.Contains(w.Body.String(), `"receipt":"evt-1"`) {
		t.Fatalf("the answer does not name the revision and the receipt: %s", w.Body)
	}
}

// R4: the event the receipt becomes names the account, never an email.
func TestOperatorReceiptEventNamesTheAccountNotAnEmail(t *testing.T) {
	rt := &captureRuntime{}
	req := operatorRequest(http.MethodPut, "System/model-policy.toml", "x")
	err := recordPlatformPolicyAct(req, rt, &PolicyAct{OperatorAccountID: testOperator, Act: "model_policy.write", Path: PolicyFilePath, Revision: "r", OldSHA256: "a", NewSHA256: "b", Bytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rt.kind != types.EventPlatformComputerModelPolicy || rt.owner != "test-user@example.com" {
		t.Fatalf("event = %s owner %s", rt.kind, rt.owner)
	}
	for _, k := range []string{"operator_account_id", "act", "path", "revision", "old_sha256", "new_sha256", "bytes"} {
		if _, ok := rt.payload[k]; !ok {
			t.Fatalf("payload lacks %s: %v", k, rt.payload)
		}
	}
	for k, v := range rt.payload {
		if s, ok := v.(string); ok && strings.Contains(s, "@") {
			t.Fatalf("payload %s carries an address-like value %q", k, s)
		}
	}
	// A request the guest cannot trust as the owner's records nothing.
	if err := recordPlatformPolicyAct(httptest.NewRequest(http.MethodPut, "/x", nil), rt, &PolicyAct{}); err == nil {
		t.Fatal("a receipt with no authenticated owner was recorded")
	}
}

type captureRuntime struct {
	kind    types.EventKind
	owner   string
	payload map[string]any
}

func (c *captureRuntime) EmitProductEvent(_ context.Context, ownerID, _ string, kind types.EventKind, payload map[string]any) (types.EventRecord, error) {
	c.kind, c.owner, c.payload = kind, ownerID, payload
	return types.EventRecord{EventID: "evt-1"}, nil
}

// R1 R5: a receipt that cannot be recorded refuses the write, and with no
// recorder there is no operator write at all.
func TestOperatorPolicyWriteNeedsItsReceipt(t *testing.T) {
	fh, root := setupFileTest(t)
	p := seedPolicy(t, root, goodPolicy)
	next := strings.Replace(goodPolicy, "low", "high", 1)
	if w := serve(fh, operatorRequest(http.MethodPut, "System/model-policy.toml", next)); w.Code != http.StatusForbidden {
		t.Fatalf("no recorder = %d", w.Code)
	}
	fh.SetPolicyActRecorder(func(*http.Request, *PolicyAct) error { return os.ErrClosed })
	if w := serve(fh, operatorRequest(http.MethodPut, "System/model-policy.toml", next)); w.Code != http.StatusInternalServerError {
		t.Fatalf("failing recorder = %d", w.Code)
	}
	if got, _ := os.ReadFile(p); string(got) != goodPolicy {
		t.Fatalf("a write landed with no receipt: %q", got)
	}
	// An invalid policy is refused before anything is recorded.
	calls := 0
	fh.SetPolicyActRecorder(func(*http.Request, *PolicyAct) error { calls++; return nil })
	if w := serve(fh, operatorRequest(http.MethodPut, "System/model-policy.toml", "[defaults]\nthis is not toml\n")); w.Code != http.StatusUnprocessableEntity || calls != 0 {
		t.Fatalf("invalid policy = %d, receipts %d", w.Code, calls)
	}
}

// R2 R3: the marker is believed only from the host peer and never in-process.
func TestOperatorMarkerIsNotBelievedFromWhoCannotSetIt(t *testing.T) {
	fh, root := setupFileTest(t)
	p := seedPolicy(t, root, goodPolicy)
	calls := 0
	fh.SetPolicyActRecorder(func(*http.Request, *PolicyAct) error { calls++; return nil })
	next := strings.Replace(goodPolicy, "low", "high", 1)

	inProcess := operatorRequest(http.MethodPut, "System/model-policy.toml", next)
	inProcess.RemoteAddr = "" // a desk's choir.ProductAPI call
	if w := serve(fh, inProcess); w.Code != http.StatusForbidden {
		t.Fatalf("in-process marker = %d", w.Code)
	}
	inGuest := operatorRequest(http.MethodPut, "System/model-policy.toml", next)
	inGuest.RemoteAddr = "127.0.0.1:5555" // any in-guest process
	if w := serve(fh, inGuest); w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("in-guest marker = %d", w.Code)
	}
	bad := operatorRequest(http.MethodPut, "System/model-policy.toml", next)
	bad.Header.Set(server.PlatformOperatorHeader, "operator@example.com")
	if w := serve(fh, bad); w.Code != http.StatusForbidden {
		t.Fatalf("malformed marker = %d", w.Code)
	}
	twice := operatorRequest(http.MethodPut, "System/model-policy.toml", next)
	twice.Header.Add(server.PlatformOperatorHeader, "99999999-9999-9999-9999-999999999999")
	if w := serve(fh, twice); w.Code != http.StatusForbidden {
		t.Fatalf("two markers = %d", w.Code)
	}
	if got, _ := os.ReadFile(p); string(got) != goodPolicy || calls != 0 {
		t.Fatalf("a refused marker wrote (%q) or recorded (%d)", got, calls)
	}
}

// R6 E1-guard: a marked request reaches only the policy surface; an ordinary
// owner write records nothing.
func TestOperatorMarkedRequestsReachOnlyThePolicySurface(t *testing.T) {
	fh, root := setupFileTest(t)
	seedPolicy(t, root, goodPolicy)
	if err := os.WriteFile(filepath.Join(root, "System", "other.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	fh.SetMoveGate(func(*http.Request) bool { return true })
	calls := 0
	fh.SetPolicyActRecorder(func(*http.Request, *PolicyAct) error { calls++; return nil })
	for name, req := range map[string]*http.Request{
		"DELETE policy": operatorRequest(http.MethodDelete, "System/model-policy.toml", ""),
		"PATCH policy":  operatorRequest(http.MethodPatch, "System/model-policy.toml", `{"move_to":"System/x.toml"}`),
		"POST dir":      operatorRequest(http.MethodPost, "System/newdir", ""),
		"PUT other":     operatorRequest(http.MethodPut, "System/other.txt", "overwritten"),
		"GET other":     operatorRequest(http.MethodGet, "System/other.txt", ""),
		"GET root":      operatorRequest(http.MethodGet, "", ""),
		"PUT revision":  operatorRequest(http.MethodPut, "System/model-policy.revisions/20261109T000000.000001Z.toml", goodPolicy),
	} {
		if w := serve(fh, req); w.Code != http.StatusForbidden && w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d", name, w.Code)
		} else if name != "GET root" && w.Code != http.StatusForbidden {
			t.Fatalf("%s was admitted: %d", name, w.Code)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(root, "System", "other.txt")); string(got) != "keep" {
		t.Fatal("a marked request wrote another file")
	}
	if _, err := os.Stat(filepath.Join(root, "System", "model-policy.toml")); err != nil {
		t.Fatal("a marked DELETE or PATCH removed the policy")
	}
	// Reads of the policy and its revisions are admitted.
	if w := serve(fh, operatorRequest(http.MethodGet, "System/model-policy.toml", "")); w.Code != http.StatusOK || w.Header().Get("X-Choir-Operator-Receipts") != "1" {
		t.Fatalf("operator read = %d, receipts header %q", w.Code, w.Header().Get("X-Choir-Operator-Receipts"))
	}
	// An ordinary owner write records nothing.
	if w := putFile(t, fh, "System/model-policy.toml", strings.Replace(goodPolicy, "low", "high", 1)); w.Code != http.StatusOK || calls != 0 {
		t.Fatalf("owner PUT = %d, receipts %d", w.Code, calls)
	}
}

// E1: a delete then a move cannot land an unvalidated policy; E2: nor can a
// write through a symlink alias.
func TestPolicyCannotBeLandedUnvalidatedByMoveOrAlias(t *testing.T) {
	fh, root := setupFileTest(t)
	fh.SetMoveGate(func(*http.Request) bool { return true })
	p := seedPolicy(t, root, goodPolicy)
	stage := filepath.Join(root, "System", "stage.toml")
	move := func(from string) *httptest.ResponseRecorder {
		req := authBodyRequest(http.MethodPatch, "/api/files/"+from, strings.NewReader(`{"move_to":"System/model-policy.toml"}`))
		return serve(fh, req)
	}
	// Delete the policy, stage a bad file, move it in: refused.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("[defaults]\nthis is not toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := move("System/stage.toml"); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("E1 bad move = %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("E1 an unvalidated policy landed by move")
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatal("E1 the refused move consumed its source")
	}
	// A valid file moves in.
	if err := os.WriteFile(stage, []byte(goodPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := move("System/stage.toml"); w.Code != http.StatusOK {
		t.Fatalf("E1 good move = %d %s", w.Code, w.Body)
	}
	if got, _ := os.ReadFile(p); string(got) != goodPolicy {
		t.Fatal("E1 the valid move did not land")
	}
	// E2: a symlink inside the root that resolves to the policy.
	if err := os.Symlink("model-policy.toml", filepath.Join(root, "System", "alias.toml")); err != nil {
		t.Fatal(err)
	}
	if w := putFile(t, fh, "System/alias.toml", "[defaults]\nthis is not toml\n"); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("E2 alias write = %d %s", w.Code, w.Body)
	}
	if got, _ := os.ReadFile(p); string(got) != goodPolicy {
		t.Fatalf("E2 an unvalidated policy landed through the alias: %q", got)
	}
	if w := putFile(t, fh, "System/alias.toml", strings.Replace(goodPolicy, "low", "high", 1)); w.Code != http.StatusOK || len(revisionNames(t, root)) != 1 {
		t.Fatalf("E2 valid alias write = %d, revisions %v", w.Code, revisionNames(t, root))
	}
}
