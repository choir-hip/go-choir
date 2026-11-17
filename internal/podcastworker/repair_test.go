package podcastworker

// Segment refusals and the download that never left the first address.
// Failure list, written first (worker.log of 2026-11-16: 57 failed lines, 45
// of them the server's 400 "segment N is empty, over 2000 bytes, ends before it
// starts, or is out of order" on long interviews, retried each time with the
// same transcript; 5 downloads refused on the first IPv6 address):
//   R1. a segment over 2000 bytes (a long run with no sentence end and no
//       pause, or words that share one timestamp) is sent whole;
//   R2. a zero or negative length segment or one that starts before the one
//       before ended is sent as it is;
//   R3. an empty segment is sent;
//   R4. a repaired transcript still fails the server's rules (ValidateSegments
//       mirrors them), or the repair changes the words;
//   R5. a refusal that remains is logged without the episode, the segment's
//       index, its bounds and the reason;
//   R6. the server's 400 on an unchanged transcript is retried after the
//       short backoff, with a full download and transcription each time;
//   R7. the guarded dialer tries only the first resolved address, so a host
//       whose first (IPv6) address refuses cannot be reached.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func words(n int, at float64, step float64) []Word {
	var w []Word
	for i := 0; i < n; i++ {
		w = append(w, Word{W: fmt.Sprintf("word%d", i), Start: at + float64(i)*step, End: at + float64(i)*step + step/2})
	}
	return w
}

func TestR1_ASegmentOverTheByteLimitIsSplitOnWordBoundaries(t *testing.T) {
	// 600 words that all carry one timestamp, no punctuation: Segments cannot cut it.
	var ws []Word
	for i := 0; i < 600; i++ {
		ws = append(ws, Word{W: fmt.Sprintf("long%d", i), Start: 10, End: 10.05})
	}
	raw := Segments(ws)
	if len(raw) != 1 || len(raw[0].Text) <= MaxSegmentBytes {
		t.Fatalf("setup: expected one oversize segment, got %d of %d bytes", len(raw), len(raw[0].Text))
	}
	fixed, rep := RepairSegments(raw)
	if err := ValidateSegments(fixed); err != nil {
		t.Fatalf("R1: still invalid: %v", err)
	}
	if rep.Split == 0 {
		t.Fatal("R1: no split counted")
	}
	var back []string
	for _, s := range fixed {
		if len(s.Text) > MaxSegmentBytes || !utf8.ValidString(s.Text) {
			t.Fatalf("R1: segment of %d bytes", len(s.Text))
		}
		back = append(back, strings.Fields(s.Text)...)
	}
	if strings.Join(back, " ") != raw[0].Text {
		t.Fatal("R4: the repair changed the words")
	}
}

func TestR2_R3_OverlapsZeroLengthAndEmptyAreRepaired(t *testing.T) {
	in := []Segment{
		{0, 10, "First sentence."},
		{9, 20, "Overlaps the first by a second."}, // start before prevEnd-0.5
		{20, 20, "Zero length."},                   // end == start
		{19, 18, "Backwards."},                     // end < start
		{30, 31, "   "},                            // empty
		{40, 45, "Fine."},
		{44, 50, "Starts inside the one before."},
	}
	fixed, rep := RepairSegments(in)
	if err := ValidateSegments(fixed); err != nil {
		t.Fatalf("R2: invalid after repair: %v (%v)", err, fixed)
	}
	if rep.Dropped < 1 {
		t.Fatalf("R3: empty not dropped: %+v", rep)
	}
	got := ""
	for _, s := range fixed {
		got += s.Text + "|"
	}
	for _, w := range []string{"First sentence.", "Overlaps the first by a second.", "Zero length.", "Fine.", "Starts inside the one before."} {
		if !strings.Contains(got, w) {
			t.Errorf("R4: words lost: %q not in %q", w, got)
		}
	}
}

func TestR4_ValidateMirrorsTheServer(t *testing.T) {
	cases := map[string][]Segment{
		"empty":     {{0, 1, " "}},
		"backwards": {{5, 5, "x"}},
		"disorder":  {{0, 10, "a"}, {2, 12, "b"}},
		"long":      {{0, 1, strings.Repeat("a", MaxSegmentBytes+1)}},
		"none":      nil,
	}
	for name, segs := range cases {
		if err := ValidateSegments(segs); err == nil {
			t.Errorf("R4: %s accepted", name)
		}
	}
	if err := ValidateSegments([]Segment{{0, 10, "a"}, {9.6, 12, "b"}}); err != nil {
		t.Errorf("a 0.4 s overlap is within the server's tolerance: %v", err)
	}
}

func TestR5_ARefusalNamesTheEpisodeSegmentAndReason(t *testing.T) {
	err := ValidateSegments([]Segment{{0, 10, "a"}, {12, 12, "b"}})
	if err == nil || !strings.Contains(err.Error(), "segment 1") || !strings.Contains(err.Error(), "12.0") || !strings.Contains(err.Error(), "ends before it starts") {
		t.Fatalf("R5: %v", err)
	}
}

type badEngine struct{ segs []Segment }

func (badEngine) Engine() string { return "fake" }
func (b badEngine) Transcribe(context.Context, string) (Result, error) {
	return Result{Segments: b.segs, Seconds: 60, FileSeconds: 60}, nil
}

func TestR5_ServeRepairsALongEpisodeAndStoresIt(t *testing.T) {
	var ws []Word
	for i := 0; i < 600; i++ {
		ws = append(ws, Word{W: fmt.Sprintf("long%d", i), Start: 10, End: 10.05})
	}
	b := &fakeBackend{pending: []PendingEpisode{ep("a")}}
	var stored []Segment
	b.onPut = func(r Result) { stored = r.Segments }
	_, logs := runServe(t, b, badEngine{Segments(ws)}, 1, ServeConfig{})
	if len(b.put) != 1 {
		t.Fatalf("R5: not stored: %v %v", b.put, logs)
	}
	if err := ValidateSegments(stored); err != nil {
		t.Fatalf("R5: stored segments invalid: %v", err)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "repaired") {
		t.Fatalf("R5: the repair is not logged: %v", logs)
	}
}

func TestR6_AServerRefusalOfATranscriptIsNotRetriedSoon(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a")}, putErr: &HTTPError{Method: "PUT", Path: "/x", Status: 400, Body: `{"error":"invalid"}`}}
	e := &fakeEngine{}
	clk := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	_ = clk
	_, logs := runServe(t, b, e, 4, ServeConfig{})
	if n := len(e.order); n != 1 {
		t.Fatalf("R6: the refused episode was transcribed %d times in the first hours", n)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "refused") {
		t.Fatalf("R6: no refusal logged: %v", logs)
	}
}

func TestR7_TheDialerTriesEveryPermittedAddress(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("2001:4860:4860::8888"), net.ParseIP("8.8.4.4")}, nil
	}
	var dialed []string
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		if strings.HasPrefix(addr, "[2001") {
			return nil, errors.New("connection refused")
		}
		c, _ := net.Pipe()
		return c, nil
	}
	d := guardedDialer(lookup, dial)
	c, err := d(context.Background(), "tcp", "ads.example.test:443")
	if err != nil {
		t.Fatalf("R7: %v (dialled %v)", err, dialed)
	}
	c.Close()
	if len(dialed) != 2 || dialed[1] != "8.8.4.4:443" {
		t.Fatalf("R7: dialled %v", dialed)
	}
	dial = func(_ context.Context, _, addr string) (net.Conn, error) {
		return nil, errors.New("refused " + addr)
	}
	d = guardedDialer(lookup, dial)
	if _, err := d(context.Background(), "tcp", "ads.example.test:443"); err == nil || !strings.Contains(err.Error(), "8.8.4.4") {
		t.Fatalf("R7: the error should name every address tried: %v", err)
	}
}

var _ = utf8.RuneError

func TestR8_TheLargestShiftIsReported(t *testing.T) {
	_, rep := RepairSegments([]Segment{{0, 10, "a b."}, {7.5, 12, "c d."}, {12, 20, "e f."}})
	if rep.Shifted != 1 || rep.MaxShift < 2.4 || rep.MaxShift > 2.6 || rep.LargeShifts != 1 || !strings.Contains(rep.String(), "largest 2.5 s") {
		t.Fatalf("R8: %+v %s", rep, rep)
	}
}
