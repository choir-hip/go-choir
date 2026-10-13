package content

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// Ways PDF capture can fail, written before the code (Cut 2 unit 2a,
// decisions §7):
//
//   - Pages run together, so a block has no page number or the wrong one.
//   - A paragraph hard-wrapped across lines is split at the wrap, so a quote
//     that crosses the wrap matches no block.
//   - Without poppler (pdftotext) the capture has no blocks at all; the
//     literal-text fallback must still give page-numbered blocks.
//   - Block ids are not the normtext cids every other capture uses.
func TestPDFBlocksArePageNumberedParagraphsAQuoteMatches(t *testing.T) {
	raw, err := os.ReadFile("testdata/rainfall-two-pages.pdf")
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, doc sourcefetch.Document) {
		t.Helper()
		if doc.Extractor != sourcefetch.PDFExtractorVersion {
			t.Fatalf("extractor = %q", doc.Extractor)
		}
		pages := map[int]bool{}
		blocks := make([]quotematch.Block, len(doc.Blocks))
		for i, b := range doc.Blocks {
			if b.Kind != sourcefetch.KindParagraph || b.Page < 1 || b.ID == "" {
				t.Fatalf("block %d = %+v", i, b)
			}
			pages[b.Page] = true
			blocks[i] = quotematch.Block{ID: b.ID, Text: b.Text}
		}
		if !pages[1] || !pages[2] || len(pages) != 2 {
			t.Fatalf("pages = %v; blocks %+v", pages, doc.Blocks)
		}
		res := quotematch.Match("rose 14 percent over the prior quarter", blocks)
		if res.Outcome != quotematch.Matched {
			t.Fatalf("quote across a line wrap = %+v; blocks %+v", res, doc.Blocks)
		}
		for _, b := range doc.Blocks {
			if b.ID == res.BlockID && b.Page != 2 {
				t.Fatalf("quote matched a page %d block, want page 2", b.Page)
			}
		}
		check := append([]sourcefetch.Block(nil), doc.Blocks...)
		sourcefetch.AssignIDs(check)
		for i := range check {
			if check[i].ID != doc.Blocks[i].ID {
				t.Fatalf("block %d id %q is not its cid %q", i, doc.Blocks[i].ID, check[i].ID)
			}
		}
	}
	t.Run("literal fallback", func(t *testing.T) {
		check(t, sourcefetch.Document{Extractor: sourcefetch.PDFExtractorVersion, Blocks: sourcefetch.PageBlocks(pdfLiteralPages(raw))})
	})
	t.Run("PDFBlocks", func(t *testing.T) {
		if _, err := exec.LookPath("pdftotext"); err != nil {
			t.Log("pdftotext unavailable: PDFBlocks uses the literal fallback")
		}
		doc, warnings := PDFBlocks(context.Background(), raw)
		check(t, doc)
		if _, err := exec.LookPath("pdftotext"); err == nil {
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v", warnings)
			}
			if got := doc.Blocks[0].Text; got != "Quarterly Rainfall Report" {
				t.Fatalf("first block = %q", got)
			}
			if !strings.Contains(doc.Projection(), "the largest rise since 2019.") {
				t.Fatalf("projection = %q", doc.Projection())
			}
		}
	})
}
