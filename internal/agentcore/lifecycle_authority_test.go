package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type lateCompletionProvider struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
	once     sync.Once
}

func newLateCompletionProvider() *lateCompletionProvider {
	return &lateCompletionProvider{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (p *lateCompletionProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	_ = ctx
	_ = emit
	close(p.started)
	<-p.release
	task.Result = "late completion"
	close(p.finished)
	return nil
}

func (p *lateCompletionProvider) ProviderName() string { return "late-completion-test" }

func (p *lateCompletionProvider) unblock() {
	p.once.Do(func() { close(p.release) })
}

func waitForTerminalRun(t *testing.T, rt *Runtime, runID string) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec, err := rt.store.GetRun(context.Background(), runID)
		if err == nil && rec.State.Terminal() {
			return rec
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %s did not become terminal", runID)
	return types.RunRecord{}
}

func TestCancelResidentRunReleasesImmediatelyAndRejectsLateCompletion(t *testing.T) {
	rt, _ := testRuntime(t)
	provider := newLateCompletionProvider()
	rt.provider = provider
	rt.cfg.ActivationBudget = time.Hour
	t.Cleanup(provider.unblock)

	rec, err := rt.StartRun(context.Background(), "block until cancelled", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	<-provider.started
	if got := rt.RunningCount(); got != 1 {
		t.Fatalf("running count before cancel = %d, want 1", got)
	}

	if err := rt.CancelRun(context.Background(), rec.RunID, "user-alice"); err != nil {
		t.Fatalf("cancel resident run: %v", err)
	}
	stored, err := rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get cancelled run: %v", err)
	}
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("cancelled run = state %q finished_at %v", stored.State, stored.FinishedAt)
	}
	if got := rt.RunningCount(); got != 0 {
		t.Fatalf("running count after cancel = %d, want 0", got)
	}

	provider.unblock()
	<-provider.finished
	rt.wg.Wait()
	stored, err = rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get run after late completion: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("state after late completion = %q, want cancelled", stored.State)
	}
}

func TestIdlePassivationCannotOverwriteCancelledRun(t *testing.T) {
	rt, _ := testRuntime(t)
	stale, err := rt.createRunWithMetadata(context.Background(), "stale passivation", "user-alice", nil)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := rt.CancelRun(context.Background(), stale.RunID, "user-alice"); err != nil {
		t.Fatalf("cancel run: %v", err)
	}

	rt.passivateIdleToolLoopRun(
		context.Background(),
		stale,
		"late passivation",
		provideriface.TokenUsage{InputTokens: 3, OutputTokens: 5},
		&toolregistry.ToolLoopPassivatedError{Reason: "idle"},
	)

	stored, err := rt.store.GetRun(context.Background(), stale.RunID)
	if err != nil {
		t.Fatalf("get run after late passivation: %v", err)
	}
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("run after late passivation = state %q finished_at %v, want cancelled terminal state", stored.State, stored.FinishedAt)
	}
}

func TestActivationBudgetProgressDeadlineTerminalizesAndReleases(t *testing.T) {
	rt, _ := testRuntime(t)
	provider := newLateCompletionProvider()
	rt.provider = provider
	rt.cfg.ActivationBudget = 25 * time.Millisecond
	t.Cleanup(provider.unblock)

	rec, err := rt.StartRun(context.Background(), "outlive activation budget", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	<-provider.started
	stored := waitForTerminalRun(t, rt, rec.RunID)
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("progress deadline run = state %q finished_at %v", stored.State, stored.FinishedAt)
	}
	if !strings.Contains(stored.Error, "activation budget") || !strings.Contains(stored.Error, "progress deadline") {
		t.Fatalf("deadline error = %q, want activation budget and progress deadline", stored.Error)
	}
	if got := rt.RunningCount(); got != 0 {
		t.Fatalf("running count after progress deadline = %d, want 0", got)
	}

	provider.unblock()
	<-provider.finished
	rt.wg.Wait()
	stored, err = rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get deadline run after late completion: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("deadline state after late completion = %q, want cancelled", stored.State)
	}
}

func TestRunListAndCancelRoutesAreWiredAndOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)
	alice, err := rt.createRunWithMetadata(context.Background(), "alice pending", "user-alice", nil)
	if err != nil {
		t.Fatalf("create alice run: %v", err)
	}
	bob, err := rt.createRunWithMetadata(context.Background(), "bob pending", "user-bob", nil)
	if err != nil {
		t.Fatalf("create bob run: %v", err)
	}

	listW := runtimeHandlerRequest(t, handler.HandleRunList, http.MethodGet, "/api/agent/loops?limit=20", "", "user-alice")
	if listW.Code != http.StatusOK {
		t.Fatalf("run list status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}
	var listResp runListResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode run list: %v", err)
	}
	if len(listResp.Runs) != 1 || listResp.Runs[0].RunID != alice.RunID {
		t.Fatalf("owner-scoped run list = %+v, want only %s", listResp.Runs, alice.RunID)
	}

	foreignW := runtimeHandlerRequest(t, handler.HandleCancel, http.MethodPost, "/api/agent/cancel", `{"loop_id":"`+bob.RunID+`"}`, "user-alice")
	if foreignW.Code != http.StatusNotFound {
		t.Fatalf("foreign cancel status = %d, want 404; body=%s", foreignW.Code, foreignW.Body.String())
	}

	cancelW := runtimeHandlerRequest(t, handler.HandleCancel, http.MethodPost, "/api/agent/cancel", `{"loop_id":"`+alice.RunID+`"}`, "user-alice")
	if cancelW.Code != http.StatusOK {
		t.Fatalf("owner cancel status = %d, want 200; body=%s", cancelW.Code, cancelW.Body.String())
	}
	cancelled, err := rt.store.GetRun(context.Background(), alice.RunID)
	if err != nil {
		t.Fatalf("get route-cancelled run: %v", err)
	}
	if cancelled.State != types.RunCancelled || cancelled.FinishedAt == nil {
		t.Fatalf("route-cancelled run = state %q finished_at %v", cancelled.State, cancelled.FinishedAt)
	}
}
