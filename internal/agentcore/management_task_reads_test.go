package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Enforce flip S1 run 2 (design section 12): the host reads the management
// task kind with no declared reader in startQueuedManagementTasks and
// closeManagementTaskOfEndedRun (d@e 847ad2417: choir.management_task
// undeclared_reader). The task kind is management's; the host's bookkeeping
// reads it as its custodian, and it does so under another desk's context too.
func TestManagementTaskHostReadsDeclareTheCustodian(t *testing.T) {
	rt, _ := testRuntime(t)
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	defer stop()
	for name, ctx := range map[string]context.Context{
		"bare":       context.Background(),
		"engineer's": withDeskReader(context.Background(), agentprofile.Engineering),
	} {
		objectgraph.ResetUndeclaredReads()
		objectgraph.ResetReaderGateCounts()
		if _, err := rt.startQueuedManagementTasks(ctx, "owner-mt", rt.cfg.ComputerID); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rec := types.RunRecord{RunID: "run-mt-ended", AgentID: "management-task:task-x", OwnerID: "owner-mt", ComputerID: rt.cfg.ComputerID,
			AgentProfile: agentprofile.Management, AgentRole: agentprofile.ManagementTask, State: types.RunCompleted, Metadata: map[string]any{"management_task_id": "task-x"}}
		rt.closeManagementTaskOfEndedRun(ctx, &rec)
		if got := objectgraph.UndeclaredReads(); len(got) != 0 {
			t.Errorf("%s: the management task kind was read with no declared reader: %v", name, got)
		}
		requireNoReaderCrossings(t, name+" context")
	}
}
