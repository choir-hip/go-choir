package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/sandbox"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureFileImportProjection struct {
	SourcePath               string
	MediaType                string
	ProjectionContent        string
	OriginalBytes            []byte
	OriginalContentHash      string
	OriginalContentHashState string
	ProjectionContentHash    string
	ImportAdapter            string
	ImportAdapterVersion     int
	LossinessScore           int
	Warnings                 []string
	ExtractionMetadata       map[string]any
}
type textureShortcutFile struct {
	Kind       string `json:"kind"`
	DocID      string `json:"doc_id"`
	Title      string `json:"title"`
	SourcePath string `json:"source_path"`
}

const (
	textureShortcutExt      = ".texture"
	textureShortcutFileKind = "texture"
	defaultTextureTitleStem = "Untitled Texture"
	defaultTextureFileStem  = "texture"
)

func normalizeTextureSourcePath(raw string) string {
	cleaned := pathpkg.Clean("/" + strings.TrimSpace(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func canonicalTextureImportTitle(sourcePath, requestedTitle string) string {
	base := strings.TrimSpace(requestedTitle)
	if base == "" {
		base = pathpkg.Base(strings.TrimSpace(sourcePath))
	}
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = defaultTextureTitleStem
	}
	base = pathpkg.Base(base)
	ext := pathpkg.Ext(base)
	if isTextureShortcutPath(base) {
		return base
	}
	stem := strings.TrimSpace(strings.TrimSuffix(base, ext))
	if stem == "" {
		stem = strings.TrimSpace(base)
	}
	if stem == "" {
		stem = defaultTextureTitleStem
	}
	return stem + textureShortcutExt
}
func slugifyTextureManifestStem(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return defaultTextureFileStem
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	stem := strings.Trim(b.String(), "-")
	if stem == "" {
		return defaultTextureFileStem
	}
	return stem
}

func shortDocIDSuffix(docID string) string {
	base := strings.TrimSpace(docID)
	if idx := strings.IndexByte(base, '-'); idx > 0 {
		base = base[:idx]
	}
	if len(base) > 8 {
		base = base[:8]
	}
	if base == "" {
		return "doc"
	}
	return base
}

func isTextureShortcutPath(sourcePath string) bool {
	ext := pathpkg.Ext(strings.TrimSpace(sourcePath))
	return strings.EqualFold(ext, textureShortcutExt)
}

func marshalTextureShortcutFile(doc types.Document, sourcePath string) ([]byte, error) {
	return json.MarshalIndent(textureShortcutFile{
		Kind:       textureShortcutFileKind,
		DocID:      doc.DocID,
		Title:      doc.Title,
		SourcePath: sourcePath,
	}, "", "  ")
}
func buildFileOpenTextureMetadata(projection textureFileImportProjection, original *types.ContentItem) json.RawMessage {
	sourcePath := strings.TrimSpace(projection.SourcePath)
	content := projection.ProjectionContent
	mediaType := projection.MediaType
	if mediaType == "" {
		mediaType = detectMediaType("", sourcePath, "")
	}
	sourcePath = strings.TrimSpace(sourcePath)
	sum := sha256.Sum256([]byte(content))
	ext := strings.TrimPrefix(strings.ToLower(pathpkg.Ext(sourcePath)), ".")
	if ext == "" {
		ext = "text"
	}
	lossinessScore := projection.LossinessScore
	warnings := append([]string{}, projection.Warnings...)
	importAdapter := projection.ImportAdapter
	if importAdapter == "" {
		importAdapter = "texture_file_open_projection"
	}
	importAdapterVersion := projection.ImportAdapterVersion
	if importAdapterVersion <= 0 {
		importAdapterVersion = 1
	}
	projectionHash := "sha256:" + hex.EncodeToString(sum[:])
	if projection.ProjectionContentHash != "" {
		projectionHash = "sha256:" + projection.ProjectionContentHash
	}
	originalHash := projectionHash
	if projection.OriginalContentHash != "" {
		originalHash = "sha256:" + projection.OriginalContentHash
	}
	metadata := map[string]any{
		"source_path":  sourcePath,
		"created_from": "file_open",
		"import_manifest": map[string]any{
			"source_path":             sourcePath,
			"source_kind":             ext,
			"source_media_type":       mediaType,
			"original_content_hash":   originalHash,
			"projection_content_hash": projectionHash,
			"projection_kind":         "texture",
			"import_adapter":          importAdapter,
			"import_adapter_version":  importAdapterVersion,
			"lossiness_score":         lossinessScore,
			"warnings":                warnings,
		},
	}
	if len(projection.ExtractionMetadata) > 0 {
		for key, value := range projection.ExtractionMetadata {
			if key == "selectors" || key == "selector_count" || key == "extraction_adapter" || key == "extraction_adapter_version" || key == "extraction_warnings" || key == "text_chars" {
				if manifest, ok := metadata["import_manifest"].(map[string]any); ok {
					manifest[key] = value
				}
			}
		}
	}
	if original != nil && original.ContentID != "" {
		metadata["original_content_item"] = map[string]any{
			"content_id":   original.ContentID,
			"source_type":  original.SourceType,
			"media_type":   original.MediaType,
			"app_hint":     original.AppHint,
			"file_path":    original.FilePath,
			"content_hash": original.ContentHash,
		}
		if manifest, ok := metadata["import_manifest"].(map[string]any); ok {
			manifest["original_content_id"] = original.ContentID
			if projection.OriginalContentHashState != "" {
				manifest["original_content_hash_state"] = projection.OriginalContentHashState
			}
			if textureFileTypeCanStoreTextProjection(original.MediaType) || projection.OriginalContentHash != "" {
				manifest["original_content_hash"] = "sha256:" + original.ContentHash
				if manifest["original_content_hash_state"] == nil {
					manifest["original_content_hash_state"] = "available_from_original_bytes"
				}
			} else {
				manifest["original_content_hash"] = ""
				if manifest["original_content_hash_state"] == nil {
					manifest["original_content_hash_state"] = "unavailable_until_binary_bytes_adapter"
				}
				manifest["original_identity_hash"] = "sha256:" + original.ContentHash
			}
		}
	}
	if migrationManifest := buildTextLikeFileOpenMigrationManifest(sourcePath, ext, mediaType, projectionHash); migrationManifest != nil {
		metadata["migration_manifest"] = migrationManifest
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return json.RawMessage(`{"created_from":"file_open"}`)
	}
	return data
}

func buildTextLikeFileOpenMigrationManifest(sourcePath, ext, mediaType, projectionHash string) map[string]any {
	mediaType = normalizeMediaType(mediaType)
	var sourceKind, adapter, gapPolicy string
	switch mediaType {
	case "text/markdown":
		sourceKind = "markdown"
		adapter = "markdown_to_texture_projection"
		gapPolicy = "repairable_gap_no_invented_citations"
	case "text/plain":
		sourceKind = "text"
		adapter = "plain_text_to_texture_projection"
		gapPolicy = "plain_text_no_implicit_citations"
	case "text/html":
		sourceKind = "html"
		adapter = "html_text_to_texture_projection"
		gapPolicy = "html_text_no_implicit_citations"
	default:
		return nil
	}
	if ext == "md" || ext == "markdown" {
		sourceKind = "markdown"
	}
	if projectionHash == "" {
		projectionHash = "sha256:" + contentHash("")
	}
	return map[string]any{
		"source_path":           sourcePath,
		"source_kind":           sourceKind,
		"source_media_type":     mediaType,
		"original_content_hash": projectionHash,
		"projection_kind":       "texture",
		"migration_adapter":     adapter,
		"migration_version":     1,
		"version_lineage":       []map[string]any{},
		"source_gap_policy":     gapPolicy,
	}
}
func (h *APIHandler) ensureTextureOriginalContentItem(ctx context.Context, ownerID, title string, projection textureFileImportProjection, now time.Time) (types.ContentItem, error) {
	sourcePath := strings.TrimSpace(projection.SourcePath)
	mediaType := projection.MediaType
	hash := projection.OriginalContentHash
	if hash == "" {
		hash = contentHash(projection.ProjectionContent)
	}
	if hash == "" {
		hash = contentHash(sourcePath)
	}
	items, err := h.rt.Store().ListContentItems(ctx, ownerID, 1000)
	if err == nil {
		for _, item := range items {
			if item.SourceType == "file" && item.FilePath == sourcePath && item.MediaType == mediaType {
				return item, nil
			}
		}
	} else {
		log.Printf("texture api: list content items for original file %s: %v", sourcePath, err)
	}
	projectionText := projection.ProjectionContent
	if !textureFileTypeCanStoreTextProjection(mediaType) && !isDocumentMedia(mediaType) {
		projectionText = ""
	}
	item := types.ContentItem{
		ContentID:   uuid.NewString(),
		OwnerID:     ownerID,
		SourceType:  "file",
		MediaType:   mediaType,
		AppHint:     normalizeAppHint(appHintForMedia(mediaType, "", sourcePath)),
		Title:       strings.TrimSpace(title),
		FilePath:    sourcePath,
		TextContent: projectionText,
		ContentHash: hash,
		Metadata:    buildOriginalFileContentMetadata(projection),
		Provenance:  json.RawMessage(`{"created_from":"texture_file_open","original_preserved":true}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if item.Title == "" {
		item.Title = fallbackContentTitle(item)
	}
	if err := h.rt.Store().CreateContentItem(ctx, item); err != nil {
		return types.ContentItem{}, err
	}
	return item, nil
}

func textureFileTypeCanStoreTextProjection(mediaType string) bool {
	switch normalizeMediaType(mediaType) {
	case "text/plain", "text/markdown", "text/html":
		return true
	default:
		return false
	}
}

func buildTextureFileImportProjection(sourcePath, initialContent string) textureFileImportProjection {
	sourcePath = strings.TrimSpace(sourcePath)
	mediaType := detectMediaType("", sourcePath, "")
	projection := textureFileImportProjection{
		SourcePath:           sourcePath,
		MediaType:            mediaType,
		ProjectionContent:    initialContent,
		ImportAdapter:        "texture_file_open_projection",
		ImportAdapterVersion: 1,
		Warnings:             []string{},
	}
	if bytes, ok := readTextureSourceFileBytes(sourcePath); ok {
		projection.OriginalBytes = bytes
		projection.OriginalContentHash = contentHashBytes(bytes)
		projection.OriginalContentHashState = "available_from_original_bytes"
		switch mediaType {
		case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
			docxProjection := extractTextureProjectionFromDocument(context.Background(), sourcePath, mediaType, bytes)
			docxProjection.SourcePath = sourcePath
			docxProjection.MediaType = mediaType
			docxProjection.OriginalBytes = bytes
			docxProjection.OriginalContentHash = projection.OriginalContentHash
			docxProjection.OriginalContentHashState = projection.OriginalContentHashState
			return docxProjection.withProjectionFallback(initialContent)
		case "application/pdf":
			pdfProjection := extractTextureProjectionFromDocument(context.Background(), sourcePath, mediaType, bytes)
			pdfProjection.SourcePath = sourcePath
			pdfProjection.MediaType = mediaType
			pdfProjection.OriginalBytes = bytes
			pdfProjection.OriginalContentHash = projection.OriginalContentHash
			pdfProjection.OriginalContentHashState = projection.OriginalContentHashState
			return pdfProjection.withProjectionFallback(initialContent)
		case "application/epub+zip", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "text/html":
			documentProjection := extractTextureProjectionFromDocument(context.Background(), sourcePath, mediaType, bytes)
			documentProjection.SourcePath = sourcePath
			documentProjection.MediaType = mediaType
			documentProjection.OriginalBytes = bytes
			documentProjection.OriginalContentHash = projection.OriginalContentHash
			documentProjection.OriginalContentHashState = projection.OriginalContentHashState
			return documentProjection.withProjectionFallback(initialContent)
		default:
			if textureFileTypeCanStoreTextProjection(mediaType) {
				projection.ProjectionContent = string(bytes)
				projection.ImportAdapter = "texture_text_file_import"
				projection.ImportAdapterVersion = 1
			}
		}
	} else if initialContent == "" && !isTextureShortcutPath(sourcePath) {
		projection.Warnings = append(projection.Warnings, "source_file_bytes_unavailable_projection_empty")
	}
	if projection.ImportAdapter == "" {
		projection.ImportAdapter = "texture_file_open_projection"
	}
	if projection.ImportAdapterVersion <= 0 {
		projection.ImportAdapterVersion = 1
	}
	projection.ProjectionContentHash = contentHash(projection.ProjectionContent)
	if projection.OriginalContentHashState == "" {
		if projection.OriginalContentHash != "" {
			projection.OriginalContentHashState = "available_from_original_bytes"
		} else if textureFileTypeCanStoreTextProjection(mediaType) {
			projection.OriginalContentHash = projection.ProjectionContentHash
			projection.OriginalContentHashState = "available_from_text_projection"
		} else {
			projection.OriginalContentHashState = "unavailable_until_binary_bytes_adapter"
			switch mediaType {
			case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
				projection.LossinessScore = 40
				projection.Warnings = appendIfMissing(projection.Warnings, "docx_projection_requires_style_adapter")
			case "application/pdf":
				projection.LossinessScore = 80
				projection.Warnings = appendIfMissing(projection.Warnings, "pdf_projection_requires_extraction_adapter")
			case "application/octet-stream":
				projection.LossinessScore = 100
				projection.Warnings = appendIfMissing(projection.Warnings, "unknown_file_type_projection_is_placeholder")
			}
		}
	}
	return projection
}

func extractTextureProjectionFromDocument(ctx context.Context, sourcePath, mediaType string, data []byte) textureFileImportProjection {
	extracted := extractContentDocument(ctx, sourcePath, mediaType, data)
	lossiness := 20
	if strings.TrimSpace(extracted.Text) == "" {
		lossiness = 95
	}
	return textureFileImportProjection{
		SourcePath:            sourcePath,
		MediaType:             extracted.MediaType,
		ProjectionContent:     extracted.Text,
		ProjectionContentHash: contentHash(extracted.Text),
		ImportAdapter:         extracted.Adapter,
		ImportAdapterVersion:  contentExtractionAdapterVersion,
		LossinessScore:        lossiness,
		Warnings:              extracted.Warnings,
		ExtractionMetadata:    extracted.Metadata,
	}
}

func (p textureFileImportProjection) withProjectionFallback(initialContent string) textureFileImportProjection {
	if strings.TrimSpace(p.ProjectionContent) == "" && strings.TrimSpace(initialContent) != "" {
		p.ProjectionContent = initialContent
		p.Warnings = appendIfMissing(p.Warnings, "projection_used_caller_supplied_initial_content")
	}
	if p.ProjectionContentHash == "" {
		p.ProjectionContentHash = contentHash(p.ProjectionContent)
	}
	if p.ImportAdapter == "" {
		p.ImportAdapter = "texture_file_open_projection"
	}
	if p.ImportAdapterVersion <= 0 {
		p.ImportAdapterVersion = 1
	}
	return p
}

func readTextureSourceFileBytes(sourcePath string) ([]byte, bool) {
	sourcePath = normalizeTextureSourcePath(sourcePath)
	if sourcePath == "" || isTextureShortcutPath(sourcePath) {
		return nil, false
	}
	filesRoot := sandbox.ResolveFilesRoot("")
	absPath := filepath.Join(filesRoot, filepath.FromSlash(sourcePath))
	cleanRoot, err := filepath.Abs(filesRoot)
	if err != nil {
		return nil, false
	}
	cleanPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, false
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return nil, false
	}
	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() || info.Size() > 25*1024*1024 {
		return nil, false
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, false
	}
	return data, true
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func extractTextureProjectionFromDOCX(data []byte) textureFileImportProjection {
	projection := textureFileImportProjection{
		ImportAdapter:        "docx_ooxml_text_table_projection",
		ImportAdapterVersion: 1,
		LossinessScore:       35,
		Warnings:             []string{"docx_styles_preserved_as_manifest_only"},
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		projection.LossinessScore = 90
		projection.Warnings = append(projection.Warnings, "docx_zip_open_failed")
		return projection
	}
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			projection.LossinessScore = 90
			projection.Warnings = append(projection.Warnings, "docx_document_xml_open_failed")
			return projection
		}
		documentXML, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			projection.LossinessScore = 90
			projection.Warnings = append(projection.Warnings, "docx_document_xml_read_failed")
			return projection
		}
		break
	}
	if len(documentXML) == 0 {
		projection.LossinessScore = 90
		projection.Warnings = append(projection.Warnings, "docx_document_xml_missing")
		return projection
	}
	projection.ProjectionContent = strings.TrimSpace(docxDocumentXMLToMarkdown(documentXML))
	projection.ProjectionContentHash = contentHash(projection.ProjectionContent)
	return projection
}

func docxDocumentXMLToMarkdown(data []byte) string {
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
		for i := 0; i < cols; i++ {
			if i < len(row) {
				out[i] = strings.ReplaceAll(row[i], "|", "\\|")
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

func extractTextureProjectionFromPDF(data []byte) textureFileImportProjection {
	projection := textureFileImportProjection{
		ImportAdapter:        "pdf_literal_text_projection",
		ImportAdapterVersion: 1,
		LossinessScore:       80,
		Warnings:             []string{"pdf_layout_is_best_effort"},
	}
	text := extractPDFLiteralText(data)
	if strings.TrimSpace(text) == "" {
		projection.LossinessScore = 95
		projection.Warnings = append(projection.Warnings, "pdf_text_extraction_empty")
	} else {
		projection.ProjectionContent = strings.TrimSpace(text)
	}
	projection.ProjectionContentHash = contentHash(projection.ProjectionContent)
	return projection
}

func extractPDFLiteralText(data []byte) string {
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
		for _, lit := range stringRE.FindAllString(array, -1) {
			parts = append(parts, decodePDFLiteralString(lit))
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

func buildOriginalFileContentMetadata(projection textureFileImportProjection) json.RawMessage {
	sourcePath := strings.TrimSpace(projection.SourcePath)
	mediaType := projection.MediaType
	projectionHash := projection.ProjectionContentHash
	if projectionHash == "" {
		projectionHash = contentHash(projection.ProjectionContent)
	}
	originalHash := projection.OriginalContentHash
	if originalHash == "" && projection.OriginalContentHashState != "unavailable_until_binary_bytes_adapter" {
		originalHash = projectionHash
	}
	metadata := map[string]any{
		"schema":                  "choir.content.original_file.v0",
		"source_path":             sourcePath,
		"media_type":              mediaType,
		"projection_content_hash": "sha256:" + projectionHash,
		"import_adapter":          projection.ImportAdapter,
		"import_adapter_version":  projection.ImportAdapterVersion,
		"lossiness_score":         projection.LossinessScore,
		"warnings":                projection.Warnings,
		"preservation":            "original_file_path_preserved_in_user_filesystem",
	}
	for key, value := range projection.ExtractionMetadata {
		metadata[key] = value
	}
	if originalHash == "" {
		metadata["original_content_hash"] = ""
		metadata["original_identity_hash"] = "sha256:" + contentHash(sourcePath)
	} else {
		metadata["original_content_hash"] = "sha256:" + originalHash
	}
	if projection.OriginalContentHashState != "" {
		metadata["original_content_hash_state"] = projection.OriginalContentHashState
	} else {
		metadata["original_content_hash_state"] = "available_from_text_projection"
	}
	if !textureFileTypeCanStoreTextProjection(mediaType) {
		metadata["text_content_policy"] = "not_embedded_for_binary_original"
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return json.RawMessage(`{"schema":"choir.content.original_file.v0"}`)
	}
	return data
}
func (h *APIHandler) ensureTextureManifest(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	return ensureTextureManifest(ctx, h.rt.Store(), ownerID, doc)
}

func (rt *Runtime) ensureCanonicalTextureProjectionPath(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	sourcePath, err := rt.ensureTextureManifest(ctx, ownerID, doc)
	if err != nil {
		return "", err
	}
	if !isTextureShortcutPath(sourcePath) {
		return "", fmt.Errorf("manifest path %q is not a Texture shortcut", sourcePath)
	}
	return sourcePath, nil
}

func (rt *Runtime) ensureTextureManifest(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	if rt == nil || rt.store == nil {
		return "", fmt.Errorf("runtime store unavailable")
	}
	return ensureTextureManifest(ctx, rt.store, ownerID, doc)
}

func ensureTextureManifest(ctx context.Context, st *store.Store, ownerID string, doc types.Document) (string, error) {
	if st == nil {
		return "", fmt.Errorf("store unavailable")
	}
	sourcePath, err := st.GetDocumentAliasSourcePath(ctx, ownerID, doc.DocID)
	if err != nil && err != store.ErrNotFound {
		return "", err
	}
	if err == store.ErrNotFound || !isTextureShortcutPath(sourcePath) {
		sourcePath, err = allocateTextureManifestPath(ctx, st, ownerID, doc)
		if err != nil {
			return "", err
		}
	}

	content, err := marshalTextureShortcutFile(doc, sourcePath)
	if err != nil {
		return "", fmt.Errorf("marshal texture shortcut: %w", err)
	}

	filesRoot := sandbox.ResolveFilesRoot("")
	absPath := filepath.Join(filesRoot, filepath.FromSlash(sourcePath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(absPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write manifest file: %w", err)
	}
	if err := st.UpsertDocumentAlias(ctx, ownerID, sourcePath, doc.DocID, time.Now().UTC()); err != nil {
		return "", err
	}
	return sourcePath, nil
}

func textureDocumentExportFilename(title, format string) string {
	format = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(format)), ".")
	if format == "" {
		format = "md"
	}
	base := strings.TrimSpace(pathpkg.Base(title))
	if base == "" || base == "." || base == "/" {
		base = "texture"
	}
	ext := pathpkg.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	base = strings.Trim(base, ". ")
	if base == "" {
		base = "texture"
	}
	return base + "." + format
}

func (h *APIHandler) ensureCanonicalTextureProjectionPath(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	sourcePath, err := h.ensureTextureManifest(ctx, ownerID, doc)
	if err != nil {
		return "", err
	}
	if !isTextureShortcutPath(sourcePath) {
		return "", fmt.Errorf("manifest path %q is not a Texture shortcut", sourcePath)
	}
	return sourcePath, nil
}

func allocateTextureManifestPath(ctx context.Context, st *store.Store, ownerID string, doc types.Document) (string, error) {
	stem := slugifyTextureManifestStem(doc.Title)
	suffix := shortDocIDSuffix(doc.DocID)
	candidates := []string{
		fmt.Sprintf("%s%s", stem, textureShortcutExt),
		fmt.Sprintf("%s-%s%s", stem, suffix, textureShortcutExt),
	}
	filesRoot := sandbox.ResolveFilesRoot("")
	for _, candidate := range candidates {
		docID, err := st.GetDocumentAlias(ctx, ownerID, candidate)
		if err == nil {
			if docID == doc.DocID {
				return candidate, nil
			}
			continue
		}
		if err != store.ErrNotFound {
			return "", err
		}
		absPath := filepath.Join(filesRoot, filepath.FromSlash(candidate))
		if _, statErr := os.Stat(absPath); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		return candidate, nil
	}
	return fmt.Sprintf("%s-%s%s", stem, uuid.New().String()[:8], textureShortcutExt), nil
}
