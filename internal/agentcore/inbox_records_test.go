package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Cut 1 L3a through a real research cell: the cell's frame shows the
// records in (cursor, head] through choir.Records(); a ReportPacket to
// Texture commits as an addressed record in the same transaction as the
// cursor advance; a completed packet is the final Report that closes the
// round Texture's Cast opened.
//
// Ways this could fail, written first: the frame is empty because the
// cursor or head is read from the wrong inbox; the Report goes to the packet
// path (no record in Texture's inbox); the round stays open because the
// Report is not final or lands in another conversation; the cursor does not
// move, so the next cell shows the Cast again; a second cell re-commits.
func TestResearchCellReadsRecordsAndReportsToTexture(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer, doc := "owner-records", rt.cfg.ComputerID, "doc-records"
	texture, research := "texture:"+doc, "research:"+doc

	cast := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-cast", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:tray-1", Kind: types.CommitmentKindDirective,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover chips", Body: `{"kind":"question","summary":"chips"}`},
		Cast:        &types.CommitmentCast{},
		Inbox:       research, SenderInbox: texture, Addressee: research,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: texture},
	}}}
	cast.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(cast)
	if _, err := s.CommitInboxRecords(ctx, cast); err != nil {
		t.Fatalf("seed Cast: %v", err)
	}

	rec := types.RunRecord{RunID: "run-research-records", AgentID: research, OwnerID: owner, ComputerID: computer,
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
			t.Fatalf("desk_go_eval: %v", err)
		}
		return out
	}

	out := cell(`import "choir"
rs := choir.Records()
fmt.Println(len(rs), rs[0].Kind, rs[0].From, rs[0].Objective)
choir.ReportPacket("texture", map[string]interface{}{"kind":"evidence_update","summary":"it shipped","claims":[]interface{}{map[string]interface{}{"text":"it shipped"}},"work_disposition":"completed"}, "")`)
	if !strings.Contains(out, "1 cast "+texture+" cover chips") {
		t.Fatalf("first cell did not see the Cast record: %s", out)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(got) != 1 || !got[0].Final || got[0].SenderInbox != research || got[0].Report == nil || got[0].Report.StopReason != "completed" {
		t.Fatalf("Texture inbox after the Report = %+v, %v", got, err)
	}
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, research, research); cursor != 1 {
		t.Fatalf("research cursor = %d, want 1", cursor)
	}
	if rounds, _ := s.OpenRounds(ctx, owner, computer, research); len(rounds) != 0 {
		t.Fatalf("round still open after the final Report: %+v", rounds)
	}
	if out := cell(`import "choir"
fmt.Println("seen", len(choir.Records()))`); !strings.Contains(out, "seen 0") {
		t.Fatalf("the next cell shows read records again: %s", out)
	}
}

// A crash boot closes every open round with one system closure addressed to
// the caster; a second boot writes nothing; a planned boot closes nothing
// (Cut 1, "Restart"). Ways this could fail, written first: the closure
// lands in another conversation or round, so the round stays open; a
// second boot mints a second closure; the closure wakes the caster; a
// planned restart loses its open round.
func TestCrashBootClosesOpenRoundsOnce(t *testing.T) {
	for _, planned := range []bool{false, true} {
		rt, s := testRuntime(t)
		ctx := context.Background()
		owner, computer := "owner-rounds", rt.cfg.ComputerID
		t.Setenv("CHOIR_OWNER_ID", owner)
		texture, research := "texture:doc-rounds", "research:doc-rounds"
		cast := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-round", Records: []types.CommitmentRecord{{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-round", Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "o"}, Cast: &types.CommitmentCast{},
			Inbox: research, SenderInbox: texture, Addressee: research, EmitterKind: types.CommitmentEmitterAgent,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
		}}}
		cast.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(cast)
		if _, err := s.CommitInboxRecords(ctx, cast); err != nil {
			t.Fatalf("seed round: %v", err)
		}
		if planned {
			WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "t"}, true)(rt)
		}
		rt.Start(ctx)
		rounds, _ := s.OpenRounds(ctx, owner, computer, research)
		closures, _ := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
		if planned {
			if len(rounds) != 1 || len(closures) != 0 {
				t.Fatalf("planned boot: open rounds %+v, closures %+v; want the round kept", rounds, closures)
			}
			continue
		}
		if len(rounds) != 0 || len(closures) != 1 || closures[0].EmitterKind != types.CommitmentEmitterSystem || !closures[0].Final {
			t.Fatalf("crash boot: open rounds %+v, closures %+v; want one system closure", rounds, closures)
		}
		rt.closeOpenRoundsAfterCrash(ctx)
		if again, _ := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10); len(again) != 1 {
			t.Fatalf("a second boot wrote %d closures, want still 1", len(again))
		}
	}
}

// Cut 1 L3b through a real management cell: management reads a Texture's
// Cast from its inbox and answers that Texture in the same conversation;
// the bare "texture" is refused (management holds a conversation with every
// Texture of its owner); a records turn's Cast to engineering is refused
// until the engineering landing.
//
// Ways this could fail, written first: the cell sees nothing because the
// management inbox is keyed by something other than management:<owner>; the
// Report lands in a new conversation so the round never closes; a bare
// "texture" is guessed to some document; an engineering Cast falls through
// to the control path and fails with an unrelated admission error.
func TestManagementCellAnswersTextureOnRecords(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-mgmt", rt.cfg.ComputerID
	texture, management := "texture:doc-mgmt", "management:"+owner

	cast := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-mgmt-cast", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:mgmt", Kind: types.CommitmentKindDirective,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "publish the page", Body: `{"kind":"execution_request","summary":"publish"}`},
		Cast:        &types.CommitmentCast{},
		Inbox:       management, SenderInbox: texture, Addressee: management,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: texture},
	}}}
	cast.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(cast)
	if _, err := s.CommitInboxRecords(ctx, cast); err != nil {
		t.Fatalf("seed Cast: %v", err)
	}
	rec := types.RunRecord{RunID: "run-mgmt-records", AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning,
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	cell := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}

	if out := cell(`choir.ReportPacket("texture", map[string]interface{}{"kind":"blocker","summary":"x","notes":[]string{"x"}}, "")`); !strings.Contains(out, "texture:<doc>") {
		t.Fatalf("a bare \"texture\" was not refused: %s", out)
	}
	if out := cell(`choir.Cast("engineering", "build it", map[string]interface{}{})`); !strings.Contains(out, "not available on this build") {
		t.Fatalf("an engineering Cast from a records turn was not refused: %s", out)
	}
	out := cell(`rs := choir.Records()
fmt.Println(len(rs), rs[0].Kind, rs[0].From)
choir.ReportPacket(rs[0].From, map[string]interface{}{"kind":"blocker","summary":"engineering is not on records yet","notes":[]string{"blocked"},"work_disposition":"completed"}, "")`)
	if !strings.Contains(out, "1 cast "+texture) {
		t.Fatalf("management did not read the Cast: %s", out)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(got) != 1 || !got[0].Final || got[0].SenderInbox != management {
		t.Fatalf("Texture inbox after management's Report = %+v, %v", got, err)
	}
	if rounds, _ := s.OpenRounds(ctx, owner, computer, management); len(rounds) != 0 {
		t.Fatalf("round still open: %+v", rounds)
	}
}

// Late arrival (Cut 1 acceptance, decision D4): after a completed turn any
// unread record starts one more turn; after a failed turn only a record
// that arrived after the turn began does (its own records stay unread and
// re-arm nothing); a crash closure re-arms nothing. Ways this could fail,
// written first: a failed turn re-arms its own poison record and loops; a
// record that arrived mid-turn is stranded because its wake found the desk
// busy; a crash-closed run wakes its desk at boot.
func TestRecordsRecheckAfterTurn(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-late", rt.cfg.ComputerID
	research, texture := "research:doc-late", "texture:doc-late"
	var woke []string
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		if kind == "inbox_record" {
			woke = append(woke, to)
		}
		return nil
	})
	commit := func(id string, at time.Time) {
		t.Helper()
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "late-" + id, Records: []types.CommitmentRecord{{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: id},
			Inbox: research, SenderInbox: texture, Addressee: research, EmitterKind: types.CommitmentEmitterAgent,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture, CommittedAt: at.Format(time.RFC3339Nano)},
		}}}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	start := time.Now().UTC()
	commit("before-turn", start.Add(-time.Minute))
	run := func(state types.RunState, errText string) *types.RunRecord {
		return &types.RunRecord{RunID: "run-" + string(state), AgentID: research, OwnerID: owner, ComputerID: computer,
			AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: state, Error: errText, CreatedAt: start}
	}

	rt.recheckRecordsInbox(ctx, run(types.RunFailed, "boom"))
	if len(woke) != 0 {
		t.Fatalf("a failed turn re-armed the record it already had: %v", woke)
	}
	commit("mid-turn", start.Add(time.Second))
	rt.recheckRecordsInbox(ctx, run(types.RunFailed, "boom"))
	if len(woke) != 1 || woke[0] != research {
		t.Fatalf("a record that arrived after the failed turn began did not start a turn: %v", woke)
	}
	rt.recheckRecordsInbox(ctx, run(types.RunCancelled, runFateInterruptedByRestart))
	if len(woke) != 1 {
		t.Fatalf("a crash-closed run woke its desk: %v", woke)
	}
	rt.recheckRecordsInbox(ctx, run(types.RunCompleted, ""))
	if len(woke) != 2 {
		t.Fatalf("a completed turn with unread records did not start one more turn: %v", woke)
	}
	// B7: a parked run's turn has ended too. A park after a committed turn
	// is a completed turn; a park that committed nothing re-arms only for
	// records that arrived after it began.
	parked := func(reason string, created time.Time) *types.RunRecord {
		r := run(types.RunPassivated, "")
		r.CreatedAt = created
		r.Metadata = map[string]any{"passivated_reason": reason}
		return r
	}
	rt.recheckRecordsInbox(ctx, parked("durable_transition_success", start))
	if len(woke) != 3 {
		t.Fatalf("a run parked after a committed turn did not start one more turn for its unread records: %v", woke)
	}
	rt.recheckRecordsInbox(ctx, parked("idle_deadline", start.Add(time.Hour)))
	if len(woke) != 3 {
		t.Fatalf("a park that committed nothing re-armed records it already had: %v", woke)
	}
}

// Cut 1 routing, invariant 6: engineering reaches Texture only through
// management. The reducer refuses an engineering act addressed to Texture
// by desk name or by a Texture inbox, for every addressed kind, before
// anything commits; management and research may address a Texture inbox.
func TestReducerRefusesEngineeringActsToTexture(t *testing.T) {
	for _, to := range []string{"texture", "texture:doc-1"} {
		for _, kind := range []string{yaegikernel.IntentReport, yaegikernel.IntentNote, yaegikernel.IntentAsk, yaegikernel.IntentCast} {
			in := yaegikernel.StagedIntent{Kind: kind, LocalID: "tray-1", ToDesk: to, Objective: "x", Body: "x", Claim: "x"}
			err := validateCellIntents(ReductionScope{FromRole: agentprofile.Engineering}, []yaegikernel.StagedIntent{in})
			if err == nil || !strings.Contains(err.Error(), "only through management") {
				t.Errorf("engineering %s to %s: want the routing refusal, got %v", kind, to, err)
			}
		}
	}
	report := yaegikernel.StagedIntent{Kind: yaegikernel.IntentReport, LocalID: "tray-1", ToDesk: "texture:doc-1", Claim: "x"}
	for _, from := range []string{agentprofile.Management, agentprofile.Research} {
		if err := validateCellIntents(ReductionScope{FromRole: from}, []yaegikernel.StagedIntent{report}); err != nil && strings.Contains(err.Error(), "only through management") {
			t.Errorf("%s report to a Texture inbox refused by the engineering rule: %v", from, err)
		}
	}
}

// Cut 1 breakage B6: a round stays open when its recipient's run ends
// without a final Report, and nothing wakes the caster. When a research or
// management run ends its turn (terminal, or parked) with no run of that
// desk still active, the runtime closes each round it is the recipient of
// with a final system Report and wakes the caster.
//
// Ways this could fail, written first: the round stays open; the closure
// wakes nobody, so Texture still waits; a crash-closed run closes rounds a
// second time (the boot already did, and must wake no one); a run that ends
// while a newer run of the same desk is active closes a round that run is
// answering; a closure is committed twice for one round.
func TestRecipientRunEndClosesItsOpenRounds(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-ended", rt.cfg.ComputerID
	research, texture := "research:doc-ended", "texture:doc-ended"
	var woke []string
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		if kind == "inbox_record" {
			woke = append(woke, to)
		}
		return nil
	})
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cast-ended", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Body: "find X"}, Cast: &types.CommitmentCast{},
		Inbox: research, SenderInbox: texture, Addressee: research, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
	}}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("commit cast: %v", err)
	}
	conversation := types.InboxConversationID(texture, research)
	open := func() bool {
		t.Helper()
		head, err := s.ConversationState(ctx, owner, computer, conversation)
		if err != nil {
			t.Fatal(err)
		}
		return head.Open
	}
	if !open() {
		t.Fatal("the Cast did not open a round")
	}
	ended := func(state types.RunState, errText string) *types.RunRecord {
		return &types.RunRecord{RunID: "run-" + string(state), AgentID: research, OwnerID: owner, ComputerID: computer,
			AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: state, Error: errText, CreatedAt: time.Now().UTC()}
	}

	rt.closeRoundsOfEndedRun(ctx, ended(types.RunCancelled, runFateInterruptedByRestart))
	if !open() || len(woke) != 0 {
		t.Fatalf("a crash-closed run closed its round or woke someone (open=%v woke=%v)", open(), woke)
	}
	rt.closeRoundsOfEndedRun(ctx, ended(types.RunFailed, "provider error"))
	if open() {
		t.Fatal("the round stayed open after its recipient's run failed without a final Report")
	}
	if len(woke) != 1 || woke[0] != texture {
		t.Fatalf("the closure did not wake the caster: %v", woke)
	}
	records, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(records) != 1 || !records[0].Final || records[0].EmitterKind != types.CommitmentEmitterSystem {
		t.Fatalf("Texture's inbox after the closure: %+v err=%v", records, err)
	}
	rt.closeRoundsOfEndedRun(ctx, ended(types.RunFailed, "provider error"))
	if records, _ := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10); len(records) != 1 || len(woke) != 1 {
		t.Fatalf("a second end closed the closed round again: records=%d woke=%v", len(records), woke)
	}
}
