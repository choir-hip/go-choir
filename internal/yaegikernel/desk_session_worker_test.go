package yaegikernel

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestDeskSessionWorkerHelperProcess is the re-executed worker. It is not a
// test in the parent process: SpawnDeskSessionWorker re-execs this same test
// binary with -test.run=TestDeskSessionWorkerHelperProcess, the child detects
// CHOIR_DESK_WORKER_CHILD=1 and serves framed cells on the inherited socket.
func TestDeskSessionWorkerHelperProcess(t *testing.T) {
	if os.Getenv("CHOIR_DESK_WORKER_CHILD") != "1" {
		return
	}
	cfg := SessionWorkerConfigFromEnv(nil, "")
	fd, conn, err := SessionWorkerSockFD(nil, "")
	fmt.Fprintf(os.Stderr, "helper: sockfd=%d conn=%v err=%v cfg=%+v\n", fd, conn != nil, err, cfg)
	if err != nil || fd < 0 || conn == nil {
		os.Exit(2)
	}
	ExecuteWorkerSessionConn(conn, cfg)
	os.Exit(0)
}
func deskWorkerConfig(t *testing.T) DeskSessionWorkerConfig {
	t.Helper()
	return DeskSessionWorkerConfig{
		Bin:       os.Args[0],
		WorkerArg: "-test.run=TestDeskSessionWorkerHelperProcess",
		Session: SessionWorkerConfig{
			AllowedPackages: DefaultSafeStdlibPackagesList(),
			ComputerID:      "host-test",
			ActivationID:    "act-desk-1",
			Epoch:           1,
			AllowedRoot:     t.TempDir(),
			Role:            "management",
		},
		ProcessGroup: true,
		ExtraEnv:     []string{"CHOIR_DESK_WORKER_CHILD=1"},
	}
}

func TestDeskSessionWorkerSpawnsEvalsAndRespawns(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	ctx := context.Background()
	w, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	if w.PID() <= 0 {
		t.Fatalf("worker pid not recorded: %d", w.PID())
	}
	if w.PID() == os.Getpid() {
		t.Fatal("worker pid equals host pid — not a separate process")
	}
	res, err := w.Eval(ctx, `x := 1 + 1`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("cell error: %s", res.Error)
	}
	// Kill mid-life: the worker dies; a fresh spawn serves the next eval.
	w.Kill()
	if !w.Dead() {
		t.Fatal("worker not marked dead after kill")
	}
	w2, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("respawn after kill: %v", err)
	}
	defer w2.Close()
	res2, err := w2.Eval(ctx, `y := 2 + 2`)
	if err != nil || res2.Error != "" {
		t.Fatalf("post-respawn eval: err=%v res=%#v", err, res2)
	}
	if w2.PID() == w.PID() {
		t.Fatal("respawn reused same pid")
	}
}

func TestDeskSessionWorkerRestrictedStdlib(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	ctx := context.Background()
	w, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	// os/exec is outside DefaultSafeStdlibPackagesList: a management-profile
	// worker must reject it (restricted stdlib per profile).
	res, err := w.Eval(ctx, `import "os/exec"; _ = exec.Command("ls")`)
	if err != nil {
		t.Fatalf("eval transport: %v", err)
	}
	if res.Error == "" {
		t.Fatal("os/exec import was not rejected by restricted stdlib")
	}
}

func TestDeskSessionWorkerEnvRoundTrip(t *testing.T) {
	cfg := DeskSessionWorkerConfig{EnvPrefix: "CHOIR_DESK_SESSION_", Session: SessionWorkerConfig{
		AllowedPackages: []string{"fmt", "strings"}, ComputerID: "c1", ActivationID: "a1",
		Epoch: 7, AllowedRoot: "/x", Role: "texture", Slot: "impl",
	}}
	env := cfg.sessionEnv()
	got := map[string]string{}
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok {
			got[k] = v
		}
	}
	back := SessionWorkerConfigFromEnv(func(k string) string { return got[k] }, "")
	if back.ComputerID != "c1" || back.ActivationID != "a1" || back.Epoch != 7 ||
		back.AllowedRoot != "/x" || back.Role != "texture" || back.Slot != "impl" {
		t.Fatalf("env round-trip mismatch: %#v", back)
	}
	if len(back.AllowedPackages) != 2 || back.AllowedPackages[0] != "fmt" {
		t.Fatalf("packages round-trip: %#v", back.AllowedPackages)
	}
}
