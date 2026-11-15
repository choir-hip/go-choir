package agentcore

import (
	"testing"
	"time"
)

// Review round 3: after a restart the trigger's tail starts at the capture's
// head (persisted), not at the snapshot event's sequence, which is the events
// appended during the capture later (the paper's is about 7,500 events).
func TestRestartTailStartsAtTheCaptureHead(t *testing.T) {
	h, clock := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-14*time.Hour), 1_007_500
	if err := saveSnapshotCapture(h.deps.DataDir, snapshotCaptureRecord{
		ComputerID: h.deps.ComputerID, StartedAt: h.newestAt.Add(-37 * time.Minute), DurationSeconds: (37 * time.Minute).Seconds(),
		SealedBytes: 4_800_000_000, HeadSequence: 1_000_000,
	}); err != nil {
		t.Fatal(err)
	}
	h.head = 1_000_000 + 30_000 // 22,500 past the event, 30,000 past the capture head
	var p snapshotProducer
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s detail=%q: 30,000 events past the capture head is over the 27,120 bound", outcome, detail)
	}
}
