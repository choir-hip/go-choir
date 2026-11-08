package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/maildesk"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// An installed package desk runs (doctrine I26): the owner's routed request
// lands in its inbox, the inbox wake starts a run under the desk's own
// profile and the image's prompt, and its cells' RequestLease and ReadMail go
// through the Mail egress.
//
// Failure modes, written before the code:
//
//	W1  the hand-off is stored but not committed to the inbox, so no wake row
//	    exists and the desk never runs;
//	W2  the wake is a profile list, so a registered package desk is ignored;
//	W3  the run starts under another profile or agent;
//	W4  another owner's inbox wakes a run;
//	W5  a cell's RequestLease/ReadMail do not reach the egress (verbs,
//	    import set or frame table missing for an app desk), or a second email
//	    is served.
func TestInstalledPackageDeskWakesAndRunsItsCells(t *testing.T) {
	deskTestWorkerBin(t)
	e := newDrillEnv(t)
	e.rt.mailbox = ownerMailbox{owner: e.owner, box: drillMailbox{
		"msg-a": {From: "a@example.com", Subject: "first", TextBody: "body a"},
		"msg-b": {From: "b@example.com", Subject: "second", TextBody: "body b"},
	}}
	ctx := context.Background()
	id, raw, tree := e.publish(t, e.owner)
	e.install(t, id, raw, tree)
	desk := "app:" + drillPkg
	inbox := desk + ":" + e.owner

	// W4: another owner's inbox wakes nothing.
	if rec, err := e.rt.WakeRecordsInbox(ctx, "owner-other", e.computer, inbox); err != nil || rec != nil {
		t.Fatalf("W4: a foreign owner's wake started %+v %v", rec, err)
	}
	// W1: the routed hand-off moves the inbox head.
	if _, err := e.rt.DeliverOwnerRequest(ctx, e.owner, "run-drill-wake", desk, "summarize the email msg-a", nil, ""); err != nil {
		t.Fatal(err)
	}
	if unread, err := e.rt.InboxUnread(ctx, e.owner, e.computer, inbox); err != nil || !unread {
		t.Fatalf("W1: the desk's inbox has no unread record: %v %v", unread, err)
	}
	// W2, W3: the wake starts the desk's run.
	run, err := e.rt.WakeRecordsInbox(ctx, e.owner, e.computer, inbox)
	if err != nil || run == nil {
		t.Fatalf("W2: the inbox wake started no run: %v %v", run, err)
	}
	if run.AgentID != inbox || run.AgentProfile != desk {
		t.Fatalf("W3: run agent %q profile %q", run.AgentID, run.AgentProfile)
	}
	if want, _ := appdesk.FirstPartyPrompt(drillPkg); want == "" || !strings.Contains(want, "mail.read") {
		t.Fatalf("W3: the image ships no prompt for the desk: %q", want)
	}

	// W5: the desk's cells reach the egress, one email only.
	seed := types.RunRecord{RunID: "run-drill-cells", AgentID: inbox, OwnerID: e.owner, ComputerID: e.computer, ChannelID: inbox,
		AgentProfile: desk, AgentRole: desk, State: types.RunRunning}
	if err := e.rt.store.CreateRun(ctx, seed); err != nil {
		t.Fatal(err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: seed.RunID, AgentID: seed.AgentID, OwnerID: e.owner, ChannelID: inbox, ComputerID: e.computer,
		Profile: desk, Role: desk, RunRecord: &seed}
	tool := newDeskGoEvalTool(e.rt, newDeskSessionWorkers(), desk)
	cell := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 20000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			t.Fatalf("desk_go_eval: %v", err)
		}
		return out
	}
	// The ref comes from the routed record the desk reads, not a seeded value.
	out := cell(`import "choir"
import "strings"
ref := func() string {
	rs := choir.Records()
	fields := strings.Fields(rs[0].Objective)
	return fields[len(fields)-1]
}()
fmt.Println("ref", ref)
l, err := choir.RequestLease(map[string]any{"grants": []string{"mail.read"}, "refs": []string{ref}}, "summarize the one email", map[string]any{"p_success": 0.8, "weight": 0.3, "impact_class": "I1", "settlement": "recipient"})
fmt.Println("lease", err == nil, len(l) > 0)
m, err := choir.ReadMail(ref)
fmt.Println("read", err, string(m))`)
	if !strings.Contains(out, "ref msg-a") || !strings.Contains(out, "lease true true") || !strings.Contains(out, "first") || !strings.Contains(out, "owner_private:mail") {
		t.Fatalf("W5: the first email through the cell: %s", out)
	}
	out = cell(`import "choir"
_, err := choir.RequestLease(map[string]any{"grants": []string{"mail.read"}, "refs": []string{"msg-b"}}, "another email", map[string]any{"p_success": 0.8, "weight": 0.3, "impact_class": "I1", "settlement": "recipient"})
fmt.Println("second", err)
_, err = choir.ReadMail("msg-b")
fmt.Println("read-b", err)`)
	if !strings.Contains(out, "one email per task") || strings.Contains(out, "second <nil>") || strings.Contains(out, "read-b <nil>") {
		t.Fatalf("W5: the second email was not refused: %s", out)
	}

}

// ownerMailbox serves box as one owner's mailbox.
type ownerMailbox struct {
	owner string
	box   drillMailbox
}

func (m ownerMailbox) Message(ctx context.Context, ownerID, ref string) (maildesk.Message, error) {
	if ownerID != m.owner {
		return maildesk.Message{}, maildesk.ErrNoMessage
	}
	return drillMailbox.Message(m.box, ctx, "owner-ma34", ref)
}

// A completed turn marks the records it was shown read for an app desk too, so a
// later wake with no new record starts nothing (a wake loop otherwise restarts
// the run on every wake).
func TestPackageDeskWakeWithNoNewRecordStartsNothing(t *testing.T) {
	e := newDrillEnv(t)
	ctx := context.Background()
	id, raw, tree := e.publish(t, e.owner)
	e.install(t, id, raw, tree)
	desk := "app:" + drillPkg
	inbox := desk + ":" + e.owner
	if _, err := e.rt.DeliverOwnerRequest(ctx, e.owner, "run-loop-1", desk, "summarize the email msg-a", nil, ""); err != nil {
		t.Fatal(err)
	}
	run, err := e.rt.WakeRecordsInbox(ctx, e.owner, e.computer, inbox)
	if err != nil || run == nil {
		t.Fatalf("first wake: %v %v", run, err)
	}
	// The run completes having been shown the inbox through its head.
	head, err := e.rt.store.InboxHead(ctx, e.owner, e.computer, inbox)
	if err != nil || head < 1 {
		t.Fatalf("head %d %v", head, err)
	}
	done := *run
	done.State = types.RunCompleted
	e.rt.noteRecordsShown(done.RunID, head)
	e.rt.markTurnRecordsRead(ctx, &done)
	if cursor, _ := e.rt.store.ReaderCursor(ctx, e.owner, e.computer, inbox, inbox); cursor != head {
		t.Fatalf("an app desk's cursor = %d, want %d", cursor, head)
	}
	_ = e.rt.terminalizeRun(ctx, run.RunID, e.owner, "done")
	if again, err := e.rt.WakeRecordsInbox(ctx, e.owner, e.computer, inbox); err != nil || again != nil {
		t.Fatalf("a second wake with no new record started %+v %v", again, err)
	}
}
