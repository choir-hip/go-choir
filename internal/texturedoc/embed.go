package texturedoc

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The embed block (docs/design/texture-evidence-embeds-te-2026-10-25.md
// section 1; Texture media design section 3). One block type with a typed
// target, so every renderer implements one switch over the kind. It is an
// additive node of choir.texture_doc.v1: a document with no embed is
// unchanged. Writers are strict (the validator refuses an unknown kind or
// attribute); renderers are tolerant (an unknown kind renders as a plain
// card), so a newer computer's document reads on an older renderer.
//
// Privacy is derived from the kind, never written: capsule_evidence is
// supervision material for the owner and is never published; image is
// private until public asset pinning exists (T2). Today no embed is
// publishable, and publication refuses a document that holds one.

const (
	// EmbedKindCapsuleEvidence is one screenshot of a frozen
	// self-development candidate: an asset-store digest bound to its
	// operation, naming the file the runtime saved it as
	// (Self-development/<operation>/screens/<name>.png) or, for an embed
	// written before the platform capture was removed, its route.
	EmbedKindCapsuleEvidence = "capsule_evidence"
	// EmbedKindImage is an owned image in the computer's asset store.
	EmbedKindImage = "image"
	// EmbedKindFile is a card for a file in the owner's Files (a report the
	// engineer saved): path, title and media type. The card opens the file in
	// the app that opens it (a PDF opens in Reader). The bytes stay in Files;
	// the card holds only the path.
	EmbedKindFile = "file"

	EmbedPrivacyPrivate = "private"
	EmbedPrivacyPublic  = "public"

	// MaxEmbedDimension bounds a viewport side, in CSS pixels.
	MaxEmbedDimension = 16384
	// MaxEmbedCaptionRunes and MaxEmbedAltRunes bound Texture's words.
	MaxEmbedCaptionRunes = 1000
	MaxEmbedAltRunes     = 500
	maxEmbedIdentRunes   = 200
	maxEmbedRouteRunes   = 512
)

// Embed is an embed block read into its fields.
type Embed struct {
	ID          string
	Kind        string
	Digest      string
	OperationID string
	ComputerID  string
	Route       string
	File        string
	Width       int
	Height      int
	Blank       bool
	Caption     string
	Alt         string
	// Path, Mime and Title are the file kind's fields.
	Path  string
	Mime  string
	Title string
}

// Privacy is derived from the kind: every kind is private today.
func (e Embed) Privacy() string {
	switch e.Kind {
	case EmbedKindCapsuleEvidence:
		// Evidence is never published (Texture media design section 3).
		return EmbedPrivacyPrivate
	default:
		// image: private until T2 pins public copies.
		return EmbedPrivacyPrivate
	}
}

// Publishable reports whether a publication may carry the embed. None may
// today.
func (e Embed) Publishable() bool { return e.Privacy() == EmbedPrivacyPublic }

// EmbedOf reads an embed node. It does not validate; the validator does.
func EmbedOf(n Node) (Embed, bool) {
	if n.Type != "embed" {
		return Embed{}, false
	}
	target, _ := n.Attrs["target"].(map[string]any)
	str := func(m map[string]any, k string) string {
		v, _ := m[k].(string)
		return strings.TrimSpace(v)
	}
	num := func(k string) int {
		switch v := target[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
		return 0
	}
	blank, _ := target["blank"].(bool)
	return Embed{
		ID: str(n.Attrs, "id"), Kind: str(n.Attrs, "kind"),
		Digest: str(target, "digest"), OperationID: str(target, "operation_id"), ComputerID: str(target, "computer_id"),
		Route: str(target, "route"), File: str(target, "file"), Width: num("width"), Height: num("height"), Blank: blank,
		Caption: str(n.Attrs, "caption"), Alt: str(n.Attrs, "alt"),
		Path: str(target, "path"), Mime: str(target, "mime"), Title: str(n.Attrs, "title"),
	}, true
}

// Embeds lists the document's embed blocks in order (they are top level
// only).
func Embeds(doc StructuredTextureDoc) []Embed {
	var out []Embed
	for _, n := range doc.Doc.Content {
		if e, ok := EmbedOf(n); ok {
			out = append(out, e)
		}
	}
	return out
}

// PrivateEmbeds lists the embeds a publication must refuse.
func PrivateEmbeds(doc StructuredTextureDoc) []Embed {
	var out []Embed
	for _, e := range Embeds(doc) {
		if !e.Publishable() {
			out = append(out, e)
		}
	}
	return out
}

// PrivateEmbedsInBody is PrivateEmbeds of a revision's body_doc JSON; a body
// that does not decode holds none here (the validator refuses it on its
// own).
func PrivateEmbedsInBody(raw json.RawMessage) []Embed {
	if len(raw) == 0 {
		return nil
	}
	var doc StructuredTextureDoc
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	return PrivateEmbeds(doc)
}

// EmbeddedDigests lists the distinct asset digests the document embeds, in
// first-seen order: what a revision pins.
func EmbeddedDigests(doc StructuredTextureDoc) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range Embeds(doc) {
		if e.Digest != "" && !seen[e.Digest] {
			seen[e.Digest] = true
			out = append(out, e.Digest)
		}
	}
	return out
}

// EmbedProjectionText is an embed's line in the text projection. The web
// editor's projection writes the same text.
func EmbedProjectionText(n Node) string {
	e, _ := EmbedOf(n)
	switch e.Kind {
	case EmbedKindCapsuleEvidence:
		return "[evidence: " + e.Caption + "]"
	case EmbedKindFile:
		return "[file: " + e.Title + "]"
	case EmbedKindImage:
		if e.Caption != "" {
			return "[image: " + e.Caption + "]"
		}
		return "[image: " + e.Alt + "]"
	}
	return "[embed]"
}

var (
	embedAttrKeys = map[string]bool{"id": true, "kind": true, "target": true, "caption": true, "alt": true, "title": true}
	// Reserved for the later kinds (Texture media design section 3).
	embedReservedKeys = map[string]bool{"selector": true, "poster": true, "transcript": true}
	embedTargetKeys   = map[string]map[string]bool{
		EmbedKindCapsuleEvidence: {"digest": true, "operation_id": true, "computer_id": true, "route": true, "file": true, "width": true, "height": true, "blank": true},
		EmbedKindImage:           {"digest": true, "width": true, "height": true},
		EmbedKindFile:            {"path": true, "mime": true},
	}
	topLevelBlockPath = regexp.MustCompile(`^doc\.content(\[\d+\])?$`)
	embedDigest       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// embedScreensRoot is where the runtime saves a candidate's screens in the
// computer's file tree.
const embedScreensRoot = "Self-development/"

// embedScreenFile says file is a relative path under embedScreensRoot with no
// climbing component.
func embedScreenFile(file string) bool {
	if !strings.HasPrefix(file, embedScreensRoot) || utf8.RuneCountInString(file) > maxEmbedRouteRunes || strings.ContainsAny(file, " \t\r\n\\") {
		return false
	}
	for _, part := range strings.Split(file, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func (v *Validator) validateEmbed(node Node, path string) error {
	if v.world || v.ownerText {
		return fmt.Errorf("%s.embed is not allowed in this document", path)
	}
	if !topLevelBlockPath.MatchString(path) {
		return fmt.Errorf("%s.embed must be a top-level block", path)
	}
	if err := requireNodeID(node, path); err != nil {
		return err
	}
	if len(node.Content) != 0 || node.Text != "" || len(node.Marks) != 0 {
		return fmt.Errorf("%s.embed must be a leaf without marks", path)
	}
	for key := range node.Attrs {
		switch {
		case key == "privacy":
			return fmt.Errorf("%s.embed attrs.privacy is derived from the kind, never written", path)
		case embedReservedKeys[key]:
			return fmt.Errorf("%s.embed attrs.%s is not supported yet", path, key)
		case !embedAttrKeys[key]:
			return fmt.Errorf("%s.embed attrs.%s is not an embed attribute", path, key)
		}
	}
	kind := stringAttr(node, "kind")
	allowed, known := embedTargetKeys[kind]
	if !known {
		return fmt.Errorf("%s.embed attrs.kind %q is not supported", path, kind)
	}
	target, ok := node.Attrs["target"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s.embed attrs.target must be an object", path)
	}
	for key := range target {
		if !allowed[key] {
			return fmt.Errorf("%s.embed target.%s is not a %s target field", path, key, kind)
		}
	}
	if kind != EmbedKindFile {
		if digest, _ := target["digest"].(string); !embedDigest.MatchString(digest) {
			return fmt.Errorf("%s.embed target.digest must be 64 lowercase hex characters", path)
		}
	}
	e, _ := EmbedOf(node)
	if _, hasTitle := node.Attrs["title"]; hasTitle && kind != EmbedKindFile {
		return fmt.Errorf("%s.embed attrs.title belongs to the file kind", path)
	}
	switch kind {
	case EmbedKindCapsuleEvidence:
		for field, value := range map[string]string{"operation_id": e.OperationID, "computer_id": e.ComputerID} {
			if !embedIdent(value) {
				return fmt.Errorf("%s.embed target.%s is required (no spaces, at most %d characters)", path, field, maxEmbedIdentRunes)
			}
		}
		_, hasRoute := target["route"]
		_, hasFile := target["file"]
		if !hasRoute && !hasFile {
			return fmt.Errorf("%s.embed target names neither a file nor a route", path)
		}
		if hasRoute && (!strings.HasPrefix(e.Route, "/") || strings.HasPrefix(e.Route, "//") || utf8.RuneCountInString(e.Route) > maxEmbedRouteRunes || strings.ContainsAny(e.Route, " \t\r\n")) {
			return fmt.Errorf("%s.embed target.route must be a path on this origin", path)
		}
		if hasFile && !embedScreenFile(e.File) {
			return fmt.Errorf("%s.embed target.file must be a screen file under %s", path, embedScreensRoot)
		}
		if err := embedDimensions(target, path, true); err != nil {
			return err
		}
		if raw, present := target["blank"]; present {
			if _, isBool := raw.(bool); !isBool {
				return fmt.Errorf("%s.embed target.blank must be true or false", path)
			}
		}
		if e.Caption == "" {
			return fmt.Errorf("%s.embed attrs.caption is required for capsule_evidence", path)
		}
	case EmbedKindImage:
		if err := embedDimensions(target, path, false); err != nil {
			return err
		}
	case EmbedKindFile:
		if err := ValidFilesPath(e.Path); err != nil {
			return fmt.Errorf("%s.embed target.path %v", path, err)
		}
		if mime := e.Mime; mime != "" && (utf8.RuneCountInString(mime) > 127 || !strings.Contains(mime, "/") || strings.ContainsAny(mime, " \t\r\n;")) {
			return fmt.Errorf("%s.embed target.mime must be a media type such as application/pdf", path)
		}
		if e.Title == "" {
			return fmt.Errorf("%s.embed attrs.title is required for file", path)
		}
		if utf8.RuneCountInString(e.Title) > MaxEmbedCaptionRunes {
			return fmt.Errorf("%s.embed attrs.title is longer than %d characters", path, MaxEmbedCaptionRunes)
		}
		if err := v.validateText(e.Title, path+".embed attrs.title"); err != nil {
			return err
		}
	}
	if kind != EmbedKindFile && e.Alt == "" {
		return fmt.Errorf("%s.embed attrs.alt is required", path)
	}
	for field, limit := range map[string]int{"caption": MaxEmbedCaptionRunes, "alt": MaxEmbedAltRunes} {
		raw, present := node.Attrs[field]
		if !present {
			continue
		}
		text, isString := raw.(string)
		if !isString {
			return fmt.Errorf("%s.embed attrs.%s must be text", path, field)
		}
		if utf8.RuneCountInString(text) > limit {
			return fmt.Errorf("%s.embed attrs.%s is longer than %d characters", path, field, limit)
		}
		if err := v.validateText(text, path+".embed attrs."+field); err != nil {
			return err
		}
	}
	return nil
}

// embedDimensions checks width and height: both required, or (optional)
// both or neither.
func embedDimensions(target map[string]any, path string, required bool) error {
	_, hasW := target["width"]
	_, hasH := target["height"]
	if !hasW && !hasH && !required {
		return nil
	}
	if hasW != hasH || !hasW {
		return fmt.Errorf("%s.embed target.width and target.height are required together", path)
	}
	for _, key := range []string{"width", "height"} {
		n, ok := target[key].(float64)
		if !ok {
			if i, isInt := target[key].(int); isInt {
				n, ok = float64(i), true
			}
		}
		if !ok || n != float64(int(n)) || n < 1 || n > MaxEmbedDimension {
			return fmt.Errorf("%s.embed target.%s must be a whole number from 1 to %d", path, key, MaxEmbedDimension)
		}
	}
	return nil
}

func embedIdent(s string) bool {
	return s != "" && utf8.RuneCountInString(s) <= maxEmbedIdentRunes && !strings.ContainsAny(s, " \t\r\n/")
}

// ValidFilesPath says p is a path inside the owner's Files: relative, no
// climbing or empty component, no backslash, control byte or blank name.
// The file card's validator and Write's {{file}} token share it.
func ValidFilesPath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("is required (a Files path such as Reports/summary.pdf)")
	case utf8.RuneCountInString(p) > maxEmbedRouteRunes:
		return fmt.Errorf("is longer than %d characters", maxEmbedRouteRunes)
	case strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/"):
		return fmt.Errorf("must be relative to Files, without a leading or trailing /")
	case strings.Contains(p, "\\"):
		return fmt.Errorf("must use / separators")
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("must not hold control characters")
		}
	}
	for _, part := range strings.Split(p, "/") {
		switch {
		case part == "" || part == "." || part == "..":
			return fmt.Errorf("must stay inside Files: no empty, . or .. component")
		case strings.TrimSpace(part) == "":
			return fmt.Errorf("must not hold a blank component")
		}
	}
	return nil
}
