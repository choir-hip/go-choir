package store

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// goldenTapeEvent is one tape event of the scripted workload with the only
// run-to-run noise removed: the tape event id (a fresh UUIDv7), its batch
// event id and digest, and the idempotency key derived from the id. Everything
// else the tape carries, the sequence, kind, authority, actor, privacy class,
// op kinds, canonical ids and op bodies (the choir.event objects, which carry
// fixed ids and timestamps), is compared byte for byte.
type goldenTapeEvent struct {
	Sequence     uint64          `json:"sequence"`
	Kind         string          `json:"kind"`
	Authority    string          `json:"authority"`
	ActorProfile string          `json:"actor_profile"`
	PrivacyClass string          `json:"privacy_class"`
	Ops          []goldenTapeOp  `json:"ops"`
	Extra        json.RawMessage `json:"-"`
}

type goldenTapeOp struct {
	Kind        string          `json:"kind"`
	CanonicalID string          `json:"canonical_id"`
	Body        json.RawMessage `json:"body"`
}

// scriptedWorkload is the flag-off workload. It touches every write path the
// sealer is designed around: batched kinds, semantic kinds, an ownerless
// event, two owners, two runs, run memory, and an event after a terminal
// event.
func scriptedWorkload(t *testing.T, h *tapeHarness) {
	t.Helper()
	ctx := context.Background()
	at := func(i int) time.Time { return telemetryBase.Add(time.Duration(i) * time.Second) }
	append1 := func(i int, id, run, owner string, kind types.EventKind) {
		t.Helper()
		if err := h.store.AppendEvent(ctx, h.event(id, run, owner, kind, at(i))); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}
	append1(1, "g-1", "run-a", "owner-1", types.EventRunSubmitted)
	append1(2, "g-2", "run-a", "owner-1", types.EventRunProgress)
	append1(3, "g-3", "run-a", "owner-1", types.EventToolInvoked)
	append1(4, "g-4", "run-a", "owner-1", types.EventToolResult)
	append1(5, "g-5", "run-a", "owner-1", types.EventRunDelta)
	if _, err := h.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		EntryID: "memory-golden", RunID: "run-a", OwnerID: "owner-1", AgentID: "agent-1",
		Kind: types.RunMemoryEntryMessage, Role: "user", CreatedAt: at(6),
	}); err != nil {
		t.Fatalf("run memory: %v", err)
	}
	append1(7, "g-7", "run-b", "owner-2", types.EventRunProgress)
	append1(8, "g-8", "run-a", "owner-1", types.EventRunCompleted)
	append1(9, "g-9", "run-a", "owner-1", types.EventChannelMessage)
	append1(10, "g-10", "run-a", "owner-1", types.EventRunProgress)
	append1(11, "g-11", "", "", types.EventRuntimeHealth)
}

func normalizedTape(t *testing.T, h *tapeHarness) []goldenTapeEvent {
	t.Helper()
	var out []goldenTapeEvent
	// The stream_seq lease batches are store bookkeeping and sit apart on the
	// harness tape (leaseSnapshot), so the workload's sequence is its position
	// beside the genesis event: the number the baseline tape had.
	for i, rec := range h.tape.snapshot() {
		ev := goldenTapeEvent{
			Sequence: uint64(i + 2), Kind: string(rec.Kind), Authority: rec.Authority,
			ActorProfile: rec.ActorProfile, PrivacyClass: rec.PrivacyClass,
		}
		for _, op := range h.tape.batchOps(t, rec) {
			ev.Ops = append(ev.Ops, goldenTapeOp{Kind: string(op.Kind), CanonicalID: op.CanonicalID, Body: op.Body})
		}
		out = append(out, ev)
	}
	return out
}

// TestFlagOffTapeIsByteIdentical (design 6.2 item 9): with batching off, the
// events a scripted workload puts on the tape are byte-identical to the
// events the unmodified code put there. The golden file was generated from
// the baseline commit before any production code changed, and regenerated
// once for the stream_seq lease (the event bodies carry stream_seq, which now
// starts at 2^20 + 1 and steps by a block per restart); set
// UPDATE_TELEMETRY_GOLDEN=1 only to regenerate it deliberately.
func TestFlagOffTapeIsByteIdentical(t *testing.T) {
	h := newTapeHarness(t)
	scriptedWorkload(t, h)
	got, err := json.MarshalIndent(normalizedTape(t, h), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	const golden = "testdata/telemetry_flag_off_tape.golden.json"
	if os.Getenv("UPDATE_TELEMETRY_GOLDEN") == "1" {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("golden regenerated")
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gl) && i < len(wl); i++ {
			if gl[i] != wl[i] {
				t.Fatalf("flag-off tape differs from the baseline golden at line %d:\n got: %s\nwant: %s", i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("flag-off tape differs from the baseline golden in length: got %d lines, want %d", len(gl), len(wl))
	}
}
