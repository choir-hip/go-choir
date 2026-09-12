//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func testSessionBroker(t *testing.T) *Broker {
	return &Broker{
		mergedDir:          t.TempDir(),
		sessionWorkers:     make(map[string]*sessionWorker),
		revokedCaps:        make(map[string]bool),
		brokerBin:          "/nonexistent/broker",
		actuator:           actuatorRLM,
		sessionWorkerReady: true,
	}
}

func TestSessionFrameCodecRoundTrip(t *testing.T) {
	frame := yaegikernel.SessionFrame{ID: "cell-9", Source: `x := 1`, Inbox: []yaegikernel.IncomingMessage{
		{ID: "m-1", FromDesk: "super", ToDesk: "cosuper", Kind: "directive", Body: "go"},
	}}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var back yaegikernel.SessionFrame
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, frame) {
		t.Fatalf("frame roundtrip = %+v, %v", back, err)
	}
	res := yaegikernel.SessionResult{ID: "cell-9", Stdout: "hi", Intents: []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "super", Body: "hi"},
	}}
	raw, err = json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var backRes yaegikernel.SessionResult
	if err := json.Unmarshal(raw, &backRes); err != nil || !reflect.DeepEqual(backRes, res) {
		t.Fatalf("result roundtrip = %+v, %v", backRes, err)
	}
}

func TestInitSessionFailsCleanWithoutBinary(t *testing.T) {
	b := testSessionBroker(t)
	cap := &capsule.Capability{AgentRunID: "run-test"}
	resp := b.handleInitSession(context.Background(), cap, nil)
	if resp.Error == "" {
		t.Fatal("init with bad binary must fail")
	}
	if len(b.sessionWorkers) != 0 {
		t.Fatalf("failed spawn left %d workers", len(b.sessionWorkers))
	}
}

func TestCloseSessionMissingIsSuccess(t *testing.T) {
	b := testSessionBroker(t)
	cap := &capsule.Capability{AgentRunID: "run-missing"}
	resp := b.handleCloseSession(context.Background(), cap, nil)
	if resp.Error != "" {
		t.Fatalf("close missing: %v", resp.Error)
	}
}

func TestGoEvalSessionFailsClosedWithoutBinary(t *testing.T) {
	b := testSessionBroker(t)
	params, _ := json.Marshal(map[string]string{"source": `1 + 1`})
	cap := &capsule.Capability{AgentRunID: "run-test"}
	resp := b.handleGoEvalSession(context.Background(), cap, params)
	// Spawn failure attempts the one-shot tools fallback and reports both
	// errors when it also fails — never a fake result, never silent.
	if resp.Error == "" || !strings.Contains(resp.Error, "session worker") {
		t.Fatalf("resp = %+v, want session-worker transport error", resp)
	}
	if !strings.Contains(resp.Error, "fallback") {
		t.Fatalf("resp = %+v, want visible fallback attempt", resp)
	}
	if len(b.sessionWorkers) != 0 {
		t.Fatalf("failed eval left %d workers", len(b.sessionWorkers))
	}
}

// TestSessionKillReapsProcessGroup is Def 2 containment: a live process group
// is SIGKILLed and reaped within 500ms.
func TestSessionKillReapsProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep unavailable: %v", err)
	}
	start := time.Now()
	if !killProcessGroup(cmd, sessionKillReapGrace) {
		t.Fatal("process group not reaped within grace")
	}
	if elapsed := time.Since(start); elapsed > sessionKillReapGrace {
		t.Fatalf("reap took %v, exceeds 500ms", elapsed)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		t.Fatal("process still alive after kill")
	}
}

// buildBrokerBinary compiles the capsule-broker for real-spawn tests (linux
// CI). It fails the test when the toolchain cannot produce the binary: a
// skipped spawn test is how B1 shipped.
func buildBrokerBinary(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable for real-spawn test")
	}
	out := filepath.Join(t.TempDir(), "capsule-broker-test")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build broker binary: %v\n%s", err, out)
	}
	return out
}

func evalCell(t *testing.T, w *sessionWorker, source string) yaegikernel.SessionResult {
	t.Helper()
	res, err := w.eval(source, nil, 60*time.Second)
	if err != nil {
		t.Fatalf("eval %q: %v", source, err)
	}
	return res
}

// TestSessionWorkerRealSpawnEndToEnd is the B1 regression: a worker spawned
// through spawnSessionWorker completes the ready handshake and serves cells
// (import persistence, real overlay write, tray-staged messaging) instead
// of dying at startup on an empty computer identity.
func TestSessionWorkerRealSpawnEndToEnd(t *testing.T) {
	bin := buildBrokerBinary(t)
	root := t.TempDir()
	w, err := spawnSessionWorker(bin, workerSessionConfig{
		computerID: "test-capsule", epoch: 1, activation: "run-e2e",
		allowedRoot: root, timeout: 60 * time.Second, role: "co-super",
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.close()
	if res := evalCell(t, w, `import "choir"`); res.Error != "" {
		t.Fatalf("import choir: %s", res.Error)
	}
	if res := evalCell(t, w, `choir.WriteFile("e2e.txt", "live")`); res.Error != "" {
		t.Fatalf("write: %s", res.Error)
	}
	content, err := os.ReadFile(filepath.Join(root, "e2e.txt"))
	if err != nil || string(content) != "live" {
		t.Fatalf("overlay file = %q, %v", content, err)
	}
	res := evalCell(t, w, `choir.Message("owner", "note", "hi")`)
	if res.Error != "" {
		t.Fatalf("message: %s", res.Error)
	}
	// RLM contract: bound cells stage into the tray for post-cell reduction
	// instead of synchronously delivering. The intent ships on the result;
	// worker-local receipts stay empty (nothing was delivered yet).
	found := false
	for _, in := range res.Intents {
		if in.Kind == yaegikernel.IntentMessage && in.ToDesk == "owner" && in.Body == "hi" {
			found = true
		}
	}
	if !found {
		t.Fatalf("staged intents = %+v, want owner/note/hi message", res.Intents)
	}
	if len(res.Receipts) != 0 {
		t.Fatalf("receipts = %v, want empty (staged, not delivered)", res.Receipts)
	}
}

// TestSessionWorkerResearcherDeniedEndToEnd is the B2 regression: a worker
// spawned with the researcher role refuses writes through the real binary.
func TestSessionWorkerResearcherDeniedEndToEnd(t *testing.T) {
	bin := buildBrokerBinary(t)
	w, err := spawnSessionWorker(bin, workerSessionConfig{
		computerID: "test-capsule", epoch: 1, activation: "run-researcher",
		allowedRoot: t.TempDir(), timeout: 60 * time.Second, role: "researcher",
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.close()
	if res := evalCell(t, w, `import "choir"`); res.Error != "" {
		t.Fatalf("import choir: %s", res.Error)
	}
	res, err := w.eval(`choir.WriteFile("x.txt", "x")`, nil, 60*time.Second)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	// Denial surfaces at compile time (symbol absent from the researcher
	// package) or at the method guard; either proves confinement.
	if !strings.Contains(res.Error, "denied") && !strings.Contains(res.Error, "no symbol WriteFile") {
		t.Fatalf("researcher write error = %q, want denial", res.Error)
	}
}

// TestAwaitReadyHandshake proves spawn readiness is verified, not assumed: a
// ready frame passes, any other first frame fails the spawn.
func TestAwaitReadyHandshake(t *testing.T) {
	ready := func(payload string) *sessionWorker {
		a, b, err := yaegikernel.SocketPair()
		if err != nil {
			t.Fatal(err)
		}
		fa := yaegikernel.NewFramedConn(a)
		if err := fa.WriteFrame(yaegikernel.StreamCell, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		return &sessionWorker{framed: yaegikernel.NewFramedConn(b)}
	}
	if err := ready("{\"id\":\"ready\"}").awaitReady(5 * time.Second); err != nil {
		t.Fatalf("ready frame: %v", err)
	}
	if err := ready("{\"id\":\"cell-1\",\"stdout\":\"hi\"}").awaitReady(5 * time.Second); err == nil {
		t.Fatal("non-ready first frame accepted")
	}
}
