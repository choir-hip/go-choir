package agentcore

// A provider quota park suspends the activation's progress deadline
// (2026-11-04 live8: management nudge loops parked on Synthetic until
// 05:20:42Z were cancelled at 05:04 by "activation budget exceeded"). Ways
// it could fail, written down for these tests:
//
//  D1 the deadline still fires during the park and the run is cancelled
//     before the reset (the live8 defect);
//  D2 the park is unbounded: a loop parks past the hard wall;
//  D3 the run ended at the wall says only "activation budget exceeded",
//     not that it waited for a quota;
//  D4 after the park the deadline is not charged again (a park is a free
//     extension of the run's own work);
//  D5 the durable deadline continuation closes a live parked run anyway.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type quotaThenOK struct {
	provideriface.Provider
	mu      sync.Mutex
	quotas  int
	retryAt time.Time
	calls   int
}

func (p *quotaThenOK) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.calls <= p.quotas {
		return nil, &provideriface.ProviderQuotaError{Provider: "synthetic", Model: req.Model, Reason: "provider quota exhausted (synthetic)",
			Message: "synthetic: quota exhausted", RetryAt: p.retryAt}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done on " + req.Provider + "/" + req.Model}, nil
}

func withHardWall(t *testing.T, d time.Duration) {
	t.Helper()
	prev := activationHardWall
	activationHardWall = d
	t.Cleanup(func() { activationHardWall = prev })
}

func runParkedLoop(ctx context.Context, rt *Runtime, p *quotaThenOK) (string, error) {
	rec := &types.RunRecord{RunID: "run-park", OwnerID: "o", ComputerID: "c", AgentID: "management:o"}
	text, _, err := toolregistry.RunToolLoop(ctx, p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, nil,
		toolregistry.WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "synthetic", Model: "m"}),
		toolregistry.WithProviderWait(rt.providerQuotaWaitHook(rec)))
	return text, err
}

func TestQuotaParkSuspendsTheActivationDeadline(t *testing.T) {
	withHardWall(t, time.Hour)
	rt := &Runtime{}
	ctx, cancel := newActivationDeadline(context.Background(), 300*time.Millisecond)
	defer cancel()
	before, _ := ctx.Deadline()
	// The park's minimum wait (5 s) outlasts the 300 ms budget many times.
	p := &quotaThenOK{quotas: 1, retryAt: time.Now().Add(time.Second)}
	started := time.Now()
	text, err := runParkedLoop(ctx, rt, p)
	if err != nil || text != "done on synthetic/m" {
		t.Fatalf("D1: run = %q, %v; want it to outlive its budget while parked and finish", text, err)
	}
	parked := time.Since(started)
	after, _ := ctx.Deadline()
	if moved := after.Sub(before); moved < parked-200*time.Millisecond {
		t.Fatalf("D4: deadline moved %s for a %s park", moved, parked)
	}
	if ctx.Err() != nil {
		t.Fatalf("the activation ended: %v", ctx.Err())
	}
	// D4: after the park the deadline is charged again.
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("D4: the deadline never fired after the park")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || !strings.Contains(activationFate(ctx), "activation budget exceeded") {
		t.Fatalf("D4: err %v fate %q", ctx.Err(), activationFate(ctx))
	}
}

func TestQuotaParkEndsAtTheHardWallWithAVisibleFate(t *testing.T) {
	withHardWall(t, 600*time.Millisecond)
	rt := &Runtime{}
	ctx, cancel := newActivationDeadline(context.Background(), 100*time.Millisecond)
	defer cancel()
	reset := time.Now().Add(time.Hour)
	p := &quotaThenOK{quotas: 1000, retryAt: reset}
	started := time.Now()
	_, err := runParkedLoop(ctx, rt, p)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("D2: err = %v, want the hard wall", err)
	}
	if took := time.Since(started); took < 500*time.Millisecond || took > 3*time.Second {
		t.Fatalf("D2: ended after %s, want at the 600 ms wall", took)
	}
	fate := activationFate(ctx)
	if !strings.Contains(fate, "activation hard wall reached") || !strings.Contains(fate, "Synthetic quota") || !strings.Contains(fate, reset.UTC().Format(time.RFC3339)) {
		t.Fatalf("D3: fate = %q", fate)
	}
	if views := rt.providerWaitViews("o", "c"); len(views) != 0 {
		t.Fatalf("the ended park is still shown: %+v", views)
	}
}

func TestLiveParkedActivationIsNotClosedByTheDurableDeadline(t *testing.T) {
	withHardWall(t, time.Hour)
	ctx, cancel := newActivationDeadline(context.Background(), 50*time.Millisecond)
	defer cancel()
	ctx.suspend(types.NewProviderQuotaWait("run", "synthetic", "m", time.Now(), time.Now().Add(time.Minute), false, time.Minute, ""))
	time.Sleep(100 * time.Millisecond)
	at, over, moved := ctx.endsAt()
	if over || !moved || !time.Now().Before(at) {
		t.Fatalf("D5: a parked activation reads as over (%v, ends %s)", over, at)
	}
	ctx.resume()
	if d, _ := ctx.Deadline(); !d.After(time.Now()) {
		t.Fatalf("D4: the deadline after a 100 ms park is %s, in the past", d)
	}
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancel: %v", ctx.Err())
	}
}

// A park re-reads the run's model from the computer's policy file: a role
// moved to another provider is seen; an unchanged file is not a change.
func TestModelReselectReadsTheCurrentPolicy(t *testing.T) {
	path := t.TempDir() + "/model-policy.toml"
	write := func(provider, model string) {
		body := "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.management]\nprovider = \"" + provider + "\"\nmodel = \"" + model + "\"\nreasoning = \"high\"\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017")
	rt := &Runtime{modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{PolicyPath: path})}
	meta := map[string]any{runMetadataAgentRole: "management", "trajectory_id": "traj-1"}
	meta = rt.modelPolicy.EnrichMetadata(context.Background(), "o", "management", meta)
	if got := provideriface.ResolvedLLMConfigFromMetadata(meta); got.Provider != "synthetic" {
		t.Fatalf("start: %+v (%v)", got, meta)
	}
	rec := &types.RunRecord{RunID: "r", OwnerID: "o", Metadata: meta}
	reselect := rt.modelReselectForRun(rec)
	if reselect == nil {
		t.Fatal("no reselect for a policy-resolved run")
	}
	if _, _, ok := reselect(context.Background()); ok {
		t.Fatal("an unchanged policy read as a change")
	}
	write("openrouter", "deepseek/deepseek-v4.1-flash-1017")
	next, _, ok := reselect(context.Background())
	if !ok || next.Provider != "openrouter" || next.Model != "deepseek/deepseek-v4.1-flash-1017" {
		t.Fatalf("after the policy moved: %+v ok=%v", next, ok)
	}
	explicit := &types.RunRecord{RunID: "x", OwnerID: "o", Metadata: map[string]any{"llm_provider": "synthetic", "llm_model": "m"}}
	if rt.modelReselectForRun(explicit) != nil {
		t.Fatal("a caller's explicit model is re-read from the policy")
	}
}

// Owner rule: a pinned model fails rather than falls back. A park moves a
// run only when the policy file was read and parsed and names another
// model for the role; a missing, unreadable or invalid file, or an unknown
// pool, keeps the run on its model (it never moves to the platform default).
func TestModelReselectKeepsTheModelWhenThePolicyIsNotUsable(t *testing.T) {
	path := t.TempDir() + "/model-policy.toml"
	write := func(body string) {
		_ = os.Chmod(path, 0o600)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	role := func(provider, model string) string {
		return "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.management]\nprovider = \"" + provider + "\"\nmodel = \"" + model + "\"\n"
	}
	write(role("synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"))
	rt := &Runtime{modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{PolicyPath: path})}
	meta := rt.modelPolicy.EnrichMetadata(context.Background(), "o", "management", map[string]any{runMetadataAgentRole: "management", "trajectory_id": "traj-1"})
	reselect := rt.modelReselectForRun(&types.RunRecord{RunID: "r", OwnerID: "o", Metadata: meta})

	for name, breakIt := range map[string]func(){
		"not found":    func() { _ = os.Remove(path) },
		"permission":   func() { write(role("openrouter", "deepseek/deepseek-v4.1-flash-1017")); _ = os.Chmod(path, 0o000) },
		"parse error":  func() { write("[roles.management\nmodel = \"openrouter/x\"\n") },
		"unknown pool": func() { write("# owner-edited\n[roles.management]\nmodel = \"pool:no-such-pool\"\n") },
	} {
		breakIt()
		if next, _, ok := reselect(context.Background()); ok {
			t.Fatalf("%s: the park moved the run to %+v", name, next)
		}
		if _, err := os.Stat(path); name == "not found" && err == nil {
			t.Fatalf("%s: the re-read wrote a default policy file", name)
		}
	}
	write(role("openrouter", "deepseek/deepseek-v4.1-flash-1017"))
	if next, _, ok := reselect(context.Background()); !ok || next.Provider != "openrouter" {
		t.Fatalf("a readable policy naming another model: %+v ok=%v", next, ok)
	}
}
