//go:build linux

package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestResolveActuatorRoute(t *testing.T) {
	cases := []struct {
		env  string
		want actuatorRoute
	}{
		{"", actuatorTools},
		{"tools", actuatorTools},
		{"TOOLS", actuatorTools},
		{"rlm", actuatorRLM},
		{"RLM", actuatorRLM},
		{"  rlm  ", actuatorRLM},
		{"bogus", actuatorTools},
	}
	for _, tc := range cases {
		t.Setenv("CHOIR_ACTUATOR", tc.env)
		if got := resolveActuatorRoute(); got != tc.want {
			t.Fatalf("CHOIR_ACTUATOR=%q -> %q, want %q", tc.env, got, tc.want)
		}
	}
}

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

func TestHandleGetActuatorAdvertisesRoute(t *testing.T) {
	b := &Broker{actuator: actuatorRLM}
	resp := b.handleGetActuator(context.Background(), nil, nil)
	if resp.Error != "" {
		t.Fatalf("get_actuator: %v", resp.Error)
	}
	var got map[string]string
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if got["requested"] != "rlm" || got["route"] != "tools" || got["session_ready"] != "false" {
		t.Fatalf("advertised route = %v, want requested=rlm route=tools session_ready=false", got)
	}
}
