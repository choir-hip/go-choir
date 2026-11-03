package main

// `choir texture export` and the texture_export MCP tool download a Texture
// document with its attachments, so a coding agent driving Choir (and the
// subagents watching it) can show the screenshots in its own chat. Ways it
// could fail, written before the code:
//   - doc.md links an attachment by its server route, which a local viewer
//     cannot open, or by an absolute path, which breaks when the directory
//     moves;
//   - an attachment is written whose bytes do not hash to the digest the
//     document declares (a swapped or corrupt blob lands as "verified");
//   - a mismatch leaves a file under the verified name, or the command
//     exits 0;
//   - a screen saved as a file in the computer's tree (target.file) is not
//     fetched when the asset store does not hold it;
//   - two embeds of the same bytes are fetched or written twice;
//   - the manifest omits the source ref, sha256, media type or size;
//   - the CLI prints more than the directory and the count;
//   - the MCP result carries no image block, a block without mimeType, or
//     base64 that is not the bytes; it inlines more images or bytes than
//     the bound, and an image over the bound is dropped instead of listed
//     with its local path;
//   - --since exports revisions at or before the cursor, or misses one
//     after it; --all-revisions puts two revisions in one directory;
//   - --revision of another document is exported as this one.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func exportPNG(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: seed, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func hexSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// exportStub serves one document's routes: the document, its history
// (newest first, as the server answers), each revision, the asset store and
// the files tree. Missing keys 404.
type exportStub struct {
	t         *testing.T
	doc       string
	title     string
	revisions []string          // oldest first
	bodies    map[string]string // revision id -> body_doc JSON
	foreign   map[string]bool   // revision ids that belong to another document
	assets    map[string][]byte // digest -> bytes served
	files     map[string][]byte // tree path -> bytes
	requests  []string
}

func (s *exportStub) serve() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r.URL.Path)
		head := s.revisions[len(s.revisions)-1]
		switch {
		case r.URL.Path == "/api/texture/documents/"+s.doc:
			_, _ = fmt.Fprintf(w, `{"doc_id":%q,"title":%q,"current_revision_id":%q}`, s.doc, s.title, head)
		case r.URL.Path == "/api/texture/documents/"+s.doc+"/history":
			var entries []string
			for i := len(s.revisions) - 1; i >= 0; i-- {
				entries = append(entries, fmt.Sprintf(`{"revision_id":%q,"doc_id":%q}`, s.revisions[i], s.doc))
			}
			_, _ = fmt.Fprintf(w, `{"doc_id":%q,"entries":[%s]}`, s.doc, strings.Join(entries, ","))
		case strings.HasPrefix(r.URL.Path, "/api/texture/revisions/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/texture/revisions/")
			body, ok := s.bodies[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			owner := s.doc
			if s.foreign[id] {
				owner = "another-doc"
			}
			version := 0
			for i, rev := range s.revisions {
				if rev == id {
					version = i + 1
				}
			}
			_, _ = fmt.Fprintf(w, `{"revision_id":%q,"doc_id":%q,"version_number":%d,"content":"projection","body_doc":%s}`, id, owner, version, body)
		case strings.HasPrefix(r.URL.Path, "/api/assets/sha256/"):
			b, ok := s.assets[strings.TrimPrefix(r.URL.Path, "/api/assets/sha256/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(b)
		case strings.HasPrefix(r.URL.Path, "/api/files/"):
			b, ok := s.files[strings.TrimPrefix(r.URL.Path, "/api/files/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(b)
		default:
			s.t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	s.t.Cleanup(srv.Close)
	return srv
}

func exportBody(blocks ...string) string {
	return `{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"d"},"content":[` + strings.Join(blocks, ",") + `]}}`
}

func exportPara(id, text string) string {
	return fmt.Sprintf(`{"type":"paragraph","attrs":{"id":%q},"content":[{"type":"text","text":%q}]}`, id, text)
}

func exportEvidence(id, digest, file, caption string) string {
	target := fmt.Sprintf(`"digest":%q,"operation_id":"selfdev-op-1","computer_id":"computer-1","width":4,"height":3,"blank":false`, digest)
	if file != "" {
		target += fmt.Sprintf(`,"file":%q`, file)
	} else {
		target += `,"route":"/"`
	}
	return fmt.Sprintf(`{"type":"embed","attrs":{"id":%q,"kind":"capsule_evidence","target":{%s},"caption":%q,"alt":"Screenshot"}}`, id, target, caption)
}

func exportImage(id, digest, alt string) string {
	return fmt.Sprintf(`{"type":"embed","attrs":{"id":%q,"kind":"image","target":{"digest":%q},"alt":%q}}`, id, digest, alt)
}

func runExport(t *testing.T, host string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv(apiKeyEnvVar, "choir_sk_export")
	var out, errOut bytes.Buffer
	code := run(append([]string{"texture", "export", "--host=" + host}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func readManifest(t *testing.T, dir string) textureExportManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m textureExportManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest: %v\n%s", err, raw)
	}
	return m
}

func TestTextureExportWritesMarkdownManifestAndVerifiedAttachments(t *testing.T) {
	shotA, shotB, screen := exportPNG(t, 4, 3, 1), exportPNG(t, 5, 3, 2), exportPNG(t, 6, 3, 3)
	a, b, sc := hexSHA(shotA), hexSHA(shotB), hexSHA(screen)
	screenFile := "Self-development/selfdev-op-1/screens/home page.png"
	stub := &exportStub{t: t, doc: "doc-1", title: "Reader change", revisions: []string{"rev-1"},
		bodies: map[string]string{"rev-1": exportBody(
			`{"type":"heading","attrs":{"id":"h1","level":2},"content":[{"type":"text","text":"Evidence"}]}`,
			exportPara("p1", "See /api/assets/sha256/"+a+" for the raw route."),
			exportEvidence("embed-a", a, "", "Front page"),
			exportEvidence("embed-a2", a, "", "Front page again"),
			exportImage("embed-b", b, "A diagram"),
			// The asset store does not hold this one: it is a screen saved as
			// a file in the computer's tree.
			exportEvidence("embed-file", sc, screenFile, "Home from the screens folder"),
		)},
		assets: map[string][]byte{a: shotA, b: shotB},
		files:  map[string][]byte{screenFile: screen},
	}
	srv := stub.serve()
	dir := filepath.Join(t.TempDir(), "export")
	code, stdout, stderr := runExport(t, srv.URL, "--out", dir, "doc-1")
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout, stderr)
	}
	var printed map[string]any
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil || len(printed) != 2 || printed["out"] != dir || printed["attachments"] != float64(4) {
		t.Fatalf("stdout must be only the out directory and the count: %q", stdout)
	}
	assetFetches := 0
	for _, p := range stub.requests {
		if p == "/api/assets/sha256/"+a {
			assetFetches++
		}
	}
	if assetFetches != 1 {
		t.Fatalf("the shared digest was fetched %d times: %v", assetFetches, stub.requests)
	}

	m := readManifest(t, dir)
	if m.Schema != "choir.texture_export.v1" || m.DocID != "doc-1" || m.RevisionID != "rev-1" || m.Document != "doc.md" || len(m.Attachments) != 4 {
		t.Fatalf("manifest %+v", m)
	}
	byBlock := map[string]textureExportAttachment{}
	for _, att := range m.Attachments {
		byBlock[att.BlockID] = att
	}
	wantBytes := map[string][]byte{"embed-a": shotA, "embed-a2": shotA, "embed-b": shotB, "embed-file": screen}
	for block, want := range wantBytes {
		att := byBlock[block]
		if att.SHA256 != hexSHA(want) || att.DeclaredSHA256 != hexSHA(want) || !att.Verified || att.MediaType != "image/png" ||
			att.Bytes != int64(len(want)) || att.Error != "" || att.SourceRef == "" || filepath.IsAbs(att.Path) || !strings.HasPrefix(att.Path, "attachments/") {
			t.Fatalf("%s: %+v", block, att)
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(att.Path)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s: file %s: %v", block, att.Path, err)
		}
		if _, err := png.Decode(bytes.NewReader(got)); err != nil {
			t.Fatalf("%s is not a valid PNG: %v", block, err)
		}
	}
	if byBlock["embed-a"].Path != byBlock["embed-a2"].Path {
		t.Fatalf("one digest, two files: %s %s", byBlock["embed-a"].Path, byBlock["embed-a2"].Path)
	}
	if byBlock["embed-a"].SourceRef != "asset:sha256:"+a || byBlock["embed-file"].SourceRef != "file:"+screenFile {
		t.Fatalf("source refs: %q %q", byBlock["embed-a"].SourceRef, byBlock["embed-file"].SourceRef)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "attachments"))
	if len(entries) != 3 {
		t.Fatalf("attachments dir holds %d files, want 3 distinct", len(entries))
	}

	md, err := os.ReadFile(filepath.Join(dir, "doc.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(md)
	for _, want := range []string{
		"# Reader change", "## Evidence",
		"![Screenshot](" + byBlock["embed-a"].Path + ")", "*Front page*",
		"![A diagram](" + byBlock["embed-b"].Path + ")",
		"![Screenshot](" + byBlock["embed-file"].Path + ")",
		"See " + byBlock["embed-a"].Path + " for the raw route.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("doc.md lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "/api/") || strings.Contains(text, dir) {
		t.Fatalf("doc.md links a server route or an absolute path:\n%s", text)
	}
}

func TestTextureExportRefusesASHA256Mismatch(t *testing.T) {
	good, swapped := exportPNG(t, 4, 3, 1), exportPNG(t, 4, 3, 9)
	declared := hexSHA(good)
	other := exportPNG(t, 3, 3, 4)
	stub := &exportStub{t: t, doc: "doc-1", title: "T", revisions: []string{"rev-1"},
		bodies: map[string]string{"rev-1": exportBody(exportEvidence("embed-bad", declared, "", "Swapped"), exportImage("embed-ok", hexSHA(other), "ok"))},
		assets: map[string][]byte{declared: swapped, hexSHA(other): other},
	}
	srv := stub.serve()
	dir := filepath.Join(t.TempDir(), "export")
	code, stdout, stderr := runExport(t, srv.URL, "--out", dir, "doc-1")
	if code == 0 || !strings.Contains(stderr, "sha256 mismatch") || stdout != "" {
		t.Fatalf("a mismatch must fail the export: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	m := readManifest(t, dir)
	var bad, ok textureExportAttachment
	for _, att := range m.Attachments {
		switch att.BlockID {
		case "embed-bad":
			bad = att
		case "embed-ok":
			ok = att
		}
	}
	if bad.Error == "" || bad.Verified || bad.Path != "" || bad.DeclaredSHA256 != declared || bad.SHA256 != hexSHA(swapped) {
		t.Fatalf("mismatch entry %+v", bad)
	}
	if !ok.Verified || ok.Path == "" {
		t.Fatalf("the good attachment is still exported: %+v", ok)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "attachments"))
	for _, e := range entries {
		if strings.Contains(e.Name(), declared[:12]) || strings.Contains(e.Name(), hexSHA(swapped)[:12]) {
			t.Fatalf("the refused bytes were written as %s", e.Name())
		}
	}
	md, _ := os.ReadFile(filepath.Join(dir, "doc.md"))
	if !strings.Contains(string(md), "not exported") {
		t.Fatalf("doc.md does not say the attachment was refused:\n%s", md)
	}
}

func TestTextureExportRefusesARevisionOfAnotherDocument(t *testing.T) {
	stub := &exportStub{t: t, doc: "doc-1", title: "T", revisions: []string{"rev-1", "rev-x"},
		bodies: map[string]string{"rev-1": exportBody(exportPara("p", "a")), "rev-x": exportBody(exportPara("p", "b"))}, foreign: map[string]bool{"rev-x": true}}
	srv := stub.serve()
	code, _, stderr := runExport(t, srv.URL, "--out", t.TempDir(), "--revision", "rev-x", "doc-1")
	if code == 0 || !strings.Contains(stderr, "not a version of this document") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if code, _, stderr := runExport(t, srv.URL, "doc-1"); code != 2 || !strings.Contains(stderr, "--out") {
		t.Fatalf("the CLI needs --out: exit %d %s", code, stderr)
	}
}

func TestTextureExportSinceExportsOnlyNewRevisions(t *testing.T) {
	shot := exportPNG(t, 4, 3, 1)
	d := hexSHA(shot)
	stub := &exportStub{t: t, doc: "doc-1", title: "T", revisions: []string{"rev-1", "rev-2", "rev-3"},
		bodies: map[string]string{
			"rev-1": exportBody(exportPara("p", "one")),
			"rev-2": exportBody(exportPara("p", "two"), exportEvidence("e", d, "", "Shot")),
			"rev-3": exportBody(exportPara("p", "three"), exportEvidence("e", d, "", "Shot")),
		},
		assets: map[string][]byte{d: shot},
	}
	srv := stub.serve()
	dirs := func(out string) []string {
		entries, _ := os.ReadDir(out)
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		return names
	}

	all := t.TempDir()
	code, stdout, stderr := runExport(t, srv.URL, "--out", all, "--all-revisions", "doc-1")
	if code != 0 || !strings.Contains(stdout, `"attachments": 2`) {
		t.Fatalf("all: exit %d %s %s", code, stdout, stderr)
	}
	if got := dirs(all); strings.Join(got, ",") != "v0001-rev-1,v0002-rev-2,v0003-rev-3" {
		t.Fatalf("all-revisions dirs %v", got)
	}
	for i, rev := range []string{"rev-1", "rev-2", "rev-3"} {
		sub := filepath.Join(all, fmt.Sprintf("v%04d-%s", i+1, rev))
		if m := readManifest(t, sub); m.RevisionID != rev {
			t.Fatalf("%s holds %s", sub, m.RevisionID)
		}
		md, _ := os.ReadFile(filepath.Join(sub, "doc.md"))
		if !strings.Contains(string(md), []string{"one", "two", "three"}[i]) {
			t.Fatalf("%s doc.md: %s", sub, md)
		}
	}

	inc := t.TempDir()
	stub.requests = nil
	code, stdout, stderr = runExport(t, srv.URL, "--out", inc, "--since", "rev-1", "doc-1")
	if code != 0 {
		t.Fatalf("since: exit %d %s %s", code, stdout, stderr)
	}
	if got := dirs(inc); strings.Join(got, ",") != "v0002-rev-2,v0003-rev-3" {
		t.Fatalf("since rev-1 exported %v", got)
	}
	for _, p := range stub.requests {
		if p == "/api/texture/revisions/rev-1" {
			t.Fatalf("since read the cursor revision itself: %v", stub.requests)
		}
	}
	var index textureExportIndex
	raw, _ := os.ReadFile(filepath.Join(inc, "index.json"))
	if err := json.Unmarshal(raw, &index); err != nil || len(index.Revisions) != 2 || index.Revisions[1].RevisionID != "rev-3" || index.Revisions[1].Dir != "v0003-rev-3" {
		t.Fatalf("index %s", raw)
	}
	// Nothing new since the head: no revision, no failure.
	code, stdout, _ = runExport(t, srv.URL, "--out", inc, "--since", "rev-3", "doc-1")
	if code != 0 || !strings.Contains(stdout, `"attachments": 0`) {
		t.Fatalf("since head: exit %d %s", code, stdout)
	}
	if code, _, stderr := runExport(t, srv.URL, "--out", inc, "--since", "rev-unknown", "doc-1"); code == 0 || !strings.Contains(stderr, "rev-unknown") {
		t.Fatalf("an unknown cursor: exit %d %s", code, stderr)
	}
	if code, _, _ := runExport(t, srv.URL, "--out", inc, "--since", "rev-1", "--revision", "rev-2", "doc-1"); code != 2 {
		t.Fatalf("--since with --revision must be a usage error, exit %d", code)
	}
}

func TestTextureExportMCPReturnsBoundedImageBlocks(t *testing.T) {
	small1, small2, small3 := exportPNG(t, 4, 3, 1), exportPNG(t, 5, 3, 2), exportPNG(t, 6, 3, 3)
	// Noise does not compress: over the per-image inline bound.
	big := make([]byte, 0)
	{
		img := image.NewRGBA(image.Rect(0, 0, 700, 700))
		seed := uint32(7)
		for i := range img.Pix {
			seed = seed*1664525 + 1013904223
			img.Pix[i] = byte(seed >> 24)
		}
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		big = buf.Bytes()
	}
	if len(big) <= mcpMaxImageBytes {
		t.Fatalf("fixture %d bytes is not over the %d bound", len(big), mcpMaxImageBytes)
	}
	stub := &exportStub{t: t, doc: "doc-1", title: "Shots", revisions: []string{"rev-1"},
		bodies: map[string]string{"rev-1": exportBody(exportPara("p", "Look."),
			exportImage("big", hexSHA(big), "big"),
			exportImage("s1", hexSHA(small1), "one"), exportImage("s2", hexSHA(small2), "two"), exportImage("s3", hexSHA(small3), "three"))},
		assets: map[string][]byte{hexSHA(big): big, hexSHA(small1): small1, hexSHA(small2): small2, hexSHA(small3): small3},
	}
	srv := stub.serve()
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_export")
	t.Setenv("CHOIR_COMPUTER", "")
	t.Setenv("CHOIR_API_KEY_FILE", "")
	dir := filepath.Join(t.TempDir(), "mcp-export")

	images := func(r mcpToolResult) []mcpContent {
		var out []mcpContent
		for _, c := range r.Content {
			if c.Type == "image" {
				out = append(out, c)
			}
		}
		return out
	}
	result := toolCall(t, "texture_export", `{"doc_id":"doc-1","out":"`+dir+`","max_images":2}`)
	if result.IsError {
		t.Fatalf("tool error: %+v", result)
	}
	if result.Content[0].Type != "text" || !strings.Contains(result.Content[0].Text, "Look.") {
		t.Fatalf("first block is not the document text: %+v", result.Content[0])
	}
	got := images(result)
	if len(got) != 2 {
		t.Fatalf("max_images 2 inlined %d", len(got))
	}
	want := [][]byte{small1, small2}
	for i, block := range got {
		raw, err := base64.StdEncoding.DecodeString(block.Data)
		if err != nil || !bytes.Equal(raw, want[i]) || block.MimeType != "image/png" || block.Text != "" {
			t.Fatalf("image block %d: mime %q err %v", i, block.MimeType, err)
		}
	}
	all := ""
	for _, c := range result.Content {
		all += c.Text
	}
	m := readManifest(t, dir)
	for _, att := range m.Attachments {
		if att.BlockID == "big" || att.BlockID == "s3" {
			if !strings.Contains(all, filepath.Join(dir, filepath.FromSlash(att.Path))) {
				t.Fatalf("%s was not inlined and its local path is not listed:\n%s", att.BlockID, all)
			}
		}
	}
	last := result.Content[len(result.Content)-1]
	if canonicalJSON([]byte(last.Text)) != canonicalJSON(result.StructuredContent) || !strings.Contains(last.Text, `"attachments":4`) && !strings.Contains(last.Text, `"attachments": 4`) {
		t.Fatalf("summary block %q / %s", last.Text, result.StructuredContent)
	}

	// Without an out directory nothing is written and the images still
	// inline; the default bound applies.
	result = toolCall(t, "texture_export", `{"doc_id":"doc-1"}`)
	if result.IsError || len(images(result)) != 3 {
		t.Fatalf("default inline: %d images, %+v", len(images(result)), result.IsError)
	}
	if result := toolCall(t, "texture_export", `{"doc_id":"doc-1","max_images":50}`); !result.IsError {
		t.Fatalf("max_images over the ceiling must be refused")
	}
	if result := toolCall(t, "texture_export", `{"doc_id":"doc-1","out":"relative/dir"}`); !result.IsError {
		t.Fatalf("a relative out directory on MCP must be refused")
	}
	if result := toolCall(t, "texture_export", `{"doc_id":"doc-1","all_revisions":true}`); !result.IsError {
		t.Fatalf("all_revisions without out must be refused")
	}
	var total int
	for _, block := range images(toolCall(t, "texture_export", `{"doc_id":"doc-1","max_images":10}`)) {
		raw, _ := base64.StdEncoding.DecodeString(block.Data)
		total += len(raw)
		if len(raw) > mcpMaxImageBytes {
			t.Fatalf("an image over the per-image bound was inlined")
		}
	}
	if total > mcpMaxInlineBytes {
		t.Fatalf("inlined %d bytes, over %d", total, mcpMaxInlineBytes)
	}
}
