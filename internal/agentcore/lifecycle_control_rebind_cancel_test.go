package agentcore

import (
	"context"
	"testing"
)

// TestFreedLifecycleControlRebindsAfterCarrierCancel is the regression for
// docs/problems/s0m-freed-control-stale-activerunid-blocks-rebind-2026-10-03.md.
// A research carrier bound mid-delivery that is cancelled (the
// open_researcher/ask kill path) must release its claim AND the freed packet
// must re-drive a fresh carrier — not re-strand because TerminalizeRun left
// agent.ActiveRunID pointing at the dead run.
func TestFreedLifecycleControlRebindsAfterCarrierCancel(t *testing.T) {
	rt, s := testRuntime(t)
	fixture := seedAtomicResearchControl(t, s, "rebind-cancel")
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	rec, err := rt.ReconcileCoagentWake(context.Background(), fixture.ownerID, fixture.agentID)
	if err != nil || rec == nil {
		t.Fatalf("mint carrier=%+v err=%v", rec, err)
	}
	if err := rt.CancelRun(context.Background(), rec.RunID, fixture.ownerID); err != nil {
		t.Fatalf("cancel carrier: %v", err)
	}
	freed, err := s.GetLifecycleUpdate(context.Background(), fixture.ownerID, fixture.computerID, fixture.trajectoryID, fixture.agentID, fixture.control.AgentID, fixture.control.ProducerUpdateID)
	if err != nil {
		t.Fatal(err)
	}
	if freed.Disposition != "pending" || freed.DeliveredToRunID != "" || freed.DeliveredAt != nil {
		t.Fatalf("freed control not re-pended: %+v", freed)
	}
	agent, err := s.GetAgentByScope(context.Background(), fixture.ownerID, fixture.computerID, fixture.agentID)
	if err != nil {
		t.Fatal(err)
	}
	if agent.ActiveRunID != "" {
		t.Fatalf("cancelled carrier left stale agent.ActiveRunID=%q — rebind gate will reject the mint", agent.ActiveRunID)
	}
	rebound, err := rt.ReconcileCoagentWake(context.Background(), fixture.ownerID, fixture.agentID)
	if err != nil {
		t.Fatalf("reconcile after cancel err=%v", err)
	}
	if rebound == nil {
		t.Fatal("no carrier minted after freed packet — packet re-stranded")
	}
	if rebound.RunID == rec.RunID {
		t.Fatalf("rebind returned the dead carrier %s", rebound.RunID)
	}
}
