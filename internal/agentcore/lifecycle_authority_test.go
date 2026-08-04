package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
func TestLegacySupervisionWriteRefusalFailsClosedWhenSupervisionWritesDisabled(t *testing.T) {
	t.Setenv("CHOIR_SUPERVISION_WRITES_DISABLED", "1")

	tests := []struct {
		name string
		rt   func(*testing.T) *Runtime
	}{
		{
			name: "no runtime or trajectory state",
			rt:   func(*testing.T) *Runtime { return nil },
		},
		{
			name: "new trajectory without legacy or supervision state",
			rt: func(t *testing.T) *Runtime {
				rt, _ := testRuntime(t)
				return rt
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rt(t).refuseLegacySupervisionWrite(context.Background(), "owner-new", "computer-new", "trajectory-new", "legacy write")
			if !errors.Is(err, ErrSupervisionAuthorityRequired) {
				t.Fatalf("refusal error = %v, want ErrSupervisionAuthorityRequired", err)
			}
			if !errors.Is(err, computerevent.ErrSupervisionWritesDisabled) {
				t.Fatalf("refusal error = %v, want ErrSupervisionWritesDisabled", err)
			}
		})
	}
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

	foreignW := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+bob.RunID+"/cancel", ``, "user-alice")
	if foreignW.Code != http.StatusNotFound {
		t.Fatalf("foreign cancel status = %d, want 404; body=%s", foreignW.Code, foreignW.Body.String())
	}

	cancelW := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+alice.RunID+"/cancel", ``, "user-alice")
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

func TestRuntimeRunListsIncludeLifecycleProjectionWithinComputerScope(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-lifecycle-list"
		docID   = "doc-lifecycle-list"
		runID   = "run-lifecycle-list"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: runID, AgentID: currentTextureAgentID(docID), OwnerID: ownerID,
		SandboxID: "sandbox-test", TrajectoryID: trajectoryID, ChannelID: docID,
		State: types.RunRunning, Prompt: "list canonical lifecycle activation",
		AgentProfile: "texture", AgentRole: "texture", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: "texture", runMetadataAgentRole: "texture",
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create lifecycle activation: %v", err)
	}

	assertListed := func(label string, runs []types.RunRecord, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		for _, run := range runs {
			if run.RunID == runID {
				return
			}
		}
		t.Fatalf("%s omitted lifecycle run %q: %+v", label, runID, runs)
	}
	runs, err := rt.ListRunsByOwner(ctx, ownerID, 10)
	assertListed("list by owner", runs, err)
	runs, err = rt.ListRunsByChannel(ctx, ownerID, docID, 10)
	assertListed("list by channel", runs, err)
}
