package agentcore

// MA1.3 follow-up from the MA1.2 re-review: every addressee-bearing act goes
// through ResolveAddress, and a CancelAct only retracts the cell's own
// commitments. Ways this could fail, written first:
//
//  1. choir.Escalate("research:d2", ...) from research:d1 mints a record and a
//     wake at another subject's desk (the routeErr was swallowed for any kind
//     outside recordsIntent).
//  2. choir.Precommit with resolver "research:d2", "engineering:a2" or
//     "management-task:t1" takes rec.Addressee = resolver and bypasses the
//     resolver.
//  3. choir.CancelAct(<a commitment addressed to research:d2, written by
//     someone else>) sends a retract packet to research:d2.
//  4. The refusal is silent, or is a downstream "agent not found" that looks
//     like a missing desk rather than a refused address.
//  5. The fix refuses legitimate addresses: Escalate to management, a
//     Precommit with resolver "rule", "research" or "management", a CancelAct
//     of the cell's own commitment.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func TestAddresseeBearingActsGoThroughTheResolver(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer, doc := "owner-scope", rt.cfg.ComputerID, "d1"
	research := "research:" + doc

	// A commitment someone else wrote, addressed to another subject's desk.
	foreign := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:foreign", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "other work", Body: `{"kind":"question","summary":"s"}`},
		Cast:      &types.CommitmentCast{}, Inbox: "research:d2", SenderInbox: "texture:d2", Addressee: "research:d2",
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: "texture:d2"},
	}
	seed := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-foreign", Records: []types.CommitmentRecord{foreign}}
	seed.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(seed)
	if _, err := s.CommitInboxRecords(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := types.RunRecord{RunID: "run-scope-1", AgentID: research, OwnerID: owner, ComputerID: computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: computer,
		Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	cell := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}
	req := `map[string]any{"p_success": 0.7, "weight": 0.4, "settlement": "recipient"}`
	pre := func(resolver string) string {
		return `choir.Precommit(map[string]any{"question": "will it hold?", "p": 0.6, "resolver": "` + resolver + `"})`
	}

	for name, src := range map[string]string{
		"escalate to another subject's research": `choir.Escalate("research:d2", "look here", ` + req + `)`,
		"escalate to an engineering assignment":  `choir.Escalate("engineering:a2", "look here", ` + req + `)`,
		"precommit resolver research:d2":         pre("research:d2"),
		"precommit resolver engineering:a2":      pre("engineering:a2"),
		"precommit resolver management-task:t1":  pre("management-task:t1"),
		"cancel another desk's commitment":       `choir.CancelAct("texture-cell:cast:foreign")`,
	} {
		out := cell(src)
		if !strings.Contains(out, "may name") {
			t.Errorf("%s: not refused by the address check: %s", name, out)
		}
	}

	// Legitimate addresses are not refused by the check.
	for name, src := range map[string]string{
		"escalate to management":      `choir.Escalate("management", "need a decision", ` + req + `)`,
		"precommit resolver rule":     pre("rule"),
		"precommit resolver desk":     pre("management"),
		"precommit resolver research": pre("research"),
	} {
		if out := cell(src); strings.Contains(out, "may name") {
			t.Errorf("%s: refused by the address check: %s", name, out)
		}
	}
}

// TestDeclineAndFeedRefuseAnUnresolvableAddress: Decline and Feed route through
// ResolveAddress, and an address that does not resolve is refused with a
// visible error before anything commits.
func TestDeclineAndFeedRefuseAnUnresolvableAddress(t *testing.T) {
	rt, st := testRuntime(t)
	scope := ReductionScope{
		FromAgentID: "research:d1", DeskAgentID: "research:d1", FromRole: agentprofile.Research,
		ChannelID: "d1", RunID: "run-df", OwnerID: "owner-df", ComputerID: "computer-df", CellID: "cell-df",
	}
	req := &types.RequestFields{PSuccess: ptrF(0.7), Weight: ptrF(0.4), Settlement: "recipient"}
	for _, kind := range []string{yaegikernel.IntentFeed, yaegikernel.IntentDecline} {
		for _, to := range []string{"nosuchdesk:x", "engineering:a2"} {
			r := &rlmCallReduction{active: true, mb: rt, st: st, ledger: st, scope: scope}
			err := r.commit(context.Background(), []yaegikernel.StagedIntent{{Kind: kind, LocalID: "t1", ToDesk: to, Body: "b", Req: req}})
			if err == nil {
				t.Errorf("%s to %q was not refused", kind, to)
			}
		}
	}
}

func ptrF(f float64) *float64 { return &f }

// TestNonRecordsPathActsResolveOrAreRefused: an app desk (no records inbox)
// Casting, Asking, Reporting or Replying to another subject's desk is refused;
// a CancelAct whose author cannot be read is refused; a bare Precommit
// resolver must be the rule resolver or a registered desk.
func TestNonRecordsPathActsResolveOrAreRefused(t *testing.T) {
	rt, st := testRuntime(t)
	app := ReductionScope{FromAgentID: "ledgerapp:u1", DeskAgentID: "ledgerapp:u1", RunID: "run-app", OwnerID: "owner-app", ComputerID: "computer-app", CellID: "cell-app"}
	r := &rlmCallReduction{active: true, mb: rt, st: st, ledger: st, scope: app}
	for _, kind := range []string{yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentReport, yaegikernel.IntentReply} {
		err := r.checkAddresseeScope(context.Background(), yaegikernel.StagedIntent{Kind: kind, LocalID: "a1", ToDesk: "research:other-doc"})
		if err == nil || !strings.Contains(err.Error(), "may name") {
			t.Errorf("app desk %s to research:other-doc: %v", kind, err)
		}
	}
	// No runtime to read the target's author: a CancelAct is refused.
	bare := &rlmCallReduction{active: true, scope: ReductionScope{DeskAgentID: "research:d1", FromAgentID: "research:d1"}}
	if err := bare.checkAddresseeScope(context.Background(), yaegikernel.StagedIntent{Kind: yaegikernel.IntentCancel, LocalID: "c1", TargetRef: "x"}); err == nil {
		t.Error("a CancelAct with no readable author was not refused")
	}
	res := ReductionScope{FromAgentID: "research:d1", DeskAgentID: "research:d1", OwnerID: "owner-app", ComputerID: "computer-app"}
	rr := &rlmCallReduction{active: true, mb: rt, st: st, ledger: st, scope: res}
	for resolver, wantRefused := range map[string]bool{"rule": false, "research": false, "management": false, "nosuchdesk": true} {
		err := rr.checkAddresseeScope(context.Background(), yaegikernel.StagedIntent{Kind: yaegikernel.IntentPrecommit, LocalID: "p1",
			Precommit: `{"question":"q","p":0.6,"resolver":"` + resolver + `"}`})
		if (err != nil) != wantRefused {
			t.Errorf("bare resolver %q: refused=%v err=%v", resolver, err != nil, err)
		}
	}
}
