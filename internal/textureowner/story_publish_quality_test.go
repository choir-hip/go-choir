package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Publish quality holds (lane Q). Ways each can fail, written before the
// code:
//   - a stub (one cited sentence per outlet) publishes;
//   - a held revision is dropped or renumbers later ones, instead of leaving
//     a gap the next passing revision fills;
//   - the hold does not reach the writer, so it cannot know to expand;
//   - a French story, or one that opens with a "Status." line, publishes;
//   - an English story with one quoted French sentence is refused.
//
// storyFiller (story_publish_test.go) is invented English filler that clears
// the word floor.

func writeAndResult(t *testing.T, h *Handler, s any, rec *types.RunRecord, base, identity, edits string) string {
	t.Helper()
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base, Write: &yaegikernel.TextureWrite{Edits: edits, Why: "report the harbor"}})
	out, err := h.CommitCellTextureAuthor(context.Background(), rec, turn, identity)
	if err != nil {
		t.Fatalf("Write %s: %v", identity, err)
	}
	return out
}

const twoCites = "\n\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}." +
	" The regulator will not appeal {{cite https://wire.example.net/merger \"the regulator said it will not appeal\"}}."

func TestAStubIsHeldWithItsReasonAndTheFullerRevisionPublishes(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "stub", types.CastPublishAuto)

	out := writeAndResult(t, h, s, &rec, headOf(t, s, rec).RevisionID, "write-stub", "@all\n# Harbor\n\nThe harbor reopens Monday."+twoCites)
	pub.none(t, "a stub under the word floor")
	if !strings.Contains(out, `"publish_held"`) || !strings.Contains(out, "-word floor") || !strings.Contains(out, "stays a gap") {
		t.Fatalf("turn result %s does not tell Texture the stub was held, with the floor", out)
	}

	full := storyWrite(t, h, s, &rec, "write-full", cited("The harbor reopens Monday."))
	v := pub.next(t)
	if v.RevisionID != full.RevisionID || v.Revision != 3 {
		t.Fatalf("published %+v; want the fuller revision %s as v3 (v2, the stub, stays a gap)", v, full.RevisionID)
	}
}

func TestANonEnglishStoryAndProcessTextAreHeldAndQuotesAreNot(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "lang", types.CastPublishAuto)

	french := strings.Repeat("Le conseil municipal a annoncé que le pont sera fermé pendant les travaux, et les habitants sont invités à utiliser la nouvelle route pour se rendre au centre. ", 5)
	out := writeAndResult(t, h, s, &rec, headOf(t, s, rec).RevisionID, "write-fr", "@all\n# Le pont\n\n"+french+twoCites)
	pub.none(t, "a French story")
	if !strings.Contains(out, "not written in English") {
		t.Fatalf("turn result %s does not name the language hold", out)
	}

	out = writeAndResult(t, h, s, &rec, headOf(t, s, rec).RevisionID, "write-status", "@all\n# Harbor\n\nStatus. Revision 4 of the harbor story.\n\n"+storyFiller+twoCites)
	pub.none(t, "a story with a Status line")
	if !strings.Contains(out, "process text") {
		t.Fatalf("turn result %s does not name the process-text hold", out)
	}

	quoted := cited("The harbor reopens Monday.") + " The mayor added: \"Le pont sera fermé pendant les travaux.\""
	storyWrite(t, h, s, &rec, "write-quote", quoted)
	pub.next(t)
}
