package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A marker an older build wrote still gates the one boot that reads it:
// planned and drained resumes; planned with a failed drain under an open
// run-memory window (or a marker that stamped none, with a failed drain,
// resumes: only builds with the window stamped it) closes; a crash never resumes.
func TestBootResumesWorkReadsAnOldBuildsWindowStamp(t *testing.T) {
	rt := telemetryRuntime(t, "computer-old-marker")
	open, closed := true, false
	cases := []struct {
		name    string
		marker  PlannedRestart
		planned bool
		want    bool
	}{
		{"drained", PlannedRestart{Reason: "platform update"}, true, true},
		{"failed drain, window open", PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &open}, true, false},
		{"failed drain, window closed", PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &closed}, true, true},
		{"failed drain, no stamp", PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true}, true, true},
		{"crash", PlannedRestart{Reason: "platform update"}, false, false},
	}
	for _, c := range cases {
		rt.bootRestart, rt.bootRestartPlanned = c.marker, c.planned
		if got := rt.bootResumesWork(); got != c.want {
			t.Errorf("%s: bootResumesWork = %v, want %v", c.name, got, c.want)
		}
	}
}

// A run's opening messages are one write: one tape event, durable on return.
func TestAppendMessagesWritesAnOpeningAsOneTapeEvent(t *testing.T) {
	rt := telemetryRuntime(t, "computer-opening")
	rec := &types.RunRecord{RunID: "run-opening", OwnerID: "owner-telemetry", AgentID: "agent-opening"}
	m := newRunMemoryManager(rt.store, rec, provideriface.Config{}, nil)
	before := rt.store.TelemetryStats().TapeEvents
	msgs := []json.RawMessage{json.RawMessage(`{"role":"system","content":"a"}`), json.RawMessage(`{"role":"user","content":"b"}`), json.RawMessage(`{"role":"user","content":"c"}`)}
	if err := m.appendMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if n := rt.store.TelemetryStats().TapeEvents - before; n != 1 {
		t.Fatalf("three opening messages took %d tape events, want 1", n)
	}
	entries, err := rt.store.ListRunMemoryEntries(context.Background(), "owner-telemetry", "run-opening")
	if err != nil || len(entries) != 3 || entries[2].Seq != 3 {
		t.Fatalf("opening entries = %+v (%v)", entries, err)
	}
}
