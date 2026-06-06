package platform

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

type publicationExportBytes struct {
	content  []byte
	metadata json.RawMessage
}

func buildPublicationExportBytes(bundle *PublicationBundle, format string) (publicationExportBytes, error) {
	if bundle == nil {
		return publicationExportBytes{}, fmt.Errorf("publication bundle is required")
	}
	metadata := publicationExportMetadata(bundle, format)
	doc := buildPublicationDocument(bundle)
	switch format {
	case "docx":
		content, err := buildPublicationDOCX(bundle, doc, metadata)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	case "pdf":
		content, err := buildPublicationPDF(bundle, doc)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	case "html":
		return publicationExportBytes{content: []byte(renderPublicationHTML(doc)), metadata: metadata}, nil
	default:
		return publicationExportBytes{content: []byte(formatPublicationExportContent(bundle, format)), metadata: metadata}, nil
	}
}

func publicationExportMetadata(bundle *PublicationBundle, format string) json.RawMessage {
	if bundle == nil {
		return json.RawMessage("{}")
	}
	sourceManifest := buildPublicationSourceManifest(bundle)
	raw, err := json.Marshal(map[string]any{
		"schema":                   "choir.publication_export.v0",
		"format":                   format,
		"publication_id":           bundle.Publication.ID,
		"publication_version_id":   bundle.Version.ID,
		"route_path":               bundle.Route.Path,
		"content_hash":             bundle.Version.ContentHash,
		"source_revision_hash":     bundle.Version.SourceRevisionHash,
		"projection_hash":          bundle.Version.ProjectionHash,
		"artifact_manifest_id":     bundle.Artifact.ManifestID,
		"generated_at":             time.Now().UTC().Format(time.RFC3339Nano),
		"provenance_scope":         "public_publication_version_only",
		"private_material_omitted": true,
		"access_policy":            json.RawMessage(firstNonEmpty(string(bundle.Policy.Access), "{}")),
		"export_policy":            json.RawMessage(firstNonEmpty(string(bundle.Policy.Export), "{}")),
		"retrieval":                bundle.Retrieval,
		"source_entities":          bundle.SourceEntities,
		"transclusions":            bundle.Transclusions,
		"source_manifest":          sourceManifest,
	})
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func buildPublicationDOCX(bundle *PublicationBundle, doc PublicationDocument, metadata json.RawMessage) ([]byte, error) {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": contentTypesXML(),
		"_rels/.rels":         packageRelsXML(),
		"docProps/core.xml":   docxCoreXML(bundle),
		"docProps/custom.xml": docxCustomXML(map[string]string{
			"ChoirPublicationID":        bundle.Publication.ID,
			"ChoirPublicationVersionID": bundle.Version.ID,
			"ChoirRoutePath":            bundle.Route.Path,
			"ChoirContentHash":          bundle.Version.ContentHash,
			"ChoirExportMetadata":       string(metadata),
			"ChoirSourceManifestSchema": doc.Manifest.Schema,
		}),
		"customXml/item1.xml": docxSourceManifestXML(manifestJSON),
		"customXml/_rels/item1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml":            docxDocumentXML(doc),
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create docx part %s: %w", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write docx part %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close docx: %w", err)
	}
	return buf.Bytes(), nil
}

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/custom.xml" ContentType="application/vnd.openxmlformats-officedocument.custom-properties+xml"/>` +
		`<Override PartName="/customXml/item1.xml" ContentType="application/xml"/>` +
		`</Types>`
}

func packageRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/custom-properties" Target="docProps/custom.xml"/>` +
		`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXml" Target="customXml/item1.xml"/>` +
		`</Relationships>`
}

func docxCoreXML(bundle *PublicationBundle) string {
	title := xmlEscape(firstNonEmpty(bundle.Publication.Title, "Published VText"))
	now := time.Now().UTC().Format(time.RFC3339)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title>` + title + `</dc:title>` +
		`<dc:creator>Choir</dc:creator>` +
		`<cp:lastModifiedBy>Choir</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

func docxCustomXML(values map[string]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`)
	pid := 2
	for key, value := range values {
		b.WriteString(`<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="`)
		b.WriteString(strconv.Itoa(pid))
		b.WriteString(`" name="`)
		b.WriteString(xmlEscape(key))
		b.WriteString(`"><vt:lpwstr>`)
		b.WriteString(xmlEscape(value))
		b.WriteString(`</vt:lpwstr></property>`)
		pid++
	}
	b.WriteString(`</Properties>`)
	return b.String()
}

func docxSourceManifestXML(manifestJSON string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<choirSourceManifest xmlns="https://choir.news/ns/publication-sources/1.0/"><json>` +
		xmlEscape(manifestJSON) +
		`</json></choirSourceManifest>`
}

func docxDocumentXML(doc PublicationDocument) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			b.WriteString(docxParagraph(block.Inlines, "Heading"+strconv.Itoa(clampInt(block.Level, 1, 6))))
		case "list_item":
			b.WriteString(docxParagraph(append([]publicationInline{{Kind: "text", Text: "• "}}, block.Inlines...), "ListParagraph"))
		case "table":
			b.WriteString(docxTable(block.Rows))
		case "rule":
			b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: ""}}, ""))
		default:
			b.WriteString(docxParagraph(block.Inlines, ""))
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: "Sources"}}, "Heading1"))
		for i, source := range doc.Manifest.Sources {
			text := fmt.Sprintf("[%d] %s", i+1, firstNonEmpty(source.Title, source.SourceEntityID))
			if source.URL != "" {
				text += " — " + source.URL
			}
			if source.SnapshotText != "" {
				text += " — " + source.SnapshotText
			}
			b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: text}}, ""))
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func docxParagraph(inlines []publicationInline, style string) string {
	var b strings.Builder
	b.WriteString(`<w:p>`)
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		b.WriteString(xmlEscape(style))
		b.WriteString(`"/></w:pPr>`)
	}
	b.WriteString(docxRuns(inlines))
	b.WriteString(`</w:p>`)
	return b.String()
}

func docxRuns(inlines []publicationInline) string {
	var b strings.Builder
	for _, inline := range inlines {
		text := inline.Text
		if inline.Kind == "source_ref" {
			text = inline.Text + " [" + firstNonEmpty(inline.SourceID, "source") + "]"
		}
		b.WriteString(`<w:r>`)
		if inline.Kind == "strong" || inline.Kind == "em" || inline.Kind == "source_ref" {
			b.WriteString(`<w:rPr>`)
			if inline.Kind == "strong" {
				b.WriteString(`<w:b/>`)
			}
			if inline.Kind == "em" {
				b.WriteString(`<w:i/>`)
			}
			if inline.Kind == "source_ref" {
				b.WriteString(`<w:vertAlign w:val="superscript"/><w:color w:val="2F5597"/>`)
			}
			b.WriteString(`</w:rPr>`)
		}
		b.WriteString(`<w:t xml:space="preserve">`)
		b.WriteString(xmlEscape(text))
		b.WriteString(`</w:t></w:r>`)
	}
	return b.String()
}

func docxTable(rows [][]publicationTableCell) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:left w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:right w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/></w:tblBorders></w:tblPr>`)
	for _, row := range rows {
		b.WriteString(`<w:tr>`)
		for _, cell := range row {
			b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr>`)
			style := ""
			inlines := cell.Inlines
			if cell.Header {
				inlines = forceStrongInlines(inlines)
			}
			b.WriteString(docxParagraph(inlines, style))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

func forceStrongInlines(inlines []publicationInline) []publicationInline {
	out := make([]publicationInline, 0, len(inlines))
	for _, inline := range inlines {
		if inline.Kind == "text" {
			inline.Kind = "strong"
		}
		out = append(out, inline)
	}
	return out
}

func buildPublicationPDF(bundle *PublicationBundle, doc PublicationDocument) ([]byte, error) {
	title := doc.Title
	lines := wrapPDFLines(publicationDocumentPlainText(doc), 92)
	if len(lines) == 0 {
		lines = []string{title}
	}
	xmp := pdfMetadataXML(bundle, doc)
	pageLineCount := 48
	pageCount := (len(lines) + pageLineCount - 1) / pageLineCount
	infoObjectNumber := 5 + pageCount*2
	kids := make([]string, 0, pageCount)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /Metadata 4 0 R >>",
		"",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len([]byte(xmp)), xmp),
	}
	for page := 0; page < pageCount; page++ {
		pageObjectNumber := 5 + page*2
		contentObjectNumber := pageObjectNumber + 1
		kids = append(kids, strconv.Itoa(pageObjectNumber)+" 0 R")
		pageLines := lines[page*pageLineCount : minInt(len(lines), (page+1)*pageLineCount)]
		streamText := pdfPageStream(pageLines)
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentObjectNumber),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len([]byte(streamText)), streamText),
		)
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pageCount)
	objects = append(objects, "<< /Producer (Choir) /Title ("+pdfEscape(title)+") >>")
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, buf.Len())
		buf.WriteString(strconv.Itoa(i + 1))
		buf.WriteString(" 0 obj\n")
		buf.WriteString(obj)
		buf.WriteString("\nendobj\n")
	}
	xref := buf.Len()
	buf.WriteString("xref\n0 ")
	buf.WriteString(strconv.Itoa(len(objects) + 1))
	buf.WriteString("\n0000000000 65535 f \n")
	for i := 1; i < len(offsets); i++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	buf.WriteString("trailer\n<< /Size ")
	buf.WriteString(strconv.Itoa(len(objects) + 1))
	buf.WriteString(" /Root 1 0 R /Info ")
	buf.WriteString(strconv.Itoa(infoObjectNumber))
	buf.WriteString(" 0 R >>\nstartxref\n")
	buf.WriteString(strconv.Itoa(xref))
	buf.WriteString("\n%%EOF\n")
	return buf.Bytes(), nil
}

func pdfPageStream(lines []string) string {
	var stream strings.Builder
	stream.WriteString("BT\n/F1 11 Tf\n50 760 Td\n14 TL\n")
	for i, line := range lines {
		if i > 0 {
			stream.WriteString("T*\n")
		}
		stream.WriteString("(")
		stream.WriteString(pdfEscape(line))
		stream.WriteString(") Tj\n")
	}
	stream.WriteString("ET\n")
	return stream.String()
}

func pdfMetadataXML(bundle *PublicationBundle, doc PublicationDocument) string {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	return `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>` +
		`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description rdf:about="" xmlns:choir="https://choir.news/ns/publication-export/1.0/" choir:publicationId="` + xmlEscape(bundle.Publication.ID) + `" choir:publicationVersionId="` + xmlEscape(bundle.Version.ID) + `" choir:routePath="` + xmlEscape(bundle.Route.Path) + `" choir:contentHash="` + xmlEscape(bundle.Version.ContentHash) + `">` +
		`<choir:exportSchema>choir.publication_export.v0</choir:exportSchema>` +
		`<choir:sourceManifest>` + xmlEscape(manifestJSON) + `</choir:sourceManifest>` +
		`</rdf:Description></rdf:RDF></x:xmpmeta>` +
		`<?xpacket end="w"?>`
}

func renderPublicationHTML(doc PublicationDocument) string {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(doc.Title))
	b.WriteString(`</title><meta name="generator" content="Choir">`)
	b.WriteString(`<script type="application/ld+json">`)
	b.WriteString(safeScriptJSON(publicationJSONLD(doc)))
	b.WriteString(`</script><script type="application/json" id="choir-source-manifest">`)
	b.WriteString(safeScriptJSON(manifestJSON))
	b.WriteString(`</script></head><body><article class="vtext-publication">`)
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			level := clampInt(block.Level, 1, 6)
			b.WriteString("<h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">")
			b.WriteString(renderHTMLInlines(block.Inlines))
			b.WriteString("</h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">\n")
		case "paragraph":
			b.WriteString("<p>")
			b.WriteString(renderHTMLInlines(block.Inlines))
			b.WriteString("</p>\n")
		case "list_item":
			b.WriteString("<ul><li>")
			b.WriteString(renderHTMLInlines(block.Inlines))
			b.WriteString("</li></ul>\n")
		case "table":
			b.WriteString("<table><tbody>")
			for _, row := range block.Rows {
				b.WriteString("<tr>")
				for _, cell := range row {
					tag := "td"
					if cell.Header {
						tag = "th"
					}
					b.WriteString("<")
					b.WriteString(tag)
					b.WriteString(">")
					b.WriteString(renderHTMLInlines(cell.Inlines))
					b.WriteString("</")
					b.WriteString(tag)
					b.WriteString(">")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table>\n")
		case "rule":
			b.WriteString("<hr>\n")
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString(`<section class="vtext-sources" aria-labelledby="vtext-sources-heading"><h2 id="vtext-sources-heading">Sources</h2><ol>`)
		for _, source := range doc.Manifest.Sources {
			b.WriteString(`<li id="source-`)
			b.WriteString(html.EscapeString(source.SourceEntityID))
			b.WriteString(`"><span class="source-title">`)
			b.WriteString(html.EscapeString(firstNonEmpty(source.Title, source.SourceEntityID)))
			b.WriteString(`</span>`)
			if source.URL != "" {
				b.WriteString(` <a href="`)
				b.WriteString(html.EscapeString(source.URL))
				b.WriteString(`">`)
				b.WriteString(html.EscapeString(source.URL))
				b.WriteString(`</a>`)
			}
			if source.SnapshotText != "" {
				b.WriteString(`<blockquote>`)
				b.WriteString(html.EscapeString(source.SnapshotText))
				b.WriteString(`</blockquote>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol></section>`)
	}
	b.WriteString("</article></body></html>\n")
	return b.String()
}

func renderHTMLInlines(inlines []publicationInline) string {
	var b strings.Builder
	for _, inline := range inlines {
		switch inline.Kind {
		case "strong":
			b.WriteString("<strong>")
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString("</strong>")
		case "em":
			b.WriteString("<em>")
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString("</em>")
		case "link":
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(inline.Href))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString(`</a>`)
		case "source_ref":
			b.WriteString(`<a class="vtext-source-ref" href="#source-`)
			b.WriteString(html.EscapeString(inline.SourceID))
			b.WriteString(`" data-source-id="`)
			b.WriteString(html.EscapeString(inline.SourceID))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString(`</a>`)
		default:
			b.WriteString(html.EscapeString(inline.Text))
		}
	}
	return b.String()
}

func publicationJSONLD(doc PublicationDocument) string {
	citations := make([]map[string]string, 0, len(doc.Manifest.Sources))
	for _, source := range doc.Manifest.Sources {
		citation := map[string]string{
			"@type":      "CreativeWork",
			"identifier": source.SourceEntityID,
			"name":       firstNonEmpty(source.Title, source.SourceEntityID),
		}
		if source.URL != "" {
			citation["url"] = source.URL
		}
		citations = append(citations, citation)
	}
	raw, err := json.Marshal(map[string]any{
		"@context": "https://schema.org",
		"@type":    "CreativeWork",
		"name":     doc.Title,
		"identifier": map[string]string{
			"publication_id":         doc.Manifest.PublicationID,
			"publication_version_id": doc.Manifest.PublicationVersionID,
		},
		"citation": citations,
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func publicationSourceManifestJSON(manifest publicationSourceManifest) string {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func safeScriptJSON(value string) string {
	value = strings.ReplaceAll(value, "</", `<\/`)
	value = strings.ReplaceAll(value, "<!--", `<\!--`)
	return value
}

func publicationDocumentPlainText(doc PublicationDocument) string {
	var b strings.Builder
	b.WriteString(doc.Title)
	b.WriteString("\n\n")
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			b.WriteString(publicationInlinesPlainText(block.Inlines))
			b.WriteString("\n\n")
		case "paragraph":
			b.WriteString(publicationInlinesPlainText(block.Inlines))
			b.WriteString("\n\n")
		case "list_item":
			b.WriteString("• ")
			b.WriteString(publicationInlinesPlainText(block.Inlines))
			b.WriteString("\n")
		case "table":
			for _, row := range block.Rows {
				values := make([]string, 0, len(row))
				for _, cell := range row {
					values = append(values, publicationInlinesPlainText(cell.Inlines))
				}
				b.WriteString(strings.Join(values, " | "))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString("Sources\n")
		for i, source := range doc.Manifest.Sources {
			b.WriteString("[")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString("] ")
			b.WriteString(firstNonEmpty(source.Title, source.SourceEntityID))
			if source.URL != "" {
				b.WriteString(" ")
				b.WriteString(source.URL)
			}
			if source.SnapshotText != "" {
				b.WriteString(" ")
				b.WriteString(source.SnapshotText)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func isMarkdownTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	return strings.Contains(lines[i], "|") && markdownTableSeparator(lines[i+1])
}

func markdownTableSeparator(line string) bool {
	clean := strings.TrimSpace(strings.Trim(line, "|"))
	if clean == "" {
		return false
	}
	for _, part := range strings.Split(clean, "|") {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			return false
		}
		for _, r := range part {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(strings.Trim(line, "|"))
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(strings.ReplaceAll(part, `\|`, "|")))
	}
	return out
}

func wrapPDFLines(text string, width int) []string {
	var out []string
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		words := strings.Fields(raw)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range words {
			if len([]rune(line))+1+len([]rune(word)) > width && line != "" {
				out = append(out, line)
				line = word
				continue
			}
			if line == "" {
				line = word
			} else {
				line += " " + word
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func xmlEscape(s string) string {
	return html.EscapeString(s)
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
