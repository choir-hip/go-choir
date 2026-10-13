package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Host-requested planned restart (owner rule "Restarts End Work (Crash) Or
// Resume It (Planned Update)"; docs/problems/owner-computer-pinned-as-canary-2026-10-13.md).
// Before a deploy refresh, the host asks the guest to write its own
// planned-restart marker; a busy guest refuses so the host leaves it for its
// next idle moment.
//
// Ways this could fail, written first:
//   - the endpoint writes a marker while runs are running or a
//     self-development operation is mid-transaction, so a deploy kills busy
//     work (the 2026-10-09 failure the deploy rule exists for);
//   - the endpoint answers 200 before the marker is on disk, so the host
//     kills the guest and the boot reads as a crash anyway;
//   - a guest (not the host peer) can write the marker and turn a later
//     crash of a neighbour into a resume;
//   - the marker omits the target or the work-state epoch, so D7 treats
//     every host-requested restart as a crash;
//   - the busy refusal is not distinguishable (409) from a failure, so the
//     host cannot tell "defer" from "broken";
//   - the marker is not consumed by the planned boot, so a later crash boot
//     resumes again (a resume loop);
//   - an operator refresh with no marker becomes a planned boot.
func TestPlannedRestartEndpointWritesDurableMarkerWhenIdle(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	handler := &APIHandler{rt: rt}

	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusOK {
		t.Fatalf("idle guest: status=%d body=%s, want 200", code, body)
	}
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil {
		t.Fatalf("200 returned before the marker was durable: %v", err)
	}
	var marker PlannedRestart
	if err := json.Unmarshal(raw, &marker); err != nil {
		t.Fatalf("marker unreadable: %v", err)
	}
	if marker.Reason != PlannedRestartPlatformRefresh || marker.Target != "761d94c5" || marker.WorkStateEpoch != CurrentWorkStateEpoch || marker.RequestedAt.IsZero() {
		t.Fatalf("marker = %+v, want reason %q target 761d94c5 epoch %d", marker, PlannedRestartPlatformRefresh, CurrentWorkStateEpoch)
	}

	// A host that decides not to restart after all clears the marker, so a
	// later crash is not read as planned.
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	handler.HandleInternalPlannedRestart(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("clear left the marker: %v", err)
	}
}

func TestPlannedRestartEndpointRefusesWhenBusy(t *testing.T) {
	rt, s := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	handler := &APIHandler{rt: rt}

	// Running run.
	rt.runningMu.Lock()
	rt.running["loop-busy"] = func() {}
	rt.runningMu.Unlock()
	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || !strings.Contains(body, `"running_runs":1`) {
		t.Fatalf("running guest: status=%d body=%s, want 409 naming running_runs", code, body)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("busy refusal wrote a marker: %v", err)
	}
	rt.runningMu.Lock()
	delete(rt.running, "loop-busy")
	rt.runningMu.Unlock()

	// Self-development operation mid-transaction.
	ctx := context.Background()
	rt.cfg.ComputerID = "computer-planned-restart-busy"
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	now := time.Now().UTC()
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		"selfdev-materializing", rt.cfg.ComputerID, "key-m", strings.Repeat("d", 64), "trajectory-m", strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateMaterializing, now, now); err != nil {
		t.Fatal(err)
	}
	code, body = postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || !strings.Contains(body, `"selfdev_active_operations":1`) {
		t.Fatalf("self-development in flight: status=%d body=%s, want 409 naming selfdev_active_operations", code, body)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("busy refusal wrote a marker: %v", err)
	}
}

func TestPlannedRestartEndpointRefusesNonHostCallerAndMissingTarget(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	handler := &APIHandler{rt: rt}

	if code, body := postPlannedRestart(t, handler, "203.0.113.9:4000", `{"target":"761d94c5"}`); code != http.StatusForbidden {
		t.Fatalf("non-host caller: status=%d body=%s, want 403", code, body)
	}
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{}`); code != http.StatusBadRequest {
		t.Fatalf("missing target: status=%d body=%s, want 400", code, body)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("a refused request wrote a marker: %v", err)
	}
}

// The boot after a host-requested marker is planned exactly once: it consumes
// the marker (same work-state epoch), parks pre-boot runs instead of closing
// them, and the next boot without a fresh marker (an operator refresh, a
// crash) closes them with the restart fate.
func TestPlannedRestartEndpointMarkerResumesOnceThenCrashBoot(t *testing.T) {
	rt, s := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	ownerID := "user-host-planned-restart"
	t.Setenv("CHOIR_OWNER_ID", ownerID)
	runs := seedPreBootRuns(t, rt, ownerID)

	if code, body := postPlannedRestart(t, &APIHandler{rt: rt}, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("mark: status=%d body=%s", code, body)
	}

	// Boot 1: the planned boot (as internal/autoputer/run.go does it).
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now().UTC())
	if !planned || marker.Reason != PlannedRestartPlatformRefresh || marker.Target != "761d94c5" {
		t.Fatalf("boot after host marker: planned=%v marker=%+v, want planned platform update to 761d94c5", planned, marker)
	}
	boot1 := testPeerRuntime(t, rt, s)
	WithBootRestart(marker, planned)(boot1)
	boot1.Start(context.Background())
	for _, want := range runs {
		got, err := s.GetRun(context.Background(), want.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State == types.RunCancelled && got.Error == runFateInterruptedByRestart {
			t.Fatalf("planned boot closed pre-boot %s run %s; the one resume was lost", want.State, want.RunID)
		}
	}
	boot1.Stop()

	// Boot 2: no marker left (an operator refresh or a crash).
	marker2, planned2 := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now().UTC())
	if planned2 {
		t.Fatalf("marker consumed twice (%+v): a crashing resume would loop", marker2)
	}
	boot2 := testPeerRuntime(t, rt, s)
	WithBootRestart(marker2, planned2)(boot2)
	boot2.Start(context.Background())
	closed := 0
	for _, want := range runs {
		got, err := s.GetRun(context.Background(), want.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State == types.RunCancelled && got.Error == runFateInterruptedByRestart {
			closed++
		}
	}
	if closed == 0 {
		t.Fatal("markerless boot after the planned one closed no pre-boot run; it was not a crash boot")
	}
}

func postPlannedRestart(t *testing.T, handler *APIHandler, remoteAddr, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/planned-restart", bytes.NewBufferString(body))
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	handler.HandleInternalPlannedRestart(rec, req)
	return rec.Code, rec.Body.String()
}
