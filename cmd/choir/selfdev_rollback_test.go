package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// rollbackStub answers the read routes `choir self-dev rollback` resolves its
// seven bindings from, and the rollbacks POST, for computer-x. It records
// every request so a test can pin which route supplied which binding and that
// nothing was posted when the CLI refused.
type rollbackStub struct {
	t  *testing.T
	mu sync.Mutex
	// ops are the operations GET /operations/{id} answers, by id.
	ops  map[string]map[string]any
	head map[string]any
	// routeGeneration is the listing's route_generation (0 omits it, as a
	// guest whose route does not resolve does).
	routeGeneration uint64
	// settled is what GET /operations (unfiltered) lists, authored in ascending created_at order.
	settled []map[string]any
	// inflight is what GET /operations?state=rollback_pending lists.
	inflight []map[string]any
	// postStatus and postBody are the rollbacks POST answer.
	postStatus int
	postBody   string
	// rollbackStates is the state sequence GET /operations/rollback-1
	// reports, the last one repeating.
	rollbackStates []string
	requests       []string
	posts          []map[string]any
}

func rhex(c string) string { return strings.Repeat(c, 64) }

func (s *rollbackStub) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	const base = "/api/computers/computer-x/self-development"
	key := r.Method + " " + strings.TrimPrefix(r.URL.Path, base)
	if r.URL.RawQuery != "" {
		key += "?" + r.URL.RawQuery
	}
	s.requests = append(s.requests, key)
	switch {
	case key == "GET /head":
		_ = json.NewEncoder(w).Encode(s.head)
	case key == "GET /operations?state=rollback_pending":
		listing := map[string]any{"operations": s.inflight}
		if s.routeGeneration != 0 {
			listing["route_generation"] = s.routeGeneration
		}
		_ = json.NewEncoder(w).Encode(listing)
	case key == "GET /operations":
		// The real listing is newest-created first; tests author ascending.
		newest := make([]map[string]any, len(s.settled))
		for i, op := range s.settled {
			newest[len(s.settled)-1-i] = op
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"operations": newest})
	case key == "POST /rollbacks":
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		s.posts = append(s.posts, body)
		w.WriteHeader(s.postStatus)
		_, _ = io.WriteString(w, s.postBody)
	case key == "GET /operations/rollback-1" && len(s.rollbackStates) > 0:
		state := s.rollbackStates[0]
		if len(s.rollbackStates) > 1 {
			s.rollbackStates = s.rollbackStates[1:]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "rollback-1", "state": state})
	case r.Method == http.MethodGet && strings.HasPrefix(strings.TrimPrefix(r.URL.Path, base), "/operations/"):
		op, ok := s.ops[strings.TrimPrefix(r.URL.Path, base+"/operations/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"self-development operation not found"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(op)
	default:
		s.t.Errorf("unexpected request %s", key)
		w.WriteHeader(http.StatusNotFound)
	}
}

// newRollbackStub is a computer with one older applied operation (op-old,
// effective head "a") and the current one (op-cur, effective head "e"), a
// settled head and route generation 7.
func newRollbackStub(t *testing.T) (*rollbackStub, *httptest.Server) {
	stub := &rollbackStub{
		t: t,
		ops: map[string]map[string]any{
			"op-old": {
				"operation_id": "op-old", "state": "applied", "effective_head": rhex("a"), "desired_head": rhex("a"),
				"materialization_receipt": rhex("7"), "checkpoint_ref": "checkpoint:sha256:" + rhex("5"),
				"route_generation": 3, "decision_actor": "owner-1",
			},
			"op-cur": {
				"operation_id": "op-cur", "state": "applied", "effective_head": rhex("e"), "desired_head": rhex("e"),
				"materialization_receipt": rhex("8"), "checkpoint_ref": "checkpoint:sha256:" + rhex("6"),
				"route_generation": 7, "decision_actor": "owner-1",
			},
			"op-failed":   {"operation_id": "op-failed", "state": "failed", "effective_head": rhex("f")},
			"op-rolled":   {"operation_id": "op-rolled", "state": "rolled_back", "effective_head": rhex("b"), "materialization_receipt": rhex("7"), "checkpoint_ref": "checkpoint:sha256:" + rhex("5")},
			"genesis-123": {"operation_id": "genesis-123", "state": "applied", "effective_head": rhex("4"), "materialization_receipt": rhex("7"), "checkpoint_ref": "checkpoint:sha256:" + rhex("5"), "decision_actor": "external-owner-genesis"},
			"op-nockpt":   {"operation_id": "op-nockpt", "state": "applied", "effective_head": rhex("c"), "materialization_receipt": rhex("7")},
		},
		head: map[string]any{
			"canonical_event_head": rhex("c"), "sequence": 9, "desired_event_head": rhex("d"), "effective_event_head": rhex("e"),
			"pending_transition_ref": "", "desired_state_commitment": rhex("1"), "effective_state_commitment": rhex("2"),
		},
		routeGeneration: 7,
		postStatus:      http.StatusCreated,
		postBody:        `{"operation_id":"rollback-1","state":"rollback_pending"}`,
	}
	server := httptest.NewServer(http.HandlerFunc(stub.serve))
	t.Cleanup(server.Close)
	return stub, server
}

func runRollback(server *httptest.Server, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	full := append([]string{"self-dev", "rollback", "--computer=computer-x", "--host=" + server.URL}, args...)
	code := run(full, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (s *rollbackStub) posted() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.posts)
}

// Each of the handler's seven bindings comes from one public read route:
// the target operation (to_applied_head, prior_materialization,
// prior_checkpoint), the event head (expected_desired_head,
// current_applied_head) and the operations listing (expected_route_generation).
// A binding taken from the wrong route is refused by the guest as a changed
// precondition, so the body is pinned field by field.
func TestSelfDevRollbackResolvesEveryBindingFromTheReadRoutes(t *testing.T) {
	stub, server := newRollbackStub(t)
	code, stdout, stderr := runRollback(server, "--to-operation=op-old", "--idempotency-key=K1")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if len(stub.posts) != 1 {
		t.Fatalf("posts = %d, want 1 (requests %v)", len(stub.posts), stub.requests)
	}
	want := map[string]any{
		"expected_desired_head":     rhex("d"),
		"current_applied_head":      rhex("e"),
		"to_applied_head":           rhex("a"),
		"prior_materialization":     rhex("7"),
		"prior_checkpoint":          "checkpoint:sha256:" + rhex("5"),
		"expected_route_generation": float64(7),
		"idempotency_key":           "K1",
	}
	if len(stub.posts[0]) != len(want) {
		t.Fatalf("body has %d fields, want exactly the handler's %d: %#v", len(stub.posts[0]), len(want), stub.posts[0])
	}
	for field, value := range want {
		if stub.posts[0][field] != value {
			t.Fatalf("%s = %#v, want %#v", field, stub.posts[0][field], value)
		}
	}
	for _, route := range []string{"GET /operations/op-old", "GET /head", "GET /operations?state=rollback_pending"} {
		found := false
		for _, got := range stub.requests {
			found = found || got == route
		}
		if !found {
			t.Fatalf("never read %s; requests %v", route, stub.requests)
		}
	}
	var printed map[string]any
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil || printed["operation_id"] != "rollback-1" || printed["state"] != "rollback_pending" {
		t.Fatalf("stdout is not the server's JSON: %q (%v)", stdout, err)
	}
}

// The CLI refuses what the guest would refuse, and what is plainly the wrong
// target, before it posts anything: a target that is not an applied
// operation, the current one, the genesis baseline, one that records no
// checkpoint, a missing one, and a head with a transition still pending.
func TestSelfDevRollbackRefusesBeforeAnyPost(t *testing.T) {
	for _, tc := range []struct {
		name, target, wantMessage string
		mutate                    func(*rollbackStub)
	}{
		{name: "failed operation", target: "op-failed", wantMessage: "not applied"},
		{name: "rolled back operation", target: "op-rolled", wantMessage: "not applied"},
		{name: "the current operation", target: "op-cur", wantMessage: "current"},
		{name: "the genesis baseline", target: "genesis-123", wantMessage: "genesis"},
		{name: "no recorded checkpoint", target: "op-nockpt", wantMessage: "checkpoint"},
		{name: "unknown operation", target: "op-nope", wantMessage: "404"},
		{name: "pending transition", target: "op-old", wantMessage: "pending", mutate: func(s *rollbackStub) {
			s.head["pending_transition_ref"] = rhex("9")
		}},
		{name: "route generation not reported", target: "op-old", wantMessage: "route generation", mutate: func(s *rollbackStub) {
			s.routeGeneration = 0
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub, server := newRollbackStub(t)
			if tc.mutate != nil {
				tc.mutate(stub)
			}
			code, stdout, stderr := runRollback(server, "--to-operation="+tc.target)
			if code == 0 {
				t.Fatalf("code = 0, want a refusal (stdout %s)", stdout)
			}
			if !strings.Contains(stderr, tc.wantMessage) {
				t.Fatalf("stderr %q does not say %q", stderr, tc.wantMessage)
			}
			if stub.posted() != 0 {
				t.Fatalf("posted %d rollbacks after refusing", stub.posted())
			}
		})
	}
}

func TestSelfDevRollbackRequiresATargetOperation(t *testing.T) {
	stub, server := newRollbackStub(t)
	code, _, stderr := runRollback(server)
	if code != 2 || !strings.Contains(stderr, "--to-operation") {
		t.Fatalf("code=%d stderr=%q, want a usage refusal naming --to-operation", code, stderr)
	}
	if len(stub.requests) != 0 {
		t.Fatalf("made requests %v with no target", stub.requests)
	}
}

// A retry carries the same idempotency key, so the guest can replay what
// landed: the explicit key as given; and, with none, a default that is the
// same on every retry of this attempt and differs for another target.
func TestSelfDevRollbackRetryReusesTheIdempotencyKey(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.postStatus, stub.postBody = http.StatusServiceUnavailable, `{"error":"rollback authority unavailable"}`
	if code, _, _ := runRollback(server, "--to-operation=op-old", "--idempotency-key=K9"); code == 0 {
		t.Fatal("a 503 exited 0")
	}
	stub.postStatus, stub.postBody = http.StatusOK, `{"operation_id":"rollback-1","state":"rollback_pending"}`
	if code, _, stderr := runRollback(server, "--to-operation=op-old", "--idempotency-key=K9"); code != 0 {
		t.Fatalf("retry code=%d stderr=%s", code, stderr)
	}
	if len(stub.posts) != 2 || stub.posts[0]["idempotency_key"] != "K9" || stub.posts[1]["idempotency_key"] != "K9" {
		t.Fatalf("explicit key not reused: %#v", stub.posts)
	}

	stub.posts = nil
	for i := 0; i < 2; i++ {
		if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
			t.Fatalf("default-key run %d code=%d stderr=%s", i, code, stderr)
		}
	}
	if len(stub.posts) != 2 {
		t.Fatalf("posts = %d, want 2", len(stub.posts))
	}
	first, _ := stub.posts[0]["idempotency_key"].(string)
	if first == "" || first != stub.posts[1]["idempotency_key"] || !strings.Contains(first, "op-old") {
		t.Fatalf("default key %q is not stable across retries and tied to the target (%#v)", first, stub.posts)
	}
}

// A conflict from the guest is the guest's answer: status and body reach the
// caller unchanged, with no second post.
func TestSelfDevRollbackSurfacesAServerConflictUnchanged(t *testing.T) {
	for _, body := range []string{
		`{"error":"rollback head preconditions changed"}`,
		`{"error":"rollback to the genesis baseline is not supported: it has no verification evidence to record the rollback against"}`,
	} {
		stub, server := newRollbackStub(t)
		stub.postStatus, stub.postBody = http.StatusConflict, body
		code, stdout, stderr := runRollback(server, "--to-operation=op-old")
		if code != 1 || stdout != "" {
			t.Fatalf("code=%d stdout=%q, want 1 and nothing on stdout", code, stdout)
		}
		if !strings.Contains(stderr, "http 409") || !strings.Contains(stderr, strings.TrimSpace(body)) {
			t.Fatalf("stderr %q does not carry the 409 and its body %s", stderr, body)
		}
		if stub.posted() != 1 {
			t.Fatalf("posted %d times, want 1", stub.posted())
		}
	}
}

// A first call that landed but whose answer was lost leaves the head
// pending on the rollback; the retry finds the rollback_pending operation for
// this target and answers with it instead of posting bindings that moved.
func TestSelfDevRollbackFindsAnInFlightRollbackToTheSameTarget(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.head["pending_transition_ref"] = rhex("9")
	stub.inflight = []map[string]any{{"operation_id": "rollback-1", "state": "rollback_pending", "base_head": rhex("a")}}
	code, stdout, stderr := runRollback(server, "--to-operation=op-old")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if stub.posted() != 0 || !strings.Contains(stdout, `"rollback-1"`) {
		t.Fatalf("posted=%d stdout=%q, want no post and the in-flight operation", stub.posted(), stdout)
	}

	// An in-flight rollback to a different target is not this one.
	stub.inflight = []map[string]any{{"operation_id": "rollback-2", "state": "rollback_pending", "base_head": rhex("b")}}
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code == 0 || !strings.Contains(stderr, "pending") {
		t.Fatalf("code=%d stderr=%q, want a pending-transition refusal", code, stderr)
	}
}

func TestSelfDevRollbackWaitPollsToRolledBack(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.rollbackStates = []string{"rollback_pending", "rollback_pending", "rolled_back"}
	code, stdout, stderr := runRollback(server, "--to-operation=op-old", "--wait", "--interval=100ms", "--deadline=5s")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"rolled_back"`) {
		t.Fatalf("stdout %q is not the settled operation", stdout)
	}

	stub, server = newRollbackStub(t)
	stub.rollbackStates = []string{"rollback_pending", "failed"}
	code, stdout, stderr = runRollback(server, "--to-operation=op-old", "--wait", "--interval=100ms", "--deadline=5s")
	if code != 1 || !strings.Contains(stdout, `"failed"`) || !strings.Contains(stderr, "failed") {
		t.Fatalf("a failed rollback: code=%d stdout=%q stderr=%q, want 1, the operation and a message", code, stdout, stderr)
	}
}

func TestSelfDevRollbackHelpNamesTheOwnerActScope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"self-dev", "rollback", "--help"}, &stdout, &stderr)
	help := stdout.String() + stderr.String()
	for _, want := range []string{"owner:acts", "computer:self_development:rollback", "-to-operation", "-wait", "CLI only"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help does not mention %q:\n%s", want, help)
		}
	}
}

func (s *rollbackStub) postedKey(i int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i >= len(s.posts) {
		return ""
	}
	k, _ := s.posts[i]["idempotency_key"].(string)
	return k
}

// op builds a listing row. created is when it was created; updated is its
// last write (which a late refusal or a degrade bumps past its settle time).
func lop(id, state, created, updated, baseHead string) map[string]any {
	return map[string]any{"operation_id": id, "state": state, "created_at": created, "updated_at": updated, "base_head": baseHead}
}

// The default key names one rollback intent: the target and the latest
// head-landing operation (applied, degraded, failed or rolled_back) by
// created_at, which no later write moves. The effective head moves when a
// rollback lands, so it must not be in the key, or a re-run after success
// starts a second rollback.
func TestSelfDevRollbackDefaultKeySurvivesTheHeadMoving(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.settled = []map[string]any{
		lop("op-old", "applied", "2026-10-26T10:00:00Z", "2026-10-26T10:05:00Z", ""),
		lop("op-cur", "applied", "2026-10-26T11:00:00Z", "2026-10-26T11:05:00Z", ""),
	}
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
		t.Fatalf("first run code=%d stderr=%s", code, stderr)
	}
	stub.head["effective_event_head"] = rhex("9") // the head moved; nothing settled since
	// A late refusal bumps an older operation's updated_at; the anchor must not drift.
	stub.settled[0]["updated_at"] = "2026-10-26T11:30:00Z"
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
		t.Fatalf("second run code=%d stderr=%s", code, stderr)
	}
	first, second := stub.postedKey(0), stub.postedKey(1)
	if first == "" || first != second {
		t.Fatalf("default key changed with the head or a late bump: %q then %q", first, second)
	}
	if !strings.Contains(first, "op-old") || !strings.Contains(first, "op-cur") || strings.Contains(first, rhex("e")[:12]) {
		t.Fatalf("default key %q must name the target and the latest operation, not the head", first)
	}
	for _, state := range []string{"applied", "degraded", "failed"} {
		stub.settled = append(stub.settled[:2], lop("op-new", state, "2026-10-26T12:00:00Z", "2026-10-26T12:00:00Z", ""))
		stub.mu.Lock()
		n := len(stub.posts)
		stub.mu.Unlock()
		if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
			t.Fatalf("%s run code=%d stderr=%s", state, code, stderr)
		}
		if k := stub.postedKey(n); k == first || !strings.Contains(k, "op-new") {
			t.Fatalf("key after a %s op = %q, want a new one naming op-new (was %q)", state, k, first)
		}
	}
}

// A rollback to the target that is already the latest act, with nothing
// else started or written since, is refused before any POST. When unsure the
// CLI falls through and lets the guest decide.
func TestSelfDevRollbackRefusesARepeatToTheSameTarget(t *testing.T) {
	stub, server := newRollbackStub(t)
	base := func() []map[string]any {
		return []map[string]any{
			lop("op-old", "applied", "2026-10-26T10:00:00Z", "2026-10-26T10:05:00Z", ""),
			lop("op-cur", "applied", "2026-10-26T11:00:00Z", "2026-10-26T11:05:00Z", ""),
			lop("rollback-0", "rolled_back", "2026-10-26T12:00:00Z", "2026-10-26T12:00:10Z", rhex("a")),
		}
	}
	stub.settled = base()
	code, _, stderr := runRollback(server, "--to-operation=op-old")
	if code == 0 || !strings.Contains(stderr, "already rolled back to op-old by rollback-0") {
		t.Fatalf("code=%d stderr=%q, want the already-rolled-back refusal", code, stderr)
	}
	if stub.posted() != 0 {
		t.Fatalf("posted %d times after the refusal", stub.posted())
	}

	for name, mutate := range map[string]func([]map[string]any) []map[string]any{
		"different earlier target": func(o []map[string]any) []map[string]any { o[2]["base_head"] = rhex("b"); return o },
		"applied since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "applied", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		"degraded since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "degraded", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		"failed since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "failed", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		// Created before the rollback began but written after it began: it may
		// have landed after, so the refusal is not certain.
		"older op written during the rollback": func(o []map[string]any) []map[string]any {
			o[1]["updated_at"] = "2026-10-26T12:00:05Z"
			return o
		},
		"materializing since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "materializing", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		"accepted since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "accepted", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		"rollback pending since": func(o []map[string]any) []map[string]any {
			return append(o, lop("rollback-1", "rollback_pending", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", rhex("a")))
		},
		"awaiting approval since": func(o []map[string]any) []map[string]any {
			return append(o, lop("op-new", "awaiting_approval", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""))
		},
		"unparseable time": func(o []map[string]any) []map[string]any { o[1]["updated_at"] = "soon"; return o },
	} {
		stub.settled = mutate(base())
		n := stub.posted()
		if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
			t.Fatalf("%s: code=%d stderr=%s, want the POST to proceed", name, code, stderr)
		}
		if stub.posted() != n+1 {
			t.Fatalf("%s: did not post", name)
		}
	}
}

// "…:00Z" sorts after "…:00.5Z" as a string but is the earlier instant, so a
// string comparison would call the older operation a late write and disarm
// the refusal.
// A rejected operation never moves the head: it does not disarm the refusal.
func TestSelfDevRollbackRejectedOperationsDoNotDisarmTheRefusal(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.settled = []map[string]any{
		lop("op-old", "applied", "2026-10-26T10:00:00Z", "2026-10-26T10:05:00Z", ""),
		lop("rollback-0", "rolled_back", "2026-10-26T12:00:00Z", "2026-10-26T12:00:10Z", rhex("a")),
		lop("op-no", "rejected", "2026-10-26T13:00:00Z", "2026-10-26T13:00:00Z", ""),
	}
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code == 0 || !strings.Contains(stderr, "already rolled back") {
		t.Fatalf("code=%d stderr=%q, want the refusal", code, stderr)
	}
}

// A landed operation whose created_at cannot be parsed cannot be ordered; it
// anchors the key (so the key rotates rather than naming its predecessor).
func TestSelfDevRollbackUnparseableCreatedAtAnchorsTheKey(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.settled = []map[string]any{
		lop("op-old", "applied", "2026-10-26T10:00:00Z", "2026-10-26T10:05:00Z", ""),
		lop("op-odd", "applied", "garbage", "2026-10-26T11:05:00Z", ""),
	}
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if k := stub.postedKey(0); !strings.Contains(k, "op-odd") {
		t.Fatalf("key %q does not name the unorderable newest-landed op-odd", k)
	}
}

func TestSelfDevRollbackComparesTimesAsInstants(t *testing.T) {
	stub, server := newRollbackStub(t)
	stub.settled = []map[string]any{
		lop("op-old", "applied", "2026-10-26T10:00:00Z", "2026-10-26T10:05:00Z", ""),
		lop("op-cur", "applied", "2026-10-26T11:00:00Z", "2026-10-26T12:00:00Z", ""),
		lop("rollback-0", "rolled_back", "2026-10-26T12:00:00.5Z", "2026-10-26T12:00:10Z", rhex("a")),
	}
	if code, _, stderr := runRollback(server, "--to-operation=op-old"); code == 0 || !strings.Contains(stderr, "already rolled back") {
		t.Fatalf("code=%d stderr=%q, want the refusal", code, stderr)
	}
}

func TestSelfDevRollbackRefusesABusyPollInterval(t *testing.T) {
	stub, server := newRollbackStub(t)
	for _, interval := range []string{"0s", "1ms", "-1s"} {
		code, _, stderr := runRollback(server, "--to-operation=op-old", "--wait", "--interval="+interval, "--deadline=200ms")
		if code == 0 || !strings.Contains(stderr, "interval") {
			t.Fatalf("--interval=%s: code=%d stderr=%q, want an interval refusal", interval, code, stderr)
		}
	}
	if stub.posted() != 0 {
		t.Fatalf("posted %d times with a busy interval", stub.posted())
	}
}

func TestSelfDevRollbackHelpSaysHowToRetryAfterAFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"self-dev", "rollback", "--help"}, &stdout, &stderr)
	help := stdout.String() + stderr.String()
	if !strings.Contains(help, "rotates after a failed rollback") || strings.Contains(help, "replays a failed rollback") || strings.Contains(help, "fresh --idempotency-key") {
		t.Fatalf("help must say the default key rotates after a failed rollback, not that it replays it:\n%s", help)
	}
}
