package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The stream_seq lease at the in-process restore swap point
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 3
// and test 4). Ways it can fail, written before the code:
//  1. RematerializeFromTape flips a reconstructed workspace into the live
//     path with no marker: the next boot trusts the reconstructed lease while
//     the numbers the live process published after the target exist only on
//     the tape, and a number is handed out twice.
//  2. The marker is written to the original workspace (which is quarantined)
//     and not to the staged one that becomes live.

// Test 4. Rematerialize, then a restart: the marker is present and the seed is
// exact.
func TestRematerializeFromTapeLeavesTheLeaseUntrustMarker(t *testing.T) {
	computerID := "computer-rematerialize-lease"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if live != nil {
			_ = live.Close()
		}
	}()
	rt, cas, acceptedHead := rematerializeTapeRuntime(t, computerID, storePath, live)
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt.selfdevUpdaterRoot = updaterRoot
	ctx := context.Background()
	seedRestoreBase(t, ctx, rt, cas, computerID, 1)
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, acceptedHead)

	// The live process allocated and published stream numbers.
	rec := &types.EventRecord{EventID: "live-1", RunID: "run-live", OwnerID: "owner-live", Kind: types.EventRunProgress}
	if err := live.AllocateEventSeq(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.RematerializeFromTape(ctx, computerID, checkpoint); err != nil {
		t.Fatalf("tape rematerialize refused: %v", err)
	}
	live = nil
	t.Cleanup(func() { _ = rt.store.Close() })
	workspace := rt.store.TexturePath()
	present, err := storeschema.LeaseUntrusted(workspace)
	if err != nil || !present {
		t.Fatalf("rematerialize flipped a workspace in with no lease untrust marker (present=%v err=%v)", present, err)
	}
	// A restart: close and open the same files.
	if err := rt.store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := choirstore.Open(rt.store.Path())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	next := &types.EventRecord{EventID: "after-restart", RunID: "run-live", OwnerID: "owner-live", Kind: types.EventRunProgress}
	if err := restarted.AllocateEventSeq(ctx, next); err != nil {
		t.Fatal(err)
	}
	st := restarted.StreamSeqLeaseStats()
	if st.Trusted || st.SeededFrom != "exact" {
		t.Fatalf("a rematerialized workspace seeded from %s (trusted=%v): %+v", st.SeededFrom, st.Trusted, st)
	}
	if next.StreamSeq == 0 {
		t.Fatal("the exact seed handed out no number")
	}
}

// Test 31 (agentcore half). The lease rows the live store wrote through the
// tape are exactly what a replay of the tape rebuilds: a replay-completeness
// probe after lease appends (a seed and a refill) is equivalent. The lease op
// carries fixed timestamps; if it carried a wall clock the live row and the
// replayed row would differ in updated_at and the content witness would
// report not_equivalent.
func TestReplayEquivalenceHoldsAfterLeaseAppends(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-lease-replay"
	rt, _ := selfdevReplayRuntime(t, computerID)
	for i := 0; i < 3; i++ {
		rec := &types.EventRecord{EventID: fmt.Sprintf("lease-ev-%d", i), RunID: "run-lease", OwnerID: "owner-lease",
			Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
		if err := rt.store.AppendEvent(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	if st := rt.store.StreamSeqLeaseStats(); st.State != "ready" || st.ReservedThrough == 0 {
		t.Fatalf("no lease was appended: %+v", st)
	}
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("a replay of the tape after lease appends is not equivalent to the live store: %#v", report.Result)
	}
}

// The deployed proof reads the lease through the product observability route
// (no SQL, no SSH): GET /api/runtime/observability carries a stream_seq_lease
// block with the eleven fields of the design's section 6. Ways it can fail:
// the block is absent; a field the proof needs is missing; reading the route
// takes a number or blocks on the allocator.
func TestGuestObservabilityServesTheStreamSeqLease(t *testing.T) {
	rt, _ := testRuntime(t)
	ctx := context.Background()
	rec := &types.EventRecord{EventID: "obs-1", RunID: "run-obs", OwnerID: "owner-obs", Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
	if err := rt.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-lease-obs")
	response := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", response.Code, response.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	raw, ok := body["stream_seq_lease"]
	if !ok {
		t.Fatalf("observability has no stream_seq_lease block: %s", response.Body.String())
	}
	var block map[string]any
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"state", "reserved_through", "allocated_through", "trusted", "untrust_reason", "seeded_from",
		"seed_ms", "cross_check_max", "refill_failed", "last_lease_error", "marker_present",
		"seed_failed", "lease_tombstoned", "seed_undecodable_rows"} {
		if _, present := block[key]; !present {
			t.Errorf("stream_seq_lease lacks %q: %s", key, raw)
		}
	}
	if block["state"] != "ready" || block["seeded_from"] != "exact" {
		t.Errorf("block = %s, want ready and seeded from exact", raw)
	}
	if after := rt.store.StreamSeqLeaseStats(); after.AllocatedThrough == 0 {
		t.Fatalf("stats after the route: %+v", after)
	}
}
