package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestExtractPPTXDocumentCreatesSlideSelectors(t *testing.T) {
	raw := buildMinimalPPTX(t, []string{
		"Choir source substrate\nDocuments become citeable source artifacts",
		"Compaction eval\nFrozen corpus beats search quota burn",
	})
	extracted := extractContentDocument(context.Background(), "deck.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", raw)
	if extracted.AppHint != "slides" {
		t.Fatalf("app hint = %q, want slides", extracted.AppHint)
	}
	if extracted.Adapter != "pptx_ooxml_slide_text_projection" {
		t.Fatalf("adapter = %q", extracted.Adapter)
	}
	if !strings.Contains(extracted.Text, "Choir source substrate") || !strings.Contains(extracted.Text, "Compaction eval") {
		t.Fatalf("extracted text = %q", extracted.Text)
	}
	if len(extracted.Selectors) != 2 || extracted.Selectors[0].ID != "slide-1" || extracted.Selectors[1].ID != "slide-2" {
		t.Fatalf("selectors = %#v", extracted.Selectors)
	}
	if got := extracted.Metadata["slide_count"]; got != 2 {
		t.Fatalf("slide_count = %#v, want 2", got)
	}
}

func TestExtractDOCXDocumentFallsBackToOOXMLSelectors(t *testing.T) {
	raw := buildMinimalExtractionDOCX(t, []string{"Proposal Title", "Opening paragraph"}, [][]string{
		{"Term", "Definition"},
		{"Source", "Durable artifact"},
	})
	extracted := extractContentDocument(context.Background(), "brief.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", raw)
	if !strings.Contains(extracted.Text, "Proposal Title") || !strings.Contains(extracted.Text, "| Term | Definition |") {
		t.Fatalf("docx extracted text = %q", extracted.Text)
	}
	if len(extracted.Selectors) == 0 {
		t.Fatalf("docx selectors missing")
	}
}

func buildMinimalPPTX(t *testing.T, slides []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create pptx part %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write pptx part %s: %v", name, err)
		}
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	for i, slide := range slides {
		var body strings.Builder
		body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>`)
		for _, line := range strings.Split(slide, "\n") {
			body.WriteString(`<p:sp><p:txBody><a:p><a:r><a:t>`)
			body.WriteString(escapeExtractionXMLText(line))
			body.WriteString(`</a:t></a:r></a:p></p:txBody></p:sp>`)
		}
		body.WriteString(`</p:spTree></p:cSld></p:sld>`)
		add("ppt/slides/slide"+strconv.Itoa(i+1)+".xml", body.String())
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close pptx zip: %v", err)
	}
	return buf.Bytes()
}

func buildMinimalExtractionDOCX(t *testing.T, paragraphs []string, table [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create docx part %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write docx part %s: %v", name, err)
		}
	}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t>`)
		body.WriteString(escapeExtractionXMLText(paragraph))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`<w:tbl>`)
	for _, row := range table {
		body.WriteString(`<w:tr>`)
		for _, cell := range row {
			body.WriteString(`<w:tc><w:p><w:r><w:t>`)
			body.WriteString(escapeExtractionXMLText(cell))
			body.WriteString(`</w:t></w:r></w:p></w:tc>`)
		}
		body.WriteString(`</w:tr>`)
	}
	body.WriteString(`</w:tbl></w:body></w:document>`)
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	add("word/document.xml", body.String())
	if err := zw.Close(); err != nil {
		t.Fatalf("close docx zip: %v", err)
	}
	return buf.Bytes()
}

func escapeExtractionXMLText(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(text)
}
