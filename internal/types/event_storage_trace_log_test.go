package types

import (
	"sort"
	"strings"
	"testing"
)

// R7 slice S2: the third storage class (docs/design/trace-segment-log-2026-10-22.md
// 2.2). Ways it can fail, written before the code:
//
//   - a kind that acceptance, the ledger or a boundary reads (an acknowledged
//     kind) lands in the class, and so, once S4 writes the log, leaves the tape;
//   - loop.compaction.completed, which run acceptance reads and which is
//     low-volume, is taken by the log with the other trace kinds;
//   - the class leaks to a kind nobody named (the default must stay the object
//     graph), or the set drifts from the designed nine without a test failing;
//   - the F-c writer stops stamping the nine kinds because their class changed
//     name: until the log writes, the event table is the sink they fall back to.

var designedTraceLogKinds = []string{
	"loop.delta", "loop.progress", "loop.retry", "loop.compaction.started",
	"tool.invoked", "tool.result", "desk.cell", "desk.egress", "texture.agent_revision.progress",
}

func TestTraceLogClassIsTheDesignedNine(t *testing.T) {
	var got []string
	for kind, class := range eventStorage {
		if class == StorageTraceLog {
			got = append(got, string(kind))
		}
	}
	sort.Strings(got)
	want := append([]string(nil), designedTraceLogKinds...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("trace_log kinds = %v\nwant %v (design trace-segment-log section 2.1; changing this set needs the design)", got, want)
	}
}

// Only batched kinds may be in the class, and loop.compaction.completed stays
// on the tape: run acceptance reads it as a checkpoint.
func TestTraceLogClassHoldsOnlyTelemetryAndNotCompactionCompleted(t *testing.T) {
	for _, kind := range EventKinds() {
		if kind.Storage() == StorageTraceLog && !kind.Batchable() {
			t.Errorf("%s is acknowledged but would leave the tape for the trace log", kind)
		}
	}
	if got := EventRunCompactionCompleted.Storage(); got != StorageEventTable {
		t.Fatalf("loop.compaction.completed storage = %q, want event_table", got)
	}
}

func TestTraceLogClassNeverLeaksToAnUnnamedKind(t *testing.T) {
	for _, kind := range []EventKind{"", "loop.unknown", "TOOL.RESULT", "tool.result ", "tool.result.v2"} {
		if got := kind.Storage(); got != StorageObjectGraph {
			t.Errorf("Storage(%q) = %q, want object_graph", kind, got)
		}
		if kind.EventTableEligible() {
			t.Errorf("EventTableEligible(%q) = true for an unnamed kind", kind)
		}
	}
}

// Until the log writes (S4), a trace-log kind falls back to the event table, so
// the F-c writer's set is the old ten: the nine and loop.compaction.completed.
func TestEventTableEligibleIsTheOldTenWhileTheLogDoesNotWrite(t *testing.T) {
	var got []string
	for _, kind := range EventKinds() {
		if kind.EventTableEligible() {
			got = append(got, string(kind))
		}
	}
	sort.Strings(got)
	want := append(append([]string(nil), designedTraceLogKinds...), "loop.compaction.completed")
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("event-table eligible kinds = %v\nwant %v", got, want)
	}
	for _, kind := range EventKinds() {
		if kind.EventTableEligible() != kind.Batchable() {
			t.Errorf("%s: eligible %t but batchable %t", kind, kind.EventTableEligible(), kind.Batchable())
		}
	}
}
