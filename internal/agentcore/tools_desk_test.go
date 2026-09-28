package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// desk_go_eval (R3b): a non-capsule desk's only tool evaluates model-authored
// Go inside a killable host session worker — never the daemon's address space.
// This proves the carrier's substrate: spawn the worker, run a cell, observe
// the model-visible stdout, and confirm the worker persists across cells. The
// canonical-ledger half is proven by rlmReductionForDeskCall sharing the same
// commitTray path the capsule carrier uses.
func deskEvalExecCtx(t *testing.T) toolregistry.ExecutionContext {
	t.Helper()
	return toolregistry.ExecutionContext{
		RunID:      "run-desk-eval",
		AgentID:    "agent-mgmt",
		OwnerID:    "owner-1",
		Profile:    agentprofile.Management,
		Role:       agentprofile.Management,
		ChannelID:  "channel-mgmt",
		ComputerID: "host-test",
		WorkingDir: t.TempDir(),
	}
}

// deskTestWorkerBin lazily compiles a real autoputer once so the session
// worker re-executes the true desk-session entrypoint, not the test harness's
// own os.Executable.
var (
	deskTestWorkerBinOnce sync.Once
	deskTestWorkerBinErr  error
)

func deskTestWorkerBin(t *testing.T) {
	t.Helper()
	deskTestWorkerBinOnce.Do(func() {
		bin := filepath.Join(os.TempDir(), "choir-autoputer-desk-worker")
		cmd := exec.Command("go", "build", "-o", bin, "../../cmd/autoputer")
		if out, err := cmd.CombinedOutput(); err != nil {
			deskTestWorkerBinErr = fmt.Errorf("build desk worker bin: %v\n%s", err, out)
			return
		}
		deskWorkerBinOverride = bin
	})
	if deskTestWorkerBinErr != nil {
		t.Fatalf("%v", deskTestWorkerBinErr)
	}
}

func TestDeskGoEvalSpawnsWorkerAndEvals(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"print(40+2);","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("desk_go_eval stdout missing 42, got: %s", out)
	}
}

// Kill mid-cell → derivable wake (R3b): a worker killed mid-cell is poisoned
// — its staged tray dies with it and the cell reduces nothing — and the next
// eval spawns a fresh worker on the same activation rather than reusing the
// corpse. The reduction is derivable: only intents from a successful cell
// ever reach the ledger, so killing mid-cell cannot mint a half-commit.
func TestDeskGoEvalKillMidCellRespawnsClean(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))
	// Establish a live worker.
	if _, err := tool.Func(ctx, json.RawMessage(`{"source":"y := 3;","timeout_ms":15000}`)); err != nil {
		t.Fatalf("seed eval: %v", err)
	}
	// Kill the activation's worker as if the cell timed out / poisoned.
	activationID := deskWorkerActivationID(deskEvalExecCtx(t))
	workers.release(activationID)
	// Next eval on the same activation spawns a fresh worker and runs clean;
	// the killed cell's bindings do not leak (y is fresh in the new worker).
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"print(6*7);","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("post-kill eval must respawn and succeed: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("post-kill eval missing 42, got: %s", out)
	}
}

func TestDeskGoEvalWorkerPersistsAcrossCells(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))
	_, err := tool.Func(ctx, json.RawMessage(`{"source":"x := 7;","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("first eval: %v", err)
	}
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"print(x*6);","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("second eval: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("worker did not persist x across cells, got: %s", out)
	}
}

func TestDeskGoEvalRejectsEmptySource(t *testing.T) {
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))
	if _, err := tool.Func(ctx, json.RawMessage(`{"source":"  "}`)); err == nil {
		t.Fatal("empty source must be rejected")
	}
}

// ApplyTexture accepts the Go-native argument shape a cell authors. Staging's
// producer failure — cells calling choir.ApplyTexture with a map literal
// instead of a JSON string (map literal is the natural Go shape; the string
// signature rejected it as a compile error, so no intent staged and the
// required-write retry looped forever, per the M11 guest run-memory pull) —
// is prevented by the verb taking any and marshaling at the seam.
func TestDeskGoEvalApplyTextureAcceptsGoValue(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Texture)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Texture
	execCtx.Role = agentprofile.Texture
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"import \"choir\"\nedit := map[string]interface{}{\n  \"op\":\"apply\",\n  \"base_revision_id\":\"rev-1\",\n  \"content\":\"new head\",\n}\nres, err2 := choir.ApplyTexture(edit)\nif err2 != nil { print(\"apply_err: \", err2) } else { print(\"staged \", res) }","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if !strings.Contains(out, "staged  tray-") {
		t.Fatalf("map-literal ApplyTexture did not stage an intent, got: %s", out)
	}
	if strings.Contains(out, "\"intents\":0") {
		t.Fatalf("no intent staged from map-literal ApplyTexture, got: %s", out)
	}
}

// The texture desk's required-write gate fires on an rlm:texture_apply
// receipt — which exists only when the staged intent commits through the
// bound authorizer. The M11 guest run-memory pull showed this exact chain
// broken: the cell's map-literal arg type-errored, no intent staged, no
// receipt minted, and the update_coagent wake redelivered forever.
type stubCellAuthorizer struct {
	seen []string
}

func (a *stubCellAuthorizer) CommitCellTextureAuthor(_ context.Context, _ *types.RunRecord, bodyJSON, _ string) (string, error) {
	a.seen = append(a.seen, bodyJSON)
	return `{"op":"apply","doc_id":"doc-1","revision_id":"rev-2"}`, nil
}

func TestDeskGoEvalApplyTextureCommitsReceipt(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	auth := &stubCellAuthorizer{}
	rt.SetTextureCellAuthorizer(auth)
	rec := types.RunRecord{
		RunID:        "run-texture-apply",
		AgentID:      "agent-texture",
		OwnerID:      "owner-1",
		ComputerID:   "autoputer-test",
		ChannelID:    "channel-texture",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		State:        types.RunRunning,
	}
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: rec.OwnerID,
		ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &rec,
	}
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Texture)
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"import \"choir\"\nchoir.ApplyTexture(map[string]interface{}{\n  \"op\":\"apply\",\n  \"base_revision_id\":\"rev-1\",\n  \"content\":\"new head\",\n})","timeout_ms":15000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if !strings.Contains(out, "rlm:texture_apply:") {
		t.Fatalf("committed apply missing rlm:texture_apply receipt, got: %s", out)
	}
	if len(auth.seen) != 1 || !strings.Contains(auth.seen[0], `"op":"apply"`) || !strings.Contains(auth.seen[0], `"base_revision_id":"rev-1"`) {
		t.Fatalf("authorizer did not receive the marshaled map literal: %v", auth.seen)
	}
}

// Reject: unknown desk targets fail the reducer's cast validation, not just
// the non-empty check — R3b's canonical-desk gate before the ledger mints.
func TestValidateSemanticActRejectsUnknownDesk(t *testing.T) {
	err := validateSemanticActIntent(yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentCast, ToDesk: "not-a-real-desk", Objective: "do thing", LocalID: "c1",
	})
	if err == nil {
		t.Fatal("cast to unknown desk must be rejected")
	}
	if err := validateSemanticActIntent(yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentCast, ToDesk: agentprofile.Research, Objective: "do thing", LocalID: "c1",
	}); err != nil {
		t.Fatalf("cast to known desk %q should pass: %v", agentprofile.Research, err)
	}
}
