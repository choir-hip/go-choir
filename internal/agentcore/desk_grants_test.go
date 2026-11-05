package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The grant arguments are the closed sets the code enforces (MA4.1): the rule
// kinds of types and the task deadline.
func TestGrantArgumentsMatchTheCodeTheyBound(t *testing.T) {
	if len(deskmanifest.PrecommitRuleKinds) != len(types.PrecommitRuleKinds) {
		t.Fatalf("precommit.rule kinds %v differ from types %v", deskmanifest.PrecommitRuleKinds, types.PrecommitRuleKinds)
	}
	for i, k := range types.PrecommitRuleKinds {
		if deskmanifest.PrecommitRuleKinds[i] != string(k) {
			t.Fatalf("precommit.rule kind %d is %q, types has %q", i, deskmanifest.PrecommitRuleKinds[i], k)
		}
	}
	task, _ := deskmanifest.Lookup(agentprofile.ManagementTask)
	if task.RunDeadline() != managementTaskDeadline {
		t.Fatalf("run.deadline grant %s, managementTaskDeadline %s", task.RunDeadline(), managementTaskDeadline)
	}
}

// B2 in the host: a run keeps the desk epoch it started under, its session key
// carries it, and the row it holds is dropped when the run ends.
func TestRunPinsItsDeskEpochAcrossAnUpdate(t *testing.T) {
	const role = "app:runpin"
	mk := func(verbs ...string) deskmanifest.Desk {
		d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: role, Verbs: verbs, Taint: deskmanifest.TaintUntrustedInput, UID: 62902, WorkerMaxBytes: 64 << 20})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if err := deskmanifest.RegisterAppAt(mk("ReadCapture", "RequestLease"), 11); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(role) })
	base := deskmanifest.EpochStateNow()
	rt := &Runtime{}
	if got := rt.pinDeskEpoch("run-a", role); got != 11 {
		t.Fatalf("first pin %d", got)
	}
	if err := deskmanifest.UpdateApp(mk("ReadCapture"), 19); err != nil {
		t.Fatal(err)
	}
	if got := rt.pinDeskEpoch("run-a", role); got != 11 {
		t.Fatalf("the run moved to epoch %d mid-run", got)
	}
	if got := rt.pinDeskEpoch("run-b", role); got != 19 {
		t.Fatalf("a run started after the update pinned %d", got)
	}
	execCtx := toolregistry.ExecutionContext{RunRecord: &types.RunRecord{AgentID: "agent-1"}}
	oldKey, _ := deskSessionKey(execCtx, 11)
	newKey, _ := deskSessionKey(execCtx, 19)
	builtin, _ := deskSessionKey(execCtx, 0)
	if oldKey == newKey || builtin != "agent:agent-1" {
		t.Fatalf("keys %q %q %q", oldKey, newKey, builtin)
	}
	if !deskmanifest.FrameAllowedAt(role, 11, "request_lease") {
		t.Fatal("the pinned row was cut")
	}
	rt.releaseRunDeskEpochs("run-a")
	if deskmanifest.FrameAllowedAt(role, 11, "request_lease") {
		t.Fatal("the pinned row outlived its run")
	}
	rt.releaseRunDeskEpochs("run-b")
	rt.releaseRunDeskEpochs("run-b")
	_ = deskmanifest.RetireApp(role)
	if got := deskmanifest.EpochStateNow(); got.Retained != base.Retained || got.Holders != 0 {
		t.Fatalf("state %+v, baseline %+v", got, base)
	}
}

// D3: the crash boot reconciles the desk registry to the tape first, then closes
// every open run in one pass, whether or not its desk is registered at this
// head, and resumes none of them.
func TestCrashBootClosesRunsOfRegisteredAndUnregisteredDesks(t *testing.T) {
	t.Setenv("CHOIR_OWNER_ID", "owner-telemetry")
	rt := telemetryRuntime(t, "computer-boot-desks")
	rt.bus = events.NewEventBus()
	ctx := context.Background()
	order := []string{}
	rt.deskReconciler = func(context.Context) error { order = append(order, "reconcile"); return nil }
	now := time.Now().UTC().Add(-time.Hour)
	for _, r := range []types.RunRecord{
		{RunID: "run-reg", AgentID: "research:reg", AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research},
		{RunID: "run-gone", AgentID: "app:gone:one", AgentProfile: "app:gone", AgentRole: "app:gone"},
	} {
		r.OwnerID, r.ComputerID, r.State, r.CreatedAt, r.UpdatedAt = "owner-telemetry", rt.cfg.ComputerID, types.RunRunning, now, now
		if err := rt.store.CreateRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt.reconcileDesksAtBoot(ctx)
	rt.closeOpenObligationsAtBoot(ctx)
	if len(order) != 1 {
		t.Fatalf("the reconciler ran %d times", len(order))
	}
	for _, id := range []string{"run-reg", "run-gone"} {
		got, err := rt.store.GetRun(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != types.RunCancelled || got.Error != runFateInterruptedByRestart {
			t.Errorf("%s after a crash boot = %s / %q", id, got.State, got.Error)
		}
	}
	tally := &bootClosureTally{}
	tally.add("research")
	tally.add("app:gone")
	if s := tally.String(); s != "research=1 unregistered=1" {
		t.Errorf("tally %q", s)
	}
	if deskOfInbox("app:pkg:inst") != "app:pkg" || deskOfInbox("research:doc") != "research" {
		t.Error("deskOfInbox")
	}
}
