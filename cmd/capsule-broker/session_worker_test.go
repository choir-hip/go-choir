//go:build linux

package main

import (
	"context"
	"encoding/json"
	"testing"

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
	if resp.Error != "" {
		t.Fatalf("transport-level error: %v", resp.Error)
	}
	var result capsule.GoEvalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 || result.Error == "" {
		t.Fatalf("result = %+v, want failed exit with error", result)
	}
	if len(b.sessionWorkers) != 0 {
		t.Fatalf("failed eval left %d workers", len(b.sessionWorkers))
	}
}
