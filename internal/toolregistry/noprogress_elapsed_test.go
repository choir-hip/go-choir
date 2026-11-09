package toolregistry

import (
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func fpOf(t *testing.T, output string) string {
	t.Helper()
	calls := []types.ToolCall{{ID: "c", Name: "desk_go_eval", Arguments: json.RawMessage(`{"source":"x"}`)}}
	fp, _, _ := stepFingerprint(calls, []types.ToolResult{{CallID: "c", Output: output, IsError: true}})
	return fp
}

// Deadline-death cells differ only in the elapsed figure inside error text.
func TestNoProgressDeadlineErrorsIgnoreElapsed(t *testing.T) {
	mk := func(msg string) string {
		b, _ := json.Marshal(map[string]any{"stdout": "", "error": msg, "duration": 1})
		return string(b)
	}
	for _, pair := range [][2]string{
		{"cell exceeded its host deadline after 4512 ms", "cell exceeded its host deadline after 9120 ms"},
		{"worker died after 1.5s", "worker died after 12.25s"},
		{"cell timed out at 2m3s", "cell timed out at 4m59s"},
		{"deadline 123ms", "deadline 4567ms"},
	} {
		if fpOf(t, mk(pair[0])) != fpOf(t, mk(pair[1])) {
			t.Errorf("%q and %q must fingerprint the same", pair[0], pair[1])
		}
	}
	if fpOf(t, mk("worker died after 1s")) == fpOf(t, mk("host deadline after 1s")) {
		t.Error("different errors must differ")
	}
}

// The capsule tool_error path is plain text with a rounded elapsed figure.
func TestNoProgressCapsuleToolErrorIgnoresElapsed(t *testing.T) {
	a := fpOf(t, "tool_error: capsule_go_eval: deadline exceeded after 30.001s")
	b := fpOf(t, "tool_error: capsule_go_eval: deadline exceeded after 31.2s")
	if a != b {
		t.Fatal("tool_error text differing only in elapsed must match")
	}
	if a == fpOf(t, "tool_error: capsule_go_eval: out of memory after 30s") {
		t.Fatal("a different error must differ")
	}
}

func TestNoProgressVolatileKeysAreOnlyKnownClocks(t *testing.T) {
	a := fpOf(t, `{"job":"j","timestamp":"t1","request_id":"r1","seq":1,"updated_at":"u1"}`)
	b := fpOf(t, `{"job":"j","timestamp":"t2","request_id":"r2","seq":2,"updated_at":"u1"}`)
	if a != b {
		t.Fatal("timestamp, request_id and seq are per-call")
	}
	if a == fpOf(t, `{"job":"j","timestamp":"t2","request_id":"r2","seq":2,"updated_at":"u2"}`) {
		t.Fatal("updated_at is kept: a changed value is a changed result")
	}
}

// Known limit: a desk cell result carries only the count of staged intents.
func TestNoProgressCellIntentCountLimit(t *testing.T) {
	a := fpOf(t, `{"stdout":"ok","intents":1,"duration":5}`)
	b := fpOf(t, `{"stdout":"ok","intents":1,"duration":9}`)
	if a != b {
		t.Fatal("same stdout and intent count fingerprint the same (documented limit)")
	}
	if a == fpOf(t, `{"stdout":"ok","intents":2}`) {
		t.Fatal("a different intent count must differ")
	}
}
