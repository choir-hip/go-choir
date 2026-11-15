package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The choir.run write counter rides in telemetry_batching as run_writes (the
// failure list is at the top of internal/store/run_write_counters_test.go).
// Here: the served block has its documented shape, says it is a count since
// boot, and agrees with the tape's own per-kind op count for choir.run on a
// flow every writer of which is a counted site (failure 10 there).
func TestObservabilityServesTheRunWriteCounters(t *testing.T) {
	rt := telemetryRuntime(t, "computer-run-write-counters-obs")
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{RunID: "run-rw-1", AgentID: "agent-rw", OwnerID: "owner-telemetry", ComputerID: "computer-run-write-counters-obs",
		State: types.RunPending, Prompt: "the prompt of the run", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"llm_fallbacks": "a/b", "texture_available_source_entities": []string{"e1", "e2"}}}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rec.State, rec.UpdatedAt = types.RunRunning, now.Add(time.Second)
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rec.UpdatedAt = now.Add(2 * time.Second) // the same state and fields, a later updated_at
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	done := now.Add(3 * time.Second)
	rec.State, rec.Result, rec.UpdatedAt, rec.FinishedAt = types.RunCompleted, "the result", done, &done
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	out := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(out, req)
	var obs struct {
		TelemetryBatching struct {
			RunWrites struct {
				Scope string `json:"scope"`
				Ops   int64  `json:"ops"`
				Lines map[string]struct {
					Ops           int64 `json:"ops"`
					BodyBytes     int64 `json:"body_bytes"`
					PromptBytes   int64 `json:"prompt_bytes"`
					MetadataBytes int64 `json:"metadata_bytes"`
				} `json:"lines"`
				TopMetadataKey map[string]struct {
					Writes int64 `json:"writes"`
				} `json:"top_metadata_key"`
				Undecodable map[string]int64 `json:"undecodable"`
			} `json:"run_writes"`
			ObjectRewrites map[string]struct {
				Ops int64 `json:"ops"`
			} `json:"object_rewrites"`
		} `json:"telemetry_batching"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &obs); err != nil {
		t.Fatalf("observability = %d %s", out.Code, out.Body.String())
	}
	rw := obs.TelemetryBatching.RunWrites
	if !strings.Contains(rw.Scope, "since boot") || !strings.Contains(rw.Scope, "restart") {
		t.Errorf("scope = %q, want it to say since boot and resets on restart", rw.Scope)
	}
	for key, want := range map[string]int64{
		"create_og/pending/new":                        1,
		"update_og/running/state":                      1,
		"update_og/running/updated_at_only":            1,
		"update_og/completed/finished_at+result+state": 1,
	} {
		if got := rw.Lines[key].Ops; got != want {
			t.Errorf("run_writes.lines[%s].ops = %d, want %d; lines %+v", key, got, want, rw.Lines)
		}
	}
	if line := rw.Lines["update_og/running/updated_at_only"]; line.PromptBytes <= 0 || line.MetadataBytes <= 0 || line.BodyBytes <= line.PromptBytes {
		t.Errorf("an updated_at-only write must still show the bytes it carried: %+v", line)
	}
	if rw.TopMetadataKey["texture_available_source_entities"].Writes != 4 {
		t.Errorf("the largest metadata key is the listed one in every write: %+v", rw.TopMetadataKey)
	}
	if len(rw.Undecodable) != 0 {
		t.Errorf("undecodable = %v on a flow of valid bodies", rw.Undecodable)
	}
	tape := obs.TelemetryBatching.ObjectRewrites["choir.run"].Ops
	if tape != 4 || rw.Ops != tape {
		t.Errorf("attributed run writes = %d, the tape counted %d choir.run ops, want both 4", rw.Ops, tape)
	}
}
