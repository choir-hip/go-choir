package texturedoc

// The file card (station L8): an embed of kind file names a file in the
// owner's Files and opens it in Reader. Ways it could fail, written before
// the code:
//   - the path is absolute, climbs out with "..", is empty, has an empty or
//     dot component, a backslash or a control byte (a card to a file outside
//     the owner's Files);
//   - the card has no title, or carries an image's digest or alt;
//   - a malformed mime is stored;
//   - a file card reads as publishable, or as an image to pin;
//   - its projection is empty, so the revision's content loses it.

import (
	"strings"
	"testing"
)

func fileNode(mutate func(attrs, target map[string]any)) Node {
	target := map[string]any{"path": "Reports/summary.pdf", "mime": "application/pdf"}
	attrs := map[string]any{"id": "embed-f", "kind": EmbedKindFile, "target": target, "title": "Summary report"}
	if mutate != nil {
		mutate(attrs, target)
	}
	return Node{Type: "embed", Attrs: attrs}
}

func TestAFileCardValidates(t *testing.T) {
	doc := docOf(para("p1", "See the report."), fileNode(nil))
	if err := Validate(doc, nil); err != nil {
		t.Fatalf("file card refused: %v", err)
	}
	e, _ := EmbedOf(doc.Doc.Content[1])
	if e.Path != "Reports/summary.pdf" || e.Mime != "application/pdf" || e.Title != "Summary report" {
		t.Fatalf("embed %+v", e)
	}
	if e.Publishable() || len(PrivateEmbeds(doc)) != 1 || len(EmbeddedDigests(doc)) != 0 {
		t.Fatalf("a file card is private and pins nothing")
	}
	if got := EmbedProjectionText(doc.Doc.Content[1]); got != "[file: Summary report]" {
		t.Fatalf("projection %q", got)
	}
}

func TestAFileCardRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(attrs, target map[string]any)
		want   string
	}{
		"absolute":    {func(_, t map[string]any) { t["path"] = "/etc/passwd" }, "relative"},
		"climbing":    {func(_, t map[string]any) { t["path"] = "Reports/../../secret.pdf" }, "inside Files"},
		"leading dot": {func(_, t map[string]any) { t["path"] = "../x.pdf" }, "inside Files"},
		"empty part":  {func(_, t map[string]any) { t["path"] = "Reports//x.pdf" }, "inside Files"},
		"backslash":   {func(_, t map[string]any) { t["path"] = `Reports\x.pdf` }, "separators"},
		"control":     {func(_, t map[string]any) { t["path"] = "Reports/x\n.pdf" }, "control"},
		"no path":     {func(_, t map[string]any) { delete(t, "path") }, "required"},
		"no title":    {func(a, _ map[string]any) { delete(a, "title") }, "title is required"},
		"bad mime":    {func(_, t map[string]any) { t["mime"] = "pdf" }, "media type"},
		"digest":      {func(_, t map[string]any) { t["digest"] = testDigest }, "not a file target field"},
		"long title":  {func(a, _ map[string]any) { a["title"] = strings.Repeat("t", MaxEmbedCaptionRunes+1) }, "longer than"},
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate(docOf(fileNode(tc.mutate)), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
	if err := Validate(docOf(evidenceNode(func(a, _ map[string]any) { a["title"] = "x" })), nil); err == nil {
		t.Fatal("title on a non-file embed was accepted")
	}
}
