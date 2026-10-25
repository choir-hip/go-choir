package texturewrite

// Embed blocks in a Texture desk's Write
// (docs/design/texture-evidence-embeds-te-2026-10-25.md section 1). The
// grammar has no embed token: only Texture's evidence path writes one. Ways
// a Write could fail them, written before the code:
//  1. the frame shows an embed as an empty block, so the desk cannot see
//     the evidence or tell which block it is;
//  2. a Write that does not touch the embed (an @insert, a @replace of
//     another block, a @move) drops or changes it;
//  3. an @replace or @delete of the embed, or an @all that leaves its line
//     out, removes the evidence without a word;
//  4. an @all that copies the frame line unchanged turns the embed into a
//     paragraph of that line;
//  5. the embed's frame line parses as a citation or a mark.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

const evidenceDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func evidenceBlock(id string) texturedoc.Node {
	return texturedoc.Node{Type: "embed", Attrs: map[string]any{
		"id": id, "kind": texturedoc.EmbedKindCapsuleEvidence,
		"target": map[string]any{"digest": evidenceDigest, "operation_id": "selfdev-op-1", "computer_id": "computer-1",
			"route": "/reader*x", "width": float64(1280), "height": float64(800), "blank": true},
		"caption": "Blank screenshot of /reader*x from candidate selfdev-op-1 (1280x800)", "alt": "Screenshot of /reader*x",
	}}
}

func embedBase() texturedoc.StructuredTextureDoc {
	return baseDoc(heading("h1", 1, "Reader change"), paragraph("p1", "The candidate froze."), evidenceBlock("embed-1"), paragraph("p2", "Closing line."))
}

func embedJSON(t *testing.T, doc texturedoc.StructuredTextureDoc) []string {
	t.Helper()
	var out []string
	for _, n := range doc.Doc.Content {
		if n.Type == "embed" {
			raw, _ := json.Marshal(n)
			out = append(out, string(raw))
		}
	}
	return out
}

// Failure modes 1 and 5.
func TestFrameShowsAnEmbedAsOneReadOnlyLine(t *testing.T) {
	blocks := DocBlocks(embedBase())
	got := blocks[2]
	if got.Kind != "embed" || got.Text != "[embed capsule_evidence /reader-x sha256:"+evidenceDigest+"]" {
		t.Fatalf("frame block = %+v", got)
	}
}

// Failure mode 2.
func TestWritesAroundAnEmbedKeepItUnchanged(t *testing.T) {
	base := embedBase()
	want := embedJSON(t, base)
	for _, script := range []string{
		"@insert end\nA new closing paragraph.",
		"@replace b2\nThe candidate froze with one capture.",
		"@move b3 after b4",
		"@all\n# Reader change\n\nThe candidate froze, rewritten.\n\n[embed capsule_evidence /reader-x sha256:" + evidenceDigest + "]\n\nClosing line.",
	} {
		res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-1", Base: base, Script: script, Resolver: newFakeCaptures()})
		if err != nil {
			t.Fatalf("%q: %v", script, err)
		}
		if got := embedJSON(t, res.Doc); len(got) != 1 || got[0] != want[0] {
			t.Fatalf("%q: embeds after = %v, want %v", script, got, want)
		}
		if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
			t.Fatalf("%q: result invalid: %v", script, err)
		}
	}
}

// Failure modes 3 and 4.
func TestAWriteCannotRemoveOrChangeAnEmbed(t *testing.T) {
	for _, script := range []string{
		"@replace b3\nNo evidence here.",
		"@delete b3",
		"@replace b2..b3\nThe candidate froze.",
		"@all\n# Reader change\n\nThe candidate froze.\n\nClosing line.",
		"@all\n# Reader change\n\nThe candidate froze.\n\n[embed capsule_evidence /reader-x sha256:" + strings.Repeat("f", 64) + "]\n\nClosing line.",
	} {
		_, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-1", Base: embedBase(), Script: script, Resolver: newFakeCaptures()})
		if err == nil {
			t.Fatalf("%q removed or changed the embed", script)
		}
		if lineOf(t, err) != 1 || !strings.Contains(err.Error(), "evidence embed b3") {
			t.Fatalf("%q: refusal %v does not name the embed at the directive", script, err)
		}
	}
}
