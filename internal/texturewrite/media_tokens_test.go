package texturewrite

// The media tokens {{image}} and {{file}} (station L8). Ways they could
// fail, written before the code:
//  1. the token is read as a citation, or as plain text, so the desk cannot
//     place the engineer's chart or report;
//  2. inline use (inside a sentence, beside text) builds a nested embed the
//     validator refuses late, or silently drops words;
//  3. a bad digest, a missing alt, a path that climbs out of Files, or a
//     Files path that is not an image reaches the document;
//  4. a Files path is not imported into the asset store, so the block names
//     bytes nothing serves;
//  5. an @all that copies the frame line of a stored image or file turns it
//     into a paragraph or a duplicate block;
//  6. the desk cannot remove its own image or file block (only evidence is
//     read-only);
//  7. the quote gate treats the block as a quote node.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

const chartDigest = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

type fakeMedia struct {
	images   map[string]bool
	imported []string
}

func (m *fakeMedia) HasImage(d string) bool { return m.images[d] }
func (m *fakeMedia) ImportImage(_ context.Context, p string) (string, int, int, error) {
	if !strings.HasSuffix(p, ".png") {
		return "", 0, 0, errors.New("bytes are not a PNG, JPEG, GIF or WebP image")
	}
	m.imported = append(m.imported, p)
	return chartDigest, 640, 480, nil
}
func (m *fakeMedia) FileInfo(_ context.Context, p string) (string, error) {
	if strings.HasSuffix(p, ".pdf") {
		return "application/pdf", nil
	}
	return "", errors.New("no such file in Files")
}

func applyMedia(t *testing.T, base texturedoc.StructuredTextureDoc, script string, m Media) (Result, error) {
	t.Helper()
	return Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-1", Base: base, Script: script, Resolver: newFakeCaptures(), Media: m})
}

func firstEmbed(t *testing.T, doc texturedoc.StructuredTextureDoc) texturedoc.Embed {
	t.Helper()
	es := texturedoc.Embeds(doc)
	if len(es) == 0 {
		t.Fatal("no embed block in the result")
	}
	return es[0]
}

func TestImageTokenWithADigestMakesAnImageBlock(t *testing.T) {
	m := &fakeMedia{images: map[string]bool{chartDigest: true}}
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{image "+chartDigest+` "Figure 1. Revenue" "Line chart of revenue by quarter"}}`, m)
	if err != nil {
		t.Fatal(err)
	}
	e := firstEmbed(t, res.Doc)
	if e.Kind != texturedoc.EmbedKindImage || e.Digest != chartDigest || e.Caption != "Figure 1. Revenue" || e.Alt != "Line chart of revenue by quarter" {
		t.Fatalf("embed %+v", e)
	}
	if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
		t.Fatalf("result invalid: %v", err)
	}
}

func TestImageTokenWithAFilesPathImportsIt(t *testing.T) {
	m := &fakeMedia{}
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{image Reports/chart.png \"Figure 1\" \"A bar chart\"}}", m)
	if err != nil {
		t.Fatal(err)
	}
	e := firstEmbed(t, res.Doc)
	if e.Digest != chartDigest || e.Width != 640 || e.Height != 480 || len(m.imported) != 1 || m.imported[0] != "Reports/chart.png" {
		t.Fatalf("embed %+v imported %v", e, m.imported)
	}
	if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
		t.Fatalf("result invalid: %v", err)
	}
}

func TestImageTokenRefusals(t *testing.T) {
	m := &fakeMedia{images: map[string]bool{}}
	base := baseDoc(paragraph("p1", "Intro."))
	for name, tc := range map[string]struct{ script, want string }{
		"inline":          {"@insert end\nSee {{image " + chartDigest + ` "c" "a"}} here.`, "block of its own"},
		"beside text":     {"@insert end\n{{image " + chartDigest + ` "c" "a"}} trailing words`, "block of its own"},
		"unknown digest":  {"@insert end\n{{image " + chartDigest + ` "c" "a"}}`, "asset store"},
		"short digest":    {"@insert end\n{{image abcdef \"c\" \"a\"}}", "not a PNG"},
		"no alt":          {"@insert end\n{{image " + chartDigest + "}}", "alt text"},
		"climbing path":   {"@insert end\n{{image ../secret.png \"c\" \"a\"}}", "inside Files"},
		"absolute path":   {"@insert end\n{{image /etc/x.png \"c\" \"a\"}}", "relative"},
		"not an image":    {"@insert end\n{{image Reports/notes.txt \"c\" \"a\"}}", "not a PNG"},
		"unterminated":    {"@insert end\n{{image Reports/chart.png \"c\" \"a\"", "}}"},
		"no ref":          {"@insert end\n{{image}}", "needs a"},
		"too many quotes": {"@insert end\n{{image Reports/chart.png \"a\" \"b\" \"c\"}}", "at most"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := applyMedia(t, base, tc.script, m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
	// Without Media a path is refused, and Check sees inline use.
	if _, err := applyMedia(t, base, "@insert end\n{{image Reports/chart.png \"c\" \"a\"}}", nil); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("path without media: %v", err)
	}
	if err := Check("@insert end\nSee {{file Reports/a.pdf \"t\"}} here.", 1, nil); err == nil || !strings.Contains(err.Error(), "block of its own") {
		t.Fatalf("Check on inline file token: %v", err)
	}
}

func TestFileTokenMakesAFileCard(t *testing.T) {
	m := &fakeMedia{}
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{file Reports/summary.pdf \"Summary report\"}}", m)
	if err != nil {
		t.Fatal(err)
	}
	e := firstEmbed(t, res.Doc)
	if e.Kind != texturedoc.EmbedKindFile || e.Path != "Reports/summary.pdf" || e.Title != "Summary report" || e.Mime != "application/pdf" {
		t.Fatalf("embed %+v", e)
	}
	if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
		t.Fatalf("result invalid: %v", err)
	}
	// A title defaults to the file name.
	res, err = applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{file Reports/summary.pdf}}", m)
	if err != nil || firstEmbed(t, res.Doc).Title != "summary.pdf" {
		t.Fatalf("default title: %v %+v", err, res)
	}
	for _, script := range []string{"@insert end\n{{file ../x.pdf \"t\"}}", "@insert end\n{{file Reports/gone.txt \"t\"}}", "@insert end\nRead {{file Reports/summary.pdf \"t\"}} now."} {
		if _, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), script, m); err == nil {
			t.Fatalf("%q was accepted", script)
		}
	}
}

func TestStoredImageAndFileFramesAreTokensTheDeskCanWriteBack(t *testing.T) {
	m := &fakeMedia{images: map[string]bool{chartDigest: true}}
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{image "+chartDigest+" \"Fig\" \"Chart\"}}\n\n{{file Reports/summary.pdf \"Report\"}}", m)
	if err != nil {
		t.Fatal(err)
	}
	blocks := DocBlocks(res.Doc)
	if len(blocks) != 3 || blocks[1].Text != "{{image "+chartDigest+` "Fig" "Chart"}}` || blocks[2].Text != `{{file Reports/summary.pdf "Report"}}` {
		t.Fatalf("frame blocks %+v", blocks)
	}
	// An @all that copies the lines keeps the same two blocks, unchanged.
	before := embedJSON(t, res.Doc)
	res2, err := applyMedia(t, res.Doc, "@all\nIntro, rewritten.\n\n"+blocks[1].Text+"\n\n"+blocks[2].Text, m)
	if err != nil {
		t.Fatal(err)
	}
	if after := embedJSON(t, res2.Doc); strings.Join(after, "|") != strings.Join(before, "|") || len(texturedoc.Embeds(res2.Doc)) != 2 {
		t.Fatalf("embeds changed: %v vs %v", after, before)
	}
	// The desk can remove its own blocks.
	res3, err := applyMedia(t, res.Doc, "@delete b2 b3", m)
	if err != nil || len(texturedoc.Embeds(res3.Doc)) != 0 {
		t.Fatalf("delete: %v %v", err, res3)
	}
}

func TestMediaBlocksAreNotQuoteNodes(t *testing.T) {
	m := &fakeMedia{}
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{image Reports/chart.png \"c\" \"a\"}}\n\n{{file Reports/summary.pdf \"t\"}}", m)
	if err != nil {
		t.Fatal(err)
	}
	checks, _ := Checks(res.Doc, "doc", "rev")
	if len(checks) != 0 || len(res.Entities) != 0 {
		t.Fatalf("media blocks made citations: %v %v", checks, res.Entities)
	}
}
