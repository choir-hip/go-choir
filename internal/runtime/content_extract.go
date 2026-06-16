package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const contentExtractionAdapterVersion = 1

type contentSelector struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label,omitempty"`
	Text      string `json:"text,omitempty"`
	StartChar int    `json:"start_char,omitempty"`
	EndChar   int    `json:"end_char,omitempty"`
}

type contentExtraction struct {
	MediaType string
	AppHint   string
	Title     string
	Text      string
	Adapter   string
	Warnings  []string
	Selectors []contentSelector
	Metadata  map[string]any
}

func extractContentDocument(ctx context.Context, sourceName, mediaType string, raw []byte) contentExtraction {
	mediaType = normalizeMediaType(mediaType)
	if mediaType == "" {
		mediaType = detectMediaType("", sourceName, "")
	}
	extraction := contentExtraction{
		MediaType: mediaType,
		AppHint:   appHintForMedia(mediaType, "", sourceName),
		Adapter:   "document_unhandled",
		Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
	}
	switch mediaType {
	case "application/pdf":
		extraction = extractPDFDocument(ctx, sourceName, raw)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		extraction = extractDOCXDocument(ctx, sourceName, raw)
	case "application/epub+zip":
		extraction = extractEPUBDocument(ctx, sourceName, raw)
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		extraction = extractPPTXDocument(ctx, sourceName, raw)
	case "text/html", "application/xhtml+xml":
		title, text := extractReadableHTML(raw)
		extraction = contentExtraction{
			MediaType: mediaType,
			AppHint:   "browser",
			Title:     title,
			Text:      text,
			Adapter:   "html_readability_lite",
			Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
		}
	default:
		if isTextMedia(mediaType) {
			text := strings.TrimSpace(string(raw))
			extraction = contentExtraction{
				MediaType: mediaType,
				AppHint:   appHintForMedia(mediaType, "", sourceName),
				Text:      text,
				Adapter:   "plain_text_decode",
				Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
			}
		}
	}
	extraction.MediaType = firstNonEmptyString(extraction.MediaType, mediaType)
	extraction.AppHint = normalizeAppHint(firstNonEmptyString(extraction.AppHint, appHintForMedia(extraction.MediaType, "", sourceName)))
	extraction.Text = strings.TrimSpace(extraction.Text)
	if len(extraction.Text) > maxStoredExtractedText {
		extraction.Text = extraction.Text[:maxStoredExtractedText]
		extraction.Warnings = appendIfMissing(extraction.Warnings, "extracted text truncated at 300KiB")
	}
	if len(extraction.Selectors) == 0 && extraction.Text != "" {
		extraction.Selectors = chunkContentSelectors(extraction.Text, "chunk", 12000)
	}
	if extraction.Metadata == nil {
		extraction.Metadata = map[string]any{}
	}
	extraction.Metadata["extraction_adapter"] = extraction.Adapter
	extraction.Metadata["extraction_adapter_version"] = contentExtractionAdapterVersion
	extraction.Metadata["extraction_warnings"] = extraction.Warnings
	extraction.Metadata["selectors"] = extraction.Selectors
	extraction.Metadata["selector_count"] = len(extraction.Selectors)
	if extraction.Text != "" {
		extraction.Metadata["text_chars"] = len(extraction.Text)
	}
	return extraction
}

func extractPDFDocument(ctx context.Context, sourceName string, raw []byte) contentExtraction {
	extraction := contentExtraction{
		MediaType: "application/pdf",
		AppHint:   "pdf",
		Adapter:   "pdf_poppler_pdftotext",
		Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
	}
	text, warnings := runDocumentToolOnTempFile(ctx, "pdftotext", []string{"-layout"}, ".pdf", raw, []string{"-"})
	extraction.Warnings = append(extraction.Warnings, warnings...)
	if strings.TrimSpace(text) != "" {
		extraction.Text = strings.TrimSpace(text)
		extraction.Selectors = pdfPageSelectors(extraction.Text)
	}
	if info, infoWarnings := runDocumentToolOnTempFile(ctx, "pdfinfo", nil, ".pdf", raw, nil); strings.TrimSpace(info) != "" {
		extraction.Metadata["pdfinfo"] = parsePDFInfo(info)
	} else {
		extraction.Warnings = append(extraction.Warnings, infoWarnings...)
	}
	if strings.TrimSpace(extraction.Text) == "" {
		if fallback := strings.TrimSpace(extractPDFLiteralText(raw)); fallback != "" {
			extraction.Adapter = "pdf_literal_text_projection_fallback"
			extraction.Text = fallback
			extraction.Selectors = pdfPageSelectors(extraction.Text)
			extraction.Warnings = appendIfMissing(extraction.Warnings, "pdf_literal_fallback_used")
		} else {
			extraction.Adapter = "pdf_extraction_unavailable"
			extraction.Warnings = appendIfMissing(extraction.Warnings, "pdf_text_extraction_empty")
		}
	}
	return extraction
}

func extractDOCXDocument(ctx context.Context, sourceName string, raw []byte) contentExtraction {
	extraction := contentExtraction{
		MediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		AppHint:   AgentProfileTexture,
		Adapter:   "docx_pandoc_markdown",
		Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
	}
	text, warnings := runDocumentToolOnTempFile(ctx, "pandoc", []string{"-f", "docx", "-t", "markdown"}, ".docx", raw, nil)
	extraction.Warnings = append(extraction.Warnings, warnings...)
	if strings.TrimSpace(text) == "" {
		extraction.Adapter = "docx_ooxml_text_table_projection"
		extraction.Warnings = appendIfMissing(extraction.Warnings, "docx_pandoc_unavailable_or_empty")
		text = extractDOCXTextFromOOXML(raw)
	}
	extraction.Text = strings.TrimSpace(text)
	extraction.Selectors = markdownStructureSelectors(extraction.Text)
	if strings.TrimSpace(extraction.Text) == "" {
		extraction.Warnings = appendIfMissing(extraction.Warnings, "docx_text_extraction_empty")
	}
	return extraction
}

func extractEPUBDocument(ctx context.Context, sourceName string, raw []byte) contentExtraction {
	extraction := contentExtraction{
		MediaType: "application/epub+zip",
		AppHint:   "epub",
		Adapter:   "epub_pandoc_markdown",
		Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
	}
	text, warnings := runDocumentToolOnTempFile(ctx, "pandoc", []string{"-f", "epub", "-t", "markdown"}, ".epub", raw, nil)
	extraction.Warnings = append(extraction.Warnings, warnings...)
	if strings.TrimSpace(text) == "" {
		extraction.Adapter = "epub_xhtml_text_projection"
		extraction.Warnings = appendIfMissing(extraction.Warnings, "epub_pandoc_unavailable_or_empty")
		text = extractEPUBTextFromZip(raw)
	}
	extraction.Text = strings.TrimSpace(text)
	extraction.Selectors = markdownStructureSelectors(extraction.Text)
	if strings.TrimSpace(extraction.Text) == "" {
		extraction.Warnings = appendIfMissing(extraction.Warnings, "epub_text_extraction_empty")
	}
	return extraction
}

func extractPPTXDocument(ctx context.Context, sourceName string, raw []byte) contentExtraction {
	_ = ctx
	extraction := contentExtraction{
		MediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		AppHint:   "slides",
		Adapter:   "pptx_ooxml_slide_text_projection",
		Metadata:  map[string]any{"source_name": strings.TrimSpace(sourceName)},
	}
	slides, notes, warnings := extractPPTXSlidesFromOOXML(raw)
	extraction.Warnings = append(extraction.Warnings, warnings...)
	var parts []string
	for i, slide := range slides {
		label := fmt.Sprintf("Slide %d", i+1)
		body := strings.TrimSpace(slide)
		if body == "" {
			continue
		}
		parts = append(parts, label+"\n\n"+body)
		extraction.Selectors = append(extraction.Selectors, contentSelector{
			ID:    fmt.Sprintf("slide-%d", i+1),
			Kind:  "slide",
			Label: label,
			Text:  body,
		})
		if extraction.Title == "" {
			extraction.Title = firstLine(body)
		}
	}
	for i, note := range notes {
		if strings.TrimSpace(note) == "" {
			continue
		}
		extraction.Selectors = append(extraction.Selectors, contentSelector{
			ID:    fmt.Sprintf("notes-%d", i+1),
			Kind:  "speaker_notes",
			Label: fmt.Sprintf("Speaker notes %d", i+1),
			Text:  strings.TrimSpace(note),
		})
	}
	extraction.Text = strings.TrimSpace(strings.Join(parts, "\n\n---\n\n"))
	extraction.Metadata["slide_count"] = len(slides)
	extraction.Metadata["notes_count"] = len(notes)
	if strings.TrimSpace(extraction.Text) == "" {
		extraction.Warnings = appendIfMissing(extraction.Warnings, "pptx_text_extraction_empty")
	}
	return extraction
}

func runDocumentToolOnTempFile(ctx context.Context, tool string, prefixArgs []string, ext string, raw []byte, suffixArgs []string) (string, []string) {
	if _, err := exec.LookPath(tool); err != nil {
		return "", []string{tool + "_unavailable"}
	}
	tmp, err := os.CreateTemp("", "choir-document-*"+ext)
	if err != nil {
		return "", []string{"tempfile_create_failed: " + err.Error()}
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return "", []string{"tempfile_write_failed: " + err.Error()}
	}
	if err := tmp.Close(); err != nil {
		return "", []string{"tempfile_close_failed: " + err.Error()}
	}
	toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := append([]string{}, prefixArgs...)
	args = append(args, tmpPath)
	args = append(args, suffixArgs...)
	cmd := exec.CommandContext(toolCtx, tool, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		warning := tool + "_failed"
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			warning += ": " + truncateString(msg, 500)
		} else {
			warning += ": " + err.Error()
		}
		return "", []string{warning}
	}
	return string(out), nil
}

func extractDOCXTextFromOOXML(raw []byte) string {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return ""
	}
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return ""
		}
		documentXML, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return ""
		}
		break
	}
	if len(documentXML) == 0 {
		return ""
	}
	return strings.TrimSpace(docxDocumentXMLToMarkdown(documentXML))
}

func extractEPUBTextFromZip(raw []byte) string {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return ""
	}
	var names []string
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		lower := strings.ToLower(file.Name)
		if strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".htm") {
			names = append(names, file.Name)
			files[file.Name] = file
		}
	}
	sort.Strings(names)
	var parts []string
	for _, name := range names {
		rc, err := files[name].Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxImportedContentBytes))
		_ = rc.Close()
		if err != nil {
			continue
		}
		_, text := extractReadableHTML(data)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractPPTXSlidesFromOOXML(raw []byte) ([]string, []string, []string) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, []string{"pptx_zip_open_failed"}
	}
	type part struct {
		index int
		text  string
	}
	var slides []part
	var notes []part
	for _, file := range reader.File {
		name := strings.ToLower(file.Name)
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		isSlide := strings.HasPrefix(name, "ppt/slides/slide")
		isNotes := strings.HasPrefix(name, "ppt/notesSlides/notesslide")
		if !isSlide && !isNotes {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		text := strings.TrimSpace(ooxmlTextRunsToText(data))
		idx := trailingNumber(name)
		if isNotes {
			notes = append(notes, part{index: idx, text: text})
		} else {
			slides = append(slides, part{index: idx, text: text})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].index < slides[j].index })
	sort.Slice(notes, func(i, j int) bool { return notes[i].index < notes[j].index })
	slideText := make([]string, len(slides))
	for i, slide := range slides {
		slideText[i] = slide.text
	}
	noteText := make([]string, len(notes))
	for i, note := range notes {
		noteText[i] = note.text
	}
	return slideText, noteText, nil
}

func ooxmlTextRunsToText(data []byte) string {
	text := string(data)
	textRE := regexp.MustCompile(`(?is)<a:t(?:\s+[^>]*)?>(.*?)</a:t>`)
	var parts []string
	for _, match := range textRE.FindAllStringSubmatch(text, -1) {
		if s := strings.TrimSpace(htmlEntityText(match[1])); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		wTextRE := regexp.MustCompile(`(?is)<w:t(?:\s+[^>]*)?>(.*?)</w:t>`)
		for _, match := range wTextRE.FindAllStringSubmatch(text, -1) {
			if s := strings.TrimSpace(htmlEntityText(match[1])); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func trailingNumber(value string) int {
	re := regexp.MustCompile(`(\d+)\D*$`)
	match := re.FindStringSubmatch(value)
	if len(match) != 2 {
		return 0
	}
	n, _ := strconv.Atoi(match[1])
	return n
}

func pdfPageSelectors(text string) []contentSelector {
	if strings.Contains(text, "\f") {
		pages := strings.Split(text, "\f")
		selectors := make([]contentSelector, 0, len(pages))
		offset := 0
		for i, page := range pages {
			page = strings.TrimSpace(page)
			if page == "" {
				offset += len(pages[i]) + 1
				continue
			}
			selectors = append(selectors, contentSelector{
				ID:        fmt.Sprintf("page-%d", i+1),
				Kind:      "page",
				Label:     fmt.Sprintf("Page %d", i+1),
				Text:      page,
				StartChar: offset,
				EndChar:   offset + len(pages[i]),
			})
			offset += len(pages[i]) + 1
		}
		return selectors
	}
	return chunkContentSelectors(text, "page", 12000)
}

func markdownStructureSelectors(text string) []contentSelector {
	var selectors []contentSelector
	headingRE := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := headingRE.FindAllStringSubmatchIndex(text, -1)
	for i, match := range matches {
		start := match[0]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		label := strings.TrimSpace(text[match[4]:match[5]])
		selectors = append(selectors, contentSelector{
			ID:        fmt.Sprintf("section-%d", i+1),
			Kind:      "section",
			Label:     label,
			Text:      strings.TrimSpace(text[start:end]),
			StartChar: start,
			EndChar:   end,
		})
	}
	if len(selectors) == 0 {
		return chunkContentSelectors(text, "chunk", 12000)
	}
	return selectors
}

func chunkContentSelectors(text, kind string, chunkSize int) []contentSelector {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 12000
	}
	var selectors []contentSelector
	for start, idx := 0, 1; start < len(text); idx++ {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		selectors = append(selectors, contentSelector{
			ID:        fmt.Sprintf("%s-%d", kind, idx),
			Kind:      kind,
			Label:     fmt.Sprintf("%s %d", strings.Title(kind), idx),
			Text:      strings.TrimSpace(text[start:end]),
			StartChar: start,
			EndChar:   end,
		})
		start = end
	}
	return selectors
}

func parsePDFInfo(raw string) map[string]any {
	out := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(key, " ", "_")))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if n, err := strconv.Atoi(value); err == nil {
			out[key] = n
		} else {
			out[key] = value
		}
	}
	return out
}

func selectorsFromContentMetadata(metadata json.RawMessage) []contentSelector {
	var envelope map[string]any
	if len(metadata) == 0 || json.Unmarshal(metadata, &envelope) != nil {
		return nil
	}
	raw, ok := envelope["selectors"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var selectors []contentSelector
	if err := json.Unmarshal(data, &selectors); err != nil {
		return nil
	}
	return selectors
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}

func decodeHTMLEntities(value string) string {
	return html.UnescapeString(value)
}

func safeBaseName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "document"
	}
	return filepath.Base(strings.TrimSpace(value))
}
