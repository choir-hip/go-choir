package texturewrite

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	ellipsisRE     = regexp.MustCompile(`\.{3}|\x{2026}`)
	legacyMarkerRE = regexp.MustCompile(`\[\d{1,3}\]|\[[^\]\n]{1,160}\]\(\s*source\s*:`)
)

// FrameDocBudget is the size of document text a frame carries in full. A
// longer document's frame carries a section digest plus the recently edited
// and cited blocks; choir.ReadDoc() returns every block.
const FrameDocBudget = 12000

// FrameHeader starts every frame turn; the runtime finds the last frame a
// run saw by it.
const FrameHeader = "Choir Texture frame."

// Frame is what Texture reads at the start of each turn: the document head
// as aliased blocks and the records addressed to it since its cursor.
type Frame struct {
	DocID      string
	RevisionID string
	Blocks     []Block
	// Changed names blocks edited by the head revision (long documents show
	// them in full).
	Changed map[string]bool
	// Dropped lists citations the head revision's Write did not carry.
	Dropped []DroppedCitation
	Records []FrameRecord
	// Captured, when set, says which frame sources have a capture.
	Captured map[string]bool
	// ThroughSeq is the last inbox record the frame shows.
	ThroughSeq int64
	// ReaderRequest, when set, names a reader's pending request for more
	// research on this story (docs/design/research-more-goes-to-research-2026-10-21.md).
	ReaderRequest string
}

// Render writes the frame as one user turn.
func (f Frame) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\ndoc=%s rev=%s through=%d\n\n", FrameHeader, f.DocID, f.RevisionID, f.ThroughSeq)
	f.renderDoc(&b)
	if f.ReaderRequest != "" {
		fmt.Fprintf(&b, "\nA reader asked for more research on this story (request %s). Research works on it and reports to you as it goes. Expand the story with what research reports: keep its text, structure and citations unless a source corrects them, and add what the new sources say. With nothing new, change nothing.\n", f.ReaderRequest)
	}
	if len(f.Dropped) > 0 {
		alias := map[string]string{}
		for _, blk := range f.Blocks {
			alias[blk.ID] = blk.Alias
		}
		b.WriteString("\nCitations your last Write dropped (the block changed and the quote no longer appears in it; cite again where the text still relies on the source):\n")
		for _, d := range f.Dropped {
			where := alias[d.BlockID]
			if where == "" {
				where = "removed block"
			}
			fmt.Fprintf(&b, "- %s: %s\n", where, d.Token)
		}
	}
	if len(f.Records) == 0 {
		b.WriteString("\nNo new records.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "\nNew records (r1..r%d):\n", len(f.Records))
	for _, r := range f.Records {
		f.renderRecord(&b, r)
	}
	return b.String()
}

func (f Frame) renderDoc(b *strings.Builder) {
	if len(f.Blocks) == 0 {
		b.WriteString("The document is empty; start it with @insert end.\n")
		return
	}
	total := 0
	for _, blk := range f.Blocks {
		total += len(blk.Text)
	}
	if total <= FrameDocBudget {
		b.WriteString("Document blocks (address them by alias in choir.Write):\n")
		for _, blk := range f.Blocks {
			writeBlock(b, blk.Alias, blk.Text)
		}
		return
	}
	fmt.Fprintf(b, "Document blocks, digested: the document is %d characters. Headings and the blocks the last revision edited are in full; read every block with choir.ReadDoc() before rewriting a section you only see digested.\n", total)
	budget := FrameDocBudget
	for _, blk := range f.Blocks {
		full := blk.Kind == "heading" || f.Changed[blk.ID]
		if full && budget-len(blk.Text) >= 0 {
			budget -= len(blk.Text)
			writeBlock(b, blk.Alias, blk.Text)
			continue
		}
		digest := strings.Join(strings.Fields(blk.Text), " ")
		if len(digest) > 80 {
			digest = strings.TrimSpace(digest[:80]) + "…"
		}
		cited := ""
		if n := strings.Count(blk.Text, "{{cite") + strings.Count(blk.Text, "{{ref"); n > 0 {
			cited = fmt.Sprintf(", %d citations", n)
		}
		fmt.Fprintf(b, "%s [%s, %d chars%s] %s\n", blk.Alias, blk.Kind, len(blk.Text), cited, digest)
	}
}

func writeBlock(b *strings.Builder, alias, text string) {
	lines := strings.Split(text, "\n")
	fmt.Fprintf(b, "%s %s\n", alias, lines[0])
	pad := strings.Repeat(" ", len(alias)+1)
	for _, l := range lines[1:] {
		b.WriteString(pad + l + "\n")
	}
}

func (f Frame) renderRecord(b *strings.Builder, r FrameRecord) {
	fmt.Fprintf(b, "%s %s from %s", r.Alias, r.Kind, r.From)
	if r.Round > 0 {
		fmt.Fprintf(b, ", round %d", r.Round)
	}
	if r.Final {
		b.WriteString(", final")
	}
	if r.RecordID != "" {
		fmt.Fprintf(b, " (record %s)", r.RecordID)
	}
	if head := firstNonEmpty(r.Objective, r.Summary); head != "" {
		b.WriteString(" — " + head)
	}
	b.WriteString("\n")
	if r.Objective != "" && r.Summary != "" && r.Summary != r.Objective {
		b.WriteString("  " + r.Summary + "\n")
	}
	if r.Text != "" {
		b.WriteString("  " + strings.ReplaceAll(r.Text, "\n", "\n  ") + "\n")
	}
	if len(r.Hypotheses) > 0 {
		b.WriteString("  hypotheses:\n")
		for _, h := range r.Hypotheses {
			fmt.Fprintf(b, "  - %s (p=%.2f) %s\n", firstNonEmpty(h.ID, "h"), h.Probability, h.Statement)
		}
	}
	if len(r.Claims) > 0 {
		b.WriteString("  claims:\n")
		for _, c := range r.Claims {
			b.WriteString("  - " + c.Text)
			if len(c.Handles) > 0 {
				b.WriteString(" [" + strings.Join(c.Handles, ", ") + "]")
			}
			b.WriteString("\n")
		}
	}
	if len(r.Sources) > 0 {
		b.WriteString("  sources:\n")
		for _, s := range r.Sources {
			b.WriteString("  - " + s.Handle)
			if s.Title != "" {
				b.WriteString(" " + s.Title)
			}
			if s.URL != "" {
				b.WriteString(" <" + s.URL + ">")
			}
			if s.CaptureID != "" {
				b.WriteString(" capture " + s.CaptureID)
			}
			if f.Captured != nil {
				if f.Captured[s.Handle] && s.Excerpt == "" {
					b.WriteString(" (captured; read it with choir.ReadCapture(\"" + s.Handle + "\", 0, 0))")
				} else if f.Captured[s.Handle] {
					b.WriteString(" (captured)")
				} else {
					b.WriteString(notCapturedHint(s))
				}
			}
			if s.Excerpt != "" {
				b.WriteString(" " + strconv.Quote(clip(s.Excerpt, 300)))
			}
			b.WriteString("\n")
		}
	}
	if r.Publish != "" {
		b.WriteString("  publish: " + r.Publish + "\n")
	}
	if len(r.Failures) > 0 {
		alias := map[string]string{}
		for _, blk := range f.Blocks {
			alias[blk.ID] = blk.Alias
		}
		b.WriteString("  unsupported citations (the cited passage does not support the words the citation closes; with choir.Write, make them say only what a cited passage says, cite a passage that says it, or cut the claim; naming the outlet does not make a claim supported):\n")
		for _, fail := range r.Failures {
			where := alias[fail.BlockID]
			if where == "" {
				where = "a block no longer in the document"
			}
			if fail.Text != "" {
				where += " " + strconv.Quote(clip(fail.Text, 400))
			}
			fmt.Fprintf(b, "  - %s cites %s %s", where, fail.Doc, strconv.Quote(clip(fail.Quote, 300)))
			if fail.Why != "" {
				b.WriteString(": " + fail.Why)
			}
			b.WriteString("\n")
		}
	}
}

// RecordInput is one inbox record as the runtime reads it.
type RecordInput struct {
	Seq       int64
	RecordID  string
	Kind      string
	From      string
	Round     int
	Final     bool
	Objective string
	Body      string
	Cast      *types.CommitmentCast
	Report    *types.CommitmentReport
	Support   *types.CommitmentSupport
}

// FrameRecord is one record as the frame shows it.
type FrameRecord struct {
	Alias      string
	Seq        int64
	RecordID   string
	Kind       string
	From       string
	Round      int
	Final      bool
	Objective  string
	Summary    string
	Text       string
	Hypotheses []types.CastHypothesis
	Claims     []FrameClaim
	Sources    []Source
	Publish    string
	// Failures are Jev's failing support verdicts (Cut 2 2a (v)).
	Failures []FrameFailure
}

// FrameFailure is one citation Jev judged unsupported: the citing
// block, the span judged when it is less than the block, the cited capture
// and quote, and why.
type FrameFailure struct {
	BlockID string
	Text    string
	Doc     string
	Quote   string
	Why     string
}

// FrameClaim is a claim with the handles of the sources it rests on.
type FrameClaim struct {
	Text    string
	Handles []string
}

// RecordsFrom aliases records r1..rM in inbox order and lists each one's
// sources by handle rN.cM: a packet's sources in order, then the captures
// an opening Cast names.
func RecordsFrom(in []RecordInput) []FrameRecord {
	out := make([]FrameRecord, 0, len(in))
	for i, r := range in {
		alias := "r" + strconv.Itoa(i+1)
		fr := FrameRecord{Alias: alias, Seq: r.Seq, RecordID: r.RecordID, Kind: r.Kind, From: r.From, Round: r.Round, Final: r.Final, Objective: r.Objective}
		var packet types.SourcePacket
		body := strings.TrimSpace(r.Body)
		isPacket := strings.HasPrefix(body, "{") && json.Unmarshal([]byte(body), &packet) == nil && (packet.Kind != "" || packet.Summary != "" || len(packet.Claims) > 0 || len(packet.Sources) > 0)
		bySourceID := map[string]string{}
		if isPacket {
			fr.Summary = packet.Summary
			for j, s := range packet.Sources {
				handle := alias + ".c" + strconv.Itoa(j+1)
				fr.Sources = append(fr.Sources, Source{Handle: handle, URL: strings.TrimSpace(s.Target.URI), CaptureID: strings.TrimSpace(s.Target.CaptureID), Title: strings.TrimSpace(s.Target.Title), Excerpt: strings.TrimSpace(s.Excerpt)})
				if id := strings.TrimSpace(s.SourceID); id != "" {
					bySourceID[id] = handle
				}
			}
			for _, c := range packet.Claims {
				claim := FrameClaim{Text: c.Text}
				for _, id := range c.SourceIDs {
					if h, ok := bySourceID[strings.TrimSpace(id)]; ok {
						claim.Handles = append(claim.Handles, h)
					}
				}
				fr.Claims = append(fr.Claims, claim)
			}
			var extra []string
			for _, q := range packet.Questions {
				extra = append(extra, "question: "+q)
			}
			for _, n := range packet.Notes {
				extra = append(extra, "note: "+n)
			}
			fr.Text = clip(strings.Join(extra, "\n"), 2000)
		} else {
			fr.Text = clip(body, 4000)
		}
		if r.Report != nil && len(fr.Claims) == 0 {
			for _, c := range r.Report.Claims {
				fr.Claims = append(fr.Claims, FrameClaim{Text: c.Text})
			}
		}
		if r.Support != nil {
			for _, v := range r.Support.Failures {
				if v.Pair == nil {
					continue
				}
				failure := FrameFailure{BlockID: v.Pair.BlockID, Doc: v.Pair.Cited.Doc, Quote: v.Pair.Cited.Quote, Why: v.Why}
				if v.Pair.Context != "" {
					failure.Text = v.Pair.Text
				}
				fr.Failures = append(fr.Failures, failure)
			}
		}
		if r.Cast != nil {
			fr.Hypotheses = r.Cast.Hypotheses
			fr.Publish = r.Cast.Publish
			for _, capture := range r.Cast.Captures {
				handle := alias + ".c" + strconv.Itoa(len(fr.Sources)+1)
				fr.Sources = append(fr.Sources, Source{Handle: handle, CaptureID: strings.TrimSpace(capture)})
			}
		}
		out = append(out, fr)
	}
	return out
}

// Handles is the frame's source table by handle.
func Handles(records []FrameRecord) map[string]Source {
	out := map[string]Source{}
	for _, r := range records {
		for _, s := range r.Sources {
			out[s.Handle] = s
		}
	}
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// notCapturedHint says what an uncaptured frame source needs. Research can
// Read a web page or an already-captured source, so the hint sends Texture
// there. A source that is an internal record (an engineering assignment
// report, a ledger ref: no http(s) URL, no capture id) is not something
// research can Read, and casting it to "capture" one only returns "record not
// found" (docs/archive/problems/adopt-1418-adopter-desks-finish-without-an-operation-2026-10-15.md).
// For that source the hint says to state the outcome in the document without
// citing it, and sends a blocker to management for one bounded re-cast, or to
// the owner.
func notCapturedHint(s Source) string {
	if s.CaptureID != "" || strings.HasPrefix(s.URL, "http://") || strings.HasPrefix(s.URL, "https://") {
		return " (not captured: it cannot be cited until research Reads it)"
	}
	return " (not captured, and research cannot Read it: an internal record, not a web source. Do not cast research for it; state its outcome in the document without citing it. If it reports a blocker, management owns one bounded re-cast, otherwise the owner decides)"
}
