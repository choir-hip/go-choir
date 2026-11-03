package textureowner

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Problem 17: work the runtime refused because the desk's turn never disposed
// its trigger reopens on the owner's next revise; work the desk itself
// refused still refuses it.
func TestTextureWorkSettledByDeskReopensAfterTriggerNotDisposed(t *testing.T) {
	const docID = "doc-p17"
	snapshot := func(status types.WorkItemStatus, reason string) types.LifecycleSnapshot {
		return types.LifecycleSnapshot{
			Trajectory: types.TrajectoryRecord{Status: types.TrajectoryLive},
			Agents:     []types.AgentRecord{{AgentID: agentprofile.Texture + ":" + docID, LifecycleVersion: 1}},
			WorkItems: []types.WorkItemRecord{{WorkItemID: "w", AssignedAgentID: agentprofile.Texture + ":" + docID,
				AuthorityProfile: agentprofile.Texture, Status: status, Reason: reason, UpdatedAt: time.Now()}},
		}
	}
	if !textureWorkSettledByDesk(snapshot(types.WorkItemRefused, TriggerNotDisposedReason+": run r"), docID) {
		t.Fatalf("a trigger-not-disposed refusal does not reopen on owner input")
	}
	if textureWorkSettledByDesk(snapshot(types.WorkItemRefused, "the desk refused this"), docID) {
		t.Fatalf("a desk refusal reopened on owner input")
	}
	if !textureWorkSettledByDesk(snapshot(types.WorkItemCompleted, ""), docID) {
		t.Fatalf("completed work no longer reopens")
	}
}
