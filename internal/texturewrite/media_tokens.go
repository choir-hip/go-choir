package texturewrite

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

// The media tokens (station L8, docs/definitions/choir-ordinary-prompt-path-
// 2026-11-09.md): a block of its own, not inline text.
//
//	{{image <digest-or-Files-path> "caption" "alt"}}   an image block (embed kind image)
//	{{file <Files-path> "title"}}                      a file card (embed kind file)
//
// An image is named by its sha256 digest in the computer's asset store, or by
// a PNG, JPEG or GIF the engineer saved in Files (Reports/chart.png): the
// runtime reads the file, stores it in the asset store and writes the digest.
// A file card names a file in Files; it opens in the app that opens it (a PDF
// in Reader). The embed block is top level only, so the token stands alone on
// its line; inline use is refused.

// Token kinds of the media tokens.
const (
	TokenImage = "image"
	TokenFile  = "file"
)

// EmbedSpec is a parsed media token. Digest, Width, Height and Mime are
// filled when Apply resolves it.
type EmbedSpec struct {
	Kind    string // TokenImage or TokenFile
	Ref     string // a digest or a Files path as written
	Caption string
	Alt     string
	Title   string
	Line    int

	Digest string
	Width  int
	Height int
	Mime   string
}

// Media is what Write needs from the computer to place an image or a file:
// the asset store and the owner's Files. A nil Media accepts a digest as
// written and refuses a Files path.
type Media interface {
	// HasImage reports whether the asset store holds the digest.
	HasImage(digest string) bool
	// ImportImage reads the Files path, checks it is an image, stores it in
	// the asset store and returns its digest and size (0 when unknown).
	ImportImage(ctx context.Context, filesPath string) (digest string, width, height int, err error)
	// FileInfo checks the Files path names a regular file and returns its
	// media type.
	FileInfo(ctx context.Context, filesPath string) (mime string, err error)
}

var (
	mediaTokenRE = regexp.MustCompile(`(?i)^\{\{(image|file)(?:[ \t}]|$)`)
	digestRefRE  = regexp.MustCompile(`^(?:artifact:)?(?:sha256:)?([0-9a-f]{64})$`)
)

const mediaUsage = `a media token is {{image <digest or Files path> "caption" "alt"}} or {{file <Files path> "title"}}, alone on its line`

// parseMediaLine reads a line that is exactly one media token. ok is false
// when the line does not start with one.
func parseMediaLine(trimmed string, line int, errs *Errors) (*EmbedSpec, bool) {
	m := mediaTokenRE.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, false
	}
	kind := strings.ToLower(m[1])
	rest := strings.TrimLeft(trimmed[2+len(m[1]):], " \t")
	refEnd := strings.IndexAny(rest, " \t}\"“")
	if refEnd <= 0 {
		errs.add(line, "{{%s}} needs a %s; %s", kind, refName(kind), mediaUsage)
		return nil, true
	}
	spec := &EmbedSpec{Kind: kind, Ref: rest[:refEnd], Line: line}
	rest = strings.TrimLeft(rest[refEnd:], " \t")
	var quotes []string
	for strings.HasPrefix(rest, `"`) || strings.HasPrefix(rest, "“") {
		q, after, ok := readQuote(rest)
		if !ok {
			errs.add(line, "unterminated quote in {{%s %s; close the quote with \" before }}", kind, spec.Ref)
			return nil, true
		}
		quotes = append(quotes, strings.TrimSpace(q))
		rest = strings.TrimLeft(after, " \t")
	}
	if !strings.HasPrefix(rest, "}}") {
		errs.add(line, "{{%s %s token must end with }} on the same line; %s", kind, spec.Ref, mediaUsage)
		return nil, true
	}
	if tail := strings.TrimSpace(rest[2:]); tail != "" {
		errs.add(line, "{{%s}} is a block of its own: put %q on a different line, not beside the token", kind, tail)
		return nil, true
	}
	switch kind {
	case TokenImage:
		if len(quotes) > 2 {
			errs.add(line, `{{image}} takes at most a caption and an alt text: {{image <ref> "caption" "alt"}}`)
			return nil, true
		}
		switch len(quotes) {
		case 1:
			spec.Caption, spec.Alt = quotes[0], quotes[0]
		case 2:
			spec.Caption, spec.Alt = quotes[0], quotes[1]
		}
		if spec.Alt == "" {
			spec.Alt = spec.Caption
		}
		if spec.Alt == "" {
			errs.add(line, `{{image %s}} needs alt text (what the image shows): {{image %s "caption" "alt"}}`, spec.Ref, spec.Ref)
			return nil, true
		}
		if !digestRefRE.MatchString(spec.Ref) {
			if err := texturedoc.ValidFilesPath(spec.Ref); err != nil {
				errs.add(line, "{{image %s}}: %s is neither a sha256 digest nor a Files path; a Files path %v", spec.Ref, spec.Ref, err)
				return nil, true
			}
		}
	case TokenFile:
		if len(quotes) > 1 {
			errs.add(line, `{{file}} takes one title: {{file <Files path> "title"}}`)
			return nil, true
		}
		if len(quotes) == 1 {
			spec.Title = quotes[0]
		}
		if spec.Title == "" {
			spec.Title = path.Base(spec.Ref)
		}
		if err := texturedoc.ValidFilesPath(spec.Ref); err != nil {
			errs.add(line, "{{file %s}}: the Files path %v", spec.Ref, err)
			return nil, true
		}
	}
	return spec, true
}

func refName(kind string) string {
	if kind == TokenImage {
		return "digest or Files path"
	}
	return "Files path"
}

// mediaTokenInline is the refusal for an image or file token inside text.
func mediaTokenInline(kind string, line int) *LineError {
	return &LineError{Line: line, Msg: "{{" + kind + "}} is a block of its own, not inline text: put it alone on its own line, with a blank line or another block around it"}
}

// text is the spec in the Write grammar: what the frame shows for the block
// and what the reconcile compares, so an @all that copies the line keeps it.
func (s EmbedSpec) text() string {
	ref := s.Ref
	if s.Digest != "" {
		ref = s.Digest
	}
	if s.Kind == TokenFile {
		return "{{file " + ref + " " + quoteToken(s.Title) + "}}"
	}
	return "{{image " + ref + " " + quoteToken(s.Caption) + " " + quoteToken(s.Alt) + "}}"
}

// resolveEmbeds fills the digest of every image spec and checks every file
// spec, in the ops' bodies. A refusal names the token's line.
func resolveEmbeds(ctx context.Context, ops []Op, media Media, errs *Errors) {
	for i := range ops {
		for j := range ops[i].Body {
			spec := ops[i].Body[j].Embed
			if spec == nil {
				continue
			}
			switch spec.Kind {
			case TokenImage:
				if m := digestRefRE.FindStringSubmatch(spec.Ref); m != nil {
					spec.Digest = m[1]
					if media != nil && !media.HasImage(spec.Digest) {
						errs.add(spec.Line, "{{image %s}}: no image with that digest is in this computer's asset store; save the picture into Files (Reports/chart.png) and name its path", spec.Ref[:min(len(spec.Ref), 20)])
					}
					continue
				}
				if media == nil {
					errs.add(spec.Line, "{{image %s}}: Files are not reachable from this Write; name the image by its sha256 digest", spec.Ref)
					continue
				}
				digest, w, h, err := media.ImportImage(ctx, spec.Ref)
				if err != nil {
					errs.add(spec.Line, "{{image %s}}: %v", spec.Ref, err)
					continue
				}
				spec.Digest, spec.Width, spec.Height = digest, w, h
			case TokenFile:
				if media == nil {
					errs.add(spec.Line, "{{file %s}}: Files are not reachable from this Write", spec.Ref)
					continue
				}
				mime, err := media.FileInfo(ctx, spec.Ref)
				if err != nil {
					errs.add(spec.Line, "{{file %s}}: %v", spec.Ref, err)
					continue
				}
				spec.Mime = mime
			}
		}
	}
}

// embedNode builds the embed block for a resolved spec.
func (w *writer) embedNode(spec *EmbedSpec, id string) texturedoc.Node {
	attrs := map[string]any{"id": w.newID("embed", id)}
	target := map[string]any{}
	if spec.Kind == TokenFile {
		attrs["kind"] = texturedoc.EmbedKindFile
		attrs["title"] = spec.Title
		target["path"] = spec.Ref
		if spec.Mime != "" {
			target["mime"] = spec.Mime
		}
	} else {
		attrs["kind"] = texturedoc.EmbedKindImage
		attrs["alt"] = spec.Alt
		if spec.Caption != "" {
			attrs["caption"] = spec.Caption
		}
		target["digest"] = spec.Digest
		if spec.Width > 0 && spec.Height > 0 {
			target["width"], target["height"] = spec.Width, spec.Height
		}
	}
	attrs["target"] = target
	return texturedoc.Node{Type: "embed", Attrs: attrs}
}

// embedFrameText is a stored embed's line in the frame. Image and file
// blocks the desk wrote are tokens it can write back; evidence is read-only.
func embedFrameText(e texturedoc.Embed) (string, bool) {
	switch e.Kind {
	case texturedoc.EmbedKindImage:
		return EmbedSpec{Kind: TokenImage, Digest: e.Digest, Caption: e.Caption, Alt: e.Alt}.text(), true
	case texturedoc.EmbedKindFile:
		return EmbedSpec{Kind: TokenFile, Ref: e.Path, Title: e.Title}.text(), true
	}
	return "", false
}
