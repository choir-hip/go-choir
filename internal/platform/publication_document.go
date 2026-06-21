package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
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
		Title:    firstNonEmpty(bundle.Publication.Title, defaultPublishedTextureTitle),
		Blocks:   publicationDocumentBlocks(bundle.Artifact.Content),
		Manifest: buildPublicationSourceManifest(bundle),
	}
	if blocks, ok := publicationDocumentBlocksFromStructured(bundle.Artifact.BodyDoc, bundle.Artifact.SourceEntities); ok {
		doc.Blocks = blocks
	}
	return doc
}

func publicationDocumentBlocksFromStructured(bodyDocRaw, sourceEntitiesRaw json.RawMessage) ([]publicationDocumentBlock, bool) {
	if strings.TrimSpace(string(bodyDocRaw)) == "" {
		return nil, false
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(bodyDocRaw, &doc); err != nil {
		return nil, false
	}
	var entities []texturedoc.SourceEntity
	if trimmed := strings.TrimSpace(string(sourceEntitiesRaw)); trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(sourceEntitiesRaw, &entities); err != nil {
			return nil, false
		}
	}
	if err := texturedoc.Validate(doc, entities); err != nil {
		return nil, false
	}
	entityTitles := make(map[string]string, len(entities))
	for _, entity := range entities {
		entityTitles[entity.SourceEntityID] = firstNonEmpty(entity.Display.Label, entity.Display.Title, entity.Target.ID, entity.Target.URI, entity.SourceEntityID)
	}
	var blocks []publicationDocumentBlock
	for _, node := range doc.Doc.Content {
		blocks = append(blocks, publicationBlocksFromStructuredNode(node, entityTitles)...)
	}
	return blocks, true
}

func publicationBlocksFromStructuredNode(node texturedoc.Node, entityTitles map[string]string) []publicationDocumentBlock {
	switch node.Type {
	case "heading":
		return []publicationDocumentBlock{{
			Kind:    "heading",
			Level:   clampInt(publicationNodeIntAttr(node, "level", 1), 1, 6),
			Inlines: publicationStructuredInlines(node.Content, entityTitles),
		}}
	case "paragraph":
		return []publicationDocumentBlock{{Kind: "paragraph", Inlines: publicationStructuredInlines(node.Content, entityTitles)}}
	case "bullet_list", "ordered_list":
		var blocks []publicationDocumentBlock
		for _, item := range node.Content {
			blocks = append(blocks, publicationBlocksFromStructuredListItem(item, entityTitles)...)
		}
		return blocks
	case "blockquote":
		var blocks []publicationDocumentBlock
		for _, child := range node.Content {
			for _, block := range publicationBlocksFromStructuredNode(child, entityTitles) {
				if len(block.Inlines) > 0 {
					block.Inlines = append([]publicationInline{{Kind: "text", Text: "> "}}, block.Inlines...)
				}
				blocks = append(blocks, block)
			}
		}
		return blocks
	case "code_block":
		return []publicationDocumentBlock{{Kind: "paragraph", Inlines: []publicationInline{{Kind: "text", Text: publicationCodeBlockText(node)}}}}
	case "horizontal_rule":
		return []publicationDocumentBlock{{Kind: "rule"}}
	case "source_embed":
		return []publicationDocumentBlock{{Kind: "paragraph", Inlines: []publicationInline{publicationStructuredSourceInline(node, entityTitles)}}}
	default:
		return nil
	}
}

func publicationBlocksFromStructuredListItem(node texturedoc.Node, entityTitles map[string]string) []publicationDocumentBlock {
	if node.Type != "list_item" {
		return publicationBlocksFromStructuredNode(node, entityTitles)
	}
	var blocks []publicationDocumentBlock
	for _, child := range node.Content {
		inlines := publicationStructuredBlockInlines(child, entityTitles)
		if len(inlines) > 0 {
			blocks = append(blocks, publicationDocumentBlock{Kind: "list_item", Inlines: inlines})
		}
	}
	return blocks
}

func publicationStructuredBlockInlines(node texturedoc.Node, entityTitles map[string]string) []publicationInline {
	switch node.Type {
	case "paragraph", "heading":
		return publicationStructuredInlines(node.Content, entityTitles)
	case "source_embed":
		return []publicationInline{publicationStructuredSourceInline(node, entityTitles)}
	default:
		var inlines []publicationInline
		for _, child := range node.Content {
			inlines = append(inlines, publicationStructuredBlockInlines(child, entityTitles)...)
		}
		return inlines
	}
}

func publicationStructuredInlines(nodes []texturedoc.Node, entityTitles map[string]string) []publicationInline {
	var out []publicationInline
	for _, node := range nodes {
		switch node.Type {
		case "text":
			out = append(out, publicationInline{Kind: publicationInlineKindForMarks(node.Marks), Text: node.Text})
		case "hard_break":
			out = append(out, publicationInline{Kind: "text", Text: "\n"})
		case "source_ref":
			out = append(out, publicationStructuredSourceInline(node, entityTitles))
		}
	}
	return mergeAdjacentTextInlines(out)
}

func publicationStructuredSourceInline(node texturedoc.Node, entityTitles map[string]string) publicationInline {
	sourceEntityID := strings.TrimSpace(publicationNodeStringAttr(node, "source_entity_id"))
	label := strings.TrimSpace(publicationNodeStringAttr(node, "label"))
	if label == "" {
		label = entityTitles[sourceEntityID]
	}
	return publicationInline{Kind: "source_ref", Text: firstNonEmpty(label, sourceEntityID), SourceID: sourceEntityID}
}

func publicationInlineKindForMarks(marks []texturedoc.Mark) string {
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			return "strong"
		case "emphasis":
			return "em"
		case "code":
			return "code"
		}
	}
	return "text"
}

func publicationCodeBlockText(node texturedoc.Node) string {
	lines := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		lines = append(lines, child.Text)
	}
	return strings.Join(lines, "\n")
}

func publicationNodeStringAttr(node texturedoc.Node, key string) string {
	if node.Attrs == nil {
		return ""
	}
	value, ok := node.Attrs[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func publicationNodeIntAttr(node texturedoc.Node, key string, fallback int) int {
	if node.Attrs == nil {
		return fallback
	}
	switch value := node.Attrs[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
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
		for _, key := range []string{"title", "label", "name"} {
			if value := strings.TrimSpace(stringField(nestedObject(raw, "display"), key)); value != "" {
				title = value
				break
			}
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
	if url == "" {
		for _, key := range []string{"uri", "url", "source_url", "href"} {
			if value := strings.TrimSpace(stringField(nestedObject(raw, "target"), key)); value != "" {
				url = value
				break
			}
		}
	}
	for _, key := range []string{"reader_artifact_state", "reader_state", "artifact_state"} {
		if value := strings.TrimSpace(stringField(raw, key)); value != "" {
			readerState = value
			break
		}
	}
	if readerState == "" {
		for _, key := range []string{"reader_artifact_state", "reader_state", "artifact_state"} {
			if value := strings.TrimSpace(stringField(nestedObject(raw, "display"), key)); value != "" {
				readerState = value
				break
			}
		}
	}
	if readerState == "" {
		for _, key := range []string{"reader_artifact_state", "reader_state", "artifact_state"} {
			if value := strings.TrimSpace(stringField(nestedObject(raw, "evidence"), key)); value != "" {
				readerState = value
				break
			}
		}
	}
	return title, url, readerState
}

func nestedObject(raw map[string]any, key string) map[string]any {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok {
		return nil
	}
	nested, _ := value.(map[string]any)
	return nested
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

func publicationSourceOrdinals(doc PublicationDocument) map[string]int {
	out := make(map[string]int, len(doc.Manifest.Sources))
	for i, source := range doc.Manifest.Sources {
		if strings.TrimSpace(source.SourceEntityID) == "" {
			continue
		}
		out[source.SourceEntityID] = i + 1
	}
	return out
}

func publicationSourceMarker(ordinals map[string]int, sourceID string) string {
	if ordinal, ok := ordinals[sourceID]; ok && ordinal > 0 {
		return strconv.Itoa(ordinal)
	}
	return "?"
}
