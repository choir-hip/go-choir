package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func rmBody(t testing.TB, e computerevent.RunMemoryEntryProjection) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustCanonicalID(t testing.TB, kind, owner, suffix string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(kind), owner, suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTapeWriteMapsStayWithinTheKeyCap(t *testing.T) {
	var m tapeWriteMaps
	for i := 0; i < 300; i++ {
		m.add(tapeEventTally{
			family: fmt.Sprintf("object:k%d", i), payloadBytes: 10,
			objKinds: []string{fmt.Sprintf("k%d", i)}, objBytes: []int64{5},
			rmKeys: []string{fmt.Sprintf("message/r%d", i)},
		})
	}
	for name, mp := range map[string]map[string]int64{"events": m.eventsByFamily, "bytes": m.bytesByFamily, "ops": m.opsByObjectKind, "opbytes": m.opBytesByObject, "rm": m.runMemoryByKind} {
		if len(mp) > maxCounterKeys {
			t.Errorf("%s has %d keys, want at most %d", name, len(mp), maxCounterKeys)
		}
		if mp["other"] == 0 {
			t.Errorf("%s has no other bucket: %v", name, mp)
		}
	}
	if got := sumCounts(m.eventsByFamily); got != 300 {
		t.Errorf("events lost to the cap: %d, want 300", got)
	}
	if got := sumCounts(m.bytesByFamily); got != 3000 {
		t.Errorf("bytes lost to the cap: %d, want 3000", got)
	}
}

func TestRunMemoryKeyReadsKindAndReasonWithoutDecoding(t *testing.T) {
	tricky := computerevent.RunMemoryEntryProjection{
		EntryID: "e1", Kind: "compaction", Reason: "threshold",
		MessageJSON: `{"kind":"evil","reason":"evil2","x":"a\"kind\":\"evil3\""}`,
		DetailsJSON: `{"reason":"evil4"}`,
	}
	cases := []struct {
		name string
		body []byte
		want string
	}{
		{"kind and reason", rmBody(t, tricky), "compaction/threshold"},
		{"kind only", rmBody(t, computerevent.RunMemoryEntryProjection{EntryID: "e2", Kind: "message", MessageJSON: `{"kind":"evil"}`}), "message"},
		{"malformed", []byte(`{not json`), "unknown"},
		{"empty", nil, "unknown"},
		{"no kind", []byte(`{"entry_id":"x"}`), "unknown"},
		{"escaped value", []byte("{\"kind\":\"a\\nb\"}"), "unknown"},
		{"absurd length", []byte(`{"kind":"` + strings.Repeat("k", 500) + `"}`), "unknown"},
	}
	for _, c := range cases {
		if got := runMemoryKey(c.body); got != c.want {
			t.Errorf("%s: runMemoryKey = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTallyTapeEventFamilies(t *testing.T) {
	rm := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: "e1", Body: rmBody(t, computerevent.RunMemoryEntryProjection{EntryID: "e1", Kind: "message", Reason: "actor_rewarm"})}
	edge := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObjectEdge, CanonicalID: "edge-1", Body: json.RawMessage(`{}`)}
	obj := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: mustCanonicalID(t, "choir.event", "owner", "abc"), Body: json.RawMessage(`{"a":1}`)}
	bad := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: "not-canonical"}

	if got := tallyTapeEvent(projectionAuthority, []computerevent.ProjectionOp{rm}, 10); got.family != "run_memory" || len(got.rmKeys) != 1 || got.rmKeys[0] != "message/actor_rewarm" {
		t.Errorf("run memory tally = %+v", got)
	}
	if got := tallyTapeEvent(projectionAuthority, []computerevent.ProjectionOp{edge}, 10); got.family != "edge" {
		t.Errorf("edge tally = %+v", got)
	}
	if got := tallyTapeEvent(projectionAuthority, []computerevent.ProjectionOp{obj}, 10); got.family != "object:choir.event" || got.objBytes[0] != 7 {
		t.Errorf("object tally = %+v", got)
	}
	if got := tallyTapeEvent(projectionAuthority, []computerevent.ProjectionOp{bad}, 10); got.family != "object:unknown" {
		t.Errorf("unparseable id tally = %+v", got)
	}
	if got := tallyTapeEvent(telemetryProjectionAuthority, []computerevent.ProjectionOp{obj}, 10); got.family != "telemetry" {
		t.Errorf("telemetry tally = %+v", got)
	}
}

func TestTapeWriteSummaryClampsBackwardReadings(t *testing.T) {
	prev := tapeWriteTotals{Events: 300, Ops: 450, Bytes: 5000, Calls: 1200}
	cur := tapeWriteTotals{Events: 310, Ops: 440, Bytes: 4000, Calls: 5}
	line := formatTapeWriteSummary(time.Hour, prev, cur)
	if strings.Contains(line, "=-") {
		t.Errorf("a negative delta in %q", line)
	}
	if !strings.Contains(line, "calls_per_event=0.00") || !strings.Contains(line, "ops=0") || !strings.Contains(line, "bytes=0") {
		t.Errorf("summary = %q", line)
	}
}

// BenchmarkRunMemoryKey compares the scan the counters use with a full decode
// of the same body.
func BenchmarkRunMemoryKey(b *testing.B) {
	body, _ := json.Marshal(computerevent.RunMemoryEntryProjection{EntryID: "e1", Kind: "message", Reason: "actor_rewarm", MessageJSON: strings.Repeat(`{"content":"x \"kind\":\"y\""}`, 200), DetailsJSON: "{}"})
	b.Run("scan", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		for i := 0; i < b.N; i++ {
			_ = runMemoryKey(body)
		}
	})
	b.Run("full_unmarshal", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		for i := 0; i < b.N; i++ {
			var head struct {
				Kind   string `json:"kind"`
				Reason string `json:"reason"`
			}
			_ = json.Unmarshal(body, &head)
		}
	})
}

func TestTapeWriteCountersSkipEmptyAndUnboundCalls(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	before := h.store.TelemetryStats()
	if err := h.store.projectionTape.appendOpsAs(context.Background(), nil, projectionAuthority); err != nil {
		t.Fatal(err)
	}
	var unbound *projectionTape
	_ = unbound.appendOpsAs(context.Background(), nil, projectionAuthority)
	after := h.store.TelemetryStats()
	if sumCounts(after.AppenderDepthAtEnter) != sumCounts(before.AppenderDepthAtEnter) || after.TapeEvents != before.TapeEvents {
		t.Errorf("an empty write was counted: %v -> %v", before.AppenderDepthAtEnter, after.AppenderDepthAtEnter)
	}
}

// The hourly summary goroutine follows the tape: started by the bind, kept by
// a rebind, stopped by Close, started again by Reopen, stopped by Close.
func TestTapeWriteSummaryLifecycle(t *testing.T) {
	h := newTapeHarness(t)
	running := func() (chan struct{}, bool) {
		h.store.tapeSummary.mu.Lock()
		defer h.store.tapeSummary.mu.Unlock()
		return h.store.tapeSummary.done, h.store.tapeSummary.cancel != nil
	}
	first, ok := running()
	if !ok {
		t.Fatal("the bind did not start the summary loop")
	}
	h.bind() // a rebind
	if again, ok := running(); !ok || again != first {
		t.Fatal("a rebind must keep the one running loop")
	}
	if err := h.store.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not stop the summary loop")
	}
	if _, ok := running(); ok {
		t.Fatal("loop state not cleared by Close")
	}
	if err := h.store.Reopen(h.path); err != nil {
		t.Fatal(err)
	}
	second, ok := running()
	if !ok || second == first {
		t.Fatal("Reopen did not start a fresh loop")
	}
	if err := h.store.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-second:
	case <-time.After(5 * time.Second):
		t.Fatal("the second Close did not stop the loop")
	}
}
