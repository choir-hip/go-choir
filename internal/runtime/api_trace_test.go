package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestHandleTraceTrajectoriesRestoresAcceptanceSnapshotRoute(t *testing.T) {
	t.Parallel()

	// Given: a real runtime store with a user-owned trajectory and product-path
	// trace events that run acceptance probes link to.
	rt, handler := testAPISetup(t)
	rec, err := rt.StartRunWithMetadata(context.Background(), "publish a small app", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileConductor,
		runMetadataAgentRole:    AgentProfileConductor,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	now := time.Now().UTC()
	for i, ev := range []types.EventRecord{
		{
			EventID:      "event-package-published",
			RunID:        rec.RunID,
			AgentID:      rec.AgentID,
			OwnerID:      "user-alice",
			TrajectoryID: rec.TrajectoryID,
			Timestamp:    now,
			Kind:         types.EventAppChangePackagePublished,
			Payload:      json.RawMessage(`{"package_id":"pkg-one"}`),
		},
		{
			EventID:      "event-adoption-verified",
			RunID:        rec.RunID,
			AgentID:      rec.AgentID,
			OwnerID:      "user-alice",
			TrajectoryID: rec.TrajectoryID,
			Timestamp:    now.Add(time.Second),
			Kind:         types.EventAppAdoptionVerified,
			Payload:      json.RawMessage(`{"adoption_id":"adopt-one"}`),
		},
		{
			EventID:      "event-adoption-promoted",
			RunID:        rec.RunID,
			AgentID:      rec.AgentID,
			OwnerID:      "user-alice",
			TrajectoryID: rec.TrajectoryID,
			Timestamp:    now.Add(2 * time.Second),
			Kind:         types.EventAppAdoptionPromoted,
			Payload:      json.RawMessage(`{"adoption_id":"adopt-one"}`),
		},
	} {
		ev.StreamSeq = int64(i + 1)
		if err := rt.Store().AppendEvent(context.Background(), &ev); err != nil {
			t.Fatalf("append event %s: %v", ev.EventID, err)
		}
	}

	// When: the legacy trace trajectory URL emitted by acceptance evidence is requested.
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/trace/trajectories/"+rec.TrajectoryID, "", "user-alice")

	// Then: the route is restored with the probe-visible trajectory and moment summaries.
	if w.Code != http.StatusOK {
		t.Fatalf("trace trajectory status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"published app package", "app adoption verified", "app adoption promoted"} {
		if !strings.Contains(body, want) {
			t.Fatalf("trace snapshot missing %q: %s", want, body)
		}
	}
	var resp traceTrajectorySnapshotResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode trace snapshot: %v", err)
	}
	if resp.Trajectory.TrajectoryID != rec.TrajectoryID {
		t.Fatalf("trajectory_id = %q, want %q", resp.Trajectory.TrajectoryID, rec.TrajectoryID)
	}
	if !resp.Trajectory.Live {
		t.Fatalf("trajectory live = false, want true: %+v", resp.Trajectory)
	}

	other := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/trace/trajectories/"+rec.TrajectoryID, "", "user-bob")
	if other.Code != http.StatusNotFound {
		t.Fatalf("other owner trace status = %d, want 404; body=%s", other.Code, other.Body.String())
	}
}
