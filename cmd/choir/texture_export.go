package main

// `choir texture export` and the texture_export MCP tool: download a Texture
// document as Markdown with every attachment it embeds, so a coding agent
// driving a Choir computer can show the computer's screenshots in its own
// chat (owner, 2026-11-03). It reads only the existing routes:
//
//	GET /api/texture/documents/{doc}          the head revision id and title
//	GET /api/texture/documents/{doc}/history  the revision chain (newest first)
//	GET /api/texture/revisions/{rev}          the body_doc and citations
//	GET /api/assets/sha256/{digest}           an embed's bytes by digest
//	GET /api/files/{path}                     a screen saved as a file
//
// An embed names its bytes by target.digest (the asset store) and, for a
// screen the runtime saved into the computer's tree, target.file; the asset
// store is read first and the file when the store does not hold it. Bytes
// that do not hash to the declared digest are refused: never written, and
// the export fails.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	textureExportSchema      = "choir.texture_export.v1"
	textureExportIndexSchema = "choir.texture_export_index.v1"
	// maxAttachmentBytes bounds one attachment read into memory.
	maxAttachmentBytes = 64 << 20

	// MCP inline bounds: an image block is base64 in one JSON-RPC line.
	mcpDefaultImages  = 4
	mcpMaxImages      = 10
	mcpMaxImageBytes  = 1 << 20 // per image, raw bytes
	mcpMaxInlineBytes = 3 << 20 // per call, raw bytes
)

const exportAssetPrefix = "/api/assets/sha256/"

var (
	exportDigest   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	exportSlugRuns = regexp.MustCompile(`[^a-z0-9]+`)
)

type textureExportAttachment struct {
	BlockID        string `json:"block_id"`
	Kind           string `json:"kind"`
	SourceRef      string `json:"source_ref"`
	SourceURL      string `json:"source_url,omitempty"`
	DeclaredSHA256 string `json:"declared_sha256,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Verified       bool   `json:"verified"`
	MediaType      string `json:"media_type,omitempty"`
	Bytes          int64  `json:"bytes"`
	Path           string `json:"path,omitempty"` // relative to the revision directory
	Caption        string `json:"caption,omitempty"`
	Alt            string `json:"alt,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	Error          string `json:"error,omitempty"`

	data  []byte
	links []string // server routes the document text may name
}

type textureExportManifest struct {
	Schema           string                    `json:"schema"`
	DocID            string                    `json:"doc_id"`
	Title            string                    `json:"title,omitempty"`
	RevisionID       string                    `json:"revision_id"`
	VersionNumber    int                       `json:"version_number,omitempty"`
	ParentRevisionID string                    `json:"parent_revision_id,omitempty"`
	CreatedAt        string                    `json:"created_at,omitempty"`
	Current          bool                      `json:"current"`
	Document         string                    `json:"document"`
	Attachments      []textureExportAttachment `json:"attachments"`

	markdown string
	dir      string // the revision's directory under out, "" for out itself
}

type textureExportIndexEntry struct {
	Dir           string `json:"dir"`
	RevisionID    string `json:"revision_id"`
	VersionNumber int    `json:"version_number,omitempty"`
	Attachments   int    `json:"attachments"`
}

type textureExportIndex struct {
	Schema            string                    `json:"schema"`
	DocID             string                    `json:"doc_id"`
	Title             string                    `json:"title,omitempty"`
	CurrentRevisionID string                    `json:"current_revision_id"`
	Revisions         []textureExportIndexEntry `json:"revisions"`
}

// textureExportResult is what the handler returns; the CLI prints only
// Summary, MCP renders the rest.
type textureExportResult struct {
	Out       string
	Revisions []*textureExportManifest // oldest first
}

type textureExportSummary struct {
	Out         string `json:"out"`
	Attachments int    `json:"attachments"`
}

func (r *textureExportResult) summary() textureExportSummary {
	n := 0
	for _, m := range r.Revisions {
		n += len(m.Attachments)
	}
	return textureExportSummary{Out: r.Out, Attachments: n}
}

type exportRevision struct {
	RevisionID       string          `json:"revision_id"`
	DocID            string          `json:"doc_id"`
	VersionNumber    int             `json:"version_number"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc"`
	Citations        json.RawMessage `json:"citations"`
	ParentRevisionID string          `json:"parent_revision_id"`
	CreatedAt        string          `json:"created_at"`
}

type exportBlob struct {
	data      []byte
	mediaType string
	err       error
}

type textureExporter struct {
	c     *client
	docID string
	cache map[string]exportBlob // route -> bytes
}

func textureExport(x *call) (any, error) {
	docID := x.trim("doc_id")
	out := x.trim("out")
	revision, since, all := x.trim("revision"), x.trim("since"), x.flag("all_revisions")
	modes := 0
	for _, set := range []bool{revision != "", since != "", all} {
		if set {
			modes++
		}
	}
	if modes > 1 {
		return nil, usagef("--revision, --since and --all-revisions are exclusive")
	}
	if x.surface == "cli" && out == "" {
		return nil, usagef("--out DIR is required")
	}
	if x.surface == "mcp" {
		if out != "" && !filepath.IsAbs(out) {
			return nil, usagef("out must be an absolute directory on the machine running choir mcp")
		}
		if (since != "" || all) && out == "" {
			return nil, usagef("since and all_revisions write one directory per revision; give out")
		}
	}

	var document struct {
		Title             string `json:"title"`
		CurrentRevisionID string `json:"current_revision_id"`
	}
	if err := x.c.do(http.MethodGet, "/api/texture/documents/"+esc(docID), nil, &document); err != nil {
		return nil, err
	}
	head := strings.TrimSpace(document.CurrentRevisionID)
	var ids []string
	switch {
	case revision != "":
		ids = []string{revision}
	case since != "" || all:
		chain, err := exportHistory(x.c, docID)
		if err != nil {
			return nil, err
		}
		if since != "" {
			at := -1
			for i, id := range chain {
				if id == since {
					at = i
				}
			}
			if at < 0 {
				return nil, fmt.Errorf("revision %s is not in this document's history", since)
			}
			chain = chain[at+1:]
		}
		ids = chain
	default:
		if head == "" {
			return nil, errors.New("document has no current revision")
		}
		ids = []string{head}
	}

	ex := &textureExporter{c: x.c, docID: docID, cache: map[string]exportBlob{}}
	result := &textureExportResult{Out: out}
	perRevisionDirs := since != "" || all
	for _, id := range ids {
		m, err := ex.revision(id, document.Title, head)
		if err != nil {
			return nil, err
		}
		if perRevisionDirs {
			m.dir = exportRevisionDir(m)
		}
		result.Revisions = append(result.Revisions, m)
	}
	if out != "" {
		if err := writeTextureExport(out, docID, document.Title, head, result, perRevisionDirs); err != nil {
			return nil, err
		}
	}
	var refused []string
	for _, m := range result.Revisions {
		for _, a := range m.Attachments {
			if a.Error != "" {
				refused = append(refused, fmt.Sprintf("%s %s: %s", m.RevisionID, a.BlockID, a.Error))
			}
		}
	}
	if len(refused) > 0 {
		where := ""
		if out != "" {
			where = " (the rest is written to " + out + ")"
		}
		return nil, fmt.Errorf("%d attachment(s) not exported%s: %s", len(refused), where, strings.Join(refused, "; "))
	}
	return result, nil
}

// exportHistory returns the document's revision ids, oldest first.
func exportHistory(c *client, docID string) ([]string, error) {
	var history struct {
		Entries []struct {
			RevisionID string `json:"revision_id"`
		} `json:"entries"`
	}
	if err := c.do(http.MethodGet, "/api/texture/documents/"+esc(docID)+"/history", nil, &history); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(history.Entries))
	for i := len(history.Entries) - 1; i >= 0; i-- {
		if id := strings.TrimSpace(history.Entries[i].RevisionID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func exportRevisionDir(m *textureExportManifest) string {
	slug := strings.Trim(exportSlugRuns.ReplaceAllString(strings.ToLower(m.RevisionID), "-"), "-")
	return fmt.Sprintf("v%04d-%s", m.VersionNumber, slug)
}

func (ex *textureExporter) revision(id, title, head string) (*textureExportManifest, error) {
	var rev exportRevision
	if err := ex.c.do(http.MethodGet, "/api/texture/revisions/"+esc(id), nil, &rev); err != nil {
		return nil, fmt.Errorf("read revision %s: %w", id, err)
	}
	if strings.TrimSpace(rev.DocID) != ex.docID {
		return nil, fmt.Errorf("revision %s is not a version of this document", id)
	}
	m := &textureExportManifest{
		Schema: textureExportSchema, DocID: ex.docID, Title: title, RevisionID: id, VersionNumber: rev.VersionNumber,
		ParentRevisionID: rev.ParentRevisionID, CreatedAt: rev.CreatedAt, Current: id == head, Document: "doc.md",
		Attachments: []textureExportAttachment{},
	}
	var body exportNode
	hasBody := len(rev.BodyDoc) > 0 && string(rev.BodyDoc) != "null"
	if hasBody {
		var doc struct {
			Doc exportNode `json:"doc"`
		}
		if json.Unmarshal(rev.BodyDoc, &doc) != nil {
			hasBody = false
		} else {
			body = doc.Doc
		}
	}
	sources := exportSources(rev.Citations)
	byBlock := map[string]int{}
	if hasBody {
		body.walk(func(n exportNode) {
			if n.Type != "embed" {
				return
			}
			a := ex.embedAttachment(n)
			byBlock[n.id()] = len(m.Attachments)
			m.Attachments = append(m.Attachments, a)
		})
	}
	for i := range sources {
		if a, ok := ex.sourceAttachment(sources[i]); ok {
			sources[i].attachment = len(m.Attachments)
			m.Attachments = append(m.Attachments, a)
		}
	}
	assignAttachmentPaths(m.Attachments)

	r := &exportRenderer{attachments: m.Attachments, byBlock: byBlock, sources: sources, numbers: map[string]int{}}
	var md strings.Builder
	if title != "" {
		md.WriteString("# " + exportLine(title) + "\n\n")
	}
	fmt.Fprintf(&md, "<!-- choir texture %s revision %s", ex.docID, id)
	if rev.VersionNumber > 0 {
		fmt.Fprintf(&md, " v%d", rev.VersionNumber)
	}
	md.WriteString(" -->\n\n")
	if hasBody {
		md.WriteString(r.blocks(body.Content, ""))
	} else {
		md.WriteString(strings.TrimSpace(rev.Content))
	}
	md.WriteString(r.sourceList())
	text := strings.TrimRight(md.String(), "\n") + "\n"
	// A server route the text names (a pasted asset or file link) points at
	// the local copy instead.
	for _, a := range m.Attachments {
		if a.Path == "" {
			continue
		}
		for _, link := range a.links {
			text = strings.ReplaceAll(text, link, a.Path)
		}
	}
	m.markdown = text
	return m, nil
}

// embedAttachment resolves one embed block to its bytes.
func (ex *textureExporter) embedAttachment(n exportNode) textureExportAttachment {
	target, _ := n.Attrs["target"].(map[string]any)
	str := func(m map[string]any, k string) string { v, _ := m[k].(string); return strings.TrimSpace(v) }
	num := func(k string) int { v, _ := target[k].(float64); return int(v) }
	a := textureExportAttachment{
		BlockID: n.id(), Kind: str(n.Attrs, "kind"), Caption: str(n.Attrs, "caption"), Alt: str(n.Attrs, "alt"),
		Width: num("width"), Height: num("height"),
	}
	digest := strings.ToLower(str(target, "digest"))
	if digest == "" {
		digest = strings.ToLower(strings.TrimPrefix(str(target, "sha256"), "sha256:"))
	}
	file := str(target, "file")
	if file == "" {
		file = str(target, "path")
	}
	var routes []string
	if digest != "" {
		if !exportDigest.MatchString(digest) {
			a.SourceRef, a.Error = "asset:sha256:"+digest, "declared digest is not a lowercase sha256"
			return a
		}
		a.DeclaredSHA256, a.SourceRef = digest, "asset:sha256:"+digest
		routes = append(routes, exportAssetPrefix+digest)
	}
	if file != "" {
		route, err := filesPath(file)
		if err != nil {
			a.Error = "file " + file + ": " + err.Error()
			return a
		}
		// A screen the runtime saved: the file is its source, the digest its
		// declared hash.
		a.SourceRef = "file:" + file
		routes = append(routes, route)
	}
	if len(routes) == 0 {
		a.SourceRef, a.Error = "embed:"+a.BlockID, "the embed names no digest and no file"
		return a
	}
	ex.fetch(&a, routes)
	return a
}

// sourceAttachment fetches a cited source whose target is a blob on this
// computer (an asset or a file route); other sources stay links.
func (ex *textureExporter) sourceAttachment(s exportSource) (textureExportAttachment, bool) {
	uri := s.URI
	if !strings.HasPrefix(uri, exportAssetPrefix) && !strings.HasPrefix(uri, "/api/files/") {
		return textureExportAttachment{}, false
	}
	a := textureExportAttachment{BlockID: s.ID, Kind: "source:" + s.Kind, SourceRef: uri, Caption: s.Title}
	if d := strings.TrimPrefix(uri, exportAssetPrefix); d != uri {
		if !exportDigest.MatchString(d) {
			a.Error = "source digest is not a lowercase sha256"
			return a, true
		}
		a.DeclaredSHA256, a.SourceRef = d, "asset:sha256:"+d
	}
	ex.fetch(&a, []string{uri})
	return a, true
}

// fetch reads the first route that holds the bytes and verifies them
// against the declared digest. A mismatch is refused, not retried elsewhere.
func (ex *textureExporter) fetch(a *textureExportAttachment, routes []string) {
	var lastErr error
	for _, route := range routes {
		blob, ok := ex.cache[route]
		if !ok {
			blob.data, blob.mediaType, blob.err = ex.c.getBlob(route, maxAttachmentBytes)
			ex.cache[route] = blob
		}
		a.links = append(a.links, route)
		if blob.err != nil {
			lastErr = blob.err
			var api *apiErrorResp
			if errors.As(blob.err, &api) && api.Status == http.StatusNotFound {
				continue
			}
			break
		}
		sum := sha256.Sum256(blob.data)
		a.SHA256, a.Bytes, a.SourceURL = hex.EncodeToString(sum[:]), int64(len(blob.data)), route
		a.MediaType = exportMediaType(blob.mediaType, blob.data, route)
		if a.DeclaredSHA256 != "" && a.SHA256 != a.DeclaredSHA256 {
			a.Error = fmt.Sprintf("sha256 mismatch: declared %s, received bytes hash to %s (%d bytes); not written", a.DeclaredSHA256, a.SHA256, len(blob.data))
			return
		}
		a.Verified = a.DeclaredSHA256 != ""
		a.data = blob.data
		lastErr = nil
		return
	}
	if lastErr != nil {
		a.Error = "fetch: " + lastErr.Error()
	}
}

// getBlob GETs raw bytes, bounded, with the response's media type.
func (c *client) getBlob(route string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, c.host+route, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.computerID != "" {
		req.Header.Set("X-Choir-Computer", c.computerID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request GET %s: %w", route, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &apiErrorResp{Status: resp.StatusCode, Body: string(body)}
	}
	if int64(len(body)) > limit {
		return nil, "", fmt.Errorf("attachment is larger than %d bytes", limit)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func exportMediaType(declared string, data []byte, route string) string {
	mt, _, _ := mime.ParseMediaType(declared)
	if mt == "" || mt == "application/octet-stream" {
		sniffed, _, _ := mime.ParseMediaType(http.DetectContentType(data))
		if sniffed != "" && sniffed != "application/octet-stream" && sniffed != "text/plain" {
			return sniffed
		}
		if byExt := mime.TypeByExtension(path.Ext(route)); byExt != "" {
			byExtType, _, _ := mime.ParseMediaType(byExt)
			return byExtType
		}
		if sniffed != "" {
			return sniffed
		}
		return "application/octet-stream"
	}
	return mt
}

var exportExtensions = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg",
	"application/pdf": ".pdf", "audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/ogg": ".ogg", "video/mp4": ".mp4",
	"video/webm": ".webm", "text/plain": ".txt", "application/json": ".json",
}

// assignAttachmentPaths names each verified attachment's file:
// attachments/<sha12>-<stem><ext>. One digest is one file.
func assignAttachmentPaths(atts []textureExportAttachment) {
	bySHA := map[string]string{}
	for i := range atts {
		a := &atts[i]
		if a.Error != "" || a.data == nil {
			continue
		}
		if p, ok := bySHA[a.SHA256]; ok {
			a.Path = p
			continue
		}
		stem := a.BlockID
		if f := strings.TrimPrefix(a.SourceRef, "file:"); f != a.SourceRef {
			stem = strings.TrimSuffix(path.Base(f), path.Ext(f))
		}
		stem = strings.Trim(exportSlugRuns.ReplaceAllString(strings.ToLower(stem), "-"), "-")
		if len(stem) > 48 {
			stem = strings.Trim(stem[:48], "-")
		}
		ext := exportExtensions[a.MediaType]
		if ext == "" {
			ext = ".bin"
		}
		name := a.SHA256[:12]
		if stem != "" {
			name += "-" + stem
		}
		a.Path = "attachments/" + name + ext
		bySHA[a.SHA256] = a.Path
	}
}

func writeTextureExport(out, docID, title, head string, result *textureExportResult, perRevision bool) error {
	if err := os.MkdirAll(out, 0o700); err != nil {
		return err
	}
	for _, m := range result.Revisions {
		dir := filepath.Join(out, m.dir)
		if err := os.MkdirAll(filepath.Join(dir, "attachments"), 0o700); err != nil {
			return err
		}
		written := map[string]bool{}
		for _, a := range m.Attachments {
			if a.Path == "" || written[a.Path] {
				continue
			}
			written[a.Path] = true
			if err := writeFileAtomic(filepath.Join(dir, filepath.FromSlash(a.Path)), a.data); err != nil {
				return err
			}
		}
		if err := writeFileAtomic(filepath.Join(dir, "doc.md"), []byte(m.markdown)); err != nil {
			return err
		}
		if err := writeJSONFile(filepath.Join(dir, "manifest.json"), m); err != nil {
			return err
		}
	}
	if !perRevision {
		return nil
	}
	// The index accumulates across incremental runs into the same out.
	index := textureExportIndex{Schema: textureExportIndexSchema, DocID: docID}
	if raw, err := os.ReadFile(filepath.Join(out, "index.json")); err == nil {
		var prior textureExportIndex
		if json.Unmarshal(raw, &prior) == nil && prior.DocID == docID {
			index.Revisions = prior.Revisions
		}
	}
	index.Title, index.CurrentRevisionID = title, head
	seen := map[string]int{}
	for i, e := range index.Revisions {
		seen[e.RevisionID] = i
	}
	for _, m := range result.Revisions {
		e := textureExportIndexEntry{Dir: m.dir, RevisionID: m.RevisionID, VersionNumber: m.VersionNumber, Attachments: len(m.Attachments)}
		if i, ok := seen[m.RevisionID]; ok {
			index.Revisions[i] = e
		} else {
			index.Revisions = append(index.Revisions, e)
		}
	}
	if index.Revisions == nil {
		index.Revisions = []textureExportIndexEntry{}
	}
	return writeJSONFile(filepath.Join(out, "index.json"), index)
}

func writeJSONFile(file string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(file, append(raw, '\n'))
}

// writeFileAtomic writes under a temporary name and renames, so a failed
// write never leaves a truncated file under the final name.
func writeFileAtomic(file string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), ".export-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// ---- Markdown ----

type exportNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []exportNode   `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []struct {
		Type  string         `json:"type"`
		Attrs map[string]any `json:"attrs,omitempty"`
	} `json:"marks,omitempty"`
}

func (n exportNode) id() string           { v, _ := n.Attrs["id"].(string); return strings.TrimSpace(v) }
func (n exportNode) attr(k string) string { v, _ := n.Attrs[k].(string); return strings.TrimSpace(v) }

func (n exportNode) walk(f func(exportNode)) {
	f(n)
	for _, c := range n.Content {
		c.walk(f)
	}
}

type exportSource struct {
	ID, Kind, URI, Title string
	attachment           int // index into attachments, -1 when a link
}

func exportSources(raw json.RawMessage) []exportSource {
	var entities []struct {
		SourceEntityID string `json:"source_entity_id"`
		Target         struct {
			Kind string `json:"kind"`
			URI  string `json:"uri"`
		} `json:"target"`
		Display struct {
			Title string `json:"title"`
			Label string `json:"label"`
		} `json:"display"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &entities) != nil {
		return nil
	}
	out := make([]exportSource, 0, len(entities))
	for _, e := range entities {
		title := e.Display.Title
		if title == "" {
			title = e.Display.Label
		}
		out = append(out, exportSource{ID: e.SourceEntityID, Kind: e.Target.Kind, URI: strings.TrimSpace(e.Target.URI), Title: title, attachment: -1})
	}
	return out
}

type exportRenderer struct {
	attachments []textureExportAttachment
	byBlock     map[string]int
	sources     []exportSource
	numbers     map[string]int
	order       []string
}

func (r *exportRenderer) blocks(nodes []exportNode, indent string) string {
	var parts []string
	for _, n := range nodes {
		if s := r.block(n, indent); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (r *exportRenderer) block(n exportNode, indent string) string {
	switch n.Type {
	case "paragraph":
		return indent + r.inline(n.Content)
	case "heading":
		level, _ := n.Attrs["level"].(float64)
		if level < 1 || level > 6 {
			level = 2
		}
		return indent + strings.Repeat("#", int(level)) + " " + r.inline(n.Content)
	case "bullet_list", "ordered_list":
		start := 1
		if v, ok := n.Attrs["start"].(float64); ok && v >= 0 {
			start = int(v)
		}
		var lines []string
		for i, item := range n.Content {
			marker := "- "
			if n.Type == "ordered_list" {
				marker = fmt.Sprintf("%d. ", start+i)
			}
			body := r.blocks(item.Content, "")
			body = strings.ReplaceAll(body, "\n", "\n"+indent+strings.Repeat(" ", len(marker)))
			lines = append(lines, indent+marker+body)
		}
		return strings.Join(lines, "\n")
	case "list_item":
		return r.blocks(n.Content, indent)
	case "blockquote":
		inner := r.blocks(n.Content, "")
		return indent + "> " + strings.ReplaceAll(inner, "\n", "\n"+indent+"> ")
	case "code_block":
		var code []string
		for _, c := range n.Content {
			code = append(code, c.Text)
		}
		return indent + "```" + n.attr("language") + "\n" + strings.Join(code, "\n") + "\n```"
	case "horizontal_rule":
		return indent + "---"
	case "embed":
		return indent + r.embed(n)
	case "text":
		return indent + n.Text
	}
	if len(n.Content) > 0 {
		return r.blocks(n.Content, indent)
	}
	return ""
}

func (r *exportRenderer) embed(n exportNode) string {
	i, ok := r.byBlock[n.id()]
	if !ok {
		return "[embed]"
	}
	a := r.attachments[i]
	alt := a.Alt
	if alt == "" {
		alt = a.Caption
	}
	alt = strings.NewReplacer("[", "(", "]", ")", "\n", " ").Replace(alt)
	var s string
	if a.Path != "" {
		if strings.HasPrefix(a.MediaType, "image/") {
			s = "![" + alt + "](" + a.Path + ")"
		} else {
			s = "[" + alt + "](" + a.Path + ")"
		}
	} else {
		s = "[" + alt + "] (not exported: " + exportLine(a.Error) + ")"
	}
	if a.Caption != "" {
		s += "\n\n*" + exportLine(a.Caption) + "*"
	}
	return s
}

func (r *exportRenderer) inline(nodes []exportNode) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			t := n.Text
			for _, m := range n.Marks {
				switch m.Type {
				case "strong":
					t = "**" + t + "**"
				case "emphasis":
					t = "*" + t + "*"
				case "code":
					t = "`" + t + "`"
				case "link":
					if href, _ := m.Attrs["href"].(string); href != "" {
						t = "[" + t + "](" + href + ")"
					}
				}
			}
			b.WriteString(t)
		case "hard_break":
			b.WriteString("  \n")
		case "source_ref":
			id := n.attr("source_entity_id")
			num, ok := r.numbers[id]
			if !ok {
				num = len(r.numbers) + 1
				r.numbers[id] = num
				r.order = append(r.order, id)
			}
			fmt.Fprintf(&b, "[%d]", num)
		default:
			b.WriteString(r.inline(n.Content))
		}
	}
	return b.String()
}

func (r *exportRenderer) sourceList() string {
	if len(r.order) == 0 {
		return ""
	}
	byID := map[string]exportSource{}
	for _, s := range r.sources {
		byID[s.ID] = s
	}
	var b strings.Builder
	b.WriteString("\n\n## Sources\n\n")
	for i, id := range r.order {
		s := byID[id]
		title := s.Title
		if title == "" {
			title = id
		}
		link := s.URI
		if s.attachment >= 0 && r.attachments[s.attachment].Path != "" {
			link = r.attachments[s.attachment].Path
		}
		fmt.Fprintf(&b, "%d. %s", i+1, exportLine(title))
		if link != "" {
			fmt.Fprintf(&b, " <%s>", link)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func exportLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// ---- surfaces ----

func renderTextureExport(w io.Writer, result any, _ *call) int {
	r, ok := result.(*textureExportResult)
	if !ok {
		return writeJSON(w, result)
	}
	return writeJSON(w, r.summary())
}

// textureExportMCP returns the newest exported revision's Markdown, its
// images as image content blocks (bounded), a listing of every attachment
// with its local path, and the CLI's summary as the last block.
func textureExportMCP(result any, x *call) mcpToolResult {
	r, ok := result.(*textureExportResult)
	if !ok || len(r.Revisions) == 0 {
		summary := textureExportSummary{Out: x.trim("out")}
		if ok {
			summary = r.summary()
		}
		raw, _ := json.Marshal(summary)
		return mcpToolResult{Content: []mcpContent{{Type: "text", Text: "No revision to export."}, {Type: "text", Text: string(raw)}}, StructuredContent: raw}
	}
	limit := int(x.num("max_images"))
	newest := r.Revisions[len(r.Revisions)-1]
	content := []mcpContent{{Type: "text", Text: newest.markdown}}
	inlined, inlineBytes := 0, 0
	seen := map[string]bool{}
	var listing strings.Builder
	listing.WriteString("Attachments")
	if r.Out != "" {
		listing.WriteString(" (written under " + r.Out + ")")
	}
	listing.WriteString(":\n")
	for ri := len(r.Revisions) - 1; ri >= 0; ri-- {
		m := r.Revisions[ri]
		for _, a := range m.Attachments {
			local := ""
			if r.Out != "" && a.Path != "" {
				local = filepath.Join(r.Out, m.dir, filepath.FromSlash(a.Path))
			}
			status := ""
			switch {
			case seen[a.SHA256]:
				status = "shown above"
			case a.MediaType != "image/png" && a.MediaType != "image/jpeg":
				status = "not inlined: " + a.MediaType + " is not PNG or JPEG"
			case inlined >= limit:
				status = fmt.Sprintf("not inlined: max_images %d reached", limit)
			case len(a.data) > mcpMaxImageBytes:
				status = fmt.Sprintf("not inlined: %d bytes is over the %d-byte image bound", len(a.data), mcpMaxImageBytes)
			case inlineBytes+len(a.data) > mcpMaxInlineBytes:
				status = fmt.Sprintf("not inlined: the call's %d-byte inline bound is reached", mcpMaxInlineBytes)
			default:
				content = append(content, mcpContent{Type: "image", Data: encodeBase64(a.data), MimeType: a.MediaType})
				inlined++
				inlineBytes += len(a.data)
				seen[a.SHA256] = true
				status = "inlined"
			}
			fmt.Fprintf(&listing, "- %s %s %s (%s, %d bytes, sha256 %s, %s)", m.RevisionID, a.BlockID, a.Kind, a.MediaType, a.Bytes, a.SHA256, status)
			if a.Caption != "" {
				fmt.Fprintf(&listing, " %q", a.Caption)
			}
			if local != "" {
				listing.WriteString(" local: " + local)
			} else if a.SourceURL != "" {
				listing.WriteString(" route: " + a.SourceURL)
			}
			listing.WriteString("\n")
		}
	}
	content = append(content, mcpContent{Type: "text", Text: listing.String()})
	raw, _ := json.Marshal(r.summary())
	content = append(content, mcpContent{Type: "text", Text: string(raw)})
	return mcpToolResult{Content: content, StructuredContent: raw}
}

const textureExportDesc = "Export a Texture document as Markdown with every attachment it embeds (screenshots and other images by asset digest, screens saved as files in the computer's tree), each sha256-verified against the digest the document declares; a mismatch is refused. " +
	"The result is the newest exported revision's Markdown, then its PNG and JPEG images as image content blocks, then a listing of every attachment (media type, size, sha256 and, with out, its local path), then {out, attachments}. " +
	"Bounds: at most max_images images (default 4, at most 10), each at most 1 MiB, at most 3 MiB inlined per call; an image over a bound is listed with its local path instead. " +
	"With out (an absolute directory on the machine running choir mcp) the same files are written there: doc.md, manifest.json and attachments/. since or all_revisions export one directory per revision and need out."

func textureExportCommand() *command {
	return &command{
		path: []string{"texture", "export"}, summary: "Download a Texture document as Markdown with its attachments (images, screens), sha256-verified",
		desc: textureExportDesc,
		params: []param{
			pStr("doc_id", "Texture document id").req().one(),
			pStr("revision", "Exact revision id to export; defaults to the current head"),
			pStr("out", "Directory to write doc.md, manifest.json and attachments/ into (created mode 0700); required on the CLI, optional on MCP (absolute)"),
			pBool("all_revisions", "Export every revision into its own subdirectory v<NNNN>-<revision>/ with an index.json"),
			pStr("since", "Export only the revisions after this revision id, one subdirectory each (for a monitor pulling what is new)"),
			pInt("max_images", "Images to inline as image content blocks (each at most 1 MiB, at most 3 MiB per call)").dflt(mcpDefaultImages).between(0, mcpMaxImages).
				onlyMCP("image content blocks exist only in an MCP result; the CLI writes the files"),
		},
		scopes: []string{"read:texture", "read:runtime"}, readOnly: true,
		handler: textureExport, render: renderTextureExport, mcpOut: textureExportMCP,
	}
}
