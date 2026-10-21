package texturewrite

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

// docs/problems/tool-call-failures-dual-model-2026-10-21.md, T5: Texture
// cited a capture by its full id with a chunk of the owner segment missing,
// and was told the source "is not captured" and to ask research to Read it.
// Failure modes:
//  1. a cited id whose key-<hash> suffix names exactly one frame source is
//     refused instead of citing that source;
//  2. an id whose hash matches no frame source is accepted (the match must
//     be the whole unique suffix, never a fuzzy one);
//  3. a refusal for an uncaptured id does not name the frame handles that
//     can be cited.

const (
	nearMissCapture = "obj:choir.web_capture:ZTU5NjIzMTAtODc2Ny00NjI0LTg3ZTUtYTliOTNhNGUxZGIx:key-a36e1b4f478a6dbfded4636026e08435fc81241ed6b743208bcd8dfc9d9189bf"
	nearMissDropped = "obj:choir.web_capture:ZTU5NjIzMTAtODc2Ny00NjI0LTg3ZTUtYTliOTNhNGUx:key-a36e1b4f478a6dbfded4636026e08435fc81241ed6b743208bcd8dfc9d9189bf"
)

func nearMissInput(script string) Input {
	caps := newFakeCaptures()
	caps.captures[nearMissCapture] = Capture{
		Doc: "cap-lander", Version: "v1", URL: "https://example.org/lander", Title: "Lander freed",
		Blocks: []quotematch.Block{{ID: "c1", Text: "The candidate was freed after a day in detention."}},
	}
	return Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-nm",
		Base:     baseDoc(heading("h1", 1, "Lander"), paragraph("p1", "Opening line.")),
		Script:   script,
		Resolver: caps,
		Handles: map[string]Source{
			"r1.c1": {Handle: "r1.c1", CaptureID: nearMissCapture},
			"r1.c2": {Handle: "r1.c2", URL: "https://example.org/court"},
		},
	}
}

func TestCitedCaptureIDWithADroppedChunkCitesTheFrameSource(t *testing.T) {
	script := "@replace b2\nHe was freed {{cite " + nearMissDropped + " \"freed after a day in detention\"}}."
	res, err := Apply(context.Background(), nearMissInput(script))
	if err != nil {
		t.Fatalf("near-miss capture id refused: %v", err)
	}
	refs := sourceRefs(res.Doc.Doc.Content[1])
	if len(refs) != 1 {
		t.Fatalf("source refs = %+v, want one", refs)
	}
	var found bool
	for _, e := range res.Entities {
		if e.SourceEntityID == refs[0].Attrs["source_entity_id"] && strings.Contains(e.Target.ID+e.Target.URI, "lander") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the citation does not name the frame's capture: refs %+v entities %+v", refs, res.Entities)
	}
}

func TestCitedCaptureIDWithAnotherHashIsRefusedWithTheFrameHandles(t *testing.T) {
	wrong := strings.Replace(nearMissDropped, "key-a36e", "key-b36e", 1)
	script := "@replace b2\nHe was freed {{cite " + wrong + " \"freed after a day in detention\"}}."
	_, err := Apply(context.Background(), nearMissInput(script))
	if err == nil {
		t.Fatal("an id with another hash was cited")
	}
	for _, want := range []string{"not captured", "r1.c1", "r1.c2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal lacks %q: %v", want, err)
		}
	}
}
