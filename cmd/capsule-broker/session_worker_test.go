//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os/exec"
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
	frame := yaegikernel.SessionFrame{ID: "cell-9", Source: `x := 1`}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var back yaegikernel.SessionFrame
	if err := json.Unmarshal(raw, &back); err != nil || back != frame {
		t.Fatalf("frame roundtrip = %+v, %v", back, err)
	}
	res := yaegikernel.SessionResult{ID: "cell-9", Stdout: "hi"}
	raw, err = json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var backRes yaegikernel.SessionResult
	if err := json.Unmarshal(raw, &backRes); err != nil || backRes != res {
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
	// Spawn failure surfaces as a transport error, mirroring the one-shot
	// path's "failed to start go_eval worker" shape — never a fake result.
	if resp.Error == "" || !strings.Contains(resp.Error, "session worker") {
		t.Fatalf("resp = %+v, want session-worker transport error", resp)
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
