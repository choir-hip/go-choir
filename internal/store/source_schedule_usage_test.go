package store

import (
	"strings"
	"testing"
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
