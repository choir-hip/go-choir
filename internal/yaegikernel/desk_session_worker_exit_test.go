package yaegikernel

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Management delegation S1 (R2-19): every kill of a desk worker is logged
// with the worker's exit cause. Run 305a582d's 860 s cell ended "frame: read
// header: EOF" and nothing recorded why the worker died (H1).
//
// Ways this could fail, written first:
//  1. a deadline on the eval context does not kill a cell that never yields
//     (a busy loop), so the host waits forever;
//  2. the kill is recorded but the worker is reused;
//  3. the exit cause of a worker that died on its own is overwritten by the
//     host's own kill of the corpse, so a crash reads as a host kill;
//  4. the crash diagnostics the worker wrote to stderr are lost;
//  5. a second reap (Kill then Close) blocks or changes the cause.

func skipWithoutSocketpair(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
}

func TestDeskSessionWorkerDeadlineKillRecordsHostKill(t *testing.T) {
	skipWithoutSocketpair(t)
	w, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	if cause := w.ExitCause(); cause != "" {
		t.Fatalf("live worker has an exit cause: %q", cause)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = w.EvalCell(ctx, `n := 0
for {
	n++
}`, nil, nil, nil, nil, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("busy cell under a deadline: err = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("deadline kill took %s", elapsed)
	}
	if !w.Dead() {
		t.Fatal("worker not dead after a deadline kill")
	}
	cause := w.ExitCause()
	if !strings.Contains(cause, "killed by the host") {
		t.Fatalf("exit cause %q does not name the host kill", cause)
	}
	w.Close()
	if again := w.ExitCause(); again != cause {
		t.Fatalf("close after kill changed the exit cause: %q -> %q", cause, again)
	}
}

func TestDeskSessionWorkerSelfExitRecordsItsOwnCause(t *testing.T) {
	skipWithoutSocketpair(t)
	cfg := deskWorkerConfig(t)
	cfg.ExtraEnv = append(cfg.ExtraEnv, "CHOIR_DESK_WORKER_CHILD_CRASH_AFTER=200ms")
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	_, err = w.EvalCell(context.Background(), `import "time"
time.Sleep(10 * time.Second)`, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("a cell whose worker died returned no error")
	}
	if !w.Dead() {
		t.Fatal("worker not dead after it exited")
	}
	w.Close()
	cause := w.ExitCause()
	for _, want := range []string{"exited on its own", "exit status 7", "fatal error: test crash"} {
		if !strings.Contains(cause, want) {
			t.Fatalf("exit cause %q does not name %q", cause, want)
		}
	}
}
