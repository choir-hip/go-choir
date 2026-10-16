package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A run's stored state can claim a loop that is gone: its end write failed
// (the 11:03 storage blip), or its loop died without one. The records wake
// read only the store, found the run "running" and consumed every wake,
// so management, and with it the paper, idled about 50 minutes until the
// progress deadline (docs/problems/failed-terminal-run-write-blocks-desk-for-an-hour-2026-10-16.md,
// step 2).
//
// Ways this could fail, written first:
//  1. a run stored "running" with no live loop still absorbs the wake (the
//     bug);
//  2. a live run, whose loop is registered in this process, is ended by
//     the wake;
//  3. a run set running a moment ago, whose loop is about to register,
//     is ended (a race with a real start);
//  4. a pending run, whose dispatch is in flight, is ended;
//  5. the dead run is replaced but left "running" beside the new one.
func TestRecordsWakeReplacesARunWithNoLiveLoop(t *testing.T) {
	cases := []struct {
		name     string
		state    types.RunState
		age      time.Duration
		live     bool
		replaced bool
	}{
		{"dead running run", types.RunRunning, 10 * time.Minute, false, true},
		{"live running run", types.RunRunning, 10 * time.Minute, true, false},
		{"just set running", types.RunRunning, 5 * time.Second, false, false},
		{"pending run", types.RunPending, 10 * time.Minute, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, s := testRuntime(t)
			ctx := context.Background()
			owner, computer := "owner-dead-run", rt.cfg.ComputerID
			management := persistentManagementAgentID(owner)
			var dispatched []string
			rt.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
				if kind == "initial_dispatch" {
					dispatched = append(dispatched, content)
				}
				return nil
			})
			agent, err := rt.EnsurePersistentManagementAgent(ctx, owner)
			if err != nil {
				t.Fatal(err)
			}
			stamp := time.Now().UTC().Add(-tc.age)
			old := types.RunRecord{
				RunID: "run-dead-" + tc.name, OwnerID: owner, ComputerID: computer, AgentID: management, ChannelID: agent.ChannelID,
				AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: tc.state, Prompt: "records turn",
				CreatedAt: stamp, UpdatedAt: stamp,
				Metadata: map[string]any{runMetadataAgentProfile: agentprofile.Management, runMetadataAgentRole: agentprofile.Management, runMetadataAgentID: management},
			}
			if err := s.CreateRun(ctx, old); err != nil {
				t.Fatal(err)
			}
			if tc.live {
				rt.runningMu.Lock()
				rt.running[old.RunID] = func() {}
				rt.runningMu.Unlock()
				t.Cleanup(func() {
					rt.runningMu.Lock()
					delete(rt.running, old.RunID)
					rt.runningMu.Unlock()
				})
			}
			if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:test", []types.CaptureArrival{
				{CaptureID: "obj:choir.web_capture:test:dead-run", Title: "a"},
			}, false); err != nil {
				t.Fatalf("arrival: %v", err)
			}
			dispatched = nil

			rec, err := rt.WakeRecordsInbox(ctx, owner, computer, management)
			if err != nil {
				t.Fatalf("wake: %v", err)
			}
			after, err := s.GetRunByOwner(ctx, owner, old.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.replaced {
				if rec != nil || after.State != tc.state {
					t.Fatalf("the wake touched a run it must leave: new=%v old state %s → %s", rec, tc.state, after.State)
				}
				return
			}
			if rec == nil || rec.RunID == old.RunID {
				t.Fatalf("a run with no live loop absorbed the wake (new run %v)", rec)
			}
			if !after.State.Terminal() {
				t.Fatalf("the dead run was left %s beside its replacement", after.State)
			}
			if len(dispatched) != 1 || dispatched[0] != rec.RunID {
				t.Fatalf("the replacement was not dispatched: %v", dispatched)
			}
		})
	}
}
