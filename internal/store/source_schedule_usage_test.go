package store

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// A poll's error text is bounded before the schedule keeps it: an upstream
// error page must not be rewritten in full on every poll
// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
func TestRecordPollBoundsErrorText(t *testing.T) {
	var s SourceSchedule
	s.RecordPoll(SourcePollUsage{Status: "error", Error: strings.Repeat("é", 4<<20)})
	got := s.Recent[0].Error
	if len(got) > sourcePollErrorLimit+len("…") || !utf8.ValidString(got) {
		t.Fatalf("saved error is %d bytes, valid utf8 %v", len(got), utf8.ValidString(got))
	}
	if s.Totals.Errors != 1 {
		t.Fatalf("errors = %d", s.Totals.Errors)
	}
}

// The crash-loop bound of a standing schedule (ruling 6, 2026-10-14) lives
// on the schedule, so it must survive the write a crash boot reads back:
// the in-flight mark and the count of crash boots in a row.
func TestSourceScheduleKeepsTheCrashBoundAcrossAWrite(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	since := time.Date(2026, 10, 14, 5, 48, 10, 0, time.UTC)
	want := SourceSchedule{OwnerID: "owner-1", ComputerID: "computer-paper", SourceID: "rss:bound",
		State: SourceScheduleActive, Generation: 8, PollInFlightSince: since, CrashesWithPollInFlight: 1}
	if err := s.PutSourceSchedule(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSourceSchedule(ctx, "owner-1", "rss:bound")
	if err != nil {
		t.Fatal(err)
	}
	if !got.PollInFlightSince.Equal(since) || got.CrashesWithPollInFlight != 1 {
		t.Fatalf("read back in flight %s, crashes %d; want %s and 1", got.PollInFlightSince, got.CrashesWithPollInFlight, since)
	}
}
