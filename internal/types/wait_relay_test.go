package types

import (
	"testing"
	"time"
)

// Failure modes written before the code:
//  1. the wait Note is not a progress relay, so Texture is never woken and
//     the record is classed as a closure;
//  2. a repeated check mints a new id (one line per check, not per wait);
//  3. the start and end of one episode share an id (the end is dropped as
//     a replay of the start);
//  4. a relay is minted for an inbox that is not a Texture, or from a
//     sender that is not an engineering assignment.

func TestEngineeringWaitRelay(t *testing.T) {
	at := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	start, ok := NewEngineeringWaitRelay("engineering:a-1", "texture:t-1", "c-1", 2, "1", WaitRelayStart, "Waiting 1 min for host memory", at, map[string]string{"wait_kind": "host_memory"})
	if !ok || !IsEngineeringProgressRelay(start) {
		t.Fatalf("1: start relay = %+v ok=%v", start, ok)
	}
	again, _ := NewEngineeringWaitRelay("engineering:a-1", "texture:t-1", "c-1", 2, "1", WaitRelayStart, "Waiting 2 min for host memory", at.Add(time.Minute), nil)
	if again.RecordID != start.RecordID {
		t.Fatalf("2: one wait episode is one record: %s vs %s", again.RecordID, start.RecordID)
	}
	end, _ := NewEngineeringWaitRelay("engineering:a-1", "texture:t-1", "c-1", 2, "1", WaitRelayEnd, "Waited 9 min for host memory", at, nil)
	next, _ := NewEngineeringWaitRelay("engineering:a-1", "texture:t-1", "c-1", 2, "2", WaitRelayStart, "x", at, nil)
	if end.RecordID == start.RecordID || next.RecordID == start.RecordID {
		t.Fatalf("3: ids collide: %s %s %s", start.RecordID, end.RecordID, next.RecordID)
	}
	if start.SubjectRefs["wait_episode"] == "" || start.SubjectRefs["wait_phase"] != WaitRelayStart || start.SubjectRefs["wait_kind"] != "host_memory" {
		t.Fatalf("subject refs = %v", start.SubjectRefs)
	}
	if _, ok := NewEngineeringWaitRelay("engineering:a-1", "management:m", "c-1", 1, "1", WaitRelayStart, "x", at, nil); ok {
		t.Fatal("4: a non-Texture inbox")
	}
	if _, ok := NewEngineeringWaitRelay("research:r", "texture:t-1", "c-1", 1, "1", WaitRelayStart, "x", at, nil); ok {
		t.Fatal("4: a non-engineering sender")
	}
	if _, ok := NewEngineeringWaitRelay("engineering:a-1", "texture:t-1", "c-1", 1, "1", WaitRelayStart, " ", at, nil); ok {
		t.Fatal("an empty body")
	}
}
