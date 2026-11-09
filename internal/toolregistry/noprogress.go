package toolregistry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The no-progress stop rule (station L6 of the ordinary prompt path,
// docs/problems/ordinary-prompt-path-2026-11-09.md section 2.2). A run that
// makes the same tool call and gets the same result noRepeatLimit times in a
// window of recent steps is told once that the result will not change; if
// it repeats the same step noRepeatLimit more times it is stopped with
// ErrNoProgress, a visible failed fate. There is no iteration cap and no
// setting: only an identical call with an identical result counts, so a poll
// whose result changes, or any distinct call, is progress.
//
// K=4: a legitimate verify-and-retry or a poll that settles is seen to
// repeat once or twice; the flint loop repeated 60+ times. Four identical
// (call, result) pairs carry no new information for the model, and the
// notice plus four more is a few calls of waste at worst, against hundreds.
const (
	noRepeatLimit  = 4
	noRepeatWindow = 3 * noRepeatLimit
)

// ErrNoProgress marks a run stopped because it repeated one step, the same
// call with the same result, and did not change course after being told.
// It is not a provider error: it never fails over a model or demotes one.
var ErrNoProgress = errors.New("tool loop made no progress")

// NoProgressError carries the repeated tool and its count.
type NoProgressError struct {
	Tool  string
	Count int
}

func (e *NoProgressError) Error() string {
	return fmt.Sprintf("stopped: %s repeated %d times with the same result", e.Tool, e.Count)
}

func (e *NoProgressError) Unwrap() error { return ErrNoProgress }

type repeatTracker struct {
	step    int
	seen    map[string][]int // step numbers of a fingerprint inside the window
	total   map[string]int   // occurrences of a fingerprint in the run
	noticed map[string]bool
}

type repeatVerdict int

const (
	repeatOK repeatVerdict = iota
	repeatNotice
	repeatStop
)

func canonicalArgs(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.TrimSpace(string(raw))
	}
	b, err := json.Marshal(v) // map keys are sorted
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(b)
}

// cellOutputKeys are the fields of a desk or capsule cell result that carry
// what the cell did; everything else (duration, session label, receipts,
// reuse) differs per call and is left out of the fingerprint.
// A desk cell result carries only the count of staged intents, not their
// content (tools_desk.go), so a cell that commits different intents each call
// with identical stdout fingerprints the same; stdout normally shows the
// difference. This is a known limit (TestNoProgressCellIntentCountLimit).
var cellOutputKeys = []string{"stdout", "stderr", "error", "exit_code", "diag", "intents", "searches", "calibration_restate"}

// volatileKey names a per-call field in a structured result. Only known
// clock and per-call counters: a general *_at suffix is kept (created_at and
// updated_at can mark real change; missing a loop is safer than a false stop).
func volatileKey(k string) bool {
	k = strings.ToLower(k)
	switch k {
	case "session", "receipts", "receipt_ref", "receipt_refs", "timestamp", "request_id", "seq", "started_at", "finished_at":
		return true
	}
	return strings.HasPrefix(k, "duration") || strings.HasPrefix(k, "elapsed") || strings.HasSuffix(k, "_ms")
}

// durationText matches elapsed figures embedded in error and plain text:
// 123ms, 1.5s, 2m3s, "after 4512 ms". The message the model sees is
// unchanged; only the fingerprint masks them.
var durationText = regexp.MustCompile(`\b(?:\d+(?:\.\d+)?\s?(?:ns|µs|us|ms|s|m|h))+\b`)

func maskDurations(s string) string { return durationText.ReplaceAllString(s, "<dur>") }

func stripVolatile(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if !volatileKey(k) {
				out[k] = stripVolatile(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = stripVolatile(val)
		}
		return out
	}
	return v
}

// resultFingerprintText reduces a tool result to its stable content and
// reports whether the result says a job is still running (a wait, not a
// repeat).
func resultFingerprintText(output string) (string, bool) {
	var obj map[string]any
	if json.Unmarshal([]byte(output), &obj) != nil || obj == nil {
		return maskDurations(output), false
	}
	running := false
	if v, ok := obj["job_park"]; ok && v != nil && v != "" {
		running = true
	}
	if v, ok := obj["jobs_awaited"].([]any); ok && len(v) > 0 {
		running = true
	}
	if _, isCell := obj["stdout"]; isCell {
		keep := map[string]any{}
		for _, k := range cellOutputKeys {
			if val, ok := obj[k]; ok {
				if str, ok := val.(string); ok && k == "error" {
					val = maskDurations(str)
				}
				keep[k] = val
			}
		}
		b, _ := json.Marshal(keep)
		return string(b), running
	}
	b, _ := json.Marshal(stripVolatile(obj))
	return maskDurations(string(b)), running
}

func stepFingerprint(calls []types.ToolCall, results []types.ToolResult) (fp, tool string, running bool) {
	h := sha256.New()
	names := make([]string, 0, len(calls))
	byID := map[string]types.ToolResult{}
	for _, r := range results {
		byID[r.CallID] = r
	}
	for i, c := range calls {
		names = append(names, c.Name)
		var buf bytes.Buffer
		buf.WriteString(c.Name)
		buf.WriteByte(0)
		buf.WriteString(canonicalArgs(json.RawMessage(c.Arguments)))
		buf.WriteByte(0)
		r, ok := byID[c.ID]
		if !ok && i < len(results) {
			r = results[i]
		}
		text, run := resultFingerprintText(r.Output)
		running = running || run
		fmt.Fprintf(&buf, "%t\x00%s\x00", r.IsError, text)
		sum := sha256.Sum256(buf.Bytes())
		h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil)), strings.Join(names, ","), running
}

// observe records one executed step and returns what the loop should do.
// Both the notice and the stop need noRepeatLimit occurrences inside the
// last noRepeatWindow steps; the notice clears the count, so the stop is
// noRepeatLimit more inside a window, not any later four.
func (t *repeatTracker) observe(calls []types.ToolCall, results []types.ToolResult) (repeatVerdict, string, int) {
	if len(calls) == 0 {
		return repeatOK, "", 0
	}
	if t.seen == nil {
		t.seen, t.total, t.noticed = map[string][]int{}, map[string]int{}, map[string]bool{}
	}
	t.step++
	fp, tool, running := stepFingerprint(calls, results)
	if running {
		return repeatOK, tool, 0
	}
	t.total[fp]++
	in := t.seen[fp][:0:0]
	for _, s := range t.seen[fp] {
		if s > t.step-noRepeatWindow {
			in = append(in, s)
		}
	}
	in = append(in, t.step)
	t.seen[fp] = in
	if len(in) < noRepeatLimit {
		return repeatOK, tool, 0
	}
	t.seen[fp] = nil
	if t.noticed[fp] {
		return repeatStop, tool, t.total[fp]
	}
	t.noticed[fp] = true
	return repeatNotice, tool, t.total[fp]
}

func noProgressNoticeText(tool string, n int) string {
	return fmt.Sprintf("Runtime notice: you made the same %s call with the same result %d times. The result will not change. Do something different, or finish with what you have.", tool, n)
}
