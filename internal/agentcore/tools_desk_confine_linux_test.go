//go:build linux

package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// TestDeskGoEvalWorkerIsConfined: desk_go_eval's worker is the real
// `autoputer desk-session` entrypoint, and it serves cells only after
// confining itself (F4). Ways it could fail: the entrypoint never calls the
// confinement (Seccomp 0, NoNewPrivs 0); a root runtime's worker keeps uid 0;
// the confinement breaks the cell.
func TestDeskGoEvalWorkerIsConfined(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	defer workers.closeAll()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"print(6*7);","timeout_ms":15000}`))
	if err != nil || !strings.Contains(out, "42") {
		t.Fatalf("desk_go_eval: err=%v out=%s", err, out)
	}
	workers.mu.Lock()
	pids := []int{}
	for _, s := range workers.sessions {
		pids = append(pids, s.worker.PID())
	}
	workers.mu.Unlock()
	if len(pids) != 1 {
		t.Fatalf("live desk sessions = %d, want 1", len(pids))
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pids[0]))
	if err != nil {
		t.Fatalf("worker status: %v", err)
	}
	status := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			status[k] = strings.TrimSpace(v)
		}
	}
	if status["NoNewPrivs"] != "1" || status["Seccomp"] != "2" {
		t.Fatalf("desk worker NoNewPrivs=%q Seccomp=%q, want 1 and 2", status["NoNewPrivs"], status["Seccomp"])
	}
	if os.Geteuid() != 0 {
		return
	}
	cell, err := user.Lookup("choir-cell-management")
	if err != nil {
		t.Fatalf("a root runtime's desk worker spawned without its cell user existing: %v", err)
	}
	if uid := strings.Fields(status["Uid"]); len(uid) != 4 || uid[0] != cell.Uid || uid[1] != cell.Uid {
		t.Fatalf("desk worker Uid = %q, want choir-cell-management (%s)", status["Uid"], cell.Uid)
	}
	if strings.Trim(status["CapEff"], "0") != "" {
		t.Fatalf("desk worker CapEff = %s, want 0", status["CapEff"])
	}
}
