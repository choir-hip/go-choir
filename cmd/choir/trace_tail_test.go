package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// `choir trace tail <trajectory-id>` follows one trajectory by cursor over the
// product routes `trace export` reads. Ways it can fail, written before the
// code:
//   - a poll re-prints events it already printed, or never advances a cursor
//     (the requests must carry after=<last seen>);
//   - history is replayed as if it were live, so a restart that closed work
//     an hour ago ends the tail now;
//   - a transient route error (a computer restart answers 5xx for ~30s) ends
//     the tail, or is swallowed without a visible line;
//   - a bad key on the very first read loops forever instead of exiting;
//   - a revoked capsule with no later assignment is missed (the round that
//     stopped), or fires even though a new assignment opened;
//   - an assignment cancelled interrupted_by_restart is missed, or two
//     terminal states collapse into one exit code;
//   - --until does not restrict which states end the tail;
//   - an awaiting_approval operation of ANOTHER trajectory ends this tail;
//   - -vv shows nothing of a tool call, or -v shows tool outputs;
//   - --json prints a line that is not JSON.
//
// The fixtures are shaped like the real payloads captured read-only from the
// 150b7de5 trajectory (lifecycle events, run events, runs, snapshot,
// self-development operations).

const tailTraj = "150b7de5-682f-5361-a065-54797a7ba54f"

type tailWorld struct {
	mu        sync.Mutex
	lifecycle []map[string]any
	runs      []map[string]any
	runEvents map[string][]map[string]any
	ops       []map[string]any
	status    string
	failAll   int // next N polls answer 503
	authFail  bool
	requests  []string
}

func newTailWorld() *tailWorld {
	return &tailWorld{runEvents: map[string][]map[string]any{}, status: "live"}
}

func (w *tailWorld) addLifecycle(kind string, extra map[string]any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	seq := len(w.lifecycle) + 1
	ev := map[string]any{
		"event_id": fmt.Sprintf("ev-%d", seq), "trajectory_id": tailTraj, "kind": kind,
		"computer_id": "computer-6c50d6e566143ae32cdb660faa60e24d",
		"reducer_seq": seq, "created_at": fmt.Sprintf("2026-10-27T01:%02d:00Z", seq),
	}
	for k, v := range extra {
		ev[k] = v
	}
	w.lifecycle = append(w.lifecycle, ev)
}

func (w *tailWorld) addRun(loop, agent, state string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range w.runs {
		if r["loop_id"] == loop {
			r["state"] = state
			return
		}
	}
	w.runs = append(w.runs, map[string]any{"loop_id": loop, "agent_id": agent, "trajectory_id": tailTraj, "state": state})
}

func (w *tailWorld) addRunEvent(loop, agent, kind string, payload map[string]any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	seq := len(w.runEvents[loop]) + 1
	w.runEvents[loop] = append(w.runEvents[loop], map[string]any{
		"event_id": fmt.Sprintf("%s-%d", loop, seq), "seq": seq, "stream_seq": 4653557 + seq,
		"ts": fmt.Sprintf("2026-10-27T01:30:%02dZ", seq), "loop_id": loop, "agent_id": agent,
		"trajectory_id": tailTraj, "kind": kind, "payload": payload,
	})
}

func (w *tailWorld) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.mu.Lock()
	w.requests = append(w.requests, r.URL.RequestURI())
	if w.authFail {
		w.mu.Unlock()
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	if w.failAll > 0 {
		w.failAll--
		w.mu.Unlock()
		rw.WriteHeader(http.StatusServiceUnavailable)
		_, _ = rw.Write([]byte(`{"error":"computer restarting"}`))
		return
	}
	defer w.mu.Unlock()
	q := r.URL.Query()
	after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
	switch {
	case r.URL.Path == "/api/trajectories/"+tailTraj:
		writeStubJSON(rw, map[string]any{"schema": "choir.durable_work.v1",
			"trajectory": map[string]any{"trajectory_id": tailTraj, "status": w.status, "reducer_seq": len(w.lifecycle)},
			"watermark":  len(w.lifecycle)})
	case r.URL.Path == "/api/trajectories/"+tailTraj+"/events":
		page := []map[string]any{}
		for _, ev := range w.lifecycle {
			if int64(ev["reducer_seq"].(int)) > after {
				page = append(page, ev)
			}
		}
		writeStubJSON(rw, map[string]any{"schema": "choir.durable_work.v1", "events": page,
			"next_cursor": len(w.lifecycle), "watermark": len(w.lifecycle)})
	case r.URL.Path == "/api/runs":
		writeStubJSON(rw, map[string]any{"runs": w.runs})
	case strings.HasPrefix(r.URL.Path, "/api/runs/") && strings.HasSuffix(r.URL.Path, "/events"):
		loop := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/events")
		page := []map[string]any{}
		for _, ev := range w.runEvents[loop] {
			if int64(ev["seq"].(int)) > after {
				page = append(page, ev)
			}
		}
		writeStubJSON(rw, map[string]any{"events": page, "next_after": 0})
	case strings.HasSuffix(r.URL.Path, "/self-development/operations"):
		out := []map[string]any{}
		for _, op := range w.ops {
			if s := q.Get("state"); s == "" || s == op["state"] {
				out = append(out, op)
			}
		}
		writeStubJSON(rw, map[string]any{"operations": out, "route_generation": 12})
	default:
		http.NotFound(rw, r)
	}
}

// tailRun runs the tailer against world with a fake clock. steps[i] runs at
// the (i+1)th sleep; after len(steps)+extra sleeps the tail is stopped (exit
// tailExitStopped), so a test can never hang.
func tailRun(t *testing.T, world *tailWorld, args []string, steps []func(), extra int) (int, string, string, []time.Duration) {
	t.Helper()
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	srv := httptest.NewServer(world)
	t.Cleanup(srv.Close)
	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 10, 27, 1, 40, 0, 0, time.UTC)
	var sleeps []time.Duration
	hooks := &tailHooks{
		now: func() time.Time { return now },
		sleep: func(d time.Duration) bool {
			sleeps = append(sleeps, d)
			now = now.Add(d)
			if n := len(sleeps); n <= len(steps) {
				steps[n-1]()
			}
			return len(sleeps) < len(steps)+extra
		},
	}
	full := append([]string{"--host", srv.URL, "--computer", "computer-6c50d6e566143ae32cdb660faa60e24d"}, args...)
	code := runTraceTail(full, &stdout, &stderr, hooks)
	return code, stdout.String(), stderr.String(), sleeps
}

func realisticWorld() *tailWorld {
	w := newTailWorld()
	w.addLifecycle("trajectory_started", map[string]any{"work_item_id": "f4ba2f42"})
	w.addLifecycle("co_super_assignment_opened", map[string]any{"schema": "choir.co_super_assignment/v1", "agent_id": "engineering:assignment-e288fc8d1dc6f3878327c90d"})
	w.addLifecycle("co_super_assignment_bound", map[string]any{"run_id": "run:assignment-e288fc8d1dc6f3878327c90d", "agent_id": "engineering:assignment-e288fc8d1dc6f3878327c90d"})
	w.addLifecycle("texture_turn_committed", map[string]any{"reason": "The owner's direction defines the implementation scope and the required proof."})
	w.addLifecycle("co_super_capsule_disposition_set", map[string]any{"reason": "freeze_requested", "agent_id": "engineering:assignment-e288fc8d1dc6f3878327c90d"})
	w.addRun("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "running")
	w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "loop.started", map[string]any{})
	w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "tool.result", map[string]any{
		"tool": "capsule_go_eval", "call_id": "call_1", "duration_ms": 124, "is_error": false,
		"arguments": map[string]any{"source": "import \"fmt\"\n\n// build it\nid, err := choir.Start([]string{\"sh\", \"-c\", \"cd /workspace && npm run build\"}, map[string]any{\"minutes\": 2})\nfmt.Println(id, err)\n"},
		"output":    `{"stdout":"job-7 <nil>\nbuilt in 4s","stderr":"warn: chunk size","error":"","exit_code":0}`,
	})
	w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "tool.result", map[string]any{
		"tool": "capsule_go_eval", "call_id": "call_2", "duration_ms": 9, "is_error": true,
		"arguments": map[string]any{"source": "x := 1"}, "output": `{"stdout":"","stderr":"boom","error":"compile","exit_code":1}`,
	})
	w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "loop.progress", map[string]any{"iteration": 1, "llm_model": "gpt-6-luna", "tool_calls": 1, "stop_reason": "tool_use"})
	w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "loop.cancelled", map[string]any{"fate": "interrupted_by_restart"})
	return w
}

func TestTraceTailLevels(t *testing.T) {
	// Level 1: turns, assignments, capsule fates, run terminalizations; no tools.
	code, out, _, _ := tailRun(t, realisticWorld(), []string{"--history", "--until", "settled", tailTraj}, nil, 1)
	if code != tailExitStopped {
		t.Fatalf("exit=%d want %d (stopped); out:\n%s", code, tailExitStopped, out)
	}
	for _, want := range []string{
		"2026-10-27T01:02:00Z", "co_super_assignment_opened", "texture_turn_committed",
		"The owner's direction defines the implementation scope", "freeze_requested",
		"loop.cancelled", "interrupted_by_restart", "engineering:e288fc8d",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("-v output lacks %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"capsule_go_eval", "npm run build", "built in 4s", "loop.progress"} {
		if strings.Contains(out, bad) {
			t.Errorf("-v output must not show %q:\n%s", bad, out)
		}
	}

	// Level 2: tool calls with first meaningful code line, job argv, exit code.
	_, out2, _, _ := tailRun(t, realisticWorld(), []string{"--history", "-vv", "--until", "settled", tailTraj}, nil, 1)
	for _, want := range []string{"capsule_go_eval", "exit=0", "exit=1", "x := 1", `sh -c cd /workspace && npm run build`} {
		if !strings.Contains(out2, want) {
			t.Errorf("-vv output lacks %q:\n%s", want, out2)
		}
	}
	if strings.Contains(out2, "// build it") || strings.Contains(out2, "built in 4s") {
		t.Errorf("-vv must skip comments and hide outputs:\n%s", out2)
	}

	// Level 3: truncated output heads.
	_, out3, _, _ := tailRun(t, realisticWorld(), []string{"--history", "-vvv", "--until", "settled", tailTraj}, nil, 1)
	for _, want := range []string{"built in 4s", "warn: chunk size", "boom", "loop.progress"} {
		if !strings.Contains(out3, want) {
			t.Errorf("-vvv output lacks %q:\n%s", want, out3)
		}
	}
	// One line per event: no raw newline from output heads.
	for _, line := range strings.Split(strings.TrimSpace(out3), "\n") {
		if !strings.HasPrefix(line, "2026-") {
			t.Errorf("line does not start with a UTC time: %q", line)
		}
	}
}

func TestTraceTailFollowsByCursor(t *testing.T) {
	w := realisticWorld()
	steps := []func(){func() {
		w.addLifecycle("texture_turn_committed", map[string]any{"reason": "second turn"})
		w.addRunEvent("run:assignment-e288fc8d1dc6f3878327c90d", "engineering:assignment-e288fc8d1dc6f3878327c90d", "tool.result", map[string]any{
			"tool": "capsule_go_eval", "duration_ms": 3, "arguments": map[string]any{"source": "later := 2"}, "output": `{"exit_code":0}`})
	}}
	code, out, _, sleeps := tailRun(t, w, []string{"--history", "-vv", "--until", "settled", tailTraj}, steps, 2)
	if code != tailExitStopped {
		t.Fatalf("exit=%d", code)
	}
	if strings.Count(out, "The owner's direction") != 1 || strings.Count(out, "second turn") != 1 || strings.Count(out, "later := 2") != 1 {
		t.Errorf("events duplicated or missing:\n%s", out)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	sawLifecycleCursor, sawRunCursor := false, false
	for _, req := range w.requests {
		if strings.Contains(req, "/events?after=5") && strings.Contains(req, "/api/trajectories/") {
			sawLifecycleCursor = true
		}
		if strings.Contains(req, "/api/runs/run:assignment-e288fc8d1dc6f3878327c90d/events") && strings.Contains(req, "after=6") {
			sawRunCursor = true
		}
	}
	if !sawLifecycleCursor || !sawRunCursor {
		t.Errorf("cursors did not advance (lifecycle=%v run=%v): %v", sawLifecycleCursor, sawRunCursor, w.requests)
	}
	for _, d := range sleeps {
		if d != 5*time.Second {
			t.Errorf("default interval = %v, want 5s", d)
		}
	}
}

func TestTraceTailHistoryIsNotLive(t *testing.T) {
	// The restart cancellation is history at start: shown only with
	// --history, and never ends the tail.
	w := realisticWorld()
	w.addLifecycle("co_super_assignment_cancelled", map[string]any{"reason": "interrupted_by_restart", "agent_id": "engineering:assignment-dda4af8d88522bebf243c7cb"})
	steps := []func(){func() { w.addLifecycle("texture_turn_committed", map[string]any{"reason": "after start"}) }}
	code, out, _, _ := tailRun(t, w, []string{tailTraj}, steps, 1)
	if code != tailExitStopped {
		t.Fatalf("history ended the tail: exit=%d\n%s", code, out)
	}
	if strings.Contains(out, "interrupted_by_restart") || strings.Contains(out, "The owner's direction") {
		t.Errorf("history printed without --history:\n%s", out)
	}
	if !strings.Contains(out, "after start") {
		t.Errorf("live event missing:\n%s", out)
	}
}

func TestTraceTailTerminalStates(t *testing.T) {
	cancelled := func(w *tailWorld) func() {
		return func() {
			w.addLifecycle("co_super_assignment_cancelled", map[string]any{"reason": "interrupted_by_restart", "agent_id": "engineering:assignment-dda4af8d88522bebf243c7cb"})
		}
	}
	awaiting := func(w *tailWorld, traj string) func() {
		return func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.ops = append(w.ops, map[string]any{"operation_id": "selfdev-1", "trajectory_id": traj, "state": "awaiting_approval"})
		}
	}

	t.Run("candidate awaiting approval exits 0", func(t *testing.T) {
		w := realisticWorld()
		code, out, _, _ := tailRun(t, w, []string{tailTraj}, []func(){awaiting(w, tailTraj)}, 3)
		if code != 0 || !strings.Contains(out, "selfdev-1") || !strings.Contains(out, "awaiting_approval") {
			t.Fatalf("exit=%d\n%s", code, out)
		}
	})
	t.Run("another trajectory's candidate is ignored", func(t *testing.T) {
		w := realisticWorld()
		code, _, _, _ := tailRun(t, w, []string{tailTraj}, []func(){awaiting(w, "other-traj")}, 2)
		if code != tailExitStopped {
			t.Fatalf("exit=%d, an unrelated operation ended the tail", code)
		}
	})
	t.Run("restart cancellation", func(t *testing.T) {
		w := realisticWorld()
		code, out, _, _ := tailRun(t, w, []string{tailTraj}, []func(){cancelled(w)}, 3)
		if code != tailExitRestart || !strings.Contains(out, "interrupted_by_restart") {
			t.Fatalf("exit=%d want %d\n%s", code, tailExitRestart, out)
		}
	})
	t.Run("until restricts terminal states", func(t *testing.T) {
		w := realisticWorld()
		code, out, _, _ := tailRun(t, w, []string{"--until", "candidate", tailTraj}, []func(){cancelled(w), awaiting(w, tailTraj)}, 3)
		if code != 0 {
			t.Fatalf("exit=%d want 0 (restart ignored under --until candidate)\n%s", code, out)
		}
		if !strings.Contains(out, "ignored") {
			t.Errorf("an ignored terminal state must still be visible:\n%s", out)
		}
	})
	t.Run("texture turn waits on the owner", func(t *testing.T) {
		w := realisticWorld()
		step := func() {
			w.addLifecycle("texture_turn_committed", map[string]any{"reason": "Blocked: waiting on the owner to choose a recovery source."})
		}
		code, out, _, _ := tailRun(t, w, []string{tailTraj}, []func(){step}, 3)
		if code != tailExitOwner || !strings.Contains(out, "waiting on the owner") {
			t.Fatalf("exit=%d want %d\n%s", code, tailExitOwner, out)
		}
	})
	t.Run("an ordinary texture turn does not end the tail", func(t *testing.T) {
		w := realisticWorld()
		step := func() {
			w.addLifecycle("texture_turn_committed", map[string]any{"reason": "The owner's direction defines the scope."})
		}
		if code, out, _, _ := tailRun(t, w, []string{tailTraj}, []func(){step}, 2); code != tailExitStopped {
			t.Fatalf("exit=%d\n%s", code, out)
		}
	})
	t.Run("trajectory settled", func(t *testing.T) {
		w := realisticWorld()
		code, out, _, _ := tailRun(t, w, []string{tailTraj}, []func(){func() { w.mu.Lock(); w.status = "settled"; w.mu.Unlock() }}, 3)
		if code != tailExitSettled || !strings.Contains(out, "settled") {
			t.Fatalf("exit=%d want %d\n%s", code, tailExitSettled, out)
		}
	})
	codes := map[int]string{0: "candidate", tailExitRestart: "restart", tailExitRevoked: "revoked", tailExitOwner: "owner", tailExitSettled: "settled"}
	if len(codes) != 5 {
		t.Fatalf("terminal exit codes are not distinct: %v", codes)
	}
}

func TestTraceTailRevokedWithoutNewAssignment(t *testing.T) {
	revoke := func(w *tailWorld) func() {
		return func() {
			w.addLifecycle("co_super_capsule_disposition_set", map[string]any{"reason": "revoked", "agent_id": "engineering:assignment-e288fc8d1dc6f3878327c90d"})
		}
	}
	t.Run("grace elapses", func(t *testing.T) {
		w := realisticWorld()
		code, out, _, sleeps := tailRun(t, w, []string{"--grace", "30s", tailTraj}, []func(){revoke(w)}, 20)
		if code != tailExitRevoked {
			t.Fatalf("exit=%d want %d\n%s", code, tailExitRevoked, out)
		}
		if len(sleeps) < 6 || len(sleeps) > 9 {
			t.Errorf("fired after %d sleeps of 5s, want about 6-8 for a 30s grace", len(sleeps))
		}
	})
	t.Run("a new assignment inside the grace clears it", func(t *testing.T) {
		w := realisticWorld()
		reopen := func() {
			w.addLifecycle("co_super_assignment_opened", map[string]any{"agent_id": "engineering:assignment-new"})
		}
		code, out, _, _ := tailRun(t, w, []string{"--grace", "30s", tailTraj}, []func(){revoke(w), func() {}, reopen}, 20)
		if code != tailExitStopped {
			t.Fatalf("exit=%d, a reopened assignment must clear the revocation\n%s", code, out)
		}
	})
}

func TestTraceTailSurvivesTransientErrors(t *testing.T) {
	w := realisticWorld()
	steps := []func(){
		func() { w.mu.Lock(); w.failAll = 6; w.mu.Unlock() }, // the next poll fails
		func() {}, // still failing
		func() {}, // recovered
	}
	// After the first successful (initial) poll, make later polls fail then recover.
	code, out, errOut, _ := tailRun(t, w, []string{"--history", "--until", "settled", tailTraj}, steps, 6)
	if code != tailExitStopped {
		t.Fatalf("exit=%d; transient errors must not end the tail\nout:%s\nerr:%s", code, out, errOut)
	}
	if strings.Count(out, "read failed") < 2 || !strings.Contains(out, "503") {
		t.Errorf("want a visible one-line 'read failed' per failing poll with the status:\n%s", out)
	}
	if !strings.Contains(out, "read recovered") {
		t.Errorf("recovery is not reported:\n%s", out)
	}
}

func TestTraceTailFirstReadAuthFailureExits(t *testing.T) {
	w := realisticWorld()
	w.authFail = true
	code, _, errOut, _ := tailRun(t, w, []string{tailTraj}, nil, 3)
	if code != 1 || !strings.Contains(errOut, "401") {
		t.Fatalf("exit=%d stderr=%q, want exit 1 naming the 401", code, errOut)
	}
}

func TestTraceTailJSONLines(t *testing.T) {
	code, out, _, _ := tailRun(t, realisticWorld(), []string{"--json", "--history", "-vv", "--until", "settled", tailTraj}, nil, 1)
	if code != tailExitStopped {
		t.Fatalf("exit=%d", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 5 {
		t.Fatalf("too few lines:\n%s", out)
	}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("not JSON: %q: %v", line, err)
		}
		for _, k := range []string{"time", "kind", "summary"} {
			if _, ok := rec[k]; !ok {
				t.Errorf("record lacks %q: %s", k, line)
			}
		}
	}
}

func TestTraceTailFlagValidation(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	for name, args := range map[string][]string{
		"interval under the 1s floor": {"--interval", "500ms", tailTraj},
		"no trajectory":               {},
		"unknown until":               {"--until", "bogus", tailTraj},
		"two trajectories":            {tailTraj, "other"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"trace", "tail", "--host", "http://127.0.0.1:1"}, args...), &stdout, &stderr); code != 2 {
			t.Errorf("%s: exit=%d want 2 (stderr %q)", name, code, stderr.String())
		}
	}
}

func TestTraceTailHelpDocumentsExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"trace", "tail", "--help"}, &stdout, &stderr)
	text := stdout.String() + stderr.String()
	if code != 0 {
		t.Fatalf("--help exit=%d", code)
	}
	for _, want := range []string{"Exit codes", "awaiting_approval", "interrupted_by_restart", "revoked", "settled", "--until", "--grace", "--interval",
		strconv.Itoa(tailExitRestart), strconv.Itoa(tailExitRevoked), strconv.Itoa(tailExitOwner), strconv.Itoa(tailExitSettled)} {
		if !strings.Contains(text, want) {
			t.Errorf("help lacks %q:\n%s", want, text)
		}
	}
}

func TestTraceTailHelpers(t *testing.T) {
	cases := map[string]string{
		"import \"fmt\"\n\n// c\nx := 1\ny := 2":           "x := 1",
		"import (\n\t\"fmt\"\n\t\"os\"\n)\nfmt.Println(1)": "fmt.Println(1)",
		"package main\n\n\nfunc f() {}":                    "func f() {}",
	}
	for src, want := range cases {
		if got := firstMeaningfulLine(src); got != want {
			t.Errorf("firstMeaningfulLine(%q) = %q, want %q", src, got, want)
		}
	}
	if got := startArgv(`id, err := choir.Start([]string{"sh", "-c", "go test ./..."}, nil)`); got != "sh -c go test ./..." {
		t.Errorf("startArgv = %q", got)
	}
	for in, want := range map[string]string{
		"engineering:assignment-e288fc8d1dc6f3878327c90d": "engineering:e288fc8d",
		"texture:747226b8-fcdd-52c9-8c7b-a7ad52823187":    "texture:747226b8",
		"management:5885aafc-eb85-4255-9818-d521020bdce2": "management:5885aafc",
	} {
		if got := agentLabel(in, ""); got != want {
			t.Errorf("agentLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if got := agentLabel("", "09b41c07-0fdb-4e89-bdcc-e6cd4c9743b5"); got != "loop:09b41c07" {
		t.Errorf("loop fallback = %q", got)
	}
	if got := levelFromArgs([]string{"-vvv", "x"}); got != 3 {
		t.Errorf("levelFromArgs = %d", got)
	}
}
