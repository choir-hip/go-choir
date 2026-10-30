package sourcefetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/normtext"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

// Ways the block extractor can fail, written before the code (Cut 2 unit
// 2a, docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md):
//
//   - The DOM walk merges paragraphs, so a quote's block is the whole page
//     (the C8 census: the old flattener had no blank lines, one block per
//     page), or it splits one paragraph at an inline <a>, <em> or <br>.
//   - Noise (site header, nav, cookie banner, ads, newsletter, footer)
//     survives as blocks, or the article's own header and headline are
//     dropped with the site's.
//   - A heading loses its level; a list item reads as a paragraph.
//   - Table rows are flattened into one block, or cells are joined in a way
//     research cannot quote (census: Wikipedia rows quoted pipe-joined); the
//     header row or caption is lost; a layout table is read as rows.
//   - Footnote markers "[12]" or links are stripped, so the block is not the
//     page's text and a link's target is lost.
//   - Duplicate paragraphs share an id, so a citation is ambiguous.
//   - Block ids use a normalization other than normtext's, so a quote's match
//     and its block's address disagree.
//   - doc_hash changes with ad or nonce churn that leaves the text alone, or
//     stays the same when the text changes.
//   - ExtractReadableHTML drifts from the block projection its four callers
//     now read.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func blockTexts(blocks []Block) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.Kind + ": " + b.Text
	}
	return out
}

func findBlock(t *testing.T, blocks []Block, prefix string) Block {
	t.Helper()
	for _, b := range blocks {
		if strings.HasPrefix(b.Text, prefix) {
			return b
		}
	}
	t.Fatalf("no block starts with %q; blocks:\n%s", prefix, strings.Join(blockTexts(blocks), "\n"))
	return Block{}
}

func TestExtractBlocksArticleKeepsStructureAndDropsNoise(t *testing.T) {
	doc := ExtractBlocks(readFixture(t, "article.html"))
	if doc.Extractor != ExtractorVersion || !strings.HasSuffix(doc.Extractor, normtext.Version) {
		t.Fatalf("extractor = %q, want %q naming the normtext version", doc.Extractor, ExtractorVersion)
	}
	if doc.Title != "Harbor Council Approves Flood Wall | Example Times" {
		t.Fatalf("title = %q", doc.Title)
	}
	want := []string{
		"heading: Harbor Council Approves Flood Wall",
		"paragraph: By Dana Ortiz, October 12, 2026",
		"paragraph: The Harbor City council voted 7 to 2 on Monday to build a 4.2-kilometre flood wall along the eastern waterfront, ending a debate that has run for six years.",
		"paragraph: Construction is expected to begin in March and cost an estimated $310 million, according to the city budget office. The state will cover about 40 percent.",
		"heading: What the wall will protect",
		"paragraph: Officials said the wall would shield roughly 18,000 homes and the main rail yard from a one-in-a-hundred-year storm surge.",
		"list_item: The eastern rail yard and freight depot",
		"list_item: Two hospitals on Water Street",
		"list_item: The sewage treatment plant at Pier 9",
		"paragraph: Read the full council report.",
		"heading: Opposition",
		"paragraph: Two council members argued that the wall would cut residents off from the waterfront and that the money should go to buyouts instead.",
		"paragraph: Read the full council report.",
	}
	got := blockTexts(doc.Blocks)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	levels := map[string]int{}
	for _, b := range doc.Blocks {
		if b.Kind == KindHeading {
			levels[b.Text] = b.Level
		} else if b.Level != 0 {
			t.Fatalf("non-heading block carries a level: %+v", b)
		}
	}
	if levels["Harbor Council Approves Flood Wall"] != 1 || levels["What the wall will protect"] != 2 || levels["Opposition"] != 3 {
		t.Fatalf("heading levels = %v", levels)
	}
	for _, noise := range []string{"cookies", "mattress", "Subscribe", "World", "newsletter", "Copyright", "Share this", "Related:"} {
		if strings.Contains(doc.Projection(), noise) {
			t.Fatalf("noise %q survived extraction:\n%s", noise, doc.Projection())
		}
	}
}

func TestExtractBlocksInlineMarkupStaysOneBlockWithLinks(t *testing.T) {
	doc := ExtractBlocks(readFixture(t, "article.html"))
	b := findBlock(t, doc.Blocks, "Construction is expected")
	if b.Kind != KindParagraph {
		t.Fatalf("kind = %q", b.Kind)
	}
	if len(b.Links) != 1 {
		t.Fatalf("links = %+v, want one", b.Links)
	}
	link := b.Links[0]
	if link.Href != "https://example.gov/budget-2026" || b.Text[link.Start:link.End] != "an estimated $310 million" {
		t.Fatalf("link = %+v covering %q", link, b.Text[link.Start:link.End])
	}
}

func TestExtractBlocksDuplicateParagraphsGetSuffixedIDs(t *testing.T) {
	doc := ExtractBlocks(readFixture(t, "article.html"))
	var ids []string
	for _, b := range doc.Blocks {
		if b.Text == "Read the full council report." {
			ids = append(ids, b.ID)
		}
	}
	cid := normtext.CID(KindParagraph, "Read the full council report.")
	if len(ids) != 2 || ids[0] != cid || ids[1] != cid+".1" {
		t.Fatalf("duplicate ids = %v, want [%s %s.1]", ids, cid, cid)
	}
	in := make([]normtext.Block, len(doc.Blocks))
	for i, b := range doc.Blocks {
		in[i] = normtext.Block{Kind: b.Kind, Text: b.Text}
	}
	for i, id := range normtext.AssignCIDs(in) {
		if doc.Blocks[i].ID != id {
			t.Fatalf("block %d id = %q, want normtext's %q", i, doc.Blocks[i].ID, id)
		}
	}
}

func TestExtractBlocksTableRowsCaptionFootnotesAndLinks(t *testing.T) {
	doc := ExtractBlocks(readFixture(t, "wiki-table.html"))
	want := []string{
		"heading: List of largest cities",
		"paragraph: The United Nations uses three definitions for what constitutes a city, as not all cities may be classified using the same criteria.[12] Cities may be defined as the city proper, the extent of the urban area, or the metropolitan area.[13]",
		"heading: List",
		"table_caption: Largest urban agglomerations, 2025 estimates",
		"table_header: City | Country | Population",
		"table_row: Tokyo | Japan | 37,036,200",
		"table_row: Delhi | India | 34,665,600[14]",
		"table_row: Shanghai | China | 30,482,100",
		"paragraph: A layout table follows.",
		"paragraph: First layout paragraph about rivers and ports.",
		"paragraph: Second layout paragraph about rail lines.",
		"heading: References",
		"note: \"World Urbanization Prospects\". United Nations. 2025.",
		"note: Demographia World Urban Areas, 2025 edition.",
	}
	got := blockTexts(doc.Blocks)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	para := findBlock(t, doc.Blocks, "The United Nations")
	hrefs := map[string]string{}
	for _, l := range para.Links {
		hrefs[para.Text[l.Start:l.End]] = l.Href
	}
	if hrefs["[12]"] != "#cite_note-12" || hrefs["city proper"] != "/wiki/City_proper" || hrefs["metropolitan area"] != "/wiki/Metropolitan_area" {
		t.Fatalf("paragraph links = %v", hrefs)
	}
	row := findBlock(t, doc.Blocks, "Tokyo")
	if len(row.Links) != 2 || row.Text[row.Links[1].Start:row.Links[1].End] != "Japan" || row.Links[1].Href != "/wiki/Japan" {
		t.Fatalf("row links = %+v", row.Links)
	}
	// The census's pipe-joined table quote lands in one row block.
	res := quotematch.Match("Delhi | India | 34,665,600", matchBlocks(doc.Blocks))
	if res.Outcome != quotematch.Matched || res.BlockID != findBlock(t, doc.Blocks, "Delhi").ID || res.Tier != quotematch.TierExact {
		t.Fatalf("row quote = %+v", res)
	}
}

func matchBlocks(blocks []Block) []quotematch.Block {
	out := make([]quotematch.Block, len(blocks))
	for i, b := range blocks {
		out[i] = quotematch.Block{ID: b.ID, Text: b.Text}
	}
	return out
}

func TestQuoteFromArticleMatchesOneBlockAndNeverSpansTwo(t *testing.T) {
	doc := ExtractBlocks(readFixture(t, "article.html"))
	blocks := matchBlocks(doc.Blocks)
	res := quotematch.Match("shield roughly 18,000 homes and the main rail yard", blocks)
	if res.Outcome != quotematch.Matched || res.Tier != quotematch.TierExact || res.BlockID != findBlock(t, doc.Blocks, "Officials said").ID {
		t.Fatalf("quote = %+v", res)
	}
	// Across the <br>: still one block.
	res = quotematch.Match("according to the city budget office. The state will cover", blocks)
	if res.Outcome != quotematch.Matched || res.BlockID != findBlock(t, doc.Blocks, "Construction").ID {
		t.Fatalf("quote across <br> = %+v", res)
	}
	// Two paragraphs never merge into one block.
	res = quotematch.Match("ending a debate that has run for six years. Construction is expected", blocks)
	if res.Outcome != quotematch.NotFound {
		t.Fatalf("a quote spanning two paragraphs matched: %+v", res)
	}
}

func TestDocHashIgnoresMarkupChurnAndTracksText(t *testing.T) {
	page := readFixture(t, "article.html")
	base := ExtractBlocks(page)
	churned := strings.Replace(string(page), `"nonce": "a8f3"`, `"nonce": "77c1"`, 1)
	churned = strings.Replace(churned, "Buy one mattress", "Two-for-one sofas", 1)
	churned = strings.Replace(churned, "<p>Officials said", "<p class=\"lede-2\" data-x=\"9\">\n  Officials   said", 1)
	if got := ExtractBlocks([]byte(churned)).DocHash(); got != base.DocHash() {
		t.Fatalf("ad, nonce and whitespace churn changed doc_hash: %s != %s", got, base.DocHash())
	}
	edited := strings.Replace(string(page), "voted 7 to 2", "voted 6 to 3", 1)
	if got := ExtractBlocks([]byte(edited)).DocHash(); got == base.DocHash() {
		t.Fatal("a changed paragraph kept the same doc_hash")
	}
	if !strings.HasPrefix(base.DocHash(), "sha256:") {
		t.Fatalf("doc_hash = %q", base.DocHash())
	}
}

func TestExtractReadableHTMLIsTheBlockProjection(t *testing.T) {
	page := readFixture(t, "article.html")
	title, text := ExtractReadableHTML(page)
	doc := ExtractBlocks(page)
	if title != doc.Title || text != doc.Projection() {
		t.Fatalf("ExtractReadableHTML = (%q, %q), want the block projection", title, text)
	}
	if !strings.Contains(text, "six years.\n\nConstruction") {
		t.Fatalf("projection does not separate blocks with a blank line:\n%s", text)
	}
}

// TextBlocks reads text that has no markup: a feed summary, a reader
// snapshot (one block per line), or a PDF page (paragraphs separated by
// blank lines, hard-wrapped lines joined).
func TestTextBlocksSplitsParagraphsOrLines(t *testing.T) {
	lines := TextBlocks("First line.\nSecond line.\n\n")
	if len(lines) != 2 || lines[0].Text != "First line." || lines[1].Text != "Second line." {
		t.Fatalf("line blocks = %+v", lines)
	}
	paras := TextBlocks("A wrapped\nparagraph here.\n\n  Second\n  paragraph.\n")
	if len(paras) != 2 || paras[0].Text != "A wrapped paragraph here." || paras[1].Text != "Second paragraph." {
		t.Fatalf("paragraph blocks = %+v", paras)
	}
	page := PageBlocks([]string{"Page one text.", "", "Page three, first.\n\nPage three, second."})
	if len(page) != 3 || page[0].Page != 1 || page[1].Page != 3 || page[2].Page != 3 || page[2].Text != "Page three, second." {
		t.Fatalf("page blocks = %+v", page)
	}
	for _, b := range append(append(lines, paras...), page...) {
		if b.Kind != KindParagraph || b.ID == "" {
			t.Fatalf("block = %+v", b)
		}
	}
}
