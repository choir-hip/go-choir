package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The management desk's frame must be bounded however long its backlog is
// (docs/evidence/paper-token-budget-2026-11-09.md section 1.3: on the
// Autopaper the unread inbox made a 5.5 MB frame, then 8.6 MB, over the
// 4 MiB transport bound, and every cell failed with "desk eval write: frame:
// payload 5489824 bytes exceeds 4194304").
//
// Ways this could fail, written first: the frame still carries the whole
// range (the cell does not start); the window is the newest records, so the
// oldest never drain; the cursor jumps to the head past records the cell
// never saw, so the backlog is lost; the cursor does not move, so every cell
// shows the same window and the backlog never drains; a record bigger than
// the frame wedges the window it sits in.
func TestManagementBacklogOverFrameIsWindowedOldestFirstAndDrains(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-window", rt.cfg.ComputerID
	management, source := "management:"+owner, "source:rss:test"
	rt.SetDispatchActor(func(_ context.Context, _, _, _, _, _, _, _ string) error { return nil })

	const records = 60
	body := strings.Repeat("x", 100<<10) // 60 x 100 KiB = 6 MB, over the 4 MiB frame
	start := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < records; i++ {
		id := fmt.Sprintf("n%02d", i)
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "window-" + id, Records: []types.CommitmentRecord{{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: id + body},
			Inbox:     management, SenderInbox: source, Addressee: management, EmitterKind: types.CommitmentEmitterSystem,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: "system:captures", CommittedAt: start.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)},
			Captures: &types.CommitmentCaptures{SourceID: "rss:test", Items: []types.CaptureArrival{{CaptureID: "cap-" + id, URL: "https://window.example/" + id}}},
		}}}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	cursor := func() int64 {
		c, _ := s.ReaderCursor(ctx, owner, computer, management, management)
		return c
	}
	rec := types.RunRecord{RunID: "run-window", AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning, CreatedAt: time.Now().UTC(),
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	calls := 0
	cell := func(src string) string {
		t.Helper()
		calls++
		execCtx.ToolCallID = fmt.Sprintf("call-%d", calls)
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 30000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			t.Fatalf("desk_go_eval: %v", err)
		}
		return out
	}

	// The first cell starts, and shows a bounded window that begins at the
	// oldest record, with the count of what is past it.
	out := cell(`rs := choir.Records()
fmt.Println("window", len(rs), rs[0].ID[:3], rs[len(rs)-1].ID[:3], "remaining", choir.Context()["records_remaining"])`)
	if strings.Contains(out, "exceeds") || !strings.Contains(out, "window ") || !strings.Contains(out, " n00 ") {
		t.Fatalf("the first cell did not get a window starting at the oldest record: %s", out)
	}
	if strings.Contains(out, "window 60 ") || !strings.Contains(out, "remaining ") || strings.Contains(out, "remaining \n") {
		t.Fatalf("the window was not bounded, or did not say what remains: %s", out)
	}

	// Each acting cell marks read exactly the window it was shown, so the
	// backlog drains oldest first until the cursor reaches the head.
	last := int64(0)
	for round := 0; cursor() < records && round < 20; round++ {
		before := cursor()
		out := cell(fmt.Sprintf(`rs := choir.Records()
fmt.Println("first", rs[len(rs)-1].Seq, len(rs))
fmt.Println(choir.Ask("research:doc-x", "draining round %d", map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"}))`, round))
		if !strings.Contains(out, "rlm:ask") {
			t.Fatalf("round %d: the acting cell did not commit: %s", round, out)
		}
		if cursor() <= before || cursor() <= last {
			t.Fatalf("round %d: the cursor did not advance past %d: %d (%s)", round, before, cursor(), out)
		}
		last = cursor()
	}
	if cursor() != records {
		t.Fatalf("the backlog did not drain to the head: cursor %d, want %d", cursor(), records)
	}
}
