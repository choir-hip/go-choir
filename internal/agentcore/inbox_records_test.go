package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
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
