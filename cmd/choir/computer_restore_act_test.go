package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// `choir computer restore` submits an accepted act and waits for its end
// (docs/problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-
// 14.md, failure list 19). The stub is a guest that answers the way the
// accepted-act routes do, with scripted status answers.

type restoreActStub struct {
	mu       sync.Mutex
	server   *httptest.Server
	posts    int
	gets     []string
	statuses []restoreStubAnswer // consumed in order; the last repeats
}

type restoreStubAnswer struct {
	code int
	body string
}

func newRestoreActStub(t *testing.T, statuses ...restoreStubAnswer) *restoreActStub {
	t.Helper()
	s := &restoreActStub{statuses: statuses}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			s.posts++
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"act_id":"ra-1","state":"accepted","status_url":"/api/computers/computer-1/lifecycle/restore-status?act=ra-1","estimate":{"tail_events":100,"replay_rate":113,"replay_rate_source":"assumed","estimated_seconds":0.9}}`)
			return
		}
		s.gets = append(s.gets, r.URL.Path+"?"+r.URL.RawQuery)
		answer := s.statuses[0]
		if len(s.statuses) > 1 {
			s.statuses = s.statuses[1:]
		}
		w.WriteHeader(answer.code)
		_, _ = io.WriteString(w, answer.body)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func replaying(applied, total int) restoreStubAnswer {
	raw, _ := json.Marshal(map[string]any{
		"act_id": "ra-1", "state": "replaying", "tail_events_total": total, "tail_events_applied": applied,
		"replay_rate": 113.0, "replay_rate_source": "measured", "estimated_seconds_remaining": float64(total-applied) / 113.0,
	})
	return restoreStubAnswer{http.StatusOK, string(raw)}
}

const restoreSucceeded = `{"act_id":"ra-1","state":"succeeded","result":{"computer_id":"computer-1","witness_matched":true,"frontend_restaged":true,"tail_events_applied":100}}`

func restoreCheckpointFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(path, []byte(`{"digest":"abc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runRestore(t *testing.T, stub *restoreActStub, extra ...string) (int, string, string) {
	t.Helper()
	args := append([]string{"computer", "restore", "--host=" + stub.server.URL, "--computer=computer-1", "--checkpoint-file=" + restoreCheckpointFile(t), "--interval=100ms"}, extra...)
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestComputerRestoreWaits(t *testing.T) {
	stub := newRestoreActStub(t, replaying(10, 100), replaying(60, 100), restoreStubAnswer{http.StatusOK, restoreSucceeded})
	code, out, errOut := runRestore(t, stub)
	if code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"witness_matched": true`) && !strings.Contains(out, `"witness_matched":true`) {
		t.Fatalf("stdout is not the result body: %s", out)
	}
	if strings.Contains(out, "act_id") {
		t.Fatalf("stdout carries the act, not the result the synchronous route returned: %s", out)
	}
	for _, want := range []string{"ra-1", "replaying", "60/100"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("stderr shows no progress line with %q: %s", want, errOut)
		}
	}
	if stub.posts != 1 || len(stub.gets) < 3 {
		t.Fatalf("posts=%d gets=%v", stub.posts, stub.gets)
	}
	for _, get := range stub.gets {
		if get != "/api/computers/computer-1/lifecycle/restore-status?act=ra-1" {
			t.Fatalf("poll %q", get)
		}
	}
}

func TestComputerRestoreNoWait(t *testing.T) {
	stub := newRestoreActStub(t, restoreStubAnswer{http.StatusOK, restoreSucceeded})
	code, out, errOut := runRestore(t, stub, "--no-wait")
	if code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"act_id"`) || !strings.Contains(out, "ra-1") || !strings.Contains(out, "restore-status") {
		t.Fatalf("--no-wait printed %s, want the act id and its status URL", out)
	}
	if len(stub.gets) != 0 {
		t.Fatalf("--no-wait polled: %v", stub.gets)
	}
}

func TestComputerRestoreStatus(t *testing.T) {
	stub := newRestoreActStub(t, replaying(5, 10))
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "restore-status", "--host=" + stub.server.URL, "--computer=computer-1", "--act=ra-1"}, &out, &errOut); code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "replaying") || len(stub.gets) != 1 || stub.gets[0] != "/api/computers/computer-1/lifecycle/restore-status?act=ra-1" {
		t.Fatalf("stdout=%s gets=%v", out.String(), stub.gets)
	}
	out.Reset()
	if code := run([]string{"computer", "restore-status", "--host=" + stub.server.URL, "--computer=computer-1"}, &out, &errOut); code != 0 {
		t.Fatalf("no act: code = %d, stderr=%s", code, errOut.String())
	}
	if stub.gets[1] != "/api/computers/computer-1/lifecycle/restore-status?" {
		t.Fatalf("no act asked for %q, want the running or newest act", stub.gets[1])
	}
}

func TestComputerRestoreFailedExitsOne(t *testing.T) {
	failed := `{"act_id":"ra-1","state":"failed","refusal":{"name":"witness_mismatch","http_status":409,"message":"rematerialize: reconstructed computer_event_index does not match checkpoint"}}`
	stub := newRestoreActStub(t, replaying(1, 100), restoreStubAnswer{http.StatusOK, failed})
	code, out, errOut := runRestore(t, stub)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stdout=%s stderr=%s", code, out, errOut)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("a refused restore printed a result: %s", out)
	}
	if !strings.Contains(errOut, "witness_mismatch") || !strings.Contains(errOut, "409") {
		t.Fatalf("stderr does not name the refusal and its status: %s", errOut)
	}

	interrupted := `{"act_id":"ra-1","state":"interrupted","reason":"interrupted by a restart"}`
	stub = newRestoreActStub(t, restoreStubAnswer{http.StatusOK, interrupted})
	code, out, errOut = runRestore(t, stub)
	if code != 1 || !strings.Contains(errOut, "interrupted by a restart") {
		t.Fatalf("interrupted: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
}

func TestComputerRestoreToleratesATransientPollError(t *testing.T) {
	stub := newRestoreActStub(t,
		restoreStubAnswer{http.StatusBadGateway, `{"error":"computer authority unavailable"}`},
		restoreStubAnswer{http.StatusServiceUnavailable, `{"error":"starting"}`},
		restoreStubAnswer{http.StatusOK, `{"act_id":"ra-1","state":"interrupted","reason":"interrupted by a restart"}`},
	)
	code, _, errOut := runRestore(t, stub)
	if code != 1 || !strings.Contains(errOut, "interrupted by a restart") {
		t.Fatalf("a 502 while the guest restarts ended the wait: code=%d stderr=%s", code, errOut)
	}
	if len(stub.gets) != 3 {
		t.Fatalf("polls = %v, want the two refused answers retried", stub.gets)
	}

	// A refusal that is not transient ends the wait.
	stub = newRestoreActStub(t, restoreStubAnswer{http.StatusForbidden, `{"error":"owner act"}`})
	if code, _, errOut := runRestore(t, stub); code != 1 || len(stub.gets) != 1 {
		t.Fatalf("a 403 was retried: code=%d polls=%v stderr=%s", code, stub.gets, errOut)
	}
}

func TestComputerRestoreTimeoutLeavesTheActRunning(t *testing.T) {
	stub := newRestoreActStub(t, replaying(1, 100))
	started := time.Now()
	code, out, errOut := runRestore(t, stub, "--timeout=1s")
	if code != 1 || time.Since(started) > 10*time.Second {
		t.Fatalf("code=%d after %s; stdout=%s stderr=%s", code, time.Since(started), out, errOut)
	}
	if !strings.Contains(errOut, "ra-1") || !strings.Contains(errOut, "restore-status") || !strings.Contains(errOut, "still running") {
		t.Fatalf("stderr does not say the act goes on and how to read it: %s", errOut)
	}
}

// The CLI says what is on the computer only when the act's record proves it
// (review round 1: "the computer is on the state it had before the restore"
// was printed over a computer the boot had left on the restored state).
func TestComputerRestoreSaysOnlyWhatIsProven(t *testing.T) {
	run1 := func(body string) (int, string, string) {
		stub := newRestoreActStub(t, restoreStubAnswer{http.StatusOK, body})
		return runRestore(t, stub)
	}
	code, _, errOut := run1(`{"act_id":"ra-1","state":"interrupted","live_state":"pre_restore","reason":"interrupted by a restart"}`)
	if code != 1 || !strings.Contains(errOut, "pre-restore state") {
		t.Fatalf("interrupted and proven: code=%d stderr=%s", code, errOut)
	}
	code, _, errOut = run1(`{"act_id":"ra-1","state":"interrupted","reason":"interrupted by a restart"}`)
	if code != 1 || strings.Contains(errOut, "pre-restore state") || strings.Contains(errOut, "before the restore") || !strings.Contains(errOut, "not proven") {
		t.Fatalf("interrupted, nothing proven: code=%d stderr=%s", code, errOut)
	}
	code, out, errOut := run1(`{"act_id":"ra-1","state":"restored","live_state":"restored","reason":"restored; restart required: the flipped store would not open"}`)
	if code != 1 || strings.TrimSpace(out) != "" || !strings.Contains(errOut, "restored") || !strings.Contains(errOut, "restart") || strings.Contains(errOut, "pre-restore") {
		t.Fatalf("restored: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	code, _, errOut = run1(`{"act_id":"ra-1","state":"failed","live_state":"unresolved","refusal":{"name":"error","http_status":500,"message":"flip undo failed"}}`)
	if code != 1 || strings.Contains(errOut, "pre-restore state") {
		t.Fatalf("failed, unresolved: code=%d stderr=%s", code, errOut)
	}
}

// An act that is open with an unresolved live state ends the wait: it will not
// move by itself, and the CLI must say so rather than poll until its deadline.
func TestComputerRestoreStopsAtAnUnresolvedAct(t *testing.T) {
	stub := newRestoreActStub(t, restoreStubAnswer{http.StatusOK, `{"act_id":"ra-1","state":"installing","live_state":"unresolved","reason":"flip completion could not be recorded"}`})
	started := time.Now()
	code, _, errOut := runRestore(t, stub)
	if code != 1 || time.Since(started) > 5*time.Second || !strings.Contains(errOut, "unresolved") {
		t.Fatalf("code=%d after %s stderr=%s", code, time.Since(started), errOut)
	}
}
