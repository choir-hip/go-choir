package actor

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// Failure list, written before the code (docs/problems/paper-dispatcher-twelve-slots-full-ninety-due-2026-11-20.md):
//   - the line omits a field the problem record asks for, or prints the id's
//     document text as the agent kind;
//   - the incorporated count counts updates the commit did not incorporate;
//   - an activation over the threshold is sampled away;
//   - fast activations write one line each (volume), or none (lost);
//   - the observation changes what is delivered.

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestAgentKindIsThePrefixBeforeTheColonOnly(t *testing.T) {
	for in, want := range map[string]string{
		"owner\x00comp\x00texture:doc-secret text": "texture",
		"research:abc": "research",
		"management":   "management",
		"":             "unknown",
	} {
		if got := agentKindOf(in); got != want {
			t.Errorf("agentKindOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestActivationDoneLineCarriesTheFields(t *testing.T) {
	start := time.Now()
	rec := activationRecord{
		agentID: "o\x00c\x00texture:doc1",
		started: start,
		pending: []Update{
			{UpdateID: "a", Kind: "inbox_record", CreatedAt: start.Add(-4 * time.Minute)},
			{UpdateID: "b", Kind: "inbox_record", CreatedAt: start.Add(-time.Minute)},
			{UpdateID: "c", Kind: "cell_terminal_deadline", CreatedAt: start.Add(-time.Hour), NotBefore: start.Add(-2 * time.Minute)},
			{UpdateID: "d", Kind: "inbox_record", CreatedAt: start.Add(-time.Second)}, // failed: not incorporated
		},
		incorporated: []string{"a", "b", "c"},
		handled:      []handledUpdate{{"inbox_record", 300 * time.Millisecond}, {"cell_terminal_deadline", 2 * time.Second}},
		commit:       40 * time.Millisecond,
		skipCause:    "noroom",
		slotWait:     90 * time.Second,
	}
	line := activationDoneLine(rec, 3*time.Second, start.Add(3*time.Second))
	for _, want := range []string{
		"dispatcher: activation done", "agent=doc1", "kind=texture", "dur=3s", "slot_wait=1m30s", "slot_wait_cause=noroom",
		"due_age=4m0s", "commit=40ms", "updates=4", "incorporated=3", "by_kind=inbox_record:2,cell_terminal_deadline:1",
		"slowest=cell_terminal_deadline:2s",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "doc1") && strings.Contains(line, "kind=doc1") {
		t.Errorf("the kind carries the id: %q", line)
	}
}

func TestSlowActivationLogsItsOwnLineAndFastOnesJoinTheSummary(t *testing.T) {
	buf := captureLog(t)
	old := activationLogOver
	activationLogOver = 80 * time.Millisecond
	t.Cleanup(func() { activationLogOver = old })

	l := openKernelLog(t)
	now := time.Now().UTC()
	appendAt(t, l, "slow1", "o\x00c\x00texture:slow", now.Add(-time.Second), time.Time{})
	appendAt(t, l, "fast1", "o\x00c\x00research:fast", now.Add(-time.Second), time.Time{})
	appendAt(t, l, "fast2", "o\x00c\x00research:fast2", now.Add(-time.Second), time.Time{})
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, mem []byte) ([]byte, error) {
		if strings.HasPrefix(u.UpdateID, "slow") {
			time.Sleep(150 * time.Millisecond)
		}
		return mem, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 5 * time.Millisecond})
	d.dispatchPending(context.Background())
	d.wg.Wait()

	out := buf.String()
	if n := strings.Count(out, "activation done"); n != 1 || !strings.Contains(out, "kind=texture") || !strings.Contains(out, "incorporated=1") {
		t.Fatalf("want exactly the slow activation's line, got %d in:\n%s", n, out)
	}
	if strings.Contains(out, "agent=fast") {
		t.Fatalf("a fast activation logged its own line:\n%s", out)
	}
	d.stats.flushIfDue(time.Now().Add(time.Hour), time.Minute)
	out = buf.String()
	for _, want := range []string{"activation summary", "activations=2", "research n=2", "k n=2", "p50=", "p90=", "max="} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	// A second flush with nothing counted writes nothing.
	before := buf.Len()
	d.stats.flushIfDue(time.Now().Add(2*time.Hour), time.Minute)
	if buf.Len() != before {
		t.Errorf("an empty window wrote a summary")
	}
}

func TestSummaryQuantiles(t *testing.T) {
	v := map[string][]time.Duration{"x": {1 * time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 4 * time.Millisecond, 100 * time.Millisecond}}
	line := activationSummaryLine(time.Minute, v, v)
	for _, want := range []string{"window=1m0s", "activations=5", "x n=5 p50=3ms p90=100ms max=100ms"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q lacks %q", line, want)
		}
	}
}
