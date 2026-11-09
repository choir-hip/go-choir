package toolregistry

import (
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The real desk_go_eval result shape (internal/agentcore/tools_desk.go): two
// calls with the same source and output but different durations and session
// labels are the same step.
func TestNoProgressCellOutputIgnoresDuration(t *testing.T) {
	calls := []types.ToolCall{{ID: "c", Name: "desk_go_eval", Arguments: json.RawMessage(`{"source":"choir.ListDir(\"/work\")"}`)}}
	mk := func(dur, sess string) []types.ToolResult {
		return []types.ToolResult{{CallID: "c", Output: `{"stdout":"=> []","stderr":"","error":"","duration":` + dur + `,"reuse":false,"diag":"","receipts":null,"intents":0,"session":"` + sess + `"}`}}
	}
	a, _, _ := stepFingerprint(calls, mk("12", "s1"))
	b, _, _ := stepFingerprint(calls, mk("873", "s2"))
	if a != b {
		t.Fatal("same cell, different duration and session must fingerprint the same")
	}
	c, _, _ := stepFingerprint(calls, []types.ToolResult{{CallID: "c", Output: `{"stdout":"=> [x]","duration":1}`}})
	if a == c {
		t.Fatal("different stdout must differ")
	}
}

func TestNoProgressStopNeedsRepeatsInsideAWindow(t *testing.T) {
	var tr repeatTracker
	calls := []types.ToolCall{{ID: "c", Name: "t", Arguments: json.RawMessage(`{}`)}}
	same := []types.ToolResult{{CallID: "c", Output: "x"}}
	other := func(i int) []types.ToolResult {
		return []types.ToolResult{{CallID: "c", Output: "other-" + string(rune('a'+i))}}
	}
	var v repeatVerdict
	for i := 0; i < 4; i++ {
		v, _, _ = tr.observe(calls, same)
	}
	if v != repeatNotice {
		t.Fatalf("verdict %v; want notice on the 4th", v)
	}
	// One occurrence every 6 steps never puts 4 inside a window of 12.
	for round := 0; round < 6; round++ {
		for i := 0; i < 5; i++ {
			tr.observe(calls, other(round*5+i))
		}
		if v, _, _ := tr.observe(calls, same); v != repeatOK {
			t.Fatalf("round %d: a sparse repeat stopped the run", round)
		}
	}
	// Four tight repeats stop it, with the true count.
	// (The last sparse repeat is still inside the window, so it stops on the
	// third.)
	var n, tight int
	for tight = 1; tight <= 4 && v != repeatStop; tight++ {
		v, _, n = tr.observe(calls, same)
	}
	if v != repeatStop || n != 4+6+(tight-1) {
		t.Fatalf("verdict %v count %d after %d tight repeats; want stop with the true count", v, n, tight-1)
	}
}

func TestNoProgressRunningJobIsNotARepeat(t *testing.T) {
	var tr repeatTracker
	calls := []types.ToolCall{{ID: "c", Name: "desk_go_eval", Arguments: json.RawMessage(`{}`)}}
	res := []types.ToolResult{{CallID: "c", Output: `{"stdout":"running","jobs_awaited":["j1"]}`}}
	for i := 0; i < 20; i++ {
		if v, _, _ := tr.observe(calls, res); v != repeatOK {
			t.Fatal("a still-running job tripped the detector")
		}
	}
}

func TestNoProgressErrorTextNeverClassifiesAsFailover(t *testing.T) {
	err := &NoProgressError{Tool: "quota_timeout_429", Count: 8}
	if IsModelFailoverError(err) || IsProviderRateLimitError(err) || isTransientProviderError(err) {
		t.Fatal("classifiers must ignore ErrNoProgress whatever the tool is called")
	}
}
