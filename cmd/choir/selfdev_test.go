package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// selfDevStub answers the self-development routes for computer-x from a
// mutable operation state and records every request.
type selfDevStub struct {
	t        *testing.T
	state    string
	requests []string
	bodies   map[string]map[string]any
}

func (s *selfDevStub) serve(w http.ResponseWriter, r *http.Request) {
	const base = "/api/computers/computer-x/self-development"
	s.requests = append(s.requests, r.Method+" "+strings.TrimPrefix(r.URL.Path, base)+queryOf(r))
	if r.Body != nil {
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			s.bodies[r.Method+" "+strings.TrimPrefix(r.URL.Path, base)] = body
		}
	}
	w.Header().Set("Content-Type", "application/json")
	hex := func(c string) string { return strings.Repeat(c, 64) }
	switch r.Method + " " + strings.TrimPrefix(r.URL.Path, base) {
	case "GET /operations":
		_, _ = io.WriteString(w, `{"operations":[{"operation_id":"op-1","state":"`+s.state+`"}]}`)
	case "GET /operations/op-1":
		_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "op-1", "state": s.state, "bundle_digest": hex("b"), "verifier_refs": []string{"verifier:jev"}})
	case "GET /head":
		_ = json.NewEncoder(w).Encode(map[string]any{"canonical_event_head": hex("c"), "sequence": 9, "desired_event_head": hex("d"), "effective_event_head": hex("e"),
			"pending_transition_ref": "", "desired_state_commitment": hex("1"), "effective_state_commitment": hex("2")})
	case "GET /mode":
		_, _ = io.WriteString(w, `{"computer_id":"computer-x","mode":"propose_only","generation":3}`)
	case "PUT /mode":
		_, _ = io.WriteString(w, `{"computer_id":"computer-x","mode":"accept_once","generation":4}`)
	case "POST /operations/op-1/decision":
		s.state = "accepted"
		_, _ = io.WriteString(w, `{"operation_id":"op-1","state":"accepted"}`)
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func queryOf(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}

func newSelfDevStub(t *testing.T, state string) (*selfDevStub, *httptest.Server) {
	stub := &selfDevStub{t: t, state: state, bodies: map[string]map[string]any{}}
	server := httptest.NewServer(http.HandlerFunc(stub.serve))
	t.Cleanup(server.Close)
	return stub, server
}

// Failure modes pinned for approve: arming accept_once for a different
// operation, bundle, head or commitment than the frozen candidate; arming
// against a stale mode generation; deciding before arming; and arming or
// deciding at all when the operation is not awaiting approval.
func TestSelfDevApproveArmsAcceptOnceForExactlyThisCandidate(t *testing.T) {
	stub, server := newSelfDevStub(t, "awaiting_approval")
	var stdout, stderr bytes.Buffer
	code := run([]string{"self-dev", "approve", "--computer=computer-x", "--operation=op-1", "--host=" + server.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	want := []string{"GET /operations/op-1", "GET /head", "GET /mode", "PUT /mode", "POST /operations/op-1/decision"}
	if strings.Join(stub.requests, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", stub.requests, want)
	}
	hex := func(c string) string { return strings.Repeat(c, 64) }
	arm := stub.bodies["PUT /mode"]
	for field, value := range map[string]any{"mode": "accept_once", "expected_generation": float64(3), "operation_id": "op-1", "bundle_digest": hex("b"),
		"expected_desired_event_head": hex("d"), "expected_effective_event_head": hex("e"), "expected_pending_transition_ref": "",
		"expected_desired_state_commitment": hex("1"), "expected_effective_state_commitment": hex("2")} {
		if arm[field] != value {
			t.Fatalf("arm %s = %#v, want %#v (body %#v)", field, arm[field], value, arm)
		}
	}
	expires, err := time.Parse(time.RFC3339Nano, arm["expires_at"].(string))
	if err != nil || expires.Location() != time.UTC || !expires.After(time.Now()) || expires.Format(time.RFC3339Nano) != arm["expires_at"] {
		t.Fatalf("expires_at %v is not a future canonical UTC time", arm["expires_at"])
	}
	decision := stub.bodies["POST /operations/op-1/decision"]
	for field, value := range map[string]any{"decision": "approve", "bundle_digest": hex("b"), "verifier_ref": "verifier:jev",
		"expected_desired_event_head": hex("d"), "expected_effective_event_head": hex("e"), "expected_pending_transition_ref": "",
		"expected_desired_state_commitment": hex("1"), "expected_effective_state_commitment": hex("2")} {
		if decision[field] != value {
			t.Fatalf("decision %s = %#v, want %#v", field, decision[field], value)
		}
	}
	if key, _ := decision["idempotency_key"].(string); key == "" {
		t.Fatal("decision has no idempotency key")
	}
}

func TestSelfDevApproveRefusesBeforeAnyWriteUnlessAwaitingApproval(t *testing.T) {
	stub, server := newSelfDevStub(t, "executing")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"self-dev", "approve", "--computer=computer-x", "--operation=op-1", "--host=" + server.URL}, &stdout, &stderr); code == 0 {
		t.Fatalf("approve of an executing operation succeeded: %s", stdout.String())
	}
	for _, request := range stub.requests {
		if !strings.HasPrefix(request, "GET ") {
			t.Fatalf("approve wrote %s for an operation not awaiting approval", request)
		}
	}
}

// Reject carries the owner's reason, needs no mode arming, and binds the same
// frozen candidate and heads.
func TestSelfDevRejectBindsTheCandidateWithAReason(t *testing.T) {
	stub, server := newSelfDevStub(t, "awaiting_approval")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"self-dev", "reject", "--computer=computer-x", "--operation=op-1", "--reason=tests fail", "--host=" + server.URL}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Join(stub.requests, ",") != "GET /operations/op-1,GET /head,POST /operations/op-1/decision" {
		t.Fatalf("requests = %v", stub.requests)
	}
	if body := stub.bodies["POST /operations/op-1/decision"]; body["decision"] != "reject" || body["reason"] != "tests fail" || body["bundle_digest"] != strings.Repeat("b", 64) {
		t.Fatalf("reject body = %#v", body)
	}
	if code := run([]string{"self-dev", "reject", "--computer=computer-x", "--operation=op-1", "--host=" + server.URL}, &stdout, &stderr); code != 2 {
		t.Fatalf("reject without a reason code=%d, want usage error", code)
	}
}

func TestSelfDevListPassesStateFilters(t *testing.T) {
	stub, server := newSelfDevStub(t, "awaiting_approval")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"self-dev", "list", "--computer=computer-x", "--state=awaiting_approval", "--state=frozen", "--host=" + server.URL}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if len(stub.requests) != 1 || stub.requests[0] != "GET /operations?state=awaiting_approval&state=frozen" {
		t.Fatalf("requests = %v", stub.requests)
	}
	if !strings.Contains(stdout.String(), `"op-1"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

// wait returns as soon as the operation reaches a wanted state, and fails
// (rather than waiting out its deadline) when the operation settles elsewhere.
func TestSelfDevWaitStopsAtWantedOrSettledState(t *testing.T) {
	_, server := newSelfDevStub(t, "awaiting_approval")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"self-dev", "wait", "--computer=computer-x", "--operation=op-1", "--state=awaiting_approval,applied", "--interval=10ms", "--deadline=2s", "--host=" + server.URL}, &stdout, &stderr); code != 0 {
		t.Fatalf("wait code=%d stderr=%s", code, stderr.String())
	}
	_, settled := newSelfDevStub(t, "failed")
	started := time.Now()
	if code := run([]string{"self-dev", "wait", "--computer=computer-x", "--operation=op-1", "--state=applied", "--interval=10ms", "--deadline=5s", "--host=" + settled.URL}, &stdout, &stderr); code != 1 {
		t.Fatalf("wait on a failed operation code=%d, want 1", code)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("wait sat out its deadline on a settled operation")
	}
}
