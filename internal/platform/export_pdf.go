package platform

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func buildPublicationPDF(bundle *PublicationBundle, doc PublicationDocument, profile publicationExportProfile) ([]byte, error) {
	title := doc.Title
	pages := renderPublicationPDFPages(doc)
	xmp := pdfMetadataXML(bundle, doc, profile)
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

func pdfMetadataXML(bundle *PublicationBundle, doc PublicationDocument, profile publicationExportProfile) string {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	return `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>` +
		`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description rdf:about="" xmlns:choir="https://choir.news/ns/publication-export/1.0/" choir:publicationId="` + xmlEscape(bundle.Publication.ID) + `" choir:publicationVersionId="` + xmlEscape(bundle.Version.ID) + `" choir:routePath="` + xmlEscape(bundle.Route.Path) + `" choir:contentHash="` + xmlEscape(bundle.Version.ContentHash) + `">` +
		`<choir:exportSchema>choir.publication_export.v0</choir:exportSchema>` +
		`<choir:exportProfile>` + xmlEscape(publicationExportProfileJSON(profile)) + `</choir:exportProfile>` +
		`<choir:sourceManifest>` + xmlEscape(manifestJSON) + `</choir:sourceManifest>` +
		`</rdf:Description></rdf:RDF></x:xmpmeta>` +
		`<?xpacket end="w"?>`
}
