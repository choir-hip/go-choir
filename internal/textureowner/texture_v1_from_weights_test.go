package textureowner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// V1 is a from-weights answer, marked as one (owner, 2026-11-16: "V1 should
// be a fast response from texture model weights. V2 should be after first
// search round research message to texture";
// docs/problems/texture-story-ask-path-writes-no-fast-v1-and-research-
// delivers-once-2026-11-16.md, change 2). Failure list, written before the
// fix; the first Write of a prompt document is v1:
//
//  1. Every later revision of a prompt document (v2, v3, ...) carries
//     model_prior_interim, because input_origin and texture_version_stage
//     ride forward from the parent; the reader cannot tell the from-weights
//     v1 from a revision written from evidence.
//  2. A first Write that cites a captured source (the desk cast first, wrote
//     once research had answered) is stamped as from-weights, though it is
//     grounded.
//  3. The from-weights v1 itself must keep the stamp: grounding_status,
//     revision_grounding and model_prior_interim.

func promptSeed(r *types.Revision) {
	r.Metadata = json.RawMessage(`{"input_origin":"user_prompt","revision_role":"input","seed_prompt":"History of SoftBank","source":"user_prompt"}`)
}

func promptDocSetup(t *testing.T, suffix string) (*Handler, *store.Store, types.RunRecord) {
	t.Helper()
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	rec := seedCellAuthorLifecycleOn(t, s, core.TextureComputerID(), "owner-v1-weights", suffix, "# Merger\n\nOpening line.\n\nClosing line.", promptSeed)
	h.createAgentMutationForRun(context.Background(), &rec)
	return h, s, rec
}

func revisionMeta(t *testing.T, rev types.Revision) map[string]any {
	t.Helper()
	return decodeRevisionMetadata(rev.Metadata)
}

// Owner, 2026-11-16: "v1 should be from model priors, with uncertainty noted".
// A new Texture's first turn commits a model-prior v1 in the same transaction
// as the Cast, before any research message exists.
func TestANewTexturesFirstTurnCommitsAModelPriorV1BeforeAnyResearchMessageExists(t *testing.T) {
	h, s, rec := promptDocSetup(t, "v1-stamp")
	base := headOf(t, s, rec)
	v1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "research on the ruling", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:v1")},
		Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nFrom what I know, a court ruled that a merger may proceed; this is from memory and not yet sourced.", Why: "fast answer from weights"}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, v1, "v1"); err != nil {
		t.Fatalf("v1: %v", err)
	}
	meta := revisionMeta(t, headOf(t, s, rec))
	if meta["model_prior_interim"] != true || meta["grounding_status"] != "model_prior_interim" || meta["revision_grounding"] != "model_prior" {
		t.Fatalf("v1 is not marked as from the model's own knowledge: %#v", meta)
	}
	if head := headOf(t, s, rec); head.RevisionID == base.RevisionID {
		t.Fatal("the first turn committed no v1")
	}
	ctx := context.Background()
	if head, err := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID); err != nil || head != 0 {
		t.Fatalf("Texture's inbox head = %d, %v: v1 must commit before any research message exists", head, err)
	}
	if head, err := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, "research:"+rec.ChannelID); err != nil || head != 1 {
		t.Fatalf("research's inbox head = %d, %v: the Cast commits with v1", head, err)
	}
}

func TestLaterRevisionsAreNotMarkedModelPrior(t *testing.T) {
	h, s, rec := promptDocSetup(t, "v2-clear")
	ctx := context.Background()
	base := headOf(t, s, rec)
	v1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "research on the ruling", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:v2")},
		Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nFrom memory, not yet sourced: a court ruled on a merger.", Why: "fast answer from weights"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, v1, "v1"); err != nil {
		t.Fatalf("v1: %v", err)
	}
	head := headOf(t, s, rec)
	v2 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Wait: "more research", Write: &yaegikernel.TextureWrite{
		Edits: "@insert after b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.", Why: "first research message"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, v2, "v2"); err != nil {
		t.Fatalf("v2: %v", err)
	}
	meta := revisionMeta(t, headOf(t, s, rec))
	for _, key := range []string{"model_prior_interim", "grounding_status", "revision_grounding"} {
		if _, present := meta[key]; present {
			t.Fatalf("v2 carries %s: %#v", key, meta)
		}
	}
	if meta["texture_version_stage"] == "interim" {
		t.Fatalf("v2 inherited texture_version_stage=interim: %#v", meta)
	}
}

func TestAFirstWriteThatCitesAnEvidenceIsNotMarkedModelPrior(t *testing.T) {
	h, s, rec := promptDocSetup(t, "first-cited")
	base := headOf(t, s, rec)
	cited := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "more research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:cited")},
		Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.", Why: "from the first message"}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, cited, "cited"); err != nil {
		t.Fatalf("cited first write: %v", err)
	}
	meta := revisionMeta(t, headOf(t, s, rec))
	if _, present := meta["model_prior_interim"]; present || meta["grounding_status"] == "model_prior_interim" {
		t.Fatalf("a revision that cites a captured source is marked from-weights: %#v", meta)
	}
}

// A first uncited Write that arrives after research has messaged is written
// from those messages, so it is not "from weights".
func TestAFirstWriteAfterResearchMessagedIsNotFromWeights(t *testing.T) {
	h, s, rec := promptDocSetup(t, "late-first")
	base := headOf(t, s, rec)
	seedTextureReport(t, s, rec, "research-cell:report:before-v1")
	late := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "more", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:late")},
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: rec.AgentID, Inbox: rec.AgentID, ExpectedSeq: 0, Seq: 1},
		Write:       &yaegikernel.TextureWrite{Edits: "@replace b2\nWritten after research messaged, no cite.", Why: "late first write"}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, late, "late-first"); err != nil {
		t.Fatalf("late first write: %v", err)
	}
	if meta := revisionMeta(t, headOf(t, s, rec)); meta["model_prior_interim"] != nil {
		t.Fatalf("a revision written after research messaged is marked from weights: %#v", meta)
	}
}
