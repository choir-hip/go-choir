package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The research desk's read ledger (problems/ordinary-prompt-path-2026-11-09.md
// 1.5 items 2-3, 3.2 item 1; definition L5). Every choir.Read of the session
// is recorded with its capture or its error. The ledger feeds three things:
//
//   - the cell result, which lists each failed Read once, so `p, _ := Read(u)`
//     never hides a 403;
//   - choir.Reads() and choir.Captures(), the whole list;
//   - ReportPacket's pack check, a warning (never a refusal) naming captured
//     reads the packet leaves out;
//   - the interim nudge: one line, once per run, after the first capture, if
//     the research desk has not yet sent a packet.

// ReadEntry is one Read of this run.
type ReadEntry struct {
	URL       string `json:"url"`
	OK        bool   `json:"ok"`
	CaptureID string `json:"capture_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (e ReadEntry) String() string {
	if e.OK {
		return fmt.Sprintf("ok %s capture_id=%s %q", e.URL, e.CaptureID, e.Title)
	}
	return fmt.Sprintf("FAILED %s: %s", e.URL, e.Error)
}

type readLedger struct {
	mu         sync.Mutex
	runID      string
	reads      []ReadEntry
	noted      int  // reads already listed in a cell result
	packetSent bool // a ReportPacket was staged this run
	nudged     bool
	warnings   []string
}

// beginRun resets the ledger when a cell of a new run binds. An empty run id
// (a frame that names no run) keeps the ledger.
func (l *readLedger) beginRun(runID string) {
	if runID == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.runID == runID {
		return
	}
	l.runID = runID
	l.reads, l.noted, l.packetSent, l.nudged, l.warnings = nil, 0, false, false, nil
}

// recordRead notes one Read's outcome. A re-read of an unchanged page returns
// the same capture and is recorded once.
func (s *ChoirScope) recordRead(url string, out JSON, err error) {
	if s == nil {
		return
	}
	e := ReadEntry{URL: strings.TrimSpace(url)}
	if err != nil {
		e.Error = err.Error()
	} else {
		var r struct {
			CaptureID string `json:"capture_id"`
			Title     string `json:"title"`
		}
		_ = json.Unmarshal(out, &r)
		e.OK, e.CaptureID, e.Title = true, r.CaptureID, r.Title
	}
	l := &s.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.OK {
		for _, prev := range l.reads {
			if prev.OK && prev.CaptureID == e.CaptureID && prev.URL == e.URL {
				return
			}
		}
	}
	l.reads = append(l.reads, e)
}

// Reads returns every Read this run made, failures included.
func (s *ChoirScope) Reads() []ReadEntry {
	l := &s.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ReadEntry{}, l.reads...)
}

// Captures returns the Reads that made a capture.
func (s *ChoirScope) Captures() []ReadEntry {
	out := []ReadEntry{}
	for _, e := range s.Reads() {
		if e.OK {
			out = append(out, e)
		}
	}
	return out
}

// notePacketSent marks that the desk reported, and checks the packet against
// the captures of this run: a capture neither named by a source's capture_id
// nor by its URI is listed in a warning for the cell result.
func (s *ChoirScope) notePacketSent(packetJSON string) {
	if s == nil {
		return
	}
	var packet types.SourcePacket
	_ = json.Unmarshal([]byte(packetJSON), &packet)
	packedID, packedURL := map[string]bool{}, map[string]bool{}
	for _, src := range packet.Sources {
		if id := strings.TrimSpace(src.Target.CaptureID); id != "" {
			packedID[id] = true
		}
		if u := strings.TrimSpace(src.Target.URI); u != "" {
			packedURL[searchHitKey(u)] = true
		}
	}
	var missing []string
	captured := s.Captures()
	for _, c := range captured {
		if packedID[c.CaptureID] || packedURL[searchHitKey(c.URL)] {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (%s)", c.CaptureID, c.URL))
	}
	l := &s.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	l.packetSent = true
	if len(missing) > 0 {
		total := len(missing)
		if total > 10 {
			missing = append(missing[:10:10], fmt.Sprintf("and %d more", total-10))
		}
		l.warnings = append(l.warnings, fmt.Sprintf("warning: this packet leaves out %d of your %d captured reads: %s. Include each one that supports a claim, or say in the packet why it does not; the packet was sent as written.", total, len(captured), strings.Join(missing, ", ")))
	}
}

// readNoteLines are the ledger's lines for one cell result: each Read that
// failed since the last result, the pack warnings, and the interim nudge.
func (s *ChoirScope) readNoteLines() []string {
	if s == nil {
		return nil
	}
	l := &s.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	captured := 0
	for i, e := range l.reads {
		if e.OK {
			captured++
		}
		if i >= l.noted && !e.OK {
			lines = append(lines, "read failed: "+e.URL+": "+e.Error)
		}
	}
	l.noted = len(l.reads)
	lines = append(lines, l.warnings...)
	l.warnings = nil
	// The nudge goes to a desk that both reads the web and reports packets
	// (its manifest row says so), never by desk name.
	if captured > 0 && !l.packetSent && !l.nudged && s.deskModule("ReportPacket") && s.deskModule("Read") {
		l.nudged = true
		lines = append(lines, fmt.Sprintf("you have %d captured sources and have sent no packet; send an interim packet now (choir.ReportPacket without work_disposition) so Texture can revise while you read on", captured))
	}
	return lines
}

// cellNoteLines is the cell result's notes: the search count lines, then the
// read ledger's.
func (s *ChoirScope) cellNoteLines() []string {
	return append(s.searchCountLines(), s.readNoteLines()...)
}
