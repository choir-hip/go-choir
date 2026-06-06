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
	profile := defaultPublicationExportProfile()
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
			"ChoirExportProfile":        profile.ID,
			"ChoirExportMetadata":       string(metadata),
			"ChoirSourceManifestSchema": doc.Manifest.Schema,
		}),
		"customXml/item1.xml": docxSourceManifestXML(manifestJSON),
		"customXml/_rels/item1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml":            docxDocumentXML(doc),
		"word/styles.xml":              docxStylesXML(),
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
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
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
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

func docxStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:spacing w:after="180" w:line="300" w:lineRule="auto"/></w:pPr><w:rPr><w:rFonts w:ascii="Aptos" w:hAnsi="Aptos"/><w:sz w:val="22"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:pPr><w:spacing w:before="0" w:after="280"/></w:pPr><w:rPr><w:b/><w:sz w:val="34"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="360" w:after="160"/></w:pPr><w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="280" w:after="120"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="220" w:after="100"/></w:pPr><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="360" w:hanging="180"/><w:spacing w:after="120"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="SourceAppendix"><w:name w:val="Source Appendix"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="120"/></w:pPr><w:rPr><w:sz w:val="20"/></w:rPr></w:style>` +
		`</w:styles>`
}

func docxDocumentXML(doc PublicationDocument) string {
	var b strings.Builder
	ordinals := publicationSourceOrdinals(doc)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	wroteTitle := false
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			style := "Heading" + strconv.Itoa(clampInt(block.Level, 1, 6))
			if block.Level == 1 && !wroteTitle {
				style = "Title"
				wroteTitle = true
			}
			b.WriteString(docxParagraph(block.Inlines, style, ordinals))
		case "list_item":
			b.WriteString(docxParagraph(append([]publicationInline{{Kind: "text", Text: "- "}}, block.Inlines...), "ListParagraph", ordinals))
		case "table":
			b.WriteString(docxTable(block.Rows, ordinals))
		case "rule":
			b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: ""}}, "", ordinals))
		default:
			b.WriteString(docxParagraph(block.Inlines, "", ordinals))
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: "Sources"}}, "Heading1", ordinals))
		for i, source := range doc.Manifest.Sources {
			text := fmt.Sprintf("[%d] %s", i+1, firstNonEmpty(source.Title, source.SourceEntityID))
			if source.URL != "" {
				text += " — " + source.URL
			}
			if source.SnapshotText != "" {
				text += " — " + source.SnapshotText
			}
			b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: text}}, "SourceAppendix", ordinals))
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func docxParagraph(inlines []publicationInline, style string, ordinals map[string]int) string {
	var b strings.Builder
	b.WriteString(`<w:p>`)
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		b.WriteString(xmlEscape(style))
		b.WriteString(`"/></w:pPr>`)
	}
	b.WriteString(docxRuns(inlines, ordinals))
	b.WriteString(`</w:p>`)
	return b.String()
}

func docxRuns(inlines []publicationInline, ordinals map[string]int) string {
	var b strings.Builder
	for _, inline := range inlines {
		if inline.Kind == "source_ref" {
			if inline.Text != "" {
				b.WriteString(docxRun(inline.Text, false, false, false))
			}
			b.WriteString(docxRun("["+publicationSourceMarker(ordinals, inline.SourceID)+"]", false, false, true))
			continue
		}
		b.WriteString(docxRun(inline.Text, inline.Kind == "strong", inline.Kind == "em", false))
	}
	return b.String()
}

func docxRun(text string, strong, em, superscript bool) string {
	var b strings.Builder
	b.WriteString(`<w:r>`)
	if strong || em || superscript {
		b.WriteString(`<w:rPr>`)
		if strong {
			b.WriteString(`<w:b/>`)
		}
		if em {
			b.WriteString(`<w:i/>`)
		}
		if superscript {
			b.WriteString(`<w:vertAlign w:val="superscript"/><w:color w:val="2F5597"/><w:sz w:val="16"/>`)
		}
		b.WriteString(`</w:rPr>`)
	}
	b.WriteString(`<w:t xml:space="preserve">`)
	b.WriteString(xmlEscape(text))
	b.WriteString(`</w:t></w:r>`)
	return b.String()
}

func docxTable(rows [][]publicationTableCell, ordinals map[string]int) string {
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
			b.WriteString(docxParagraph(inlines, style, ordinals))
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
	pages := renderPublicationPDFPages(doc)
	xmp := pdfMetadataXML(bundle, doc)
	pageCount := len(pages)
	if pageCount == 0 {
		pages = []string{pdfTextOp(50, 740, "F2", 18, title)}
		pageCount = 1
	}
	infoObjectNumber := 6 + pageCount*2
	kids := make([]string, 0, pageCount)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /Metadata 5 0 R >>",
		"",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
		fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len([]byte(xmp)), xmp),
	}
	for page := 0; page < pageCount; page++ {
		pageObjectNumber := 6 + page*2
		contentObjectNumber := pageObjectNumber + 1
		kids = append(kids, strconv.Itoa(pageObjectNumber)+" 0 R")
		streamText := pages[page]
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", contentObjectNumber),
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

type publicationPDFRenderer struct {
	pages   []string
	current strings.Builder
	y       float64
}

func renderPublicationPDFPages(doc PublicationDocument) []string {
	r := publicationPDFRenderer{y: 744}
	ordinals := publicationSourceOrdinals(doc)
	hasTitleHeading := false
	for _, block := range doc.Blocks {
		if block.Kind == "heading" && block.Level == 1 {
			hasTitleHeading = true
			break
		}
	}
	if !hasTitleHeading && strings.TrimSpace(doc.Title) != "" {
		r.writeWrappedText(doc.Title, 50, 512, "F2", 22, 26, 14)
	}
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			level := clampInt(block.Level, 1, 6)
			size := 22.0
			leading := 26.0
			before := 16.0
			if level == 2 {
				size, leading, before = 16, 20, 14
			} else if level >= 3 {
				size, leading, before = 13, 17, 12
			}
			r.writeWrappedText(publicationInlinesPDFText(block.Inlines, ordinals), 50, 512, "F2", size, leading, before)
		case "paragraph":
			r.writeWrappedText(publicationInlinesPDFText(block.Inlines, ordinals), 50, 512, "F1", 11, 15, 5)
		case "list_item":
			r.writeWrappedText("- "+publicationInlinesPDFText(block.Inlines, ordinals), 64, 498, "F1", 11, 15, 3)
		case "table":
			r.writeTable(block.Rows, ordinals)
		case "rule":
			r.ensureSpace(16)
			r.current.WriteString(fmt.Sprintf("0.75 G %.2f %.2f m %.2f %.2f l S 0 G\n", 50.0, r.y, 562.0, r.y))
			r.y -= 16
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		r.writeWrappedText("Sources", 50, 512, "F2", 16, 20, 18)
		for i, source := range doc.Manifest.Sources {
			text := fmt.Sprintf("[%d] %s", i+1, firstNonEmpty(source.Title, source.SourceEntityID))
			if source.URL != "" {
				text += " - " + source.URL
			}
			if source.SnapshotText != "" {
				text += " - " + source.SnapshotText
			}
			r.writeWrappedText(text, 50, 512, "F1", 9.5, 13, 2)
		}
	}
	r.finishPage()
	return r.pages
}

func (r *publicationPDFRenderer) finishPage() {
	if r.current.Len() == 0 {
		return
	}
	r.pages = append(r.pages, r.current.String())
	r.current.Reset()
	r.y = 744
}

func (r *publicationPDFRenderer) ensureSpace(height float64) {
	if r.y-height >= 52 {
		return
	}
	r.finishPage()
}

func (r *publicationPDFRenderer) writeWrappedText(text string, x, width float64, font string, size, leading, before float64) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	lines := wrapTextForPDF(text, int(width/(size*0.52)))
	height := before + float64(len(lines))*leading
	r.ensureSpace(height)
	r.y -= before
	for _, line := range lines {
		r.current.WriteString(pdfTextOp(x, r.y, font, size, line))
		r.y -= leading
	}
}

func (r *publicationPDFRenderer) writeTable(rows [][]publicationTableCell, ordinals map[string]int) {
	if len(rows) == 0 {
		return
	}
	r.ensureSpace(24)
	r.y -= 8
	tableX := 50.0
	tableWidth := 512.0
	cols := 1
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	cellWidth := tableWidth / float64(cols)
	for _, row := range rows {
		cellLines := make([][]string, cols)
		rowLines := 1
		for i := 0; i < cols; i++ {
			text := ""
			if i < len(row) {
				text = publicationInlinesPDFText(row[i].Inlines, ordinals)
			}
			cellLines[i] = wrapTextForPDF(text, int((cellWidth-12)/(9.5*0.52)))
			if len(cellLines[i]) > rowLines {
				rowLines = len(cellLines[i])
			}
		}
		rowHeight := 16 + float64(rowLines-1)*12
		r.ensureSpace(rowHeight + 4)
		yTop := r.y
		for i := 0; i < cols; i++ {
			x := tableX + float64(i)*cellWidth
			if len(row) > 0 && row[0].Header {
				r.current.WriteString(fmt.Sprintf("0.95 g %.2f %.2f %.2f %.2f re f 0 g 0 G\n", x, yTop-rowHeight, cellWidth, rowHeight))
			}
			r.current.WriteString(fmt.Sprintf("0.65 G %.2f %.2f %.2f %.2f re S 0 G\n", x, yTop-rowHeight, cellWidth, rowHeight))
			font := "F1"
			if i < len(row) && row[i].Header {
				font = "F2"
			}
			lineY := yTop - 12
			for _, line := range cellLines[i] {
				r.current.WriteString(pdfTextOp(x+6, lineY, font, 9.5, line))
				lineY -= 12
			}
		}
		r.y -= rowHeight
	}
	r.y -= 12
}

func publicationInlinesPDFText(inlines []publicationInline, ordinals map[string]int) string {
	var b strings.Builder
	for _, inline := range inlines {
		b.WriteString(inline.Text)
		if inline.Kind == "source_ref" {
			b.WriteString(" [")
			b.WriteString(publicationSourceMarker(ordinals, inline.SourceID))
			b.WriteString("]")
		}
	}
	return b.String()
}

func wrapTextForPDF(text string, width int) []string {
	if width < 12 {
		width = 12
	}
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

func pdfTextOp(x, y float64, font string, size float64, text string) string {
	return fmt.Sprintf("0 g BT /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", font, size, x, y, pdfEscape(pdfWinAnsiText(text)))
}

func pdfWinAnsiText(text string) string {
	replacer := strings.NewReplacer(
		"\u2013", "-",
		"\u2014", "-",
		"\u2018", "'",
		"\u2019", "'",
		"\u201c", `"`,
		"\u201d", `"`,
		"\u2022", "-",
		"\u00a0", " ",
	)
	text = replacer.Replace(text)
	var b strings.Builder
	for _, r := range text {
		if r == '\n' || r == '\t' || (r >= 32 && r <= 126) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('?')
	}
	return b.String()
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
	profile := defaultPublicationExportProfile()
	ordinals := publicationSourceOrdinals(doc)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(doc.Title))
	b.WriteString(`</title><meta name="generator" content="Choir">`)
	b.WriteString(`<meta name="choir-export-profile" content="`)
	b.WriteString(html.EscapeString(profile.ID))
	b.WriteString(`"><style>`)
	b.WriteString(publicationHTMLProfileCSS(profile))
	b.WriteString(`</style>`)
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
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">\n")
		case "paragraph":
			b.WriteString("<p>")
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</p>\n")
		case "list_item":
			b.WriteString("<ul><li>")
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</li></ul>\n")
		case "table":
			b.WriteString(`<table class="vtext-table"><tbody>`)
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
					b.WriteString(renderHTMLInlines(cell.Inlines, ordinals))
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

func renderHTMLInlines(inlines []publicationInline, ordinals map[string]int) string {
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
			b.WriteString(`<sup>[`)
			b.WriteString(html.EscapeString(publicationSourceMarker(ordinals, inline.SourceID)))
			b.WriteString(`]</sup></a>`)
		default:
			b.WriteString(html.EscapeString(inline.Text))
		}
	}
	return b.String()
}

func publicationHTMLProfileCSS(profile publicationExportProfile) string {
	return `:root{color-scheme:light;--choir-text:#171717;--choir-muted:#5f6673;--choir-rule:#d6dbe3;--choir-accent:#1d4f91;--choir-source-bg:#f7f9fc}` +
		`html{background:#f4f5f7}` +
		`body{margin:0;color:var(--choir-text);font:16px/1.62 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}` +
		`.vtext-publication{box-sizing:border-box;max-width:820px;margin:42px auto;padding:56px 64px;background:#fff;box-shadow:0 1px 8px rgba(15,23,42,.08)}` +
		`h1{font-size:30px;line-height:1.18;margin:0 0 24px;font-weight:750;letter-spacing:0}` +
		`h2{font-size:22px;line-height:1.28;margin:34px 0 14px;font-weight:720;letter-spacing:0}` +
		`h3{font-size:18px;line-height:1.35;margin:28px 0 10px;font-weight:700;letter-spacing:0}` +
		`h4,h5,h6{font-size:16px;line-height:1.4;margin:24px 0 8px;font-weight:700;letter-spacing:0}` +
		`p{margin:0 0 16px}` +
		`ul{margin:0 0 16px 1.25rem;padding:0}` +
		`li{margin:0 0 8px}` +
		`.vtext-table{border-collapse:collapse;width:100%;margin:22px 0 28px;font-size:14px;line-height:1.45}` +
		`.vtext-table th,.vtext-table td{border:1px solid var(--choir-rule);padding:9px 11px;vertical-align:top;text-align:left}` +
		`.vtext-table th{background:#f1f4f8;font-weight:700}` +
		`.vtext-source-ref{color:var(--choir-accent);text-decoration:none;border-bottom:1px solid rgba(29,79,145,.28)}` +
		`.vtext-source-ref sup{font-size:.72em;line-height:0;margin-left:2px}` +
		`.vtext-sources{margin-top:44px;padding-top:22px;border-top:1px solid var(--choir-rule);font-size:14px;color:#2f3744}` +
		`.vtext-sources h2{font-size:18px;margin:0 0 14px}` +
		`.vtext-sources ol{padding-left:1.35rem}` +
		`.vtext-sources li{margin-bottom:14px}` +
		`.vtext-sources blockquote{margin:8px 0 0;padding:10px 12px;background:var(--choir-source-bg);border-left:3px solid var(--choir-rule);color:var(--choir-muted)}` +
		`@media(max-width:760px){.vtext-publication{margin:0;padding:32px 22px;box-shadow:none}h1{font-size:26px}}`
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
