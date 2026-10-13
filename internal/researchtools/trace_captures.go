package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/normtext"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// The trace bundle's captures layer (Cut 2 amendment A5): the computer's
// captures in the trace window, plus any capture the trajectory names, each
// with its blocks' cids and normalized text, so the quote census can rerun
// on a bundle. Bounds keep a bundle finite: at most traceCaptureListLimit
// captures are listed from the store, at most traceCaptureChars characters
// of block text are read per capture, and once traceCaptureTotalChars are in
// the layer the remaining captures are listed without blocks. Every bound
// that bites says so in a note.
const (
	traceCaptureListLimit  = 500
	traceCaptureChars      = 200000
	traceCaptureTotalChars = 20000000
)

// TraceBlock is one block of a capture in the trace: its cid, kind, page,
// text, and the normtext the cid hashes.
type TraceBlock struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Page     int    `json:"page,omitempty"`
	Text     string `json:"text"`
	Normtext string `json:"normtext"`
}

// TraceCapture is one capture in the trace.
type TraceCapture struct {
	CaptureID       string       `json:"capture_id"`
	ComputerID      string       `json:"computer_id,omitempty"`
	URL             string       `json:"url,omitempty"`
	Title           string       `json:"title,omitempty"`
	FetchedAt       string       `json:"fetched_at,omitempty"`
	CreatedAt       string       `json:"created_at,omitempty"`
	InWindow        bool         `json:"in_window"`
	MediaType       string       `json:"media_type,omitempty"`
	Extractor       string       `json:"extractor,omitempty"`
	DocHash         string       `json:"doc_hash,omitempty"`
	BlockCount      int          `json:"block_count"`
	RawBytes        int          `json:"raw_bytes"`
	RawPresent      bool         `json:"raw_present"`
	Blocks          []TraceBlock `json:"blocks"`
	BlocksTruncated bool         `json:"blocks_truncated,omitempty"`
	Note            string       `json:"note,omitempty"`
}

// TraceCaptures is the captures layer's payload.
type TraceCaptures struct {
	ComputerID     string         `json:"computer_id"`
	Since          string         `json:"since,omitempty"`
	Until          string         `json:"until,omitempty"`
	WindowComplete bool           `json:"window_complete"`
	Captures       []TraceCapture `json:"captures"`
	Note           string         `json:"note,omitempty"`
}

// traceCaptureRead is the part of corpusd's capture read the trace keeps.
type traceCaptureRead struct {
	CaptureID  string              `json:"capture_id"`
	ComputerID string              `json:"computer_id"`
	Title      string              `json:"title"`
	URL        string              `json:"url"`
	FetchedAt  string              `json:"fetched_at"`
	MediaType  string              `json:"media_type"`
	RawBytes   int                 `json:"raw_bytes"`
	Extractor  string              `json:"extractor"`
	DocHash    string              `json:"doc_hash"`
	BlockCount int                 `json:"block_count"`
	Blocks     []sourcefetch.Block `json:"blocks"`
	NextBlock  int                 `json:"next_block"`
}

// TraceCapturesInWindow lists the computer's captures created in
// [since, until] (a zero since has no lower bound) and the captures named by
// ids, newest first, each read with its blocks.
func TraceCapturesInWindow(ctx context.Context, sink CaptureSink, ownerID, computerID string, since, until time.Time, ids []string) (TraceCaptures, error) {
	out := TraceCaptures{ComputerID: computerID, Captures: []TraceCapture{}}
	if !since.IsZero() {
		out.Since = since.UTC().Format(time.RFC3339)
	}
	if !until.IsZero() {
		out.Until = until.UTC().Format(time.RFC3339)
	}
	if sink == nil {
		return out, fmt.Errorf("captures: the shared capture store is not configured")
	}
	objs, err := sink.ListCaptures(ctx, ownerID, computerID, traceCaptureListLimit)
	if err != nil {
		return out, err
	}
	var notes []string
	out.WindowComplete = true
	if len(objs) >= traceCaptureListLimit {
		oldest := objs[len(objs)-1].CreatedAt
		for _, obj := range objs {
			if obj.CreatedAt.Before(oldest) {
				oldest = obj.CreatedAt
			}
		}
		if since.IsZero() || !oldest.Before(since) {
			out.WindowComplete = false
			notes = append(notes, fmt.Sprintf("the store listed its newest %d captures for this computer, all inside the window; older captures in the window are missing", traceCaptureListLimit))
		}
	}
	type entry struct {
		id       string
		created  time.Time
		inWindow bool
	}
	seen := map[string]bool{}
	var entries []entry
	for _, obj := range objs {
		if obj.ObjectKind != objectgraph.WebCaptureObjectKind || seen[obj.CanonicalID] {
			continue
		}
		if (!since.IsZero() && obj.CreatedAt.Before(since)) || (!until.IsZero() && obj.CreatedAt.After(until)) {
			continue
		}
		seen[obj.CanonicalID] = true
		entries = append(entries, entry{id: obj.CanonicalID, created: obj.CreatedAt, inWindow: true})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].created.After(entries[j].created) })
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			entries = append(entries, entry{id: id})
		}
	}
	total := 0
	var unreadable []string
	for _, e := range entries {
		c := TraceCapture{CaptureID: e.id, InWindow: e.inWindow, Blocks: []TraceBlock{}}
		if !e.created.IsZero() {
			c.CreatedAt = e.created.UTC().Format(time.RFC3339Nano)
		}
		if total >= traceCaptureTotalChars {
			c.Note = fmt.Sprintf("blocks not included: the layer already holds %d characters of block text", traceCaptureTotalChars)
			out.Captures = append(out.Captures, c)
			continue
		}
		raw, err := sink.ReadCapture(ctx, e.id, 0, traceCaptureChars)
		if err != nil {
			if !e.inWindow {
				unreadable = append(unreadable, e.id)
				continue
			}
			c.Note = "capture read failed: " + err.Error()
			out.Captures = append(out.Captures, c)
			continue
		}
		var read traceCaptureRead
		if err := json.Unmarshal(raw, &read); err != nil {
			c.Note = "capture read undecodable: " + err.Error()
			out.Captures = append(out.Captures, c)
			continue
		}
		c.ComputerID, c.URL, c.Title, c.FetchedAt = read.ComputerID, read.URL, read.Title, read.FetchedAt
		c.MediaType, c.Extractor, c.DocHash, c.BlockCount = read.MediaType, read.Extractor, read.DocHash, read.BlockCount
		c.RawBytes, c.RawPresent = read.RawBytes, read.RawBytes > 0
		for _, b := range read.Blocks {
			c.Blocks = append(c.Blocks, TraceBlock{ID: b.ID, Kind: b.Kind, Page: b.Page, Text: b.Text, Normtext: normtext.Normalize(b.Text)})
			total += len([]rune(b.Text))
		}
		switch {
		case read.Extractor == "":
			c.Note = "written before captures were block documents: no blocks and no raw bytes"
		case read.NextBlock > 0:
			c.BlocksTruncated = true
			c.Note = fmt.Sprintf("blocks %d to %d not included: the capture's text exceeds %d characters", read.NextBlock, read.BlockCount-1, traceCaptureChars)
		}
		out.Captures = append(out.Captures, c)
	}
	if len(unreadable) > 0 {
		notes = append(notes, "named ids not readable as captures: "+strings.Join(unreadable, ", "))
	}
	out.Note = strings.Join(notes, "; ")
	return out, nil
}
