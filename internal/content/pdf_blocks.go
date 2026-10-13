package content

import (
	"context"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// PDFBlocks reads a PDF into a capture's block document (Cut 2 unit 2a,
// decisions §7): each page's text, in pdftotext's reading order, is split
// into paragraph blocks carrying the page number, so a quote from a PDF is
// checked against one block like any other capture. Layout (columns,
// tables, figures) is not reconstructed. Without pdftotext, the literal
// text of each page's content stream is used, one block per page. The title
// is the PDF's own Title when pdfinfo reports one.
func PDFBlocks(ctx context.Context, raw []byte) (sourcefetch.Document, []string) {
	doc := sourcefetch.Document{Extractor: sourcefetch.PDFExtractorVersion}
	text, warnings := runDocumentToolOnTempFile(ctx, "pdftotext", nil, ".pdf", raw, []string{"-"})
	var pages []string
	if strings.TrimSpace(text) != "" {
		pages = strings.Split(text, "\f")
	} else {
		pages = pdfLiteralPages(raw)
		if len(pages) > 0 {
			warnings = appendIfMissing(warnings, "pdf_literal_fallback_used")
		} else {
			warnings = appendIfMissing(warnings, "pdf_text_extraction_empty")
		}
	}
	doc.Blocks = sourcefetch.PageBlocks(pages)
	if info, _ := runDocumentToolOnTempFile(ctx, "pdfinfo", nil, ".pdf", raw, nil); strings.TrimSpace(info) != "" {
		if title, ok := parsePDFInfo(info)["title"].(string); ok {
			doc.Title = strings.TrimSpace(title)
		}
	}
	return doc, warnings
}

var pdfStreamRE = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)

// pdfLiteralPages returns the literal text of each content stream that has
// any, in file order, as one page each with its lines joined: a crude
// fallback that reads only uncompressed streams.
func pdfLiteralPages(raw []byte) []string {
	var pages []string
	for _, m := range pdfStreamRE.FindAllSubmatch(raw, -1) {
		text := strings.TrimSpace(ExtractPDFLiteralText(m[1]))
		if text == "" {
			continue
		}
		pages = append(pages, strings.Join(strings.Fields(text), " "))
	}
	return pages
}
