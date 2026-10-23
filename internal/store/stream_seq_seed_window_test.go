package store

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// The windows of the stream_seq seed: how far back through the newest
// objects of any kind it looks for an event, and what it does when it finds
// none. Ways it can fail, written before the code:
//  1. The first window holds an event but the seed keeps widening, or reads
//     more than one window: the cost is no longer the window's.
//  2. A newest event behind more other-kind objects than the first window
//     is missed: the widening does not happen.
//  3. A store with no events at all pays for the widest window and then
//     warns, or guesses a number other than 0.
//  4. Events exist but sit behind more objects than every window: the seed
//     is silent. It must say so, and still seed from the event table.

func shrinkSeedWindows(t *testing.T, windows ...int) {
	t.Helper()
	old := streamSeedWindows
	streamSeedWindows = windows
	t.Cleanup(func() { streamSeedWindows = old })
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestStreamSeqSeedStopsAtTheFirstWindowWithAnEvent(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 8, 0)
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 6 {
		t.Fatalf("stream_seq = %d, want 6", rec.StreamSeq)
	}
	stats := s.EngineMutexStats()
	if calls := stats["(*DoltStore).ListNewestObjectHeaders"].Calls; calls != 1 {
		t.Fatalf("the seed read %d windows, want the first only", calls)
	}
	if calls := stats["(*DoltStore).HasObjectOfKind"].Calls; calls != 0 {
		t.Fatalf("the seed probed for the kind %d times after finding an event", calls)
	}
}

func TestStreamSeqSeedWidensPastOtherKinds(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 60, 0) // behind the first window, inside the second
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 6 {
		t.Fatalf("stream_seq = %d, want 6", rec.StreamSeq)
	}
	if calls := s.EngineMutexStats()["(*DoltStore).ListNewestObjectHeaders"].Calls; calls != 2 {
		t.Fatalf("the seed read %d windows, want 2", calls)
	}
}

func TestStreamSeqSeedWithNoEventsIsQuiet(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	seedWakeOutbox(t, s, 300, 0)
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC())
	if rec.StreamSeq != 1 {
		t.Fatalf("stream_seq = %d, want 1", rec.StreamSeq)
	}
	if strings.Contains(buf.String(), "stream_seq seed") {
		t.Fatalf("a store without events warned: %s", buf.String())
	}
}

func TestStreamSeqSeedSaysWhenEventsAreOutOfReach(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 400, 0) // behind every window
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if !strings.Contains(buf.String(), "stream_seq seed") || !strings.Contains(buf.String(), "out of reach") {
		t.Fatalf("events exist beyond every window and the seed did not say so:\n%s", buf.String())
	}
}
