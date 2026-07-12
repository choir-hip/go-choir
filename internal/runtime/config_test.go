package runtime

import (
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestLoadConfigDefaultsResearcherCount(t *testing.T) {
	t.Setenv("SANDBOX_ID", "")
	t.Setenv("RUNTIME_STORE_PATH", "")
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "")
	t.Setenv("RUNTIME_SUPERVISION_INTERVAL", "")
	t.Setenv("RUNTIME_ACTIVATION_BUDGET", "")
	t.Setenv("RUNTIME_RESEARCHER_COUNT", "")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "")

	cfg := LoadConfig()
	if cfg.ResearcherCount != DefaultResearcherCount {
		t.Fatalf("researcher_count = %d, want %d", cfg.ResearcherCount, DefaultResearcherCount)
	}
	if cfg.ActivationBudget != DefaultActivationBudget {
		t.Fatalf("activation_budget = %s, want %s", cfg.ActivationBudget, DefaultActivationBudget)
	}
	if cfg.TextureActorParkIdle != DefaultTextureActorParkIdle {
		t.Fatalf("texture_actor_park_idle = %s, want %s", cfg.TextureActorParkIdle, DefaultTextureActorParkIdle)
	}
	if cfg.PromptRoot == "" {
		t.Fatal("prompt_root should not be empty")
	}
}

func TestLoadConfigReadsResearcherCount(t *testing.T) {
	t.Setenv("RUNTIME_RESEARCHER_COUNT", "5")
	t.Setenv("RUNTIME_SUPERVISION_INTERVAL", "7s")
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "3s")
	t.Setenv("RUNTIME_ACTIVATION_BUDGET", "90s")
	t.Setenv("RUNTIME_SKILLS_ROOT", "/tmp/choir-skills")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "45s")

	cfg := LoadConfig()
	if cfg.ResearcherCount != 5 {
		t.Fatalf("researcher_count = %d, want 5", cfg.ResearcherCount)
	}
	if cfg.TextureActorParkIdle != 45*time.Second {
		t.Fatalf("texture_actor_park_idle = %s, want 45s", cfg.TextureActorParkIdle)
	}
	if cfg.SupervisionInterval != 7*time.Second {
		t.Fatalf("supervision interval = %s, want 7s", cfg.SupervisionInterval)
	}
	if cfg.ProviderTimeout != 3*time.Second {
		t.Fatalf("provider timeout = %s, want 3s", cfg.ProviderTimeout)
	}
	if cfg.ActivationBudget != 90*time.Second {
		t.Fatalf("activation_budget = %s, want 90s", cfg.ActivationBudget)
	}
	if cfg.PromptRoot == "" {
		t.Fatal("prompt_root should not be empty")
	}
	if cfg.SkillsRoot != "/tmp/choir-skills" {
		t.Fatalf("skills_root = %q, want env value", cfg.SkillsRoot)
	}
}

func TestLoadConfigFallsBackOnInvalidResearcherCount(t *testing.T) {
	_ = os.Setenv("RUNTIME_RESEARCHER_COUNT", "-2")
	t.Cleanup(func() { _ = os.Unsetenv("RUNTIME_RESEARCHER_COUNT") })

	cfg := LoadConfig()
	if cfg.ResearcherCount != DefaultResearcherCount {
		t.Fatalf("researcher_count = %d, want fallback %d", cfg.ResearcherCount, DefaultResearcherCount)
	}
}

func TestLoadConfigReadsEnableTestAPIs(t *testing.T) {
	t.Setenv("RUNTIME_ENABLE_TEST_APIS", "true")

	cfg := LoadConfig()
	if !cfg.EnableTestAPIs {
		t.Fatal("enable_test_apis = false, want true")
	}
}

func TestLoadConfigDefaultsPromotionSourceRepoOutsideGitWorktree(t *testing.T) {
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", "")
	t.Setenv("RUNTIME_WORKER_REPO_REMOTE", "")
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	cfg := LoadConfig()
	if cfg.PromotionSourceRepo != DefaultPromotionSourceRepo {
		t.Fatalf("promotion source repo = %q, want %q", cfg.PromotionSourceRepo, DefaultPromotionSourceRepo)
	}
}

func TestLoadConfigReadsObscuraCDPScreenshots(t *testing.T) {
	t.Setenv("CHOIR_OBSCURA_CDP_SCREENSHOTS", "true")

	cfg := LoadConfig()
	if !cfg.ObscuraCDPScreenshots {
		t.Fatal("obscura_cdp_screenshots = false, want true")
	}
}

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

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/agent/loops?limit=20", "", "user-alice")
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

	foreignW := registeredRuntimeRequest(
		t,
		handler,
		http.MethodPost,
		"/api/agent/cancel",
		`{"loop_id":"`+bob.RunID+`"}`,
		"user-alice",
	)
	if foreignW.Code != http.StatusNotFound {
		t.Fatalf("foreign cancel status = %d, want 404; body=%s", foreignW.Code, foreignW.Body.String())
	}

	cancelW := registeredRuntimeRequest(
		t,
		handler,
		http.MethodPost,
		"/api/agent/cancel",
		`{"loop_id":"`+alice.RunID+`"}`,
		"user-alice",
	)
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
