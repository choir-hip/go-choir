package modelpolicy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// scriptedVerifyProvider answers each call by its position: errs[i] is call
// i's error (nil answers). Calls beyond the script answer.
type scriptedVerifyProvider struct {
	errs   []error
	models []string
}

func (p *scriptedVerifyProvider) Execute(context.Context, *types.RunRecord, provideriface.EventEmitFunc) error {
	return nil
}
func (p *scriptedVerifyProvider) ProviderName() string { return "scripted" }
func (p *scriptedVerifyProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	i := len(p.models)
	p.models = append(p.models, req.Model)
	if i < len(p.errs) && p.errs[i] != nil {
		return nil, p.errs[i]
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "ok", Model: req.Model}, nil
}

func failoverManager(t *testing.T, provider provideriface.ToolLoopProvider, pool string) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:"+pool+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewManager(ManagerConfig{PolicyPath: path, Provider: provider})
}

var (
	errVerifyQuota   = errors.New("gateway client: provider openrouter model x: provider quota exhausted (openrouter); resets at later (circuit open)")
	errVerifyCircuit = errors.New("gateway client: provider openrouter model x: circuit open (model unhealthy)")
	errVerifyBad     = errors.New("gateway client: 400 Bad Request: malformed prompt")
)

func verifyArgs(image bool) verifyModelCapabilityArgs {
	a := verifyModelCapabilityArgs{Role: "verifier", Prompt: "check"}
	if image {
		a.ImageFixture = "red_pixel_png"
	}
	return a
}

func TestVerifyFailsOverOnQuotaOfFirstMember(t *testing.T) {
	p := &scriptedVerifyProvider{errs: []error{errVerifyQuota}}
	out, err := failoverManager(t, p, "default").runVerify(verifierContext(), verifyArgs(false))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(p.models) != 2 || p.models[0] == p.models[1] {
		t.Fatalf("models tried = %v, want two distinct", p.models)
	}
	// The outcome names the member that answered, not the first.
	if out.Model != p.models[1] || out.ResponseModel != p.models[1] {
		t.Fatalf("outcome model = %q/%q, want %q", out.Model, out.ResponseModel, p.models[1])
	}
}

func TestVerifyFailsOverOnCircuitOpen(t *testing.T) {
	p := &scriptedVerifyProvider{errs: []error{errVerifyCircuit, errVerifyQuota}}
	out, err := failoverManager(t, p, "default").runVerify(verifierContext(), verifyArgs(false))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(p.models) != 3 || out.Model != p.models[2] {
		t.Fatalf("models=%v outcome=%q", p.models, out.Model)
	}
}

func TestVerifySkipsTextOnlyMemberForImageCall(t *testing.T) {
	old := verifyImageRefusal
	t.Cleanup(func() { verifyImageRefusal = old })
	refused := map[string]bool{}
	verifyImageRefusal = func(model string) string {
		if refused[model] {
			return "text only"
		}
		return ""
	}
	p := &positionalProvider{refused: refused}
	if _, err := failoverManager(t, p, "default").runVerify(verifierContext(), verifyArgs(true)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, called := range p.models[1:] {
		if refused[called] {
			t.Fatalf("text-only member %q was sent an image call", called)
		}
	}
	if len(p.models) != 2 {
		t.Fatalf("models = %v, want the quota member then the first image-capable one", p.models)
	}
}

// positionalProvider fails the first call with a quota and, on it, marks every
// pool member but the primary and the pool's last as text-only. The pool order
// depends on a time-based key, so the refusal is set by what was called first.
type positionalProvider struct {
	refused map[string]bool
	models  []string
}

func (p *positionalProvider) Execute(context.Context, *types.RunRecord, provideriface.EventEmitFunc) error {
	return nil
}
func (p *positionalProvider) ProviderName() string { return "positional" }
func (p *positionalProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.models = append(p.models, req.Model)
	if len(p.models) == 1 {
		// Everything except the primary and the pool's last member is text-only.
		members := modelcatalog.Pools()[modelcatalog.DefaultPool]
		for _, mem := range members {
			if mem.Model != req.Model && mem.Model != members[len(members)-1].Model {
				p.refused[mem.Model] = true
			}
		}
		return nil, errVerifyQuota
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "ok", Model: req.Model}, nil
}

func TestVerifyEveryMemberExhaustedNamesEach(t *testing.T) {
	n := len(modelcatalog.Pools()[modelcatalog.DefaultPool])
	errs := make([]error, n)
	for i := range errs {
		errs[i] = errVerifyQuota
	}
	p := &scriptedVerifyProvider{errs: errs}
	_, err := failoverManager(t, p, "default").runVerify(verifierContext(), verifyArgs(false))
	if err == nil {
		t.Fatal("want an error")
	}
	if len(p.models) != n {
		t.Fatalf("tried %d, want %d", len(p.models), n)
	}
	for _, model := range p.models {
		if !strings.Contains(err.Error(), model) {
			t.Fatalf("error does not name %q: %v", model, err)
		}
	}
	var mce *ModelCallError
	if !errors.As(err, &mce) {
		t.Fatalf("err %T, want ModelCallError", err)
	}
	if _, ok := provideriface.AsProviderQuota(err); ok {
		t.Log("typed quota preserved")
	}
}

func TestVerifyNoFailoverPoolTriesOnce(t *testing.T) {
	p := &scriptedVerifyProvider{errs: []error{errVerifyQuota}}
	_, err := failoverManager(t, p, modelcatalog.SyntheticLunaPool).runVerify(verifierContext(), verifyArgs(false))
	if err == nil || len(p.models) != 1 {
		t.Fatalf("err=%v models=%v, want one failed call", err, p.models)
	}
}

func TestVerifyNonTransientErrorDoesNotFailOver(t *testing.T) {
	p := &scriptedVerifyProvider{errs: []error{errVerifyBad}}
	_, err := failoverManager(t, p, "default").runVerify(verifierContext(), verifyArgs(false))
	if err == nil || len(p.models) != 1 {
		t.Fatalf("err=%v models=%v, want one failed call", err, p.models)
	}
	if !strings.Contains(err.Error(), "malformed prompt") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyPinnedModelDoesNotFailOver(t *testing.T) {
	p := &scriptedVerifyProvider{errs: []error{errVerifyQuota}}
	a := verifyArgs(false)
	a.Provider, a.Model = "opencode-go", "deepseek-v4.1-flash"
	_, err := NewManager(ManagerConfig{Provider: p}).runVerify(verifierContext(), a)
	if err == nil || len(p.models) != 1 {
		t.Fatalf("err=%v models=%v", err, p.models)
	}
}

// The caller's deadline ending during a member's call is not that member's
// failure: the loop stops instead of calling the rest on a dead context
// (Devin review 1).
func TestVerifyDoesNotFailOverOnTheCallersDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(verifierContext())
	p := &cancelingVerifyProvider{cancel: cancel}
	if _, err := failoverManager(t, p, "default").runVerify(ctx, verifyArgs(false)); err == nil {
		t.Fatal("a call on a dead context answered")
	}
	if p.calls != 1 {
		t.Fatalf("calls = %d, want 1", p.calls)
	}
}

type cancelingVerifyProvider struct {
	cancel context.CancelFunc
	calls  int
}

func (p *cancelingVerifyProvider) Execute(context.Context, *types.RunRecord, provideriface.EventEmitFunc) error {
	return nil
}
func (p *cancelingVerifyProvider) ProviderName() string { return "canceling" }
func (p *cancelingVerifyProvider) CallWithTools(context.Context, provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.calls++
	p.cancel()
	return nil, context.DeadlineExceeded
}
