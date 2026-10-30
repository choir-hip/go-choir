package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// C1e B0 (docs/archive/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md,
// acceptance 6): the checker's I7 read work items and packets from the
// trajectory snapshot and passed when those arrays were missing, so it would
// pass vacuously once B5 deletes them. The census is the authoritative,
// computer-wide list of retired work-state objects. Failure modes pinned: an
// unauthenticated read is served; an empty census is indistinguishable from
// an unserved one (no kinds named); a work item is missing or lacks its
// route; another owner's objects are listed.
func TestCensusListsRetiredWorkStateObjects(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	get := func(owner string) (int, censusResponse) {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/census", nil)
		req.RemoteAddr = ""
		if owner != "" {
			req.Header.Set("X-Authenticated-User", owner)
		}
		rec := httptest.NewRecorder()
		NewAPIHandler(rt).HandleCensus(rec, req)
		var body censusResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}
	if code, _ := get(""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated census = %d", code)
	}
	code, empty := get("user-census")
	if code != http.StatusOK || len(empty.Kinds) != 2 || len(empty.Objects) != 0 || empty.Truncated {
		t.Fatalf("empty census = %d %+v, want both kinds named and no objects", code, empty)
	}
	for _, owner := range []string{"user-census", "user-other"} {
		if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{TrajectoryID: "traj-" + owner, OwnerID: owner, Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
			SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion}}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{TrajectoryID: "traj-" + owner, OwnerID: owner, Objective: "census", AssignedAgentID: "research:r1"}); err != nil {
			t.Fatal(err)
		}
	}
	_, got := get("user-census")
	if len(got.Objects) != 1 || got.Counts[got.Objects[0].Kind] != 1 {
		t.Fatalf("census = %+v, want the owner's one work item", got)
	}
	o := got.Objects[0]
	if o.To != "research:r1" || o.TrajectoryID != "traj-user-census" || o.CanonicalID == "" {
		t.Fatalf("census object = %+v, want its route and trajectory", o)
	}
}
