// Package texturedoc defines the D1 internal spike for Choir-owned
// StructuredTextureDoc v1 validation and projection.
package texturedoc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
)

const SchemaV1 = "choir.texture_doc.v1"

type StructuredTextureDoc struct {
	Schema string `json:"schema"`
	Doc    Node   `json:"doc"`
}

type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type SourceEntity struct {
	SourceEntityID       string                 `json:"source_entity_id"`
	Target               SourceTarget           `json:"target"`
	Selectors            []SourceSelector       `json:"selectors,omitempty"`
	Display              SourceDisplay          `json:"display"`
	Evidence             SourceEvidence         `json:"evidence"`
	Provenance           SourceEntityProvenance `json:"provenance"`
	ReaderSnapshot       map[string]any         `json:"reader_snapshot,omitempty"`
	ReaderSnapshotStatus map[string]any         `json:"reader_snapshot_status,omitempty"`
}

type SourceTarget struct {
	Kind     string         `json:"kind"`
	URI      string         `json:"uri,omitempty"`
	ID       string         `json:"id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type SourceSelector struct {
	Kind string         `json:"kind"`
	Data map[string]any `json:"data,omitempty"`
}

type SourceDisplay struct {
	Mode        string `json:"mode"`
	Title       string `json:"title,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type SourceEvidence struct {
	State               string   `json:"state"`
	OpenSurface         string   `json:"open_surface"`
	Relation            string   `json:"relation,omitempty"`
	ResearchState       string   `json:"research_state,omitempty"`
	Uncertainty         string   `json:"uncertainty,omitempty"`
	ReaderArtifactState string   `json:"reader_artifact_state,omitempty"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
}

type SourceEntityProvenance struct {
	CreatedBy           string `json:"created_by"`
	CreatedAt           string `json:"created_at,omitempty"`
	SourceSystem        string `json:"source_system,omitempty"`
	ImportArtifact      string `json:"import_artifact,omitempty"`
	RightsScope         string `json:"rights_scope,omitempty"`
	UntrustedSourceText bool   `json:"untrusted_source_text,omitempty"`
}

type Validator struct {
	entities map[string]SourceEntity
	refs     map[string]bool
}

func Validate(doc StructuredTextureDoc, entities []SourceEntity) error {
	validator, err := newValidator(entities)
	if err != nil {
		return err
	}
	if strings.TrimSpace(doc.Schema) != SchemaV1 {
		return fmt.Errorf("schema %q is not %q", doc.Schema, SchemaV1)
	}
	if doc.Doc.Type != "doc" {
		return fmt.Errorf("root node type %q is not doc", doc.Doc.Type)
	}
	if nodeID(doc.Doc) == "" {
		return fmt.Errorf("doc attrs.id is required")
	}
	if len(doc.Doc.Content) == 0 {
		return fmt.Errorf("doc content must contain at least one block")
	}
	for i := range doc.Doc.Content {
		if err := validator.validateBlock(doc.Doc.Content[i], "doc.content"); err != nil {
			return err
		}
	}
	for sourceEntityID := range validator.entities {
		if !validator.refs[sourceEntityID] {
			return fmt.Errorf("source_entity_id %q is not referenced by a source_ref or source_embed node", sourceEntityID)
		}
	}
	return nil
}

func newValidator(entities []SourceEntity) (*Validator, error) {
	validator := &Validator{
		entities: make(map[string]SourceEntity, len(entities)),
		refs:     make(map[string]bool),
	}
	for i, entity := range entities {
		if err := validateSourceEntity(entity); err != nil {
			return nil, fmt.Errorf("source_entities[%d]: %w", i, err)
		}
		if _, exists := validator.entities[entity.SourceEntityID]; exists {
			return nil, fmt.Errorf("source_entities[%d]: duplicate source_entity_id %q", i, entity.SourceEntityID)
		}
		validator.entities[entity.SourceEntityID] = entity
	}
	return validator, nil
}

func validateSourceEntity(entity SourceEntity) error {
	if strings.TrimSpace(entity.SourceEntityID) == "" {
		return fmt.Errorf("source_entity_id is required")
	}
	if !validSourceTargetKind(entity.Target.Kind) {
		return fmt.Errorf("target.kind %q is not supported", entity.Target.Kind)
	}
	for i, selector := range entity.Selectors {
		if !validSelectorKind(selector.Kind) {
			return fmt.Errorf("selectors[%d].kind %q is not supported", i, selector.Kind)
		}
	}
	if !validDisplayMode(entity.Display.Mode) {
		return fmt.Errorf("display.mode %q is not supported", entity.Display.Mode)
	}
	if sourcecontract.NormalizeEvidenceState(entity.Evidence.State) == "" {
		return fmt.Errorf("evidence.state %q is not supported", entity.Evidence.State)
	}
	if !validOpenSurface(entity.Evidence.OpenSurface) {
		return fmt.Errorf("evidence.open_surface %q is not supported", entity.Evidence.OpenSurface)
	}
	if entity.Evidence.ReaderArtifactState != "" && sourcecontract.NormalizeReaderArtifactState(entity.Evidence.ReaderArtifactState) == "" {
		return fmt.Errorf("evidence.reader_artifact_state %q is not supported", entity.Evidence.ReaderArtifactState)
	}
	if strings.TrimSpace(entity.Provenance.CreatedBy) == "" {
		return fmt.Errorf("provenance.created_by is required")
	}
	return nil
}

func (v *Validator) validateBlock(node Node, path string) error {
	switch node.Type {
	case "paragraph", "heading":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		if node.Type == "heading" {
			level, ok := intAttr(node, "level")
			if !ok || level < 1 || level > 6 {
				return fmt.Errorf("%s.heading attrs.level must be 1..6", path)
			}
		}
		for i := range node.Content {
			if err := v.validateInline(node.Content[i], fmt.Sprintf("%s.%s.content[%d]", path, node.Type, i)); err != nil {
				return err
			}
		}
	case "bullet_list", "ordered_list":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		if node.Type == "ordered_list" {
			if start, ok := intAttr(node, "start"); ok && start < 1 {
				return fmt.Errorf("%s.ordered_list attrs.start must be positive", path)
			}
		}
		if len(node.Content) == 0 {
			return fmt.Errorf("%s.%s must contain at least one list_item", path, node.Type)
		}
		for i := range node.Content {
			if node.Content[i].Type != "list_item" {
				return fmt.Errorf("%s.%s.content[%d] must be list_item, got %q", path, node.Type, i, node.Content[i].Type)
			}
			if err := v.validateBlock(node.Content[i], fmt.Sprintf("%s.%s.content[%d]", path, node.Type, i)); err != nil {
				return err
			}
		}
	case "list_item", "blockquote":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		if len(node.Content) == 0 {
			return fmt.Errorf("%s.%s must contain at least one block", path, node.Type)
		}
		for i := range node.Content {
			if err := v.validateBlock(node.Content[i], fmt.Sprintf("%s.%s.content[%d]", path, node.Type, i)); err != nil {
				return err
			}
		}
	case "code_block":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		for i := range node.Content {
			child := node.Content[i]
			if child.Type != "text" {
				return fmt.Errorf("%s.code_block.content[%d] must be text, got %q", path, i, child.Type)
			}
			if len(child.Marks) > 0 {
				return fmt.Errorf("%s.code_block.content[%d] must not carry marks", path, i)
			}
			if err := validateTextPayload(child.Text, fmt.Sprintf("%s.code_block.content[%d]", path, i)); err != nil {
				return err
			}
		}
	case "horizontal_rule":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		if len(node.Content) != 0 || node.Text != "" {
			return fmt.Errorf("%s.horizontal_rule must be a leaf block", path)
		}
	case "source_embed":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		sourceEntityID := stringAttr(node, "source_entity_id")
		if sourceEntityID == "" {
			return fmt.Errorf("%s.source_embed attrs.source_entity_id is required", path)
		}
		if _, ok := v.entities[sourceEntityID]; !ok {
			return fmt.Errorf("%s.source_embed source_entity_id %q does not resolve", path, sourceEntityID)
		}
		displayMode := stringAttr(node, "display_mode")
		if !validDisplayMode(displayMode) || displayMode == "numbered_ref" || displayMode == "inline_chip" {
			return fmt.Errorf("%s.source_embed attrs.display_mode %q is not a block display mode", path, displayMode)
		}
		if len(node.Content) != 0 || node.Text != "" || len(node.Marks) != 0 {
			return fmt.Errorf("%s.source_embed must be a leaf block without text, content, or marks", path)
		}
		v.refs[sourceEntityID] = true
	default:
		return fmt.Errorf("%s unsupported block node type %q", path, node.Type)
	}
	return nil
}

func (v *Validator) validateInline(node Node, path string) error {
	switch node.Type {
	case "text":
		if node.Text == "" {
			return fmt.Errorf("%s.text must not be empty", path)
		}
		if err := validateMarks(node.Marks, path); err != nil {
			return err
		}
		return validateTextPayload(node.Text, path)
	case "hard_break":
		if len(node.Content) != 0 || node.Text != "" || len(node.Marks) != 0 {
			return fmt.Errorf("%s.hard_break must be a leaf without marks", path)
		}
	case "source_ref":
		if err := requireNodeID(node, path); err != nil {
			return err
		}
		sourceEntityID := stringAttr(node, "source_entity_id")
		if sourceEntityID == "" {
			return fmt.Errorf("%s.source_ref attrs.source_entity_id is required", path)
		}
		if _, ok := v.entities[sourceEntityID]; !ok {
			return fmt.Errorf("%s.source_ref source_entity_id %q does not resolve", path, sourceEntityID)
		}
		if displayMode := stringAttr(node, "display_mode"); displayMode != "numbered_ref" {
			return fmt.Errorf("%s.source_ref attrs.display_mode must be numbered_ref, got %q", path, displayMode)
		}
		if len(node.Content) != 0 || node.Text != "" || len(node.Marks) != 0 {
			return fmt.Errorf("%s.source_ref must be an atom leaf without marks", path)
		}
		v.refs[sourceEntityID] = true
	default:
		return fmt.Errorf("%s unsupported inline node type %q", path, node.Type)
	}
	return nil
}

func validateMarks(marks []Mark, path string) error {
	for i, mark := range marks {
		switch mark.Type {
		case "strong", "emphasis", "code":
		default:
			return fmt.Errorf("%s.marks[%d] unsupported mark type %q", path, i, mark.Type)
		}
	}
	return nil
}

var legacySourceSyntaxes = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "raw source token", re: regexp.MustCompile(`(?i)\{\{\s*source\s*:`)},
	{name: "markdown source link", re: regexp.MustCompile(`(?i)\[[^\]]+\]\(\s*source\s*:[^)]+\)`)},
	{name: "bracket source citation", re: regexp.MustCompile(`(?i)\[\s*source\s*:[^\]]+\]`)},
	{name: "prose source handle", re: regexp.MustCompile(`(?im)(^|\n)\s*source\s*:`)},
	{name: "unresolved numbered citation", re: regexp.MustCompile(`\[\d+\]`)},
}

func validateTextPayload(text, path string) error {
	for _, syntax := range legacySourceSyntaxes {
		if syntax.re.MatchString(text) {
			return fmt.Errorf("%s contains legacy %s syntax; use source_ref or source_embed nodes", path, syntax.name)
		}
	}
	return nil
}

func requireNodeID(node Node, path string) error {
	if nodeID(node) == "" {
		return fmt.Errorf("%s.%s attrs.id is required", path, node.Type)
	}
	return nil
}

func nodeID(node Node) string {
	return stringAttr(node, "id")
}

func stringAttr(node Node, key string) string {
	if node.Attrs == nil {
		return ""
	}
	if value, ok := node.Attrs[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func intAttr(node Node, key string) (int, bool) {
	if node.Attrs == nil {
		return 0, false
	}
	switch value := node.Attrs[key].(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		if value == float64(int(value)) {
			return int(value), true
		}
	}
	return 0, false
}

func validSourceTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "web_url", "url", "source_service_item", "content_item", "image", "video", "audio", "pdf",
		"transcript", "texture_span", "publication_span", "source_viewer_artifact",
		"reader_artifact", "file_artifact", "publication_version":
		return true
	default:
		return false
	}
}

func validSelectorKind(kind string) bool {
	normalized := sourcecontract.NormalizeSelectorKind(kind)
	switch normalized {
	case sourcecontract.SelectorKindWholeResource,
		sourcecontract.SelectorKindTextQuote,
		sourcecontract.SelectorKindTextPosition,
		sourcecontract.SelectorKindParagraphHeading,
		sourcecontract.SelectorKindPageRange,
		sourcecontract.SelectorKindTimestampRange,
		sourcecontract.SelectorKindTranscriptSegment,
		sourcecontract.SelectorKindTableRange,
		sourcecontract.SelectorKindTableCell,
		sourcecontract.SelectorKindImageRegion,
		sourcecontract.SelectorKindByteRange,
		sourcecontract.SelectorKindDataVintage,
		sourcecontract.SelectorKindSelectorSet:
		return true
	default:
		return false
	}
}

func validDisplayMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "numbered_ref", "inline_chip", "block_embed", "excerpt", "player", "image_preview",
		"pdf_pages", "transcript", "source_window":
		return true
	default:
		return false
	}
}

func validOpenSurface(openSurface string) bool {
	switch sourcecontract.NormalizeOpenSurface(openSurface) {
	case sourcecontract.OpenSurfaceSource,
		sourcecontract.OpenSurfaceWebLens,
		sourcecontract.OpenSurfaceTexture,
		sourcecontract.OpenSurfaceVideo,
		sourcecontract.OpenSurfaceImage,
		sourcecontract.OpenSurfaceAudio,
		sourcecontract.OpenSurfacePDF,
		sourcecontract.OpenSurfaceTranscript,
		sourcecontract.OpenSurfaceFile,
		sourcecontract.OpenSurfaceSourceWindow:
		return true
	default:
		return false
	}
}
