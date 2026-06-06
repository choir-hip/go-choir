package platform

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
