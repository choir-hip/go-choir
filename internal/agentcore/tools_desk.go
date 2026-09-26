package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// desk_go_eval (R3b): the host desk-cell carrier's sole tool for a non-capsule
// desk under actuator=rlm. It evaluates model-authored Go source in a killable
// host session worker — never in the daemon's address space — and reduces the
// cell's staged intents through the same ledger path capsule_go_eval uses.
// The worker is re-executed from the daemon binary (autoputer desk-session);
// SpawnDeskSessionWorker supplies the socketpair + process-group hardening.

const deskWorkerArg = "desk-session"

// deskSessionWorkers owns one host session worker per desk activation. A
// worker is lazily spawned on first eval and persists for the activation; a
// dead/poisoned worker is dropped and respawned on the next eval.
type deskSessionWorkers struct {
	mu      sync.Mutex
	workers map[string]*yaegikernel.DeskSessionWorker
}

func newDeskSessionWorkers() *deskSessionWorkers {
	return &deskSessionWorkers{workers: map[string]*yaegikernel.DeskSessionWorker{}}
}

// deskSessionWorkers returns the Runtime's shared worker pool, lazily
// allocated so desk cells never spawn a worker before one is requested.
func (rt *Runtime) deskSessionWorkers() *deskSessionWorkers {
	rt.deskWorkersMu.Lock()
	defer rt.deskWorkersMu.Unlock()
	if rt.deskWorkers == nil {
		rt.deskWorkers = newDeskSessionWorkers()
	}
	return rt.deskWorkers
}

// deskWorkerFor returns (or spawns) the activation's host session worker.
// bin is the daemon's own executable; cfg carries the desk identity/bounds.
func (m *deskSessionWorkers) deskWorkerFor(activationID string, cfg yaegikernel.DeskSessionWorkerConfig) (*yaegikernel.DeskSessionWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.workers == nil {
		m.workers = map[string]*yaegikernel.DeskSessionWorker{}
	}
	if w := m.workers[activationID]; w != nil && !w.Dead() {
		return w, nil
	}
	if old := m.workers[activationID]; old != nil {
		old.Close()
	}
	w, err := yaegikernel.SpawnDeskSessionWorker(cfg)
	if err != nil {
		delete(m.workers, activationID)
		return nil, err
	}
	m.workers[activationID] = w
	return w, nil
}

// release terminates and forgets the activation's worker.
func (m *deskSessionWorkers) release(activationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.workers[activationID]; w != nil {
		w.Close()
		delete(m.workers, activationID)
	}
}

// newDeskGoEvalTool evaluates model-authored Go source for a non-capsule desk
// inside a dedicated host session worker. It mirrors newCapsuleGoEvalTool but
// carries no capsule obligation: the desk's authority is its profile/role, and
// the containment boundary is the subprocess, not a guest VM. Staged intents
// reduce through rlmReductionForDeskCall onto the canonical ledger.
func newDeskGoEvalTool(rt *Runtime, workers *deskSessionWorkers, deskRole string) toolregistry.Tool {
	type args struct {
		Source    string `json:"source"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{
		Name:        "desk_go_eval",
		Description: "Evaluate model-authored Go source for this desk in a killable host subprocess (restricted stdlib, per-role choir verbs).",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"source":     map[string]any{"type": "string", "description": "Raw Go source for one REPL cell; stage desk acts with choir.* verbs."},
			"timeout_ms": map[string]any{"type": "integer", "description": "Evaluation timeout in milliseconds."},
		}, []string{"source"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			execCtx := toolregistry.ExecutionContextFrom(ctx)
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if strings.TrimSpace(input.Source) == "" {
				return "", fmt.Errorf("desk_go_eval: source is required")
			}
			activationID := deskWorkerActivationID(execCtx)
			if activationID == "" {
				return "", fmt.Errorf("desk_go_eval: no activation identity for desk worker")
			}
			workerCfg := yaegikernel.DeskSessionWorkerConfig{
				Bin:       deskWorkerBinary(),
				WorkerArg: deskWorkerArg,
				Session: yaegikernel.SessionWorkerConfig{
					AllowedPackages: yaegikernel.DefaultSafeStdlibPackagesList(),
					ComputerID:      execCtx.ComputerID,
					ActivationID:    activationID,
					Epoch:           deskWorkerEpoch(execCtx),
					AllowedRoot:     deskWorkerRoot(execCtx),
					Role:            deskRole,
				},
				ProcessGroup: true,
			}
			w, err := workers.deskWorkerFor(activationID, workerCfg)
			if err != nil {
				return "", fmt.Errorf("desk_go_eval: spawn worker: %w", err)
			}
			reduction := rlmReductionForDeskCall(ctx, rt)
			evalCtx := ctx
			if input.TimeoutMS > 0 {
				var cancel context.CancelFunc
				evalCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMS)*time.Millisecond)
				defer cancel()
			}
			res, evalErr := w.Eval(evalCtx, input.Source)
			result := yaegikernel.SessionResult{}
			if evalErr != nil {
				if w.Dead() {
					workers.release(activationID)
				}
				result.Error = evalErr.Error()
			} else {
				result = res
			}
			if reduction.active && result.Error == "" {
				if rerr := reduction.commit(ctx, result.Intents); rerr != nil {
					return "", rerr
				}
				for _, in := range reduction.receipt.Intents {
					result.Receipts = append(result.Receipts, fmt.Sprintf("rlm:%s:%d", in.Kind, in.Seq))
				}
			}
			out, _ := json.Marshal(map[string]any{
				"stdout":   result.Stdout,
				"stderr":   result.Stderr,
				"error":    result.Error,
				"duration": result.DurationMs,
				"reuse":    result.Reuse,
				"diag":     result.DiagKind,
				"receipts": result.Receipts,
				"intents":  len(result.Intents),
			})
			return string(out), nil
		},
	}
}

// deskWorkerBinary is the executable the session worker re-executes as
// `autoputer desk-session` — normally the daemon's own os.Executable. The
// override lets tests point at a real compiled autoputer instead of the test
// harness binary.
var deskWorkerBinOverride string

func deskWorkerBinary() string {
	if deskWorkerBinOverride != "" {
		return deskWorkerBinOverride
	}
	if bin, err := os.Executable(); err == nil {
		return bin
	}
	return "autoputer"
}

// deskWorkerActivationID identifies the desk activation a worker serves. Run
// identity is the stable key; the run's record supplies it when the tool
// context lacks an explicit activation field.
func deskWorkerActivationID(execCtx toolregistry.ExecutionContext) string {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.RunID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(execCtx.RunID)
}

// deskWorkerEpoch returns the handle-issuer epoch the worker fences on. For a
// non-capsule desk there is no capsule epoch; a positive constant satisfies
// the issuer's epoch>0 constraint. Desk epochs gain a canonical source when
// R3c binds desk activations to a lifecycle version.
func deskWorkerEpoch(execCtx toolregistry.ExecutionContext) uint64 {
	return 1
}

// deskWorkerRoot bounds the worker's filesystem root. Desks without file
// verbs get a scratch dir; the allowed root is the run's working dir.
func deskWorkerRoot(execCtx toolregistry.ExecutionContext) string {
	if wd := strings.TrimSpace(execCtx.WorkingDir); wd != "" {
		return wd
	}
	return os.TempDir()
}
