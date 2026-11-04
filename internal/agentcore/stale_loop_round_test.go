package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A management loop left over from the previous round ends after a new
// owner request's Cast reached management, and its run-end closure (B6)
// closed the new round too: the next loop read the Cast and its engineering
// Cast was refused "the round ... is not open" (live deepseek calendar r1,
// 2026-11-02: loop b3fe1201 completed 13 s after the Cast to texture:29c0be04
// and wrote ...|texture:29c0be04...:r1:run-completed). The order reproduced:
// the old loop runs; the new Cast arrives; the old loop completes; the next
// loop casts to engineering.
//
// Ways this could fail, written first: the run end closes every open round
// addressed to management, the new one included; the new round is held but
// no loop is woken to read it, so it waits for the next restart; the hold
// also keeps a round the ended loop did read and leave unanswered (B6
// regresses); the engineering Cast's round check still refuses.
func TestStaleManagementLoopEndDoesNotCloseANewerRound(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-stale-loop", rt.cfg.ComputerID
	management := "management:" + owner
	oldTexture, newTexture := "texture:doc-files", "texture:doc-calendar"
	var woke []string
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		if kind == "inbox_record" {
			woke = append(woke, to)
		}
		return nil
	})
	cast := func(id, from string, at time.Time) {
		t.Helper()
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cmd-" + id, Records: []types.CommitmentRecord{{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: id, Body: `{"kind":"execution_request","summary":"x"}`},
			Cast:      &types.CommitmentCast{},
			Inbox:     management, SenderInbox: from, Addressee: management, EmitterKind: types.CommitmentEmitterAgent,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: from, CommittedAt: at.Format(time.RFC3339Nano)},
		}}}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	open := func(from string) bool {
		t.Helper()
		head, err := s.ConversationState(ctx, owner, computer, types.InboxConversationID(from, management))
		if err != nil {
			t.Fatal(err)
		}
		return head.Open
	}

	started := time.Now().UTC().Add(-time.Minute)
	// The Files round's Cast, which the old loop read (its cursor is past it).
	cast("cast-files", oldTexture, started.Add(-time.Minute))
	read := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "read-files",
		Cursor: &types.InboxCursorAdvance{ReaderAgentID: management, Inbox: management, ExpectedSeq: 0, Seq: 1}}
	read.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(read)
	if _, err := s.CommitInboxRecords(ctx, read); err != nil {
		t.Fatalf("advance cursor: %v", err)
	}
	// The old loop is running; the new owner request's Cast arrives.
	cast("cast-calendar", newTexture, started.Add(30*time.Second))
	// The old loop completes without a final Report.
	old := &types.RunRecord{RunID: "run-stale-files", AgentID: management, OwnerID: owner, ComputerID: computer,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunCompleted, CreatedAt: started}
	rt.recheckRecordsInbox(ctx, old)

	if open(oldTexture) {
		t.Fatal("the round the ended loop read and left unanswered stayed open (B6)")
	}
	if !open(newTexture) {
		t.Fatal("the ended loop closed a round whose Cast arrived after it began and that it never read")
	}
	if len(woke) == 0 || woke[len(woke)-1] != management {
		t.Fatalf("no management loop was woken to read the held round: %v", woke)
	}

	// The next loop casts to engineering for the new request.
	parent, err := s.ListInboxRecords(ctx, owner, computer, management, 1, 2)
	if err != nil || len(parent) != 1 {
		t.Fatalf("new Cast: %+v %v", parent, err)
	}
	spec, _ := json.Marshal(map[string]string{"parent": parent[0].RecordID})
	next := &types.RunRecord{RunID: "run-calendar", AgentID: management, OwnerID: owner, ComputerID: computer,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning}
	_, err = rt.planEngineeringCast(ctx, ReductionScope{OwnerID: owner, ComputerID: computer, DeskAgentID: management, FromRole: agentprofile.Management, RunID: next.RunID},
		next, yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, Req: testRequest(), LocalID: "tray-1", ToDesk: agentprofile.Engineering, Objective: "build calendar", Statement: string(spec)})
	if err != nil && strings.Contains(err.Error(), "is not open") {
		t.Fatalf("the engineering Cast was refused for a closed round: %v", err)
	}
}
