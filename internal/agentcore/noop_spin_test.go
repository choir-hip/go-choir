package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The failure modes of docs/problems/engineer-noop-spin-2026-11-04.md, written
// before the detector.

func spinBatch(source, output string) ([]types.ToolCall, []types.ToolResult) {
	args, _ := json.Marshal(map[string]string{"source": source})
	return []types.ToolCall{{ID: "c", Name: "capsule_go_eval", Arguments: args}}, []types.ToolResult{{CallID: "c", Output: output}}
}

func tick(n int) ([]types.ToolCall, []types.ToolResult) {
	return spinBatch(fmt.Sprintf(" old := \"x\"\n_ = old\n_ = choir.Context()\nprintln(\"advance-a%d\")\n", n),
		fmt.Sprintf(`{"stdout":"advance-a%d\n","stderr":"","exit_code":0,"receipt_ref":"capsule-go-eval:sha256:%d"}`, n, n))
}

func TestNoopSpinNudgesOnceThenEnds(t *testing.T) {
	d := &noopSpinDetector{}
	nudges := 0
	for n := 1; n <= noopSpinEndAt+3; n++ {
		count, nudge, stop := d.observe(tick(n))
		if nudge {
			nudges++
			if count != noopSpinNudgeAt {
				t.Fatalf("nudge at %d", count)
			}
		}
		if stop != (n >= noopSpinEndAt) {
			t.Fatalf("cell %d: stop=%v", n, stop)
		}
	}
	if nudges != 1 {
		t.Fatalf("nudges = %d, want exactly 1", nudges)
	}
	if !strings.Contains(noopSpinNudge(6), "6 cells in a row changed nothing") || noopSpinError(12).Error() != "engineer spun 12 no-op cells" {
		t.Fatal("nudge or fate text")
	}
}

func TestNoopSpinResetsOnEffect(t *testing.T) {
	cases := map[string]string{
		"job intent":    `{"stdout":"x","exit_code":0,"intents":[{"kind":"job"}],"staged_intent_ids":["rlm:job:0"]}`,
		"job park":      `{"stdout":"x","exit_code":0,"job_park":"job-1","jobs_awaited":["job-1"]}`,
		"error":         `{"stdout":"x","exit_code":0,"error":"boom"}`,
		"foreign shape": `{"result":{"stdout":"x"}}`,
		"nonzero":       `{"stdout":"x","exit_code":1}`,
		"freeze":        `{"stdout":"x","exit_code":0,"freeze_result":{"ok":true}}`,
		"fate":          `{"stdout":"x","exit_code":0,"fate_terminal":true}`,
		"job started":   `{"stdout":"x","exit_code":0,"job_results":[{"op":"start"}]}`,
	}
	for name, out := range cases {
		d := &noopSpinDetector{}
		for n := 1; n < noopSpinNudgeAt; n++ {
			d.observe(tick(n))
		}
		if c, _, _ := d.observe(spinBatch(`println("a1")`, out)); c != 0 {
			t.Fatalf("%s: count %d, want reset", name, c)
		}
		if c, _, _ := d.observe(tick(99)); c != 1 {
			t.Fatalf("%s: count after reset %d", name, c)
		}
	}
}

func TestNoopSpinDifferentWorkResets(t *testing.T) {
	d := &noopSpinDetector{}
	for n := 1; n < noopSpinNudgeAt; n++ {
		d.observe(tick(n))
	}
	// A real read cell: different source and different output.
	if c, nudge, _ := d.observe(spinBatch(`r,_ := choir.Exec("ls"); println(r.Stdout)`, `{"stdout":"a.go\n","exit_code":0,"receipt_ref":"r"}`)); c != 1 || nudge {
		t.Fatalf("count %d nudge %v", c, nudge)
	}
	// The same exec poll whose output changes in words is not a tick.
	d = &noopSpinDetector{}
	for _, w := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"} {
		if _, _, stop := d.observe(spinBatch(`r,_ := choir.Exec("ls"); println(r.Stdout)`, `{"stdout":"`+w+`","exit_code":0}`)); stop {
			t.Fatal("a changing poll ended the run")
		}
	}
}

func TestNoopSpinOtherToolResets(t *testing.T) {
	d := &noopSpinDetector{}
	for n := 1; n < noopSpinNudgeAt; n++ {
		d.observe(tick(n))
	}
	if c, _, _ := d.observe([]types.ToolCall{{Name: "other"}}, []types.ToolResult{{}}); c != 0 {
		t.Fatalf("count %d", c)
	}
}
