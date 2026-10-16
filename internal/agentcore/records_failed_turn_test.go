package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// failingTurnProvider scripts management's model: the first call of each
// turn runs one cell that reads choir.Records(); the second either fails
// (a provider outage mid-turn) or ends the turn.
type failingTurnProvider struct {
	*provider.StubProvider
	mu      sync.Mutex
	fail    bool
	outputs []string
}

func (p *failingTurnProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	if !strings.Contains(raw, "call-read-records") {
		args, _ := json.Marshal(map[string]any{"source": `rs := choir.Records()
n := 0
for _, r := range rs { if r.Captures != nil { n += len(r.Captures.Items) } }
fmt.Println("records", len(rs), "captures", n)`, "timeout_ms": 15000})
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "test-model",
			ToolCalls: []types.ToolCall{{ID: "call-read-records", Name: "desk_go_eval", Arguments: args}}}, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outputs = append(p.outputs, raw[strings.Index(raw, "call-read-records"):])
	if p.fail {
		return nil, errors.New("gateway call failed: provider test: refused (scripted outage)")
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
}

func (p *failingTurnProvider) lastOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.outputs) == 0 {
		return ""
	}
	return p.outputs[len(p.outputs)-1]
}

// The paper's 08:22 burst (docs/problems/
// captures-undisposed-after-failed-management-runs-2026-10-14.md): run
// a8081845 read records 508-514 in a cell that staged nothing, the
// provider failed, and the cursor had already moved past all 28 captures.
// No later run was shown them.
//
// Ways this could fail, written first:
//
//	(a) a management cell that only reads marks the records read, so a
//	    turn that fails afterwards consumes captures it never disposed;
//	(b) after the failed turn nothing wakes management again, so the
//	    captures wait for an unrelated arrival;
//	(c) the re-wake is immediate, so a provider outage hot-loops;
//	(d) the next turn is not shown the captures the failed turn read;
//	(e) a completed turn does not mark what it read, so every later turn
//	    is shown the same records again.
func TestManagementRecordsSurviveFailedTurn(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	model := &failingTurnProvider{StubProvider: provider.NewStubProvider(0), fail: true}
	rt.provider = model
	if err := rt.InstallDefaultAgentTools(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	owner, computer := "owner-failed-turn", rt.cfg.ComputerID
	management := persistentManagementAgentID(owner)

	var mu sync.Mutex
	var immediate []string
	type scheduled struct {
		to, kind  string
		notBefore time.Time
	}
	var later []scheduled
	rt.SetDispatchActor(func(dctx context.Context, ownerID, computerID, to, kind, content, _, _ string) error {
		switch kind {
		case "initial_dispatch":
			go func() {
				rec, err := s.GetRunByOwner(dctx, ownerID, strings.TrimSpace(content))
				if err == nil {
					rt.ExecuteActivationSync(dctx, &rec)
				}
			}()
		case "inbox_record":
			mu.Lock()
			immediate = append(immediate, to)
			mu.Unlock()
		}
		return nil
	})
	rt.SetScheduleActor(func(_ context.Context, _, _, to, kind, _, _, _ string, notBefore time.Time) error {
		mu.Lock()
		later = append(later, scheduled{to, kind, notBefore})
		mu.Unlock()
		return nil
	})
	cursor := func() int64 {
		c, _ := s.ReaderCursor(ctx, owner, computer, management, management)
		return c
	}

	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:test", []types.CaptureArrival{
		{CaptureID: "obj:choir.web_capture:test:key-a", Title: "a"},
		{CaptureID: "obj:choir.web_capture:test:key-b", Title: "b"},
	}, false); err != nil {
		t.Fatalf("arrival: %v", err)
	}
	turn := func() types.RunRecord {
		t.Helper()
		rec, err := rt.WakeRecordsInbox(ctx, owner, computer, management)
		if err != nil || rec == nil {
			t.Fatalf("wake management: %v, %v", rec, err)
		}
		return waitForStoredRunTerminalState(t, s, rec.RunID, 30*time.Second)
	}

	failed := turn()
	if failed.State != types.RunFailed {
		t.Fatalf("scripted outage: run state %s (%s), want failed", failed.State, failed.Error)
	}
	if out := model.lastOutput(); !strings.Contains(out, "records 1 captures 2") {
		t.Fatalf("the failed turn's cell did not read the arrival: %s", out)
	}
	// The stored state lands before the run's post-turn hook runs; the
	// scheduled wake is the hook's last step.
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(later) > 0 })
	if got := cursor(); got != 0 {
		t.Fatalf("(a) the failed turn marked the captures read: cursor %d, want 0", got)
	}
	mu.Lock()
	if len(immediate) != 0 {
		t.Fatalf("(c) the failed turn re-woke management at once: %v", immediate)
	}
	if len(later) != 1 || later[0].to != management || later[0].kind != "inbox_record" || time.Until(later[0].notBefore) < time.Minute {
		t.Fatalf("(b) the failed turn scheduled no backed-off wake for its unread records: %+v", later)
	}
	mu.Unlock()

	// The scheduled wake fires; the provider is back.
	model.mu.Lock()
	model.fail = false
	model.mu.Unlock()
	done := turn()
	if done.State != types.RunCompleted {
		t.Fatalf("retry turn: state %s (%s), want completed", done.State, done.Error)
	}
	if out := model.lastOutput(); !strings.Contains(out, "records 1 captures 2") {
		t.Fatalf("(d) the retry turn was not shown the captures the failed turn read: %s", out)
	}
	eventually(t, func() bool { return cursor() == 1 })
	if rec, err := rt.WakeRecordsInbox(ctx, owner, computer, management); err != nil || rec != nil {
		t.Fatalf("(e) a completed turn left its records unread: woke %v, %v", rec, err)
	}
}

// eventually waits up to ten seconds for cond.
func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within 10s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The failed-turn retry is bounded and backs off, and a crash never
// re-arms. Ways this could fail, written first: (a) every failed turn
// retries at the same delay, so an outage costs a turn every two minutes
// forever; (b) the retries never stop; (c) a completed turn does not reset
// the count, so a later outage gets no retry; (d) a crash-closed run
// schedules a wake (the restart rule: a crash never resumes work); (e) a
// research turn, whose rounds have their own closure, starts retrying too;
// (f) a failed turn with nothing unread schedules a wake.
func TestManagementFailedTurnRetryIsBoundedAndBacksOff(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-retry", rt.cfg.ComputerID
	management := persistentManagementAgentID(owner)
	var delays []time.Duration
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	rt.SetScheduleActor(func(_ context.Context, _, _, to, kind, _, _, _ string, notBefore time.Time) error {
		if kind == "inbox_record" {
			delays = append(delays, time.Until(notBefore).Round(time.Minute))
		}
		return nil
	})
	start := time.Now().UTC()
	run := func(agent, profile string, state types.RunState, errText string) *types.RunRecord {
		return &types.RunRecord{RunID: "run-" + string(state), AgentID: agent, OwnerID: owner, ComputerID: computer,
			AgentProfile: profile, AgentRole: profile, State: state, Error: errText, CreatedAt: start.Add(time.Hour)}
	}
	fail := func() {
		rt.recheckRecordsInbox(ctx, run(management, agentprofile.Management, types.RunFailed, "outage"))
	}

	fail()
	if len(delays) != 0 {
		t.Fatalf("(f) a failed turn with nothing unread scheduled a wake: %v", delays)
	}
	rt.recordsFailedTurns.Delete(management)
	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:test", []types.CaptureArrival{{CaptureID: "obj:choir.web_capture:test:key-r"}}, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < failedRecordsTurnRetryLimit+2; i++ {
		fail()
	}
	want := []time.Duration{2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute}
	if len(delays) != len(want) {
		t.Fatalf("(b) %d failed turns scheduled %d retries %v, want %v", failedRecordsTurnRetryLimit+2, len(delays), delays, want)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("(a) retry delays %v, want %v", delays, want)
		}
	}
	rt.recheckRecordsInbox(ctx, run(management, agentprofile.Management, types.RunCompleted, ""))
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, management, management); cursor != 0 {
		t.Fatalf("a completed turn that was shown nothing marked records read: cursor %d", cursor)
	}
	fail()
	if len(delays) != len(want)+1 || delays[len(delays)-1] != 2*time.Minute {
		t.Fatalf("(c) a completed turn did not reset the backoff: %v", delays)
	}
	n := len(delays)
	rt.recheckRecordsInbox(ctx, run(management, agentprofile.Management, types.RunCancelled, runFateInterruptedByRestart))
	if len(delays) != n {
		t.Fatalf("(d) a crash-closed run scheduled a wake: %v", delays)
	}
	research := "research:doc-retry"
	commit := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "retry-research", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "retry-research-note", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "x"},
		Inbox:     research, SenderInbox: "texture:doc-retry", Addressee: research, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: "texture:doc-retry"},
	}}}
	commit.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(commit)
	if _, err := s.CommitInboxRecords(ctx, commit); err != nil {
		t.Fatal(err)
	}
	rt.recheckRecordsInbox(ctx, run(research, agentprofile.Research, types.RunFailed, "outage"))
	if len(delays) != n {
		t.Fatalf("(e) a failed research turn scheduled a retry: %v", delays)
	}
}
