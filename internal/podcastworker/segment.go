package podcastworker

import (
	"encoding/json"
	"fmt"
	"strings"
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
