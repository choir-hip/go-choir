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
	"unicode/utf8"
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
	switch format {
	case "docx":
		content, err := buildPublicationDOCX(bundle, metadata)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	case "pdf":
		content, err := buildPublicationPDF(bundle, metadata)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	default:
		return publicationExportBytes{content: []byte(formatPublicationExportContent(bundle, format)), metadata: metadata}, nil
	}
}

func publicationExportMetadata(bundle *PublicationBundle, format string) json.RawMessage {
	if bundle == nil {
		return json.RawMessage("{}")
	}
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
	})
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func buildPublicationDOCX(bundle *PublicationBundle, metadata json.RawMessage) ([]byte, error) {
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
		}),
		"word/document.xml":            docxDocumentXML(bundle),
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
		`</Types>`
}

func packageRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/custom-properties" Target="docProps/custom.xml"/>` +
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

func docxDocumentXML(bundle *PublicationBundle) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, block := range markdownBlocks(bundle.Artifact.Content) {
		switch block.kind {
		case "heading":
			b.WriteString(docxParagraph(block.text, "Heading"+strconv.Itoa(clampInt(block.level, 1, 6))))
		case "list":
			b.WriteString(docxParagraph("* "+block.text, "ListParagraph"))
		case "table":
			b.WriteString(docxTable(block.rows))
		default:
			b.WriteString(docxParagraph(block.text, ""))
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func docxParagraph(text, style string) string {
	var b strings.Builder
	b.WriteString(`<w:p>`)
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		b.WriteString(xmlEscape(style))
		b.WriteString(`"/></w:pPr>`)
	}
	b.WriteString(`<w:r><w:t xml:space="preserve">`)
	b.WriteString(xmlEscape(text))
	b.WriteString(`</w:t></w:r></w:p>`)
	return b.String()
}

func docxTable(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:left w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:right w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/></w:tblBorders></w:tblPr>`)
	for _, row := range rows {
		b.WriteString(`<w:tr>`)
		for _, cell := range row {
			b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr>`)
			b.WriteString(docxParagraph(cell, ""))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

func buildPublicationPDF(bundle *PublicationBundle, metadata json.RawMessage) ([]byte, error) {
	title := firstNonEmpty(bundle.Publication.Title, "Published VText")
	lines := wrapPDFLines(title+"\n\n"+bundle.Artifact.Content, 92)
	if len(lines) == 0 {
		lines = []string{title}
	}
	xmp := pdfMetadataXML(bundle, metadata)
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

func pdfMetadataXML(bundle *PublicationBundle, metadata json.RawMessage) string {
	return `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>` +
		`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description rdf:about="" xmlns:choir="https://choir.news/ns/publication-export/1.0/" choir:publicationId="` + xmlEscape(bundle.Publication.ID) + `" choir:publicationVersionId="` + xmlEscape(bundle.Version.ID) + `" choir:routePath="` + xmlEscape(bundle.Route.Path) + `" choir:contentHash="` + xmlEscape(bundle.Version.ContentHash) + `">` +
		`<choir:metadata>` + xmlEscape(string(metadata)) + `</choir:metadata>` +
		`</rdf:Description></rdf:RDF></x:xmpmeta>` +
		`<?xpacket end="w"?>`
}

type markdownBlock struct {
	kind  string
	level int
	text  string
	rows  [][]string
}

func markdownBlocks(content string) []markdownBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	blocks := []markdownBlock{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || line == "---" {
			continue
		}
		if isMarkdownTableStart(lines, i) {
			rows := [][]string{}
			rows = append(rows, splitMarkdownTableRow(lines[i]))
			i += 2
			for i < len(lines) && strings.Contains(lines[i], "|") {
				rows = append(rows, splitMarkdownTableRow(lines[i]))
				i++
			}
			i--
			blocks = append(blocks, markdownBlock{kind: "table", rows: rows})
			continue
		}
		if strings.HasPrefix(line, "#") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			if level > 0 && level < len(line) && line[level] == ' ' {
				blocks = append(blocks, markdownBlock{kind: "heading", level: level, text: strings.TrimSpace(line[level:])})
				continue
			}
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			blocks = append(blocks, markdownBlock{kind: "list", text: strings.TrimSpace(line[2:])})
			continue
		}
		blocks = append(blocks, markdownBlock{kind: "paragraph", text: line})
	}
	return blocks
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
			if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) > width && line != "" {
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
