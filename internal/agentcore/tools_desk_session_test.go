package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// deskRunCtx is a desk cell's context for one run of one agent.
func deskRunCtx(t *testing.T, root, agentID, runID string) context.Context {
	t.Helper()
	rec := &types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: "owner-1", ComputerID: "host-test",
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management}
	return toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: runID, AgentID: agentID, OwnerID: "owner-1", ComputerID: "host-test",
		Profile: agentprofile.Management, Role: agentprofile.Management,
		WorkingDir: root, RunRecord: rec,
	})
}

type deskCellOut struct {
	Stdout   string   `json:"stdout"`
	Error    string   `json:"error"`
	Session  string   `json:"session"`
	Searches []string `json:"searches"`
}

func evalDeskCell(t *testing.T, tool toolregistry.Tool, ctx context.Context, source string) deskCellOut {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"source": source, "timeout_ms": 15000})
	out, err := tool.Func(ctx, raw)
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	var cell deskCellOut
	if err := json.Unmarshal([]byte(out), &cell); err != nil {
		t.Fatalf("decode cell result %s: %v", out, err)
	}
	return cell
}

// PD2: a desk's session outlives its run. The next run of the same agent
// reads what the earlier run defined and is told the session continues; a
// different agent, a run with no run record (a choir.Research child) and a
// session idle past retention all start empty and are told so.
func TestDeskSessionContinuesAcrossRunsOfOneAgent(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	clock := time.Now()
	workers.now = func() time.Time { return clock }
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	root := t.TempDir()

	first := evalDeskCell(t, tool, deskRunCtx(t, root, "research:doc-1", "run-a"), "x := 7;")
	if first.Error != "" || !strings.HasPrefix(first.Session, "new session") {
		t.Fatalf("first run must start a new session and say so: %+v", first)
	}
	again := evalDeskCell(t, tool, deskRunCtx(t, root, "research:doc-1", "run-a"), "print(x);")
	if again.Session != "" {
		t.Fatalf("only a run's first cell carries the session line, got %q", again.Session)
	}

	follow := evalDeskCell(t, tool, deskRunCtx(t, root, "research:doc-1", "run-b"), "print(x*6);")
	if follow.Error != "" || !strings.Contains(follow.Stdout, "42") {
		t.Fatalf("a later run of the same desk must read the earlier run's value: %+v", follow)
	}
	if !strings.HasPrefix(follow.Session, "continuing session") || !strings.Contains(follow.Session, "2 cells") {
		t.Fatalf("a later run must be told its session continues: %q", follow.Session)
	}

	other := evalDeskCell(t, tool, deskRunCtx(t, root, "research:doc-2", "run-c"), "print(x);")
	if other.Error == "" || !strings.HasPrefix(other.Session, "new session") {
		t.Fatalf("another desk must not see this desk's values: %+v", other)
	}

	childExec := toolregistry.ExecutionContextFrom(deskRunCtx(t, root, "research:doc-1", "run-b"))
	childExec.RunRecord = nil
	childExec.RunID = "run-b"
	child := evalDeskCell(t, tool, toolregistry.WithExecutionContext(context.Background(), childExec), "print(x);")
	if child.Error == "" {
		t.Fatalf("a run without a run record must not join the agent's session: %+v", child)
	}

	clock = clock.Add(deskSessionRetention + time.Minute)
	late := evalDeskCell(t, tool, deskRunCtx(t, root, "research:doc-1", "run-d"), "print(x);")
	if late.Error == "" || !strings.HasPrefix(late.Session, "new session") {
		t.Fatalf("a session idle past retention must be reclaimed and the next run told: %+v", late)
	}
}

func deskPoolCfg(t *testing.T) func(run string) yaegikernel.DeskSessionWorkerConfig {
	t.Helper()
	return func(run string) yaegikernel.DeskSessionWorkerConfig {
		return yaegikernel.DeskSessionWorkerConfig{
			Bin: deskWorkerBinary(), WorkerArg: deskWorkerArg, ProcessGroup: true,
			Session: yaegikernel.SessionWorkerConfig{
				AllowedPackages: yaegikernel.DefaultSafeStdlibPackagesList(),
				ComputerID:      "host-test", ActivationID: "desk", Epoch: 1,
				AllowedRoot: "/tmp", Role: agentprofile.Management,
				MemoryLimitBytes: deskWorkerMemoryLimitBytes,
			},
		}
	}
}

// One REPL per desk (Cut 0 artifact 5): a run that finds its agent's session
// mid-cell for another run waits for that cell and then evaluates in the
// same worker, told which run it shared with. It never gets a second REPL.
func TestDeskSessionBusySessionIsWaitedForNeverDuplicated(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	cfg := deskPoolCfg(t)
	busy, err := workers.acquire(context.Background(), "agent:a", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	got := make(chan deskSessionLease, 1)
	errs := make(chan error, 1)
	go func() {
		lease, err := workers.acquire(context.Background(), "agent:a", true, "run-2", cfg("run-2"))
		if err != nil {
			errs <- err
			return
		}
		got <- lease
	}()
	select {
	case lease := <-got:
		t.Fatalf("a second run must wait for the busy session, got lease %q at once", lease.key)
	case err := <-errs:
		t.Fatalf("acquire while busy: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	// Another desk is not stalled by the wait: the pool mutex is free.
	otherDesk, err := workers.acquire(context.Background(), "agent:b", true, "run-3", cfg("run-3"))
	if err != nil {
		t.Fatalf("another desk must not wait behind agent:a: %v", err)
	}
	workers.done(otherDesk.key)
	workers.done(busy.key)
	select {
	case lease := <-got:
		if lease.key != busy.key || lease.worker != busy.worker {
			t.Fatalf("the waiting run must get the same session, got key %q", lease.key)
		}
		if !strings.Contains(lease.notice, "run-1") {
			t.Fatalf("the waiting run must be told which run shares its REPL: %q", lease.notice)
		}
		workers.done(lease.key)
	case err := <-errs:
		t.Fatalf("acquire after release: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting run never got the session after it was released")
	}
	if n := workers.count(); n != 2 {
		t.Fatalf("one session per desk: want 2 live sessions (agent:a, agent:b), got %d", n)
	}
}

// A waiter whose context ends returns without the session, and the session
// stays with its holder.
func TestDeskSessionCancelledWaiterReturnsWithoutSession(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	cfg := deskPoolCfg(t)
	busy, err := workers.acquire(context.Background(), "agent:a", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if lease, err := workers.acquire(ctx, "agent:a", true, "run-2", cfg("run-2")); err == nil {
		t.Fatalf("a cancelled waiter must not get the session, got %q", lease.key)
	} else if !strings.Contains(err.Error(), "run-1") {
		t.Fatalf("the waiter's error must name the run holding the session: %v", err)
	}
	if workers.count() != 1 || busy.worker.Dead() {
		t.Fatalf("the holder's session must be untouched by the cancelled waiter")
	}
	workers.done(busy.key)
	again, err := workers.acquire(context.Background(), "agent:a", true, "run-2", cfg("run-2"))
	if err != nil || again.worker != busy.worker {
		t.Fatalf("after release the same session serves the next run: err=%v", err)
	}
	workers.done(again.key)
}

// Idle sessions beyond the pool's bound are closed least recently used first.
func TestDeskSessionPoolBoundsIdleSessions(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	clock := time.Now()
	workers.now = func() time.Time { return clock }
	workers.maxIdle = 2
	t.Cleanup(workers.closeAll)
	cfg := deskPoolCfg(t)
	first, err := workers.acquire(context.Background(), "agent:a", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	workers.done(first.key)
	for _, key := range []string{"agent:b", "agent:c", "agent:d"} {
		clock = clock.Add(time.Second)
		lease, err := workers.acquire(context.Background(), key, true, "run-x", cfg("run-x"))
		if err != nil {
			t.Fatalf("acquire %s: %v", key, err)
		}
		workers.done(lease.key)
	}
	if n := workers.count(); n > 2 {
		t.Fatalf("idle sessions must stay within the bound: %d live", n)
	}
	if !first.worker.Dead() {
		t.Fatalf("the least recently used idle session must be closed first")
	}
}
