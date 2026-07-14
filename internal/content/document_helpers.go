package content

import (
	"regexp"
	"strings"
)

// DOCXDocumentXMLToMarkdown projects WordprocessingML paragraphs and tables to Markdown.
func DOCXDocumentXMLToMarkdown(data []byte) string {
	text := string(data)
	tableRE := regexp.MustCompile(`(?is)<w:tbl\b.*?</w:tbl>`)
	paragraphRE := regexp.MustCompile(`(?is)<w:p\b.*?</w:p>`)
	var out strings.Builder
	last := 0
	for _, loc := range tableRE.FindAllStringIndex(text, -1) {
		for _, paragraph := range paragraphRE.FindAllString(text[last:loc[0]], -1) {
			if paragraphText := strings.TrimSpace(docxParagraphText(paragraph)); paragraphText != "" {
				out.WriteString(paragraphText)
				out.WriteString("\n\n")
			}
		}
		rows := docxTableRows(text[loc[0]:loc[1]])
		if len(rows) > 0 {
			out.WriteString(markdownTable(rows))
			out.WriteString("\n\n")
		}
		last = loc[1]
	}
	for _, paragraph := range paragraphRE.FindAllString(text[last:], -1) {
		if paragraphText := strings.TrimSpace(docxParagraphText(paragraph)); paragraphText != "" {
			out.WriteString(paragraphText)
			out.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(out.String())
}

func docxParagraphText(xmlFragment string) string {
	textRE := regexp.MustCompile(`(?is)<w:t(?:\s+[^>]*)?>(.*?)</w:t>`)
	var parts []string
	for _, match := range textRE.FindAllStringSubmatch(xmlFragment, -1) {
		parts = append(parts, htmlEntityText(match[1]))
	}
	return strings.Join(parts, "")
}

func docxTableRows(tableXML string) [][]string {
	rowRE := regexp.MustCompile(`(?is)<w:tr\b.*?</w:tr>`)
	cellRE := regexp.MustCompile(`(?is)<w:tc\b.*?</w:tc>`)
	var rows [][]string
	for _, rowXML := range rowRE.FindAllString(tableXML, -1) {
		var row []string
		for _, cellXML := range cellRE.FindAllString(rowXML, -1) {
			row = append(row, strings.TrimSpace(docxParagraphText(cellXML)))
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func markdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return ""
	}
	normalize := func(row []string) []string {
		out := make([]string, cols)
		for i := range cols {
			if i < len(row) {
				out[i] = strings.ReplaceAll(row[i], "|", `\|`)
			}
		}
		return out
	}
	var b strings.Builder
	b.WriteString("| ")
	b.WriteString(strings.Join(normalize(rows[0]), " | "))
	b.WriteString(" |\n| ")
	separators := make([]string, cols)
	for i := range separators {
		separators[i] = "---"
	}
	b.WriteString(strings.Join(separators, " | "))
	b.WriteString(" |")
	for _, row := range rows[1:] {
		b.WriteString("\n| ")
		b.WriteString(strings.Join(normalize(row), " | "))
		b.WriteString(" |")
	}
	return b.String()
}

func htmlEntityText(text string) string {
	replacements := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
	)
	return replacements.Replace(text)
}

// ExtractPDFLiteralText provides the existing best-effort fallback for PDF string operators.
func ExtractPDFLiteralText(data []byte) string {
	raw := string(data)
	literalRE := regexp.MustCompile(`\((?:\\.|[^\\()])+\)\s*Tj`)
	arrayRE := regexp.MustCompile(`\[(?s:.*?)\]\s*TJ`)
	stringRE := regexp.MustCompile(`\((?:\\.|[^\\()])+\)`)
	var parts []string
	for _, match := range literalRE.FindAllString(raw, -1) {
		if loc := stringRE.FindStringIndex(match); loc != nil {
			parts = append(parts, decodePDFLiteralString(match[loc[0]:loc[1]]))
		}
	}
	for _, array := range arrayRE.FindAllString(raw, -1) {
		for _, literal := range stringRE.FindAllString(array, -1) {
			parts = append(parts, decodePDFLiteralString(literal))
		}
	}
	return strings.Join(parts, "\n")
}

func decodePDFLiteralString(literal string) string {
	literal = strings.TrimPrefix(strings.TrimSuffix(literal, ")"), "(")
	var b strings.Builder
	escaped := false
	for _, r := range literal {
		if escaped {
			switch r {
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'b', 'f':
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
