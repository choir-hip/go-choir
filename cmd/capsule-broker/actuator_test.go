//go:build linux

package main

import (
	"testing"
)

func TestEffectiveRouteFallsBackWithoutSessionWorker(t *testing.T) {
	if got := (&Broker{actuator: actuatorRLM}).effectiveRoute(); got != actuatorTools {
		t.Fatalf("rlm without session worker = %q, want tools fallback", got)
	}
	if got := (&Broker{actuator: actuatorRLM, sessionWorkerReady: true}).effectiveRoute(); got != actuatorRLM {
		t.Fatalf("rlm with session worker = %q, want rlm", got)
	}
	if got := (&Broker{actuator: actuatorTools}).effectiveRoute(); got != actuatorTools {
		t.Fatalf("tools = %q, want tools", got)
	}
	if got := (*Broker)(nil).effectiveRoute(); got != actuatorTools {
		t.Fatalf("nil broker = %q, want tools", got)
	}
}
