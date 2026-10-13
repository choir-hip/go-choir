package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/runs/{id}/events is the owner-scoped read of the primary run-event
// reader (Store.ListEventsAfter) that the Cut 0 trace export pages through
// (docs/reports/reeng-trace-bundle-format-2026-10-12.md, layer run_events).
//
// Ways it can fail, written before the code:
//   - it leaks another owner's run: a foreign run must answer 404 with the
//     same body as a run that does not exist, so existence is not revealed;
//   - an unknown run answers 200 with an empty list, which a bundle would
//     record as "empty" instead of a failed read;
//   - events come back out of per-run seq order, or include another run's
//     events;
//   - paging skips or repeats an event at a page boundary, reports
//     next_after=0 while events remain (silent truncation), or keeps
//     reporting a cursor after the last page (an exporter loops forever);
//   - limit is not clamped, so limit=100000 returns an unbounded page, or a
//     zero/negative/garbage limit is honoured instead of falling back;
//   - a malformed or negative after is silently read as 0 and returns the
//     first page again;
//   - a non-GET method is accepted on the events path;
//   - a caller without a host-sourced identity reads events.

func appendTestRunEvents(t *testing.T, rt *Runtime, run *types.RunRecord, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		payload, _ := json.Marshal(map[string]any{"i": i})
		ev := &types.EventRecord{
			EventID:   fmt.Sprintf("%s-ev-%03d", run.RunID, i),
			Timestamp: time.Now().UTC(),
			RunID:     run.RunID,
			OwnerID:   run.OwnerID,
			Kind:      types.EventKind("desk.cell"),
			Payload:   payload,
		}
		if err := rt.store.AppendEvent(context.Background(), ev); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}
}

type runEventsPage struct {
	Events    []types.EventRecord `json:"events"`
	NextAfter int64               `json:"next_after"`
}

func getRunEventsPage(t *testing.T, handler *APIHandler, path, user string) (int, runEventsPage, string) {
	t.Helper()
	w := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodGet, path, "", user)
	var page runEventsPage
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode %s: %v; body=%s", path, err, w.Body.String())
		}
	}
	return w.Code, page, w.Body.String()
}

func TestRunEventsRouteOwnerScopedPagedInSeqOrder(t *testing.T) {
	rt, handler := testAPISetup(t)
	ctx := context.Background()
	alice, err := rt.createRunWithMetadata(ctx, "alice run", "user-alice", nil)
	if err != nil {
		t.Fatalf("create alice run: %v", err)
	}
	bob, err := rt.createRunWithMetadata(ctx, "bob run", "user-bob", nil)
	if err != nil {
		t.Fatalf("create bob run: %v", err)
	}
	appendTestRunEvents(t, rt, alice, 5)
	appendTestRunEvents(t, rt, bob, 2)

	// Page through alice's run two at a time; every page is in seq order,
	// pages are contiguous, and the last page reports next_after=0.
	var all []types.EventRecord
	after := int64(0)
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatalf("paging did not terminate; after=%d", after)
		}
		code, page, body := getRunEventsPage(t, handler, fmt.Sprintf("/api/runs/%s/events?after=%d&limit=2", alice.RunID, after), "user-alice")
		if code != http.StatusOK {
			t.Fatalf("page after=%d status=%d body=%s", after, code, body)
		}
		if len(page.Events) > 2 {
			t.Fatalf("page after=%d returned %d events, limit 2", after, len(page.Events))
		}
		all = append(all, page.Events...)
		if page.NextAfter == 0 {
			break
		}
		if len(page.Events) == 0 || page.NextAfter != page.Events[len(page.Events)-1].Seq {
			t.Fatalf("next_after=%d does not name the last event of the page %+v", page.NextAfter, page.Events)
		}
		after = page.NextAfter
	}
	// Alice's run carries the submission event(s) plus the five appended.
	if len(all) < 5 {
		t.Fatalf("paged %d events, want at least the 5 appended", len(all))
	}
	for i, ev := range all {
		if ev.RunID != alice.RunID {
			t.Fatalf("event %d belongs to run %q, want %q", i, ev.RunID, alice.RunID)
		}
		if i > 0 && ev.Seq <= all[i-1].Seq {
			t.Fatalf("events out of seq order or repeated at %d: %d after %d", i, ev.Seq, all[i-1].Seq)
		}
	}
	unpaged, err := rt.Store().ListEventsAfter(ctx, alice.RunID, 0, 1000)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(unpaged) != len(all) {
		t.Fatalf("paged %d events, store holds %d", len(all), len(unpaged))
	}

	// A foreign run answers exactly like a missing run.
	foreignCode, _, foreignBody := getRunEventsPage(t, handler, "/api/runs/"+bob.RunID+"/events", "user-alice")
	missingCode, _, missingBody := getRunEventsPage(t, handler, "/api/runs/run-does-not-exist/events", "user-alice")
	if foreignCode != http.StatusNotFound || missingCode != http.StatusNotFound {
		t.Fatalf("foreign=%d missing=%d, want 404 for both", foreignCode, missingCode)
	}
	if foreignBody != missingBody {
		t.Fatalf("foreign body %q differs from missing body %q", foreignBody, missingBody)
	}

	// Bad cursors are refused, not read as zero.
	for _, q := range []string{"after=-1", "after=abc"} {
		code, _, body := getRunEventsPage(t, handler, "/api/runs/"+alice.RunID+"/events?"+q, "user-alice")
		if code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s, want 400", q, code, body)
		}
	}

	// Non-GET is refused; no identity is refused.
	if w := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+alice.RunID+"/events", "", "user-alice"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST events status=%d, want 405", w.Code)
	}
	if w := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodGet, "/api/runs/"+alice.RunID+"/events", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous events status=%d, want 401", w.Code)
	}
}

func TestRunEventsRouteClampsLimit(t *testing.T) {
	if got := clampRunEventsLimit(""); got != defaultRunEventsLimit {
		t.Fatalf("empty limit -> %d, want default %d", got, defaultRunEventsLimit)
	}
	for raw, want := range map[string]int{
		"1":      1,
		"1000":   1000,
		"1001":   maxRunEventsLimit,
		"100000": maxRunEventsLimit,
		"0":      defaultRunEventsLimit,
		"-5":     defaultRunEventsLimit,
		"abc":    defaultRunEventsLimit,
	} {
		if got := clampRunEventsLimit(raw); got != want {
			t.Errorf("limit %q -> %d, want %d", raw, got, want)
		}
	}

	rt, handler := testAPISetup(t)
	run, err := rt.createRunWithMetadata(context.Background(), "clamp run", "user-alice", nil)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	appendTestRunEvents(t, rt, run, 3)
	code, page, body := getRunEventsPage(t, handler, "/api/runs/"+run.RunID+"/events?limit=100000", "user-alice")
	if code != http.StatusOK || page.NextAfter != 0 || len(page.Events) < 3 {
		t.Fatalf("clamped read status=%d next_after=%d events=%d body=%s", code, page.NextAfter, len(page.Events), body)
	}
}

// GET /api/runs?trajectory_id= returns the trajectory's runs, including
// Management lifecycle-control runs that carry an empty TrajectoryID and
// name the trajectory only in metadata.assignment_trajectory_id.
func TestRunListFiltersByTrajectoryIncludingAssignmentRuns(t *testing.T) {
	rt, handler := testAPISetup(t)
	ctx := context.Background()
	const trajectoryID = "traj-filter-1"
	now := time.Now().UTC()
	seed := func(runID, owner, trajectory string, metadata map[string]any) {
		t.Helper()
		if err := rt.store.CreateRun(ctx, types.RunRecord{
			RunID: runID, AgentID: "agent-" + runID, OwnerID: owner,
			TrajectoryID: trajectory, State: types.RunCompleted, Prompt: runID,
			CreatedAt: now, UpdatedAt: now, Metadata: metadata,
		}); err != nil {
			t.Fatalf("create run %s: %v", runID, err)
		}
	}
	seed("run-direct", "user-alice", trajectoryID, map[string]any{runMetadataTrajectoryID: trajectoryID})
	seed("run-control", "user-alice", "", map[string]any{"assignment_trajectory_id": trajectoryID})
	seed("run-other", "user-alice", "traj-other", map[string]any{runMetadataTrajectoryID: "traj-other"})
	seed("run-foreign", "user-bob", trajectoryID, map[string]any{runMetadataTrajectoryID: trajectoryID})

	w := runtimeHandlerRequest(t, handler.HandleRunList, http.MethodGet, "/api/runs?trajectory_id="+trajectoryID, "", "user-alice")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp runListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]bool{}
	for _, run := range resp.Runs {
		got[run.RunID] = true
	}
	if len(got) != 2 || !got["run-direct"] || !got["run-control"] {
		t.Fatalf("trajectory runs = %v, want exactly run-direct and run-control", got)
	}
}
