package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestStubProviderExecutePreservesProgressResultAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stub := NewStubProvider(time.Millisecond)
		stub.Result = "custom result"
		run := &types.RunRecord{}
		var kinds []types.EventKind
		err := stub.Execute(context.Background(), run, func(kind types.EventKind, _ string, _ json.RawMessage) {
			kinds = append(kinds, kind)
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if run.Result != "custom result" {
			t.Fatalf("result = %q, want custom result", run.Result)
		}
		if len(kinds) < 2 || kinds[0] != types.EventRunProgress || kinds[len(kinds)-1] != types.EventRunDelta {
			t.Fatalf("event kinds = %v, want progress then delta", kinds)
		}
	})

	t.Run("failure", func(t *testing.T) {
		wantErr := errors.New("provider failed")
		stub := &StubProvider{FailErr: wantErr}
		run := &types.RunRecord{}
		err := stub.Execute(context.Background(), run, func(types.EventKind, string, json.RawMessage) {})
		if !errors.Is(err, wantErr) {
			t.Fatalf("Execute error = %v, want %v", err, wantErr)
		}
		if run.Result != "" {
			t.Fatalf("failed run result = %q, want empty", run.Result)
		}
	})
}

func TestStubProviderExecuteCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stub := NewStubProvider(time.Second)
	err := stub.Execute(ctx, &types.RunRecord{}, func(types.EventKind, string, json.RawMessage) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
}

func TestStubProviderConductorTextureDecision(t *testing.T) {
	stub := NewStubProvider(0)
	run := &types.RunRecord{
		AgentProfile: agentprofile.Conductor,
		Prompt:       "Draft a durable Texture about source-grounded planning beyond nine words",
		Metadata: map[string]any{
			"requested_app": agentprofile.Texture,
		},
	}
	var delta map[string]string
	err := stub.Execute(context.Background(), run, func(kind types.EventKind, _ string, payload json.RawMessage) {
		if kind == types.EventRunDelta {
			if err := json.Unmarshal(payload, &delta); err != nil {
				t.Fatalf("decode delta: %v", err)
			}
		}
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var decision map[string]any
	if err := json.Unmarshal([]byte(run.Result), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision["action"] != "open_app" || decision["app"] != agentprofile.Texture {
		t.Fatalf("decision = %#v, want Texture open_app", decision)
	}
	if decision["seed_prompt"] != run.Prompt {
		t.Fatalf("seed_prompt = %q, want %q", decision["seed_prompt"], run.Prompt)
	}
	if decision["title"] != "Draft a durable Texture about source-grounded planning beyond nine" {
		t.Fatalf("title = %q", decision["title"])
	}
	if decision["create_initial_version"] != false {
		t.Fatalf("create_initial_version = %#v, want false", decision["create_initial_version"])
	}
	if delta["text"] != run.Result || delta["provider"] != "stub" {
		t.Fatalf("delta = %#v, want result and stub provider", delta)
	}

	custom := NewStubProvider(0)
	custom.Result = "caller supplied"
	customRun := &types.RunRecord{AgentProfile: agentprofile.Conductor, Metadata: map[string]any{"requested_app": agentprofile.Texture}}
	if err := custom.Execute(context.Background(), customRun, func(types.EventKind, string, json.RawMessage) {}); err != nil {
		t.Fatalf("custom Execute: %v", err)
	}
	if customRun.Result != "caller supplied" || strings.Contains(customRun.Result, "open_app") {
		t.Fatalf("custom result was rewritten: %q", customRun.Result)
	}
}
