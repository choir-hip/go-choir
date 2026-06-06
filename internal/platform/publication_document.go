package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type PublicationDocument struct {
	Title    string
	Blocks   []publicationDocumentBlock
	Manifest publicationSourceManifest
}

type publicationDocumentBlock struct {
	Kind    string
	Level   int
	Inlines []publicationInline
	Rows    [][]publicationTableCell
}

type publicationTableCell struct {
	Inlines []publicationInline
	Header  bool
}

type publicationInline struct {
	Kind     string
	Text     string
	Href     string
	SourceID string
}

type publicationSourceManifest struct {
	Schema                 string                          `json:"schema"`
	PublicationID          string                          `json:"publication_id"`
	PublicationVersionID   string                          `json:"publication_version_id"`
	RoutePath              string                          `json:"route_path"`
	ContentHash            string                          `json:"content_hash"`
	SourceRevisionHash     string                          `json:"source_revision_hash"`
	ProjectionHash         string                          `json:"projection_hash"`
	GeneratedAt            string                          `json:"generated_at"`
	AccessPolicy           json.RawMessage                 `json:"access_policy"`
	ExportPolicy           json.RawMessage                 `json:"export_policy"`
	Sources                []publicationSourceManifestItem `json:"sources"`
	Transclusions          []PublicationTransclusion       `json:"transclusions"`
	PrivateMaterialOmitted bool                            `json:"private_material_omitted"`
}

type publicationSourceManifestItem struct {
	SourceEntityID      string          `json:"source_entity_id"`
	Title               string          `json:"title,omitempty"`
	URL                 string          `json:"url,omitempty"`
	OpenSurface         string          `json:"open_surface,omitempty"`
	EvidenceState       json.RawMessage `json:"evidence_state,omitempty"`
	ReaderArtifactState string          `json:"reader_artifact_state,omitempty"`
	Selector            json.RawMessage `json:"selector,omitempty"`
	SnapshotText        string          `json:"snapshot_text,omitempty"`
	SnapshotHash        string          `json:"snapshot_hash,omitempty"`
	Entity              json.RawMessage `json:"entity,omitempty"`
}

func buildPublicationDocument(bundle *PublicationBundle) PublicationDocument {
	if bundle == nil {
		return PublicationDocument{}
	}
	doc := PublicationDocument{
		Title:    firstNonEmpty(bundle.Publication.Title, "Published VText"),
		Blocks:   publicationDocumentBlocks(bundle.Artifact.Content),
		Manifest: buildPublicationSourceManifest(bundle),
	}
	return doc
}

func buildPublicationSourceManifest(bundle *PublicationBundle) publicationSourceManifest {
	manifest := publicationSourceManifest{
		Schema:                 "choir.publication_sources.v1",
		PublicationID:          bundle.Publication.ID,
		PublicationVersionID:   bundle.Version.ID,
		RoutePath:              bundle.Route.Path,
		ContentHash:            bundle.Version.ContentHash,
		SourceRevisionHash:     bundle.Version.SourceRevisionHash,
		ProjectionHash:         bundle.Version.ProjectionHash,
		GeneratedAt:            time.Now().UTC().Format(time.RFC3339Nano),
		AccessPolicy:           json.RawMessage(firstNonEmpty(string(bundle.Policy.Access), "{}")),
		ExportPolicy:           json.RawMessage(firstNonEmpty(string(bundle.Policy.Export), "{}")),
		Transclusions:          bundle.Transclusions,
		PrivateMaterialOmitted: true,
	}
	transclusionsByEntity := make(map[string]PublicationTransclusion, len(bundle.Transclusions))
	for _, transclusion := range bundle.Transclusions {
		transclusionsByEntity[transclusion.SourceEntityID] = transclusion
	}
	for _, entity := range bundle.SourceEntities {
		item := publicationSourceManifestItem{
			SourceEntityID: entity.SourceEntityID,
			OpenSurface:    entity.OpenSurface,
			Entity:         entity.Entity,
		}
		item.Title, item.URL, item.ReaderArtifactState = sourceEntityDisplayFields(entity)
		if transclusion, ok := transclusionsByEntity[entity.SourceEntityID]; ok {
			item.Selector = transclusion.SourceSelector
			item.SnapshotText = transclusion.SnapshotText
			item.SnapshotHash = sourceSnapshotHash(transclusion.SnapshotText, transclusion.ContentHash)
			item.EvidenceState = evidenceStateFromSelector(transclusion.SourceSelector)
		}
		manifest.Sources = append(manifest.Sources, item)
	}
	return manifest
}

func sourceEntityDisplayFields(entity PublicationSourceEntity) (title, url, readerState string) {
	var raw map[string]any
	if len(entity.Entity) > 0 {
		_ = json.Unmarshal(entity.Entity, &raw)
	}
	for _, key := range []string{"title", "label", "name"} {
		if value := strings.TrimSpace(stringField(raw, key)); value != "" {
			title = value
			break
		}
	}
	if title == "" {
		title = firstNonEmpty(entity.SourceEntityID, entity.ID)
	}
	for _, key := range []string{"url", "source_url", "href"} {
		if value := strings.TrimSpace(stringField(raw, key)); value != "" {
			url = value
			break
		}
	}
	for _, key := range []string{"reader_artifact_state", "reader_state", "artifact_state"} {
		if value := strings.TrimSpace(stringField(raw, key)); value != "" {
			readerState = value
			break
		}
	}
	return title, url, readerState
}

func stringField(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(jsonString(typed)), "\n", " "), "\t", " "), `"`))
	}
}

func jsonString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func sourceSnapshotHash(snapshotText, contentHash string) string {
	if strings.TrimSpace(contentHash) != "" {
		return contentHash
	}
	if snapshotText == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(snapshotText))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func evidenceStateFromSelector(selector json.RawMessage) json.RawMessage {
	if len(selector) == 0 {
		return nil
	}
	var raw struct {
		EvidenceState json.RawMessage `json:"evidence_state"`
	}
	if err := json.Unmarshal(selector, &raw); err != nil || len(raw.EvidenceState) == 0 {
		return nil
	}
	return raw.EvidenceState
}

func publicationDocumentBlocks(content string) []publicationDocumentBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var blocks []publicationDocumentBlock
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if line == "---" {
			blocks = append(blocks, publicationDocumentBlock{Kind: "rule"})
			continue
		}
		if isMarkdownTableStart(lines, i) {
			header := splitMarkdownTableRow(lines[i])
			rows := [][]publicationTableCell{publicationTableCells(header, true)}
			i += 2
			for i < len(lines) && strings.Contains(lines[i], "|") {
				rows = append(rows, publicationTableCells(splitMarkdownTableRow(lines[i]), false))
				i++
			}
			i--
			blocks = append(blocks, publicationDocumentBlock{Kind: "table", Rows: rows})
			continue
		}
		if strings.HasPrefix(line, "#") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			if level > 0 && level < len(line) && line[level] == ' ' {
				blocks = append(blocks, publicationDocumentBlock{
					Kind:    "heading",
					Level:   clampInt(level, 1, 6),
					Inlines: parsePublicationInlines(strings.TrimSpace(line[level:])),
				})
				continue
			}
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			blocks = append(blocks, publicationDocumentBlock{Kind: "list_item", Inlines: parsePublicationInlines(strings.TrimSpace(line[2:]))})
			continue
		}
		blocks = append(blocks, publicationDocumentBlock{Kind: "paragraph", Inlines: parsePublicationInlines(line)})
	}
	return blocks
}

func publicationTableCells(values []string, header bool) []publicationTableCell {
	cells := make([]publicationTableCell, 0, len(values))
	for _, value := range values {
		cells = append(cells, publicationTableCell{Inlines: parsePublicationInlines(value), Header: header})
	}
	return cells
}

var publicationLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func parsePublicationInlines(text string) []publicationInline {
	var out []publicationInline
	for len(text) > 0 {
		loc := publicationLinkPattern.FindStringSubmatchIndex(text)
		if loc == nil {
			out = append(out, parsePublicationStyledText(text)...)
			break
		}
		if loc[0] > 0 {
			out = append(out, parsePublicationStyledText(text[:loc[0]])...)
		}
		label := text[loc[2]:loc[3]]
		href := text[loc[4]:loc[5]]
		inline := publicationInline{Kind: "link", Text: label, Href: href}
		if strings.HasPrefix(href, "source:") {
			inline.Kind = "source_ref"
			inline.SourceID = strings.TrimPrefix(href, "source:")
		}
		out = append(out, inline)
		text = text[loc[1]:]
	}
	return mergeAdjacentTextInlines(out)
}

func parsePublicationStyledText(text string) []publicationInline {
	var out []publicationInline
	for len(text) > 0 {
		strong := strings.Index(text, "**")
		em := strings.Index(text, "*")
		if strong >= 0 && (em < 0 || strong <= em) {
			if strong > 0 {
				out = append(out, publicationInline{Kind: "text", Text: text[:strong]})
			}
			rest := text[strong+2:]
			end := strings.Index(rest, "**")
			if end < 0 {
				out = append(out, publicationInline{Kind: "text", Text: text[strong:]})
				break
			}
			out = append(out, publicationInline{Kind: "strong", Text: rest[:end]})
			text = rest[end+2:]
			continue
		}
		if em >= 0 {
			if em > 0 {
				out = append(out, publicationInline{Kind: "text", Text: text[:em]})
			}
			rest := text[em+1:]
			end := strings.Index(rest, "*")
			if end < 0 {
				out = append(out, publicationInline{Kind: "text", Text: text[em:]})
				break
			}
			out = append(out, publicationInline{Kind: "em", Text: rest[:end]})
			text = rest[end+1:]
			continue
		}
		out = append(out, publicationInline{Kind: "text", Text: text})
		break
	}
	return out
}

func mergeAdjacentTextInlines(in []publicationInline) []publicationInline {
	out := make([]publicationInline, 0, len(in))
	for _, inline := range in {
		if inline.Text == "" && inline.Href == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Kind == "text" && inline.Kind == "text" {
			out[len(out)-1].Text += inline.Text
			continue
		}
		out = append(out, inline)
	}
	return out
}

func publicationInlinesPlainText(inlines []publicationInline) string {
	var b strings.Builder
	for _, inline := range inlines {
		b.WriteString(inline.Text)
	}
	return b.String()
}
