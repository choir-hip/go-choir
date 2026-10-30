package quotematch

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/normtext"
)

// Failure modes pinned here (docs/definitions/choir-reeng-cut2-evidence-to-
// publication-2026-10-12.md unit 2a, and the C8 quote census in
// docs/archive/reports/reeng-c8-quote-census-2026-10-13.md):
//   - a tolerant tier accepts a quote the source does not say, or the
//     rendered span is the model's words instead of the source's own bytes;
//   - tolerant offsets land on the wrong bytes after a multibyte character,
//     an NBSP, a curly quote or a line break, so the span is shifted or cuts
//     a rune;
//   - a quote crossing a line break inside one block is refused (the census
//     found the whitespace tier fires almost only for line-break crossings);
//   - a quote is "found" by stitching the end of one block to the start of
//     the next: a quote never spans blocks;
//   - ellipsis segments match out of order, or in two different blocks;
//   - a tolerant match in an earlier block beats an exact match in a later
//     one (the cascade is tier first, then block order);
//   - a pipe-joined table row the model built cannot match a row block whose
//     cells are joined by " | ";
//   - an empty, whitespace-only or punctuation-only quote matches anything.

func blocksOf(texts ...string) []Block {
	kinds := make([]normtext.Block, len(texts))
	for i, text := range texts {
		kinds[i] = normtext.Block{Kind: "paragraph", Text: text}
	}
	ids := normtext.AssignCIDs(kinds)
	out := make([]Block, len(texts))
	for i, text := range texts {
		out[i] = Block{ID: ids[i], Text: text}
	}
	return out
}

// assertSpan checks that a match's reported span is the block's own bytes.
func assertSpan(t *testing.T, r Result, blocks []Block, wantExcerpt string) {
	t.Helper()
	if r.Outcome != Matched {
		t.Fatalf("outcome = %q, want matched (result %+v)", r.Outcome, r)
	}
	var block *Block
	for i := range blocks {
		if blocks[i].ID == r.BlockID {
			block = &blocks[i]
		}
	}
	if block == nil {
		t.Fatalf("block id %q not among the blocks", r.BlockID)
	}
	if r.Start < 0 || r.End > len(block.Text) || r.Start >= r.End {
		t.Fatalf("span [%d,%d) outside block of %d bytes", r.Start, r.End, len(block.Text))
	}
	if len(r.Spans) == 0 {
		t.Fatalf("no segment spans")
	}
	for _, s := range r.Spans {
		if got := block.Text[s.Start:s.End]; got != s.Text {
			t.Fatalf("segment span text %q != block bytes %q", s.Text, got)
		}
	}
	if r.Spans[0].Start != r.Start || r.Spans[len(r.Spans)-1].End != r.End {
		t.Fatalf("covering span [%d,%d) does not bound segments %+v", r.Start, r.End, r.Spans)
	}
	if r.Excerpt != wantExcerpt {
		t.Fatalf("excerpt = %q, want %q", r.Excerpt, wantExcerpt)
	}
}

func TestExactMatch(t *testing.T) {
	blocks := blocksOf("Intro paragraph.", "The council voted 7 to 2 on Tuesday night.")
	r := Match("voted 7 to 2", blocks)
	assertSpan(t, r, blocks, "voted 7 to 2")
	if r.Tier != TierExact || r.BlockID != blocks[1].ID || r.Tolerant() {
		t.Fatalf("tier %q block %q tolerant %v", r.Tier, r.BlockID, r.Tolerant())
	}
	if r.Start != strings.Index(blocks[1].Text, "voted") {
		t.Fatalf("start = %d", r.Start)
	}
}

func TestMatchCrossingNewlineInsideOneBlock(t *testing.T) {
	blocks := blocksOf("The budget rose\nby four percent this year.")
	r := Match("The budget rose by four percent", blocks)
	assertSpan(t, r, blocks, "The budget rose\nby four percent")
	if r.Tier != TierWSCase || !r.Tolerant() {
		t.Fatalf("tier = %q, want %q", r.Tier, TierWSCase)
	}
}

func TestCaseDifference(t *testing.T) {
	blocks := blocksOf("Officials Said The Plan Would Proceed.")
	r := Match("officials said the plan would proceed", blocks)
	assertSpan(t, r, blocks, "Officials Said The Plan Would Proceed")
	if r.Tier != TierWSCase {
		t.Fatalf("tier = %q", r.Tier)
	}
}

func TestCurlyVersusStraightQuotes(t *testing.T) {
	blocks := blocksOf("She said \u201cwe\u2019re not done\u201d and left.")
	r := Match(`said "we're not done"`, blocks)
	assertSpan(t, r, blocks, "said \u201cwe\u2019re not done\u201d")
	if r.Tier != TierWSCase {
		t.Fatalf("tier = %q", r.Tier)
	}
}

func TestPunctuationDifference(t *testing.T) {
	blocks := blocksOf("The U.S. economy, by most measures, grew.")
	r := Match("the US economy by most measures grew", blocks)
	assertSpan(t, r, blocks, "The U.S. economy, by most measures, grew")
	if r.Tier != TierPunct {
		t.Fatalf("tier = %q", r.Tier)
	}
}

func TestEllipsisSegmentsInOrderInOneBlock(t *testing.T) {
	text := "Alpha begins the list, beta follows, gamma is third and delta ends it."
	blocks := blocksOf("Unrelated alpha text.", text)
	for _, quote := range []string{"Alpha begins ... delta ends it", "Alpha begins\u2026gamma is third\u2026delta ends"} {
		r := Match(quote, blocks)
		if r.Outcome != Matched || r.BlockID != blocks[1].ID || r.Tier != TierExact {
			t.Fatalf("%q: %+v", quote, r)
		}
		var want []string
		for _, s := range r.Spans {
			want = append(want, s.Text)
		}
		assertSpan(t, r, blocks, strings.Join(want, " ... "))
	}
	r := Match("Alpha begins ... delta ends it", blocks)
	if len(r.Spans) != 2 || r.Spans[0].Text != "Alpha begins" || r.Spans[1].Text != "delta ends it" {
		t.Fatalf("spans = %+v", r.Spans)
	}
	if r.Start != 0 || r.End != len("Alpha begins the list, beta follows, gamma is third and delta ends it") {
		t.Fatalf("covering span [%d,%d)", r.Start, r.End)
	}
}

func TestEllipsisSegmentsOutOfOrderNotFound(t *testing.T) {
	blocks := blocksOf("Alpha begins the list, beta follows, gamma is third and delta ends it.")
	if r := Match("delta ends it ... Alpha begins", blocks); r.Outcome != NotFound {
		t.Fatalf("out-of-order segments matched: %+v", r)
	}
}

func TestEllipsisSegmentsSplitAcrossBlocksNotFound(t *testing.T) {
	blocks := blocksOf("The mayor announced the plan.", "Critics called it reckless.")
	if r := Match("The mayor announced ... called it reckless", blocks); r.Outcome != NotFound {
		t.Fatalf("segments in two blocks matched: %+v", r)
	}
}

func TestQuoteSpanningTwoBlocksNotFound(t *testing.T) {
	blocks := blocksOf("The first sentence ends here.", "The second begins there.")
	for _, quote := range []string{
		"ends here. The second begins",
		"ends here The second begins",
		"ends here.\n\nThe second begins",
	} {
		if r := Match(quote, blocks); r.Outcome != NotFound {
			t.Fatalf("%q spanning two blocks matched: %+v", quote, r)
		}
	}
}

func TestMultibyteAndNBSPOffsetsMapToSourceBytes(t *testing.T) {
	text := "Le caf\u00e9\u00a0\u201ccr\u00e8me\u201d co\u00fbte 3\u00a0\u20ac\n\u00e0 Paris, dit-on."
	blocks := blocksOf("\u65e5\u672c\u8a9e\u306e\u6587\u3002", text)
	r := Match("CAF\u00c9 \"cr\u00e8me\" co\u00fbte 3 \u20ac \u00e0 Paris", blocks)
	assertSpan(t, r, blocks, "caf\u00e9\u00a0\u201ccr\u00e8me\u201d co\u00fbte 3\u00a0\u20ac\n\u00e0 Paris")
	if r.Tier != TierWSCase {
		t.Fatalf("tier = %q", r.Tier)
	}

	// A decomposed accent in the source matches a composed one in the quote,
	// and the span keeps the source's decomposed bytes.
	blocks = blocksOf("Un cafe\u0301 noir, s\u2019il vous pla\u00eet.")
	r = Match("caf\u00e9 noir s'il vous", blocks)
	assertSpan(t, r, blocks, "cafe\u0301 noir, s\u2019il vous")
	if r.Tier != TierPunct {
		t.Fatalf("tier = %q", r.Tier)
	}

	// CJK text after multibyte runes: exact offsets are byte offsets.
	blocks = blocksOf("\u65e5\u672c\u8a9e\u306e\u6587\u3002\u6771\u4eac\u306f\u5927\u304d\u3044\u3002")
	r = Match("\u6771\u4eac\u306f", blocks)
	assertSpan(t, r, blocks, "\u6771\u4eac\u306f")
	if r.Start != len("\u65e5\u672c\u8a9e\u306e\u6587\u3002") {
		t.Fatalf("start = %d", r.Start)
	}
}

func TestPipeJoinedTableRow(t *testing.T) {
	blocks := blocksOf("City | Country | Population | Date", "Tokyo | Japan | 37,400,068 | 2018", "Delhi | India | 28,514,000 | 2018")
	r := Match("Tokyo | Japan | 37,400,068", blocks)
	assertSpan(t, r, blocks, "Tokyo | Japan | 37,400,068")
	if r.Tier != TierExact || r.BlockID != blocks[1].ID {
		t.Fatalf("%+v", r)
	}
	r = Match("tokyo |  japan | 37,400,068 | 2018", blocks)
	assertSpan(t, r, blocks, "Tokyo | Japan | 37,400,068 | 2018")
	if r.Tier != TierWSCase {
		t.Fatalf("tier = %q", r.Tier)
	}
	// A row stitched from two different rows is not in any one block.
	if r := Match("Tokyo | Japan | 28,514,000", blocks); r.Outcome != NotFound {
		t.Fatalf("stitched row matched: %+v", r)
	}
}

func TestEmptyAndContentlessQuotesNotFound(t *testing.T) {
	blocks := blocksOf("Some text... with dots and   spaces.")
	for _, quote := range []string{"", "   ", "\n\t", "...", "\u2026", ". . .", "!!"} {
		if r := Match(quote, blocks); r.Outcome != NotFound {
			t.Fatalf("quote %q matched: %+v", quote, r)
		}
	}
	if r := Match("anything", nil); r.Outcome != NotFound {
		t.Fatalf("no blocks: %+v", r)
	}
}

func TestExactInLaterBlockBeatsTolerantInEarlierBlock(t *testing.T) {
	blocks := blocksOf("THE RATE FELL SHARPLY", "the rate fell sharply")
	r := Match("the rate fell sharply", blocks)
	if r.Tier != TierExact || r.BlockID != blocks[1].ID {
		t.Fatalf("%+v", r)
	}
}

func TestNotFound(t *testing.T) {
	blocks := blocksOf("The council voted 7 to 2.")
	if r := Match("The council voted unanimously", blocks); r.Outcome != NotFound || r.Tier != "" || r.BlockID != "" {
		t.Fatalf("%+v", r)
	}
}

func TestVersionNamesNormalization(t *testing.T) {
	if !strings.Contains(Version, normtext.Version) {
		t.Fatalf("Version %q does not embed normtext.Version %q", Version, normtext.Version)
	}
	if NotCaptured == NotFound || NotCaptured == Matched {
		t.Fatalf("outcomes collide")
	}
}
