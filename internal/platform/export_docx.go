package platform

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func buildPublicationDOCX(bundle *PublicationBundle, doc PublicationDocument, metadata json.RawMessage, profile publicationExportProfile) ([]byte, error) {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	rels := docxDocumentRelationships(doc)
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
			"ChoirCitationPlacement":    profile.CitationPlacement,
			"ChoirSourceDetailLevel":    profile.SourceDetailLevel,
			"ChoirMetadataPolicy":       publicationMetadataPolicyString(profile.MetadataPolicy),
			"ChoirExportMetadata":       string(metadata),
			"ChoirSourceManifestSchema": doc.Manifest.Schema,
		}),
		"customXml/item1.xml": docxSourceManifestXML(manifestJSON),
		"customXml/_rels/item1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml":            docxDocumentXML(doc, rels),
		"word/styles.xml":              docxStylesXML(profile),
		"word/_rels/document.xml.rels": docxDocumentRelsXML(rels),
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

type docxRelationshipIndex struct {
	sourceURLs map[string]string
	linkURLs   map[string]string
	idsByURL   map[string]string
}

func docxDocumentRelationships(doc PublicationDocument) docxRelationshipIndex {
	rels := docxRelationshipIndex{
		sourceURLs: map[string]string{},
		linkURLs:   map[string]string{},
		idsByURL:   map[string]string{"styles.xml": "rIdStyles"},
	}
	next := 2
	addURL := func(url string) string {
		url = strings.TrimSpace(url)
		if !docxAllowedExternalURL(url) {
			return ""
		}
		if id, ok := rels.idsByURL[url]; ok {
			return id
		}
		id := "rId" + strconv.Itoa(next)
		next++
		rels.idsByURL[url] = id
		return id
	}
	for _, source := range doc.Manifest.Sources {
		if id := addURL(source.URL); id != "" {
			rels.sourceURLs[source.SourceEntityID] = id
		}
	}
	var walkInlines func([]publicationInline)
	walkInlines = func(inlines []publicationInline) {
		for _, inline := range inlines {
			if inline.Kind == "link" {
				if id := addURL(inline.Href); id != "" {
					rels.linkURLs[inline.Href] = id
				}
			}
		}
	}
	for _, block := range doc.Blocks {
		walkInlines(block.Inlines)
		for _, row := range block.Rows {
			for _, cell := range row {
				walkInlines(cell.Inlines)
			}
		}
	}
	return rels
}

func docxAllowedExternalURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func docxDocumentRelsXML(rels docxRelationshipIndex) string {
	type relationship struct {
		id     string
		target string
		mode   string
		typ    string
	}
	items := []relationship{{
		id:     "rIdStyles",
		target: "styles.xml",
		typ:    "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles",
	}}
	type relURL struct {
		url string
		id  string
	}
	urls := make([]relURL, 0, len(rels.idsByURL))
	for url, id := range rels.idsByURL {
		if url == "styles.xml" {
			continue
		}
		urls = append(urls, relURL{url: url, id: id})
	}
	sort.Slice(urls, func(i, j int) bool {
		return urls[i].id < urls[j].id
	})
	for _, entry := range urls {
		items = append(items, relationship{
			id:     entry.id,
			target: entry.url,
			mode:   "External",
			typ:    "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
		})
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, item := range items {
		b.WriteString(`<Relationship Id="`)
		b.WriteString(xmlEscape(item.id))
		b.WriteString(`" Type="`)
		b.WriteString(xmlEscape(item.typ))
		b.WriteString(`" Target="`)
		b.WriteString(xmlEscape(item.target))
		b.WriteString(`"`)
		if item.mode != "" {
			b.WriteString(` TargetMode="`)
			b.WriteString(xmlEscape(item.mode))
			b.WriteString(`"`)
		}
		b.WriteString(`/>`)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
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
	title := xmlEscape(firstNonEmpty(bundle.Publication.Title, defaultPublishedTextureTitle))
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

func docxStylesXML(profile publicationExportProfile) string {
	bodyHalfPoints := strconv.Itoa(clampInt(profile.Typography.BodySizePX*2, 18, 32))
	titleHalfPoints := strconv.Itoa(clampInt(firstNonZero(profile.Headings.TitleSizePX, 30)*2, 44, 84))
	h1HalfPoints := strconv.Itoa(clampInt(firstNonZero(profile.Headings.H1SizePX, 30)*2, 40, 76))
	h2HalfPoints := strconv.Itoa(clampInt(firstNonZero(profile.Headings.H2SizePX, 22)*2, 32, 64))
	h3HalfPoints := strconv.Itoa(clampInt(firstNonZero(profile.Headings.H3SizePX, 18)*2, 28, 52))
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:spacing w:after="180" w:line="300" w:lineRule="auto"/></w:pPr><w:rPr><w:rFonts w:ascii="Aptos" w:hAnsi="Aptos"/><w:sz w:val="` + bodyHalfPoints + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:pPr><w:spacing w:before="0" w:after="280"/></w:pPr><w:rPr><w:b/><w:sz w:val="` + titleHalfPoints + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="360" w:after="160"/></w:pPr><w:rPr><w:b/><w:sz w:val="` + h1HalfPoints + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="280" w:after="120"/></w:pPr><w:rPr><w:b/><w:sz w:val="` + h2HalfPoints + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="220" w:after="100"/></w:pPr><w:rPr><w:b/><w:sz w:val="` + h3HalfPoints + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="360" w:hanging="180"/><w:spacing w:after="120"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="SourceAppendix"><w:name w:val="Source Appendix"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="120"/></w:pPr><w:rPr><w:sz w:val="20"/></w:rPr></w:style>` +
		`</w:styles>`
}

func docxDocumentXML(doc PublicationDocument, rels docxRelationshipIndex) string {
	var b strings.Builder
	ordinals := publicationSourceOrdinals(doc)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>`)
	wroteTitle := false
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			style := "Heading" + strconv.Itoa(clampInt(block.Level, 1, 6))
			if block.Level == 1 && !wroteTitle {
				style = "Title"
				wroteTitle = true
			}
			b.WriteString(docxParagraph(block.Inlines, style, ordinals, rels))
		case "list_item":
			b.WriteString(docxParagraph(append([]publicationInline{{Kind: "text", Text: "- "}}, block.Inlines...), "ListParagraph", ordinals, rels))
		case "table":
			b.WriteString(docxTable(block.Rows, ordinals, rels))
		case "rule":
			b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: ""}}, "", ordinals, rels))
		default:
			b.WriteString(docxParagraph(block.Inlines, "", ordinals, rels))
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString(docxParagraph([]publicationInline{{Kind: "text", Text: "Sources"}}, "Heading1", ordinals, rels))
		for i, source := range doc.Manifest.Sources {
			sourceTitle := firstNonEmpty(source.Title, fmt.Sprintf("Source %d", i+1))
			sourceInlines := []publicationInline{{Kind: "text", Text: fmt.Sprintf("[%d] ", i+1)}}
			if relID := rels.sourceURLs[source.SourceEntityID]; relID != "" {
				sourceInlines = append(sourceInlines, publicationInline{Kind: "docx_hyperlink", Text: sourceTitle, Href: relID})
			} else {
				sourceInlines = append(sourceInlines, publicationInline{Kind: "text", Text: sourceTitle})
			}
			details := []string{}
			if evidence := publicationEvidenceStateLabel(source.EvidenceState); evidence != "" {
				details = append(details, evidence)
			}
			if source.ReaderArtifactState != "" {
				details = append(details, source.ReaderArtifactState)
			}
			if source.OpenSurface != "" {
				details = append(details, "opens in "+source.OpenSurface)
			}
			if len(details) > 0 {
				sourceInlines = append(sourceInlines, publicationInline{Kind: "text", Text: " (" + strings.Join(details, "; ") + ")"})
			}
			if source.SnapshotText != "" {
				sourceInlines = append(sourceInlines, publicationInline{Kind: "text", Text: " - " + source.SnapshotText})
			}
			b.WriteString(docxParagraph(sourceInlines, "SourceAppendix", ordinals, rels))
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func docxParagraph(inlines []publicationInline, style string, ordinals map[string]int, rels docxRelationshipIndex) string {
	var b strings.Builder
	b.WriteString(`<w:p>`)
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		b.WriteString(xmlEscape(style))
		b.WriteString(`"/></w:pPr>`)
	}
	b.WriteString(docxRuns(inlines, ordinals, rels))
	b.WriteString(`</w:p>`)
	return b.String()
}

func docxRuns(inlines []publicationInline, ordinals map[string]int, rels docxRelationshipIndex) string {
	var b strings.Builder
	for _, inline := range inlines {
		if inline.Kind == "docx_hyperlink" {
			b.WriteString(docxHyperlinkRun(inline.Href, inline.Text))
			continue
		}
		if inline.Kind == "link" {
			if relID := rels.linkURLs[inline.Href]; relID != "" {
				b.WriteString(docxHyperlinkRun(relID, inline.Text))
				continue
			}
		}
		if inline.Kind == "source_ref" {
			if relID := rels.sourceURLs[inline.SourceID]; relID != "" && inline.Text != "" {
				b.WriteString(docxHyperlinkRun(relID, inline.Text))
			} else if inline.Text != "" {
				b.WriteString(docxRun(inline.Text, false, false, false))
			}
			b.WriteString(docxRun("["+publicationSourceMarker(ordinals, inline.SourceID)+"]", false, false, true))
			continue
		}
		b.WriteString(docxRun(inline.Text, inline.Kind == "strong", inline.Kind == "em", false))
	}
	return b.String()
}

func docxHyperlinkRun(relID, text string) string {
	if relID == "" {
		return docxRun(text, false, false, false)
	}
	return `<w:hyperlink r:id="` + xmlEscape(relID) + `" w:history="1">` +
		`<w:r><w:rPr><w:color w:val="2F5597"/><w:u w:val="single"/></w:rPr><w:t xml:space="preserve">` +
		xmlEscape(text) +
		`</w:t></w:r></w:hyperlink>`
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

func docxTable(rows [][]publicationTableCell, ordinals map[string]int, rels docxRelationshipIndex) string {
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
			b.WriteString(docxParagraph(inlines, style, ordinals, rels))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

func publicationEvidenceStateLabel(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var evidence struct {
		State    string `json:"state"`
		Relation string `json:"relation"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return ""
	}
	return firstNonEmpty(evidence.State, evidence.Relation)
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
