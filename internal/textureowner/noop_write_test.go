package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A Write that changes nothing a reader sees is refused at the desk's commit
// (docs/problems/unsupported-cites-duplicates-and-churn-2026-11-10.md, Q3):
// before, 8% of the revision steps of the most revised stories republished
// identical text as a new version, each costing the writer's calls and a
// judge round. Ways it can fail: N1 a Write with the same text and cites but
// different bytes (whitespace) commits; N2 a Write that changes only a cite
// (same text, another quote) is refused as no change; N3 a real text change
// is refused; N4 the refusal does not name its reason; N5 a refused Write
// leaves a new revision behind.
func TestWriteThatChangesNothingIsRefusedAndCommitsNothing(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "noop-write")
	ctx := context.Background()
	const first = "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}."
	write := func(edits, commandID string) (string, error) {
		head := headOf(t, s, rec)
		body := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Done: "states the ruling", Write: &yaegikernel.TextureWrite{Edits: edits, Why: "state the ruling"}})
		return h.CommitCellTextureAuthor(ctx, &rec, body, commandID)
	}
	if _, err := write(first, "noop-1"); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	head := headOf(t, s, rec)

	for name, edits := range map[string]string{
		"the same Write again":       first,
		"whitespace only (N1)":       "@replace b2\nThe court  ruled that the merger may proceed  {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.",
		"a surrounding newline (N1)": first + "\n",
	} {
		_, err := write(edits, "noop-"+strings.ReplaceAll(name, " ", "-"))
		if err == nil || !strings.Contains(err.Error(), "changes nothing") {
			t.Fatalf("%s: err = %v; want a refusal naming that the Write changes nothing (N4)", name, err)
		}
		if after := headOf(t, s, rec); after.RevisionID != head.RevisionID {
			t.Fatalf("%s: a refused Write left revision %s (N5)", name, after.RevisionID)
		}
	}

	// N2 and N3 are decided by the comparison itself, not by a second commit:
	// after the first Write's Done the request is closed, so a second real
	// commit would meet the concurrency check (X3, as in the C1 tests).
	baseDoc, baseEntities, err := texturewrite.Base(head)
	if err != nil {
		t.Fatal(err)
	}
	same := func(edits string) bool {
		t.Helper()
		res, err := texturewrite.Apply(ctx, texturewrite.Input{OwnerID: rec.OwnerID, DocID: head.DocID, Seed: head.RevisionID,
			Base: baseDoc, Entities: baseEntities, Script: edits, Resolver: h.textureCaptures(), Media: h.textureMedia()})
		if err != nil {
			t.Fatalf("apply %q: %v", edits, err)
		}
		return writeChangesNothing(baseDoc, baseEntities, res.Doc, res.Entities, head.DocID)
	}
	if !same(first) {
		t.Fatal("the same Write is not seen as no change")
	}
	if same("@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"The appeal was filed on Monday by the regulator\"}}.") {
		t.Fatal("N2: a Write that changes only its citation is seen as no change")
	}
	if same("@replace b2\nThe court ruled that the merger may proceed on Monday {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.") {
		t.Fatal("N3: a Write that changes its text is seen as no change")
	}
}
