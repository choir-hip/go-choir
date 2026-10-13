package sourcefetch

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/normtext"
)

// A captured page is a block document (Cut 2 unit 2a): an ordered list of
// typed blocks, each addressed by its content id (normtext.CID; the k-th
// later duplicate is cid.k). A citation names one block, and a quote is
// checked against that block alone (internal/quotematch).
//
// The extractor versions name the rules that turn bytes into blocks,
// including the normalization the ids hash. A capture records the version
// that made it; changing any rule (what counts as a block, what is noise,
// how table cells join, how whitespace collapses) must bump the version, so
// later tuning makes new captures and never re-addresses the blocks a frozen
// citation names.
const (
	// ExtractorVersion names ExtractBlocks, the HTML DOM walk.
	ExtractorVersion = "html1+" + normtext.Version
	// TextExtractorVersion names TextBlocks, for text with no markup (a feed
	// summary or reader snapshot).
	TextExtractorVersion = "text1+" + normtext.Version
	// PDFExtractorVersion names PageBlocks over a PDF's per-page text.
	PDFExtractorVersion = "pdf1+" + normtext.Version
)

// Block kinds.
const (
	KindHeading       = "heading"
	KindParagraph     = "paragraph"
	KindListItem      = "list_item"
	KindTableCaption  = "table_caption"
	KindTableHeader   = "table_header"
	KindTableRow      = "table_row"
	KindFigureCaption = "figure_caption"
	KindNote          = "note"
	KindCode          = "code"
)

// Block is one block of a captured document. Text is the block's own words
// with whitespace collapsed; a table row's text is its trimmed cells joined
// with " | ". Level is set for headings (1..6) and Page for PDF blocks
// (1-based). Links are byte ranges of Text with the href the page gave them.
type Block struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Level int    `json:"level,omitempty"`
	Page  int    `json:"page,omitempty"`
	Text  string `json:"text"`
	Links []Link `json:"links,omitempty"`
}

// Link marks Text[Start:End] as a link to Href.
type Link struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Href  string `json:"href"`
}

// Document is the block form of one fetched page.
type Document struct {
	Extractor string  `json:"extractor"`
	Title     string  `json:"title,omitempty"`
	Blocks    []Block `json:"blocks"`
}

// Projection is the document's plain text: the blocks' text in order,
// separated by blank lines.
func (d Document) Projection() string { return Projection(d.Blocks) }

// DocHash is the document's identity; see DocHash.
func (d Document) DocHash() string { return DocHash(d.Blocks) }

// Projection joins the blocks' text with blank lines.
func Projection(blocks []Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, b.Text)
	}
	return strings.Join(parts, "\n\n")
}

// DocHash hashes the ordered blocks' kind, level, page and normalized text.
// It is what decides whether a re-capture changed: markup, script nonces,
// ads that are dropped as noise, and whitespace leave it alone, so changed
// bytes alone never make a new capture.
func DocHash(blocks []Block) string {
	h := sha256.New()
	for _, b := range blocks {
		h.Write([]byte(b.Kind))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(b.Level)))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(b.Page)))
		h.Write([]byte{0})
		h.Write([]byte(normtext.Normalize(b.Text)))
		h.Write([]byte{0x1e})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// AssignIDs sets each block's ID to its content id in document order
// (normtext.AssignCIDs: the k-th later duplicate is cid.k).
func AssignIDs(blocks []Block) {
	in := make([]normtext.Block, len(blocks))
	for i, b := range blocks {
		in[i] = normtext.Block{Kind: b.Kind, Text: b.Text}
	}
	for i, id := range normtext.AssignCIDs(in) {
		blocks[i].ID = id
	}
}

// TextBlocks splits text with no markup into paragraph blocks. Text with
// blank lines is split on them and each paragraph's wrapped lines are
// joined; text without any (a reader snapshot, one element per line) is
// split into one block per line.
func TextBlocks(text string) []Block {
	blocks := textBlocks(text, 0)
	AssignIDs(blocks)
	return blocks
}

// PageBlocks splits each page's text into paragraph blocks carrying their
// 1-based page number. Pages are split on blank lines, or by line when a
// page has none, as TextBlocks does. Empty pages make no blocks but keep
// later pages' numbers.
func PageBlocks(pages []string) []Block {
	var blocks []Block
	for i, page := range pages {
		blocks = append(blocks, textBlocks(page, i+1)...)
	}
	AssignIDs(blocks)
	return blocks
}

func textBlocks(text string, page int) []Block {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	var paragraphs []string
	lines := strings.Split(text, "\n")
	// A blank line counts only between two lines of text.
	hasBlank, seenText, blankAfterText := false, false, false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankAfterText = seenText
			continue
		}
		if blankAfterText {
			hasBlank = true
			break
		}
		seenText = true
	}
	if hasBlank {
		var cur []string
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				if len(cur) > 0 {
					paragraphs = append(paragraphs, strings.Join(cur, " "))
					cur = nil
				}
				continue
			}
			cur = append(cur, line)
		}
		if len(cur) > 0 {
			paragraphs = append(paragraphs, strings.Join(cur, " "))
		}
	} else {
		paragraphs = lines
	}
	var blocks []Block
	for _, p := range paragraphs {
		var t inlineText
		t.text(p)
		if s := t.String(); s != "" {
			blocks = append(blocks, Block{Kind: KindParagraph, Page: page, Text: s})
		}
	}
	return blocks
}

// inlineText accumulates a block's text, collapsing every whitespace run to
// one space and dropping leading and trailing space as it goes, so link
// offsets recorded while writing stay valid in the final text.
type inlineText struct {
	b     strings.Builder
	space bool
	open  []openLink
	links []Link
}

type openLink struct {
	start int
	href  string
}

func (t *inlineText) text(s string) {
	for _, r := range s {
		if isSpace(r) {
			t.space = true
			continue
		}
		if t.space && t.b.Len() > 0 {
			t.b.WriteByte(' ')
		}
		t.space = false
		for i := range t.open {
			if t.open[i].start < 0 {
				t.open[i].start = t.b.Len()
			}
		}
		t.b.WriteRune(r)
	}
}

// breakSpace records a boundary that reads as a space (a <br>, or the edge
// of a block-level element inside a table cell).
func (t *inlineText) breakSpace() { t.space = true }

func (t *inlineText) openLink(href string) {
	t.open = append(t.open, openLink{start: -1, href: href})
}

func (t *inlineText) closeLink() {
	if len(t.open) == 0 {
		return
	}
	l := t.open[len(t.open)-1]
	t.open = t.open[:len(t.open)-1]
	if l.start >= 0 && t.b.Len() > l.start && l.href != "" {
		t.links = append(t.links, Link{Start: l.start, End: t.b.Len(), Href: l.href})
	}
}

func (t *inlineText) String() string { return t.b.String() }

func (t *inlineText) reset() {
	t.b.Reset()
	t.space = false
	t.links = nil
	for i := range t.open {
		t.open[i].start = -1
	}
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\f', '\v', ' ', ' ', ' ', ' ', ' ', ' ', '　':
		return true
	}
	return r >= ' ' && r <= ' '
}
