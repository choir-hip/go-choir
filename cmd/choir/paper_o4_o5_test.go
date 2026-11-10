package main

// `choir paper schedule` and `choir paper restart` (O4 and O5,
// docs/design/platform-operator-account-2026-11-06.md section 9, L3 and L4).
// Ways they could fail:
//   - the verbs leave the operator route, or send X-Choir-Computer;
//   - a feed URL can be sent with a schedule start;
//   - restart goes without a reason, or a refusal (live capsule, held) exits 0
//     so a refused restart reads as done;
//   - a schedule or restart verb appears as an MCP tool.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type paperO45Fake struct {
	mu       sync.Mutex
	requests []string
	bodies   []string
	computer []string
	restart  func(w http.ResponseWriter)
}

func newPaperO45Fake(t *testing.T) *paperO45Fake {
	t.Helper()
	f := &paperO45Fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
		f.bodies = append(f.bodies, string(raw))
		f.computer = append(f.computer, r.Header.Get("X-Choir-Computer"))
		restart := f.restart
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == paperComputersRoute:
			_ = json.NewEncoder(w).Encode(map[string]any{"computers": []map[string]string{{"computer_id": paperID1}}})
		case strings.HasSuffix(r.URL.Path, "/restart"):
			if restart != nil {
				restart(w)
				return
			}
			_, _ = w.Write([]byte(`{"outcome":"refreshed","receipt":"evt-r1","target":"abc"}`))
		case strings.HasSuffix(r.URL.Path, "/api/autopaper/sources/schedules"):
			_, _ = w.Write([]byte(`{"schedules":[],"receipt":"evt-s1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	return f
}

func TestPaperScheduleAndRestartVerbs(t *testing.T) {
	f := newPaperO45Fake(t)
	prefix := paperComputersRoute + "/" + paperID1

	if code, out, errOut := runPaper("schedule", "list"); code != 0 || !strings.Contains(out, "schedules") {
		t.Fatalf("list: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := runPaper("schedule", "start", "--source", "rss:example", "--interval-minutes", "30", "--reason", "slow it"); code != 0 || !strings.Contains(out, "evt-s1") {
		t.Fatalf("start: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := runPaper("schedule", "stop", "--source", "rss:example", "--reason", "paused"); code != 0 {
		t.Fatalf("stop: %d %s", code, errOut)
	}
	if code, out, errOut := runPaper("restart", "--reason", "pick up the policy"); code != 0 || !strings.Contains(out, "evt-r1") || !strings.Contains(out, "refreshed") {
		t.Fatalf("restart: %d %s %s", code, out, errOut)
	}
	want := []string{
		"GET " + paperComputersRoute,
		"GET " + prefix + "/guest/api/autopaper/sources/schedules",
		"GET " + paperComputersRoute,
		"POST " + prefix + "/guest/api/autopaper/sources/schedules",
		"GET " + paperComputersRoute,
		"DELETE " + prefix + "/guest/api/autopaper/sources/schedules?reason=paused&source_id=rss%3Aexample",
		"GET " + paperComputersRoute,
		"POST " + prefix + "/restart",
	}
	if strings.Join(f.requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(f.requests, "\n"), strings.Join(want, "\n"))
	}
	for _, c := range f.computer {
		if c != "" {
			t.Fatalf("X-Choir-Computer was sent: %v", f.computer)
		}
	}
	var start map[string]any
	if err := json.Unmarshal([]byte(f.bodies[3]), &start); err != nil || start["source_id"] != "rss:example" || start["interval_minutes"] != float64(30) || start["reason"] != "slow it" || len(start) != 3 {
		t.Fatalf("start body = %s", f.bodies[3])
	}
	var restart map[string]any
	if err := json.Unmarshal([]byte(f.bodies[7]), &restart); err != nil || restart["reason"] != "pick up the policy" || len(restart) != 1 {
		t.Fatalf("restart body = %s", f.bodies[7])
	}
}

// L3: no reason, no restart; no source, no schedule act; and no feed flag.
func TestPaperRestartNeedsAReasonAndScheduleTakesNoFeed(t *testing.T) {
	f := newPaperO45Fake(t)
	if code, _, _ := runPaper("restart"); code == 0 {
		t.Fatal("restart without a reason succeeded")
	}
	if code, _, _ := runPaper("schedule", "start"); code == 0 {
		t.Fatal("schedule start without a source succeeded")
	}
	if code, _, _ := runPaper("schedule", "start", "--source", "rss:a", "--feed-url", "https://x.example/f.xml"); code == 0 {
		t.Fatal("schedule start accepted a feed URL flag")
	}
	if code, _, _ := runPaper("schedule", "stop"); code == 0 {
		t.Fatal("schedule stop without a source succeeded")
	}
	for _, r := range f.requests {
		if strings.HasPrefix(r, "POST") || strings.HasPrefix(r, "DELETE") {
			t.Fatalf("a refused verb sent %q", r)
		}
	}
}

// L4: a refusal is an error with the server's reason code, and the exit is non-zero.
func TestPaperRestartRefusalExitsNonZeroWithItsCode(t *testing.T) {
	f := newPaperO45Fake(t)
	f.restart = func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"outcome":"deferred","reason_code":"live_capsule","receipt":"evt-r2"}`))
	}
	code, out, errOut := runPaper("restart", "--reason", "r")
	if code == 0 {
		t.Fatalf("a refused restart exited 0: %s", out)
	}
	if !strings.Contains(errOut+out, "live_capsule") {
		t.Fatalf("the reason code is not shown: out %q err %q", out, errOut)
	}
}

func TestPaperScheduleAndRestartAreNotMCPTools(t *testing.T) {
	for _, cmd := range mcpCommands() {
		if cmd.path[0] == "paper" {
			t.Fatalf("%s is exposed as an MCP tool", cmd.name())
		}
	}
}
