package texturewrite

// H-OG3 (docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md):
// a citation's Ref keeps the span the quote check found, so a reader can
// mark the exact words without matching again. Failure modes, written
// before the code:
//
//	F8  offsets in bytes are read as characters: the mark lands on the
//	    wrong words when multi-byte text precedes the quote;
//	F9  a tolerant match's span indexes normalized text, not the block;
//	F10 an old Ref without a span stops decoding, or a Ref without a span
//	    changes its JSON bytes;
//	F11 an ellipsis quote loses its segments;
//	F12 a later Write drops the span of a carried citation.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const spanBlockText = "Le café a été fermé. The court ruled that the merger may proceed — “finally”. The court ruled again on Monday."

func spanCaptures() *fakeCaptures {
	return &fakeCaptures{captures: map[string]Capture{
		"https://example.org/cafe": {
			Doc: "cap-cafe", Version: "v1", URL: "https://example.org/cafe", Title: "Café",
			Blocks: []quotematch.Block{
				{ID: "k0", Text: "An earlier block."},
				{ID: "k1", Text: spanBlockText},
			},
		},
	}}
}

// codePoints slices text by Unicode code points, the unit of RefSpan.
func codePoints(text string, start, end int) string {
	r := []rune(text)
	if start < 0 || end > len(r) || start > end {
		return "<out of range>"
	}
	return string(r[start:end])
}

func citeRef(t *testing.T, script string) (types.Ref, texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity) {
	t.Helper()
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-span",
		Base:     baseDoc(paragraph("p1", "Opening line.")),
		Script:   script,
		Resolver: spanCaptures(),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, block := range DocBlocks(res.Doc) {
		for _, n := range sourceRefs(block.Node) {
			if ref, ok := RefOf(n); ok {
				return ref, res.Doc, res.Entities
			}
		}
	}
	t.Fatal("no citation written")
	return types.Ref{}, res.Doc, res.Entities
}

func TestRefSpanMarksTheMatchedWordsInCodePoints(t *testing.T) {
	// F8: "é" and "—" are multi-byte before and inside the block; the quote
	// "court ruled again" occurs once, after an earlier "court ruled".
	ref, doc, entities := citeRef(t, "@replace b1\nThe court ruled again {{cite https://example.org/cafe \"The court ruled again on Monday\"}}.")
	if ref.CID != "k1" || ref.Span == nil {
		t.Fatalf("ref = %+v, want block k1 with a span", ref)
	}
	got := codePoints(spanBlockText, ref.Span.Start, ref.Span.End)
	if got != "The court ruled again on Monday" || got != ref.Quote {
		t.Fatalf("span [%d,%d) = %q, quote %q", ref.Span.Start, ref.Span.End, got, ref.Quote)
	}
	sum := sha256.Sum256([]byte(got))
	if ref.Span.TextHash != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("text_hash = %s, want the sha256 of the covered text", ref.Span.TextHash)
	}
	if len(ref.Span.Segments) != 0 {
		t.Fatalf("a plain quote has no segments: %+v", ref.Span.Segments)
	}
	if err := texturedoc.Validate(doc, entities); err != nil {
		t.Fatalf("document with a span does not validate: %v", err)
	}
}

func TestRefSpanOfTolerantMatchIndexesTheOriginalBlock(t *testing.T) {
	// F9: case and quote marks differ; the span still covers the block's
	// own words, which is what Quote (the excerpt) holds.
	ref, _, _ := citeRef(t, "@replace b1\nIt went ahead {{cite https://example.org/cafe \"the COURT ruled that the merger may proceed - \\\"finally\\\"\"}}.")
	if ref.Span == nil {
		t.Fatalf("tolerant match has no span: %+v", ref)
	}
	got := codePoints(spanBlockText, ref.Span.Start, ref.Span.End)
	if got != ref.Quote || !strings.HasPrefix(got, "The court ruled that the merger") {
		t.Fatalf("span covers %q, quote %q", got, ref.Quote)
	}
}

func TestRefSpanKeepsEveryEllipsisSegment(t *testing.T) {
	// F11: two segments in one block.
	ref, _, _ := citeRef(t, "@replace b1\nTwice {{cite https://example.org/cafe \"Le café a été fermé ... ruled again on Monday\"}}.")
	if ref.Span == nil || len(ref.Span.Segments) != 2 {
		t.Fatalf("span = %+v, want two segments", ref.Span)
	}
	var parts []string
	for _, seg := range ref.Span.Segments {
		parts = append(parts, codePoints(spanBlockText, seg.Start, seg.End))
	}
	if strings.Join(parts, " ... ") != ref.Quote {
		t.Fatalf("segments %q do not rebuild the quote %q", parts, ref.Quote)
	}
	if ref.Span.Start != ref.Span.Segments[0].Start || ref.Span.End != ref.Span.Segments[1].End {
		t.Fatalf("span [%d,%d) does not cover its segments %+v", ref.Span.Start, ref.Span.End, ref.Span.Segments)
	}
}

func TestRefSpanSurvivesALaterWrite(t *testing.T) {
	// F12: a later Write that leaves the cited block alone, and one that
	// rewrites it but keeps the token, both keep the span.
	ref, doc, entities := citeRef(t, "@replace b1\nThe court ruled again {{cite https://example.org/cafe \"The court ruled again on Monday\"}}.")
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-span-2", Base: doc, Entities: entities,
		Script: "@insert after b1\nA new paragraph.", Resolver: spanCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	carried, ok := RefOf(sourceRefs(res.Doc.Doc.Content[0])[0])
	if !ok || carried.Span == nil || carried.Span.Start != ref.Span.Start || carried.Span.End != ref.Span.End || carried.Span.TextHash != ref.Span.TextHash {
		t.Fatalf("carried ref = %+v, want span %+v", carried, ref.Span)
	}
	blocks := DocBlocks(res.Doc)
	res, err = Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-span-3", Base: res.Doc, Entities: res.Entities,
		Script: "@replace b1\nOn Monday: " + blocks[0].Text, Resolver: spanCaptures()})
	if err != nil {
		t.Fatalf("Apply rewrite: %v", err)
	}
	carried, ok = RefOf(sourceRefs(res.Doc.Doc.Content[0])[0])
	if !ok || carried.Span == nil || carried.Span.Start != ref.Span.Start || carried.Span.End != ref.Span.End || carried.Span.TextHash != ref.Span.TextHash {
		t.Fatalf("rewritten block's ref = %+v, want span %+v", carried, ref.Span)
	}
}

func TestRefWithoutSpanKeepsItsBytes(t *testing.T) {
	// F10: an old Ref decodes with no span, and re-encodes to the same
	// bytes, so existing manifests and pair keys are unchanged.
	old := `{"doc":"cap-court","version":"v1","cid":"c1","hash":"sha256:abc","quote":"the merger may proceed"}`
	var ref types.Ref
	if err := json.Unmarshal([]byte(old), &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Span != nil {
		t.Fatalf("old ref decoded a span: %+v", ref.Span)
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != old {
		t.Fatalf("old ref re-encodes as %s", raw)
	}
	node := texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"ref": map[string]any{"doc": "cap-court", "version": "v1", "cid": "c1", "hash": "sha256:abc", "quote": "q"}}}
	if got, ok := RefOf(node); !ok || got.Span != nil {
		t.Fatalf("RefOf old node = %+v", got)
	}
}
