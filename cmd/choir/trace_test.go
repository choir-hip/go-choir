package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// `choir trace export` writes one bundle per
// docs/reports/reeng-trace-bundle-format-2026-10-12.md. Ways the layer-status
// logic can fail, written before the code:
//   - a route that fails (5xx, 403, 404) leaves its layer "empty" instead of
//     "unserved" with the HTTP status in the note (the checker then passes
//     on absence);
//   - a route that answers with nothing is marked "present", or one that
//     answers with items is marked "empty";
//   - a derived layer (cells, egress, deliveries, turn_commits) is "empty"
//     while its parent is "unserved";
//   - paging stops after the first page (lifecycle events or run events),
//     or never stops;
//   - runs named only by lifecycle events or the snapshot, or Management
//     control runs found only through ?trajectory_id=, are missing;
//   - a run id that does not resolve is silently dropped;
//   - run events are not ordered by (loop_id, seq);
//   - layers with no Cut 0 route (records, search_arrivals,
//     research_children) are anything but "unserved";
//   - the file is written world-readable, or the printed summary carries the
//     API key;
//   - the printed sha256 is not the digest of the written file;
//   - the captures layer (Cut 2 amendment A5) asks for no window, or misses
//     a capture a report cites inside its JSON-string packet, or is
//     anything but "unserved" when its route fails.

type traceStub struct {
	mu       sync.Mutex
	failures map[string]int // path prefix -> status
	requests []string
}

func (s *traceStub) fail(prefix string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failures == nil {
		s.failures = map[string]int{}
	}
	s.failures[prefix] = status
}

func writeStubJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *traceStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, r.URL.RequestURI())
	for prefix, status := range s.failures {
		if strings.HasPrefix(r.URL.Path, prefix) {
			s.mu.Unlock()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"stubbed failure"}`))
			return
		}
	}
	s.mu.Unlock()
	q := r.URL.Query()
	after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
	switch {
	case r.URL.Path == "/api/trajectories/traj-1":
		writeStubJSON(w, map[string]any{
			"schema":     "choir.durable_work.v1",
			"trajectory": map[string]any{"trajectory_id": "traj-1"},
			"work_items": []any{map[string]any{"work_item_id": "wi-1", "created_by_loop_id": "run-wi"}},
			"agents":     []any{map[string]any{"agent_id": "texture:doc-1"}},
			"co_super_assignments": []any{
				map[string]any{"assignment_id": "as-1", "run_id": "run-assign"},
			},
			"updates":       []any{map[string]any{"update_id": "up-1", "from_loop_id": "run-direct"}},
			"document":      map[string]any{"doc_id": "doc-1"},
			"head_revision": map[string]any{"revision_id": "rev-2", "doc_id": "doc-1"},
		})
	case r.URL.Path == "/api/trajectories/traj-1/events":
		// Three events served one per page regardless of limit, to prove
		// paging continues to the watermark.
		all := []map[string]any{
			{"event_id": "le-1", "kind": "work_item_created", "reducer_seq": 1, "run_id": "run-direct"},
			{"event_id": "le-2", "kind": "texture_turn_committed", "reducer_seq": 2, "run_id": "run-event-only"},
			{"event_id": "le-3", "kind": "refs_recorded", "reducer_seq": 3, "reason": "platform publication accepted", "artifact_refs": []string{"/wire/story-1"}},
		}
		page := []map[string]any{}
		next := after
		if int(after) < len(all) {
			page = append(page, all[after])
			next = after + 1
		}
		writeStubJSON(w, map[string]any{"schema": "choir.durable_work.v1", "events": page, "next_cursor": next, "watermark": 3})
	case r.URL.Path == "/api/runs":
		if q.Get("trajectory_id") != "traj-1" {
			http.Error(w, "missing trajectory filter", http.StatusBadRequest)
			return
		}
		writeStubJSON(w, map[string]any{"runs": []any{
			map[string]any{"run_id": "run-direct"},
			map[string]any{"run_id": "run-control", "metadata": map[string]any{"assignment_trajectory_id": "traj-1"}},
		}})
	case strings.HasPrefix(r.URL.Path, "/api/runs/") && strings.HasSuffix(r.URL.Path, "/events"):
		runID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/events")
		var events []map[string]any
		if runID == "run-direct" {
			events = []map[string]any{
				{"event_id": "e1", "seq": 1, "loop_id": runID, "kind": "tool.invoked", "payload": map[string]any{"tool": "desk_go_eval"}},
				{"event_id": "e2", "seq": 2, "loop_id": runID, "kind": "desk.cell", "payload": map[string]any{"desk_session": "agent:a", "worker_pid": 7, "run_id": runID, "duration_ms": 5,
					// Journal cut R4: a cell's per-call egress rows ride its event.
					"egress": []map[string]any{
						{"request_id": "r2", "response_request_id": "r2", "verb": "read", "desk_session": "agent:a", "worker_pid": 7, "run_id": runID, "is_error": false},
						{"request_id": "r3", "response_request_id": "r3", "verb": "search_start", "desk_session": "agent:a", "worker_pid": 7, "run_id": runID, "is_error": true, "error": "boom"},
					}}},
				{"event_id": "e3", "seq": 3, "loop_id": runID, "kind": "desk.egress", "payload": map[string]any{"request_id": "r1", "response_request_id": "r1", "verb": "search_start", "run_id": runID}},
			}
		}
		// One event per page.
		page := []map[string]any{}
		next := int64(0)
		for _, ev := range events {
			if int64(ev["seq"].(int)) > after {
				page = append(page, ev)
				if int64(ev["seq"].(int)) < int64(len(events)) {
					next = int64(ev["seq"].(int))
				}
				break
			}
		}
		writeStubJSON(w, map[string]any{"events": page, "next_after": next})
	case strings.HasPrefix(r.URL.Path, "/api/runs/"):
		runID := strings.TrimPrefix(r.URL.Path, "/api/runs/")
		switch runID {
		case "run-direct":
			writeStubJSON(w, map[string]any{"loop_id": runID, "trajectory_id": "traj-1", "created_at": "2026-10-13T09:00:00Z", "metadata": map[string]any{"doc_id": "doc-1"}})
		case "run-control", "run-event-only", "run-assign", "run-wi":
			writeStubJSON(w, map[string]any{"loop_id": runID, "trajectory_id": "traj-1", "created_at": "2026-10-13T09:05:00Z", "metadata": map[string]any{"doc_id": "doc-1"}})
		default:
			http.Error(w, `{"error":"run not found"}`, http.StatusNotFound)
		}
	case r.URL.Path == "/api/texture/documents/doc-1/revisions":
		writeStubJSON(w, map[string]any{"revisions": []any{
			map[string]any{"revision_id": "rev-1", "doc_id": "doc-1"},
			map[string]any{"revision_id": "rev-2", "doc_id": "doc-1", "metadata": map[string]any{"corpusd_route_path": "/wire/story-1"}},
		}})
	case strings.HasPrefix(r.URL.Path, "/api/texture/documents/"):
		http.Error(w, `{"error":"document not found"}`, http.StatusNotFound)
	case r.URL.Path == "/api/platform/publications/resolve":
		if q.Get("route") != "/wire/story-1" {
			http.NotFound(w, r)
			return
		}
		writeStubJSON(w, map[string]any{"route": map[string]any{"route_path": "/wire/story-1"}})
	case r.URL.Path == "/api/inboxes/texture:doc-1/records":
		writeStubJSON(w, map[string]any{"inbox": "texture:doc-1", "head": 1, "cursor": 1, "open_rounds": []any{},
			"records": []any{map[string]any{"record_id": "r-report", "inbox": "texture:doc-1", "inbox_seq": 1, "final": true,
				"prediction": map[string]any{"hypothesis": `{"sources":[{"source_id":"s1","target":{"uri":"https://example.com/a","capture_id":"cap-cited"}}]}`}}}, "next_after": 0})
	case r.URL.Path == "/api/inboxes/research:doc-1/records":
		writeStubJSON(w, map[string]any{"inbox": "research:doc-1", "head": 1, "cursor": 1, "open_rounds": []any{},
			"records": []any{map[string]any{"record_id": "r-cast", "inbox": "research:doc-1", "inbox_seq": 1}}, "next_after": 0})
	case strings.HasPrefix(r.URL.Path, "/api/inboxes/"):
		writeStubJSON(w, map[string]any{"head": 0, "cursor": 0, "records": []any{}, "open_rounds": []any{}, "next_after": 0})
	case r.URL.Path == "/api/captures":
		if q.Get("since") == "" || q.Get("until") == "" || len(q["id"]) != 1 || q["id"][0] != "cap-cited" {
			http.Error(w, "window or cited id missing: "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}
		writeStubJSON(w, map[string]any{"computer_id": "computer-1", "window_complete": true, "captures": []any{
			map[string]any{"capture_id": "cap-window", "in_window": true, "raw_present": true, "blocks": []any{map[string]any{"id": "c1", "text": "x", "normtext": "x"}}},
			map[string]any{"capture_id": "cap-cited", "in_window": false, "raw_present": true, "blocks": []any{}},
		}})
	case r.URL.Path == "/api/runtime/obligations":
		writeStubJSON(w, map[string]any{"computer_id": "computer-1", "restart": map[string]any{"kind": "crash", "reason": "no marker"}})
	case r.URL.Path == "/api/runtime/census":
		writeStubJSON(w, map[string]any{"computer_id": "computer-1", "kinds": []string{"k1", "k2"}, "counts": map[string]int{"k1": 0, "k2": 0}, "objects": []any{}})
	case r.URL.Path == "/api/runtime/observability":
		writeStubJSON(w, map[string]any{"computer_id": "computer-1", "build": map[string]any{"commit": "abc", "deployed_commit": "abc", "built_at": "t"}, "recent_log": []string{"boot"}})
	default:
		http.NotFound(w, r)
	}
}

func runTraceExportForTest(t *testing.T, stub *traceStub) (traceBundle, string, string) {
	t.Helper()
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)
	out := filepath.Join(t.TempDir(), "bundle.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"trace", "export", "traj-1", "--host", srv.URL, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("trace export exit=%d stderr=%s", code, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat bundle: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode = %v, want 0600", info.Mode().Perm())
	}
	sum := sha256.Sum256(raw)
	if !strings.Contains(stdout.String(), hex.EncodeToString(sum[:])) {
		t.Fatalf("stdout does not carry the bundle sha256: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "choir_sk_") || strings.Contains(string(raw), "choir_sk_") {
		t.Fatalf("output carries an API key")
	}
	var bundle traceBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	return bundle, stdout.String(), stderr.String()
}

func layerStatus(t *testing.T, b traceBundle, name string) *traceLayer {
	t.Helper()
	layer, ok := b.Layers[name]
	if !ok || layer == nil {
		t.Fatalf("bundle has no layer %q", name)
	}
	return layer
}

func itemIDs(t *testing.T, layer *traceLayer, key string) []string {
	t.Helper()
	ids := []string{}
	for _, item := range layer.Items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
			t.Fatalf("decode item: %v", err)
		}
		ids = append(ids, fmt.Sprint(m[key]))
	}
	return ids
}

func TestTraceExportHealthyBundleLayers(t *testing.T) {
	bundle, _, _ := runTraceExportForTest(t, &traceStub{})
	if bundle.Schema != "choir.trace_bundle.v1" || bundle.TrajectoryID != "traj-1" || bundle.ComputerID != "computer-1" || bundle.Build.Commit != "abc" {
		t.Fatalf("bundle header = %+v", bundle)
	}
	want := map[string]string{
		"trajectory":        "present",
		"lifecycle_events":  "present",
		"runs":              "present",
		"run_events":        "present",
		"cells":             "present",
		"egress":            "present",
		"deliveries":        "present",
		"records":           "present",
		"inboxes":           "present",
		"turn_commits":      "present",
		"revisions":         "present",
		"publications":      "present",
		"search_arrivals":   "unserved",
		"research_children": "unserved",
		"captures":          "present",
		"obligations":       "present",
		"census":            "present",
		"boot":              "present",
	}
	for name, status := range want {
		layer := layerStatus(t, bundle, name)
		if layer.Status != status {
			t.Errorf("layer %s status = %q (note %q), want %q", name, layer.Status, layer.Note, status)
		}
		if len(layer.Sources) == 0 && status != "unserved" {
			t.Errorf("layer %s names no source route", name)
		}
	}
	if len(bundle.Layers) != len(want) {
		t.Errorf("bundle has %d layers, want %d", len(bundle.Layers), len(want))
	}
	if got := len(layerStatus(t, bundle, "lifecycle_events").Items); got != 3 {
		t.Fatalf("lifecycle events paged = %d, want 3", got)
	}
	runs := strings.Join(itemIDs(t, layerStatus(t, bundle, "runs"), "loop_id"), ",")
	for _, id := range []string{"run-direct", "run-control", "run-event-only", "run-assign", "run-wi"} {
		if !strings.Contains(runs, id) {
			t.Errorf("runs layer %s misses %s", runs, id)
		}
	}
	events := layerStatus(t, bundle, "run_events")
	if ids := strings.Join(itemIDs(t, events, "event_id"), ","); ids != "e1,e2,e3" {
		t.Fatalf("run events = %s, want e1,e2,e3 in seq order across pages", ids)
	}
	if got := itemIDs(t, layerStatus(t, bundle, "cells"), "worker_pid"); len(got) != 1 || got[0] != "7" {
		t.Fatalf("cells = %v", got)
	}
	// The egress layer is "one item per egress call": the rows folded into a
	// desk.cell event (journal cut R4), at the cell's place in the run's
	// order, and the standalone desk.egress events older tapes, refusals and
	// search_done rows still use.
	egress := layerStatus(t, bundle, "egress")
	if got := strings.Join(itemIDs(t, egress, "request_id"), ","); got != "r2,r3,r1" {
		t.Fatalf("egress = %s, want the cell's folded rows r2,r3 then the standalone r1", got)
	}
	if got := strings.Join(itemIDs(t, egress, "cell_event_id"), ","); got != "e2,e2,<nil>" {
		t.Fatalf("egress cell_event_id = %s, want a folded row to name its cell event", got)
	}
	if got := strings.Join(itemIDs(t, egress, "event_id"), ","); got != "e2#egress.0,e2#egress.1,e3" {
		t.Fatalf("egress event_id = %s", got)
	}
	for i, item := range egress.Items {
		var m map[string]any
		_ = json.Unmarshal(item, &m)
		if m["loop_id"] != "run-direct" || m["seq"] == nil || m["ts"] == nil {
			t.Fatalf("egress item %d lacks the event identity the layer adds: %v", i, m)
		}
	}
	if got := len(layerStatus(t, bundle, "deliveries").Items); got != 2 {
		t.Fatalf("deliveries = %d items, want the update and the work item", got)
	}
	if got := itemIDs(t, layerStatus(t, bundle, "turn_commits"), "event_id"); len(got) != 1 || got[0] != "le-2" {
		t.Fatalf("turn commits = %v", got)
	}
	if got := len(layerStatus(t, bundle, "revisions").Items); got != 2 {
		t.Fatalf("revisions = %d, want 2", got)
	}
	if got := len(layerStatus(t, bundle, "publications").Items); got != 1 {
		t.Fatalf("publications = %d, want 1", got)
	}
	if got := strings.Join(itemIDs(t, layerStatus(t, bundle, "captures"), "capture_id"), ","); got != "cap-window,cap-cited" {
		t.Fatalf("captures = %s", got)
	}
}

func TestTraceExportFailingRoutesAreUnservedNeverEmpty(t *testing.T) {
	stub := &traceStub{}
	stub.fail("/api/trajectories/traj-1/events", http.StatusInternalServerError)
	stub.fail("/api/runtime/obligations", http.StatusForbidden)
	stub.fail("/api/runtime/census", http.StatusNotFound)
	stub.fail("/api/texture/", http.StatusForbidden)
	stub.fail("/api/captures", http.StatusServiceUnavailable)
	bundle, _, _ := runTraceExportForTest(t, stub)

	for name, status := range map[string]string{
		"lifecycle_events": "500",
		"turn_commits":     "",
		"obligations":      "403",
		"census":           "404",
		"revisions":        "403",
		"captures":         "503",
	} {
		layer := layerStatus(t, bundle, name)
		if layer.Status != "unserved" {
			t.Errorf("layer %s status = %q, want unserved", name, layer.Status)
		}
		if status != "" && !strings.Contains(layer.Note, status) {
			t.Errorf("layer %s note %q does not name HTTP %s", name, layer.Note, status)
		}
	}
	// The snapshot still answered, so trajectory and deliveries stand.
	if got := layerStatus(t, bundle, "trajectory").Status; got != "present" {
		t.Fatalf("trajectory status = %q", got)
	}
}

func TestTraceExportDerivedLayersFollowUnservedParent(t *testing.T) {
	stub := &traceStub{}
	stub.fail("/api/trajectories/traj-1", http.StatusNotFound)
	stub.fail("/api/runs", http.StatusServiceUnavailable)
	bundle, _, _ := runTraceExportForTest(t, stub)
	for _, name := range []string{"trajectory", "deliveries", "runs", "run_events", "cells", "egress", "lifecycle_events", "turn_commits"} {
		if got := layerStatus(t, bundle, name).Status; got != "unserved" {
			t.Errorf("layer %s status = %q, want unserved", name, got)
		}
	}
}

func TestTraceExportEmptyRunEventsAreEmptyNotUnserved(t *testing.T) {
	stub := &traceStub{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/runs/") && strings.HasSuffix(r.URL.Path, "/events") {
			writeStubJSON(w, map[string]any{"events": []any{}, "next_after": 0})
			return
		}
		stub.ServeHTTP(w, r)
	}))
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "b.json")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"trace", "export", "--host", srv.URL, "--out", out, "traj-1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var bundle traceBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, name := range []string{"run_events", "cells", "egress"} {
		if got := layerStatus(t, bundle, name).Status; got != "empty" {
			t.Errorf("layer %s status = %q, want empty", name, got)
		}
	}
}
