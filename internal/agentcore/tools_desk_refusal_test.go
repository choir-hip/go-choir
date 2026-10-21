package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// SEC-02: a frame the daemon refuses from a desk worker is a trace event, so
// the refusal is evidence and not only a string the model reads.
//
// Ways this could fail, written first: the event lands on no run (the refusal
// hook's ctx carries no execution context); a forged frame is recorded as an
// ordinary error; the event omits which desk and which action were refused.
func TestDeskRefusalIsRecordedOnTheRunTrace(t *testing.T) {
	rt, s := testRuntime(t)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-refusal-events"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "texture:doc", OwnerID: execCtx.OwnerID, TrajectoryID: "traj-refusal"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	rt.recordDeskRefusal(ctx, execCtx, "agent:texture:doc", yaegikernel.FrameRefusal{
		Role: agentprofile.Texture, Action: yaegikernel.ActionProductAPI, Stream: yaegikernel.StreamBrokerEgress,
		Reason: `refused: desk "texture" may not call product_api`, RequestID: "req-1", WorkerPID: 4242,
	})
	// A refusal whose hook ctx lost the execution context still lands on the
	// run, through the spawning tool's context.
	rt.recordDeskRefusal(context.Background(), execCtx, "agent:texture:doc", yaegikernel.FrameRefusal{
		Role: agentprofile.Texture, Action: yaegikernel.ActionRead, Stream: yaegikernel.StreamBrokerEgress,
		Reason: "refused: slot token does not match the worker the host started", RequestID: "req-2", WorkerPID: 4242, Forged: true,
	})

	evs, err := s.ListEvents(context.Background(), execCtx.RunID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var got []map[string]any
	for _, ev := range evs {
		if ev.Kind != types.EventDeskEgress {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		got = append(got, p)
		if ev.TrajectoryID != "traj-refusal" {
			t.Fatalf("refusal event not keyed to the run's trajectory: %q", ev.TrajectoryID)
		}
	}
	if len(got) != 2 {
		t.Fatalf("want two refusal events, got %d: %v", len(got), got)
	}
	first, second := got[0], got[1]
	if first["refused"] != true || first["desk"] != "texture" || first["verb"] != "product_api" || first["is_error"] != true || first["forged"] != false {
		t.Fatalf("first refusal event wrong: %v", first)
	}
	if second["refused"] != true || second["forged"] != true || second["request_id"] != "req-2" {
		t.Fatalf("forged-frame event must say forged: %v", second)
	}
}
