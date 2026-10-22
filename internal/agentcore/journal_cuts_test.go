package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Journal cuts R1 and R3 (docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md,
// "Build: journal cuts R1 to R4"): three run-event lines are published on the
// live bus and not appended to the tape, because nothing reads them durably:
// the loop.progress line written before each model call (phase
// provider_call_started), the texture.agent_revision.progress copy of every
// progress line, and tool.invoked (the call's arguments and timing ride its
// tool.result). The live ticker and the Texture stream read the bus.
//
// Ways this could fail, written first: a bus-only line is also appended; a
// bus-only line is dropped from the bus, so the ticker or the Texture
// stream goes quiet; the response line that cost extraction reads is cut with
// it; the result loses the arguments the invoked row carried; the latency the
// probe paired from two lines is not on the one that remains.

type busCapture struct {
	ch <-chan events.RuntimeEvent
}

func captureBus(rt *Runtime) (*busCapture, func()) {
	ch := rt.EventBus().SubscribeWithBuffer(512)
	return &busCapture{ch: ch}, func() { rt.EventBus().Unsubscribe(ch) }
}

// drain returns every event the bus delivered for runID so far.
func (c *busCapture) drain(runID string) []types.EventRecord {
	var out []types.EventRecord
	for {
		select {
		case ev := <-c.ch:
			if ev.Record.RunID == runID {
				out = append(out, ev.Record)
			}
		default:
			return out
		}
	}
}

func payloadPhase(t *testing.T, rec types.EventRecord) string {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(rec.Payload, &p); err != nil {
		t.Fatalf("decode %s payload: %v", rec.Kind, err)
	}
	phase, _ := p["phase"].(string)
	return phase
}

func TestRunAppendsOnlyWhatReadersReadDurably(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := registry.Register(toolregistry.Tool{
		Name: "read_file",
		Func: func(context.Context, json.RawMessage) (string, error) { return "file contents", nil },
	}); err != nil {
		t.Fatal(err)
	}
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"/tmp/a.txt"}`)}},
			Usage:      provideriface.TokenUsage{InputTokens: 15, OutputTokens: 10},
		},
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn", Text: "done",
			Usage: provideriface.TokenUsage{InputTokens: 25, OutputTokens: 5},
		},
	)
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, registry)
	defer rt.Stop()
	live, stop := captureBus(rt)
	defer stop()

	rec, err := rt.StartRun(context.Background(), "read the test file", "user-journal-cuts")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	waitForStoredRunTerminalState(t, s, rec.RunID, 5*time.Second)
	waitForEvents(t, s, rec.RunID, []types.EventKind{types.EventRunCompleted, types.EventToolResult}, 3*time.Second)
	time.Sleep(100 * time.Millisecond)

	stored, err := s.ListEvents(context.Background(), rec.RunID, 200)
	if err != nil {
		t.Fatal(err)
	}
	var toolLoopLines int
	for _, ev := range stored {
		switch ev.Kind {
		case types.EventToolInvoked:
			t.Errorf("tool.invoked was appended; it is published on the bus only: %s", ev.Payload)
		case types.EventRunProgress:
			switch phase := payloadPhase(t, ev); phase {
			case "provider_call_started":
				t.Errorf("the provider_call_started line was appended; it is published on the bus only")
			case "tool_loop":
				toolLoopLines++
				var p map[string]any
				_ = json.Unmarshal(ev.Payload, &p)
				// The response line carries what the request line was read for.
				for _, key := range []string{"provider_call_ms", "messages", "tools", "system_sha256", "last_user_sha256"} {
					if _, ok := p[key]; !ok {
						t.Errorf("tool_loop line lacks %q: %s", key, ev.Payload)
					}
				}
				if p["input_tokens"] == nil || p["output_tokens"] == nil {
					t.Errorf("tool_loop line lost its token counts (cost extraction reads them): %s", ev.Payload)
				}
			}
		case types.EventToolResult:
			var p map[string]any
			_ = json.Unmarshal(ev.Payload, &p)
			args, _ := p["arguments"].(map[string]any)
			if args["path"] != "/tmp/a.txt" || p["call_id"] != "call-1" || p["tool"] != "read_file" {
				t.Errorf("tool.result must carry the call's arguments: %s", ev.Payload)
			}
			if p["started_at"] == nil || p["duration_ms"] == nil {
				t.Errorf("tool.result must carry started_at and duration_ms: %s", ev.Payload)
			}
		}
	}
	if toolLoopLines != 2 {
		t.Errorf("tool_loop lines appended = %d, want one per model call (2)", toolLoopLines)
	}

	// The counters FP0 reads on the observability surface.
	byKind := s.TelemetryStats().EventsAppendedByKind
	if byKind["loop.progress"] != 2 || byKind["tool.invoked"] != 0 || byKind["tool.result"] != 1 {
		t.Errorf("events_appended_by_kind = %v, want loop.progress 2, tool.invoked 0, tool.result 1", byKind)
	}

	// The bus still carries the lines the live readers use.
	var started, invoked, resultLive int
	for _, ev := range live.drain(rec.RunID) {
		switch ev.Kind {
		case types.EventRunProgress:
			if payloadPhase(t, ev) == "provider_call_started" {
				started++
			}
		case types.EventToolInvoked:
			invoked++
		case types.EventToolResult:
			resultLive++
		}
	}
	if started != 2 || invoked != 1 || resultLive != 1 {
		t.Errorf("bus carried provider_call_started=%d tool.invoked=%d tool.result=%d, want 2, 1, 1", started, invoked, resultLive)
	}
}

func TestTextureProgressIsPublishedNotAppended(t *testing.T) {
	rt, s := testRuntime(t)
	live, stop := captureBus(rt)
	defer stop()
	rec := &types.RunRecord{
		RunID: "run-texture-progress", AgentID: "texture:doc-progress", OwnerID: "owner-1",
		AgentProfile: agentprofile.Texture,
		Metadata:     map[string]any{"doc_id": "doc-progress"},
	}
	emit := rt.runEventEmitter(context.Background(), rec)
	turn, _ := json.Marshal(map[string]any{"iteration": 1, "input_tokens": 10, "output_tokens": 2})
	emit(types.EventRunProgress, "tool_loop", turn)
	call, _ := json.Marshal(map[string]any{"iteration": 1, "phase": "provider_call_started"})
	emit(types.EventRunProgress, "provider_call", call)

	stored, err := s.ListEvents(context.Background(), rec.RunID, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range stored {
		if ev.Kind == types.EventTextureAgentRevisionProgress {
			t.Errorf("texture.agent_revision.progress was appended: %s", ev.Payload)
		}
		if ev.Kind == types.EventRunProgress && payloadPhase(t, ev) == "provider_call_started" {
			t.Errorf("provider_call_started was appended")
		}
	}
	if n := len(stored); n != 1 {
		t.Errorf("stored %d events, want only the tool_loop line", n)
	}

	phases := map[string]int{}
	for _, ev := range live.drain(rec.RunID) {
		if ev.Kind == types.EventTextureAgentRevisionProgress {
			var p map[string]string
			_ = json.Unmarshal(ev.Payload, &p)
			if p["doc_id"] != "doc-progress" || p["loop_id"] != rec.RunID {
				t.Errorf("texture progress payload = %v", p)
			}
			phases[p["phase"]]++
		}
	}
	if phases["tool_loop"] != 1 || phases["provider_call"] != 1 {
		t.Errorf("the Texture stream saw progress phases %v, want one tool_loop and one provider_call", phases)
	}
}
