package podcastworker

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// Word is one word of parakeet.cpp's JSON output (`--timestamps --json`).
type Word struct {
	W     string  `json:"w"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ParakeetOutput is the part of the CLI's JSON this worker reads.
type ParakeetOutput struct {
	Text  string `json:"text"`
	Words []Word `json:"words"`
}

// Segment is a sentence or pause bounded stretch of speech.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// ParseParakeet decodes the CLI's output.
func ParseParakeet(raw []byte) (*ParakeetOutput, error) {
	var out ParakeetOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse parakeet output: %w", err)
	}
	if len(out.Words) == 0 {
		return nil, fmt.Errorf("parakeet output has no words")
	}
	return &out, nil
}

const (
	// pauseBreak is a silence that ends a segment even with no punctuation.
	pauseBreak = 0.9
	// softMax is the length past which a segment ends at the next small gap.
	softMax = 30.0
	// hardMax ends a segment whatever the gaps are.
	hardMax = 45.0
	// softGap is the gap that counts as a small gap past softMax.
	softGap = 0.25
)

// Segments cuts words into sentence or pause bounded segments: a segment
// ends after a word that ends a sentence, before a pause of 0.9 s or more,
// and, to bound its length, at a small gap past 30 s or at 45 s. Clip
// boundaries are chosen from these, so every clip starts and ends cleanly.
func Segments(words []Word) []Segment {
	var out []Segment
	var cur []Word
	flush := func() {
		if len(cur) == 0 {
			return
		}
		parts := make([]string, len(cur))
		for i, w := range cur {
			parts[i] = w.W
		}
		out = append(out, Segment{Start: cur[0].Start, End: cur[len(cur)-1].End, Text: strings.Join(parts, " ")})
		cur = nil
	}
	for i, w := range words {
		if w.End < w.Start || strings.TrimSpace(w.W) == "" {
			continue
		}
		cur = append(cur, w)
		next := i + 1
		var gap float64
		if next < len(words) {
			gap = words[next].Start - w.End
		}
		length := w.End - cur[0].Start
		switch {
		case next >= len(words):
			flush()
		case endsSentence(w.W):
			flush()
		case gap >= pauseBreak:
			flush()
		case length >= softMax && gap >= softGap:
			flush()
		case length >= hardMax:
			flush()
		}
	}
	flush()
	return out
}

func endsSentence(w string) bool {
	w = strings.TrimRight(w, `"')]}»”’`)
	return strings.HasSuffix(w, ".") || strings.HasSuffix(w, "?") || strings.HasSuffix(w, "!")
}

const (
	// MaxSegmentBytes is the server's ceiling on one segment's text
	// (internal/platform podcastMaxSegmentBytes); MaxSegments is its ceiling on
	// a transcript's segments (podcastMaxSegments).
	MaxSegmentBytes = 2000
	MaxSegments     = 40000
	// splitBytes is where a long segment is cut, under the ceiling.
	splitBytes = 1800
	// overlapTolerance is how far a segment may start before the one before
	// ended (the server accepts half a second).
	overlapTolerance = 0.5
	// minSegmentSeconds is the length given to a segment with none.
	minSegmentSeconds = 0.05
)

// Repairs counts what RepairSegments changed.
type Repairs struct {
	Dropped  int // empty segments removed
	Split    int // segments cut for length (extra parts made)
	Shifted  int // segments moved to start after the one before ended
	Extended int // segments given a length
}

// Any reports whether anything was repaired.
func (r Repairs) Any() bool { return r != Repairs{} }

func (r Repairs) String() string {
	return fmt.Sprintf("%d empty dropped, %d long split, %d overlapping moved, %d zero-length extended", r.Dropped, r.Split, r.Shifted, r.Extended)
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// RepairSegments makes a transcript satisfy the server's rules without
// changing a word (docs/problems: a long interview was refused whole for one
// segment, and the worker sent the same transcript again): empty segments are
// dropped; a segment of no length is given 0.05 s; one that starts before the
// previous ended (by more than the server's half second) is moved to start
// where that ended; one over the byte ceiling is cut on word boundaries into
// parts whose times share the segment's span in proportion to their bytes.
func RepairSegments(in []Segment) ([]Segment, Repairs) {
	var rep Repairs
	out := make([]Segment, 0, len(in))
	prevEnd := 0.0
	for _, s := range in {
		text := strings.TrimSpace(strings.ToValidUTF8(s.Text, ""))
		if text == "" {
			rep.Dropped++
			continue
		}
		start, end := s.Start, s.End
		if !finite(start) || !finite(end) {
			start, end = prevEnd, prevEnd
		}
		if start < prevEnd-overlapTolerance {
			start = prevEnd
			rep.Shifted++
		}
		if end <= start {
			end = start + minSegmentSeconds
			rep.Extended++
		}
		parts := splitText(text, splitBytes)
		if len(parts) > 1 {
			rep.Split += len(parts) - 1
		}
		at := start
		for i, part := range parts {
			next := at + (end-start)*float64(len(part))/float64(len(text))
			if i == len(parts)-1 {
				next = end
			}
			if next <= at {
				next = at + minSegmentSeconds
			}
			out = append(out, Segment{Start: at, End: next, Text: part})
			at = next
		}
		prevEnd = out[len(out)-1].End
	}
	return out, rep
}

// splitText cuts text into parts of at most max bytes on word boundaries (a
// single word longer than max is cut on a rune boundary).
func splitText(text string, max int) []string {
	if len(text) <= max {
		return []string{text}
	}
	var parts []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	for _, w := range strings.Fields(text) {
		for len(w) > max {
			cut := max
			for cut > 0 && !utf8.RuneStart(w[cut]) {
				cut--
			}
			flush()
			parts = append(parts, w[:cut])
			w = w[cut:]
		}
		if cur.Len() > 0 && cur.Len()+1+len(w) > max {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	flush()
	return parts
}

// ValidateSegments applies the server's rules to a transcript and names the
// first segment that breaks one: its index, bounds, size and the reason.
func ValidateSegments(segs []Segment) error {
	if len(segs) == 0 || len(segs) > MaxSegments {
		return fmt.Errorf("a transcript needs 1 to %d segments, has %d", MaxSegments, len(segs))
	}
	prevEnd := 0.0
	for i, s := range segs {
		text := strings.TrimSpace(s.Text)
		why := ""
		switch {
		case !finite(s.Start) || !finite(s.End):
			why = "a time is not a finite number"
		case s.End <= s.Start:
			why = "it ends before it starts"
		case s.Start < prevEnd-overlapTolerance:
			why = fmt.Sprintf("it is out of order (the one before ended at %.1f s)", prevEnd)
		case text == "":
			why = "it is empty"
		case len(text) > MaxSegmentBytes:
			why = fmt.Sprintf("it is over %d bytes", MaxSegmentBytes)
		case !utf8.ValidString(text):
			why = "its text is not valid UTF-8"
		}
		if why != "" {
			return fmt.Errorf("segment %d (%.1f-%.1f s, %d bytes): %s", i, s.Start, s.End, len(s.Text), why)
		}
		prevEnd = s.End
	}
	return nil
}
