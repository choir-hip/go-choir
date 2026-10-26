package textureowner

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Texture names the owner's request by its revision so a cast carries it
// verbatim (docs/problems/texture-retypes-owner-directions-2026-10-25.md,
// chosen fix). Ways it can fail, written before the code: the request
// turn prints the original request with no revision id, so a cell can only
// retype it; the id is not the document's first revision; the id prints
// where no root revision was read (a seed carried in metadata names none).
func TestOriginalRequestNamesItsRevision(t *testing.T) {
	prompt := wakeAfterTheRequestWasRewritten(t, "ids", holdSwitches{hold: true, directions: true}).Prompt
	want := originalLabel + holdRequest + "\n(revision revision-hold-root-ids)"
	if !strings.Contains(prompt, want) {
		t.Fatalf("the original request does not name its revision; want %q in:\n%s", want, prompt)
	}

	off := wakeAfterTheRequestWasRewritten(t, "ids-off", holdSwitches{directions: true}).Prompt
	if strings.Contains(off, "revision-hold-root-ids-off)") {
		t.Fatalf("a request read from no root revision names one:\n%s", off)
	}
}

// A seeded document (prompt bar, handoff) carries its request as seed_prompt
// on an owner-written first revision; the request turn names that revision
// too, with or without the owner-directions walk (F2, review 272). Ways it
// can fail: the walk to the root runs only without a seed, so a seeded
// request has no id; the walk replaces the seed's text with the root's body.
func TestSeededRequestNamesItsRevision(t *testing.T) {
	for _, tc := range []struct {
		suffix, label string
		directions    bool
	}{
		{"seed-ids-on", originalLabel, true},
		{"seed-ids-off", "The owner's request:\n", false},
	} {
		prompt := wakeAfterOwnerReviseAndAgentRevision(t, tc.suffix, tc.directions)
		want := tc.label + directionsSeed + "\n(revision revision-directions-seed-" + tc.suffix + ")"
		if !strings.Contains(prompt, want) {
			t.Fatalf("the seeded request does not name its revision; want %q in:\n%s", want, prompt)
		}
	}
}

// A passivated run reactivated on a later head carries that head's author
// kind with its id (F1, review 272): the frame names the owner revision from
// the pair, and a stale author kind either hid a new owner revision (the
// retype returns) or named a Texture revision (the cast is refused). Ways it
// can fail: reactivation moves current_revision_id and leaves
// current_author_kind as the run was created.
func TestReactivatedRunCarriesTheHeadsAuthorKind(t *testing.T) {
	w := storyWaitingOnResearch(t, "idle")
	s := w.core.Store()
	ctx := t.Context()
	// The run as a reactivation left it before the fix: the head is the
	// owner's revise, the author kind is stale.
	rec, err := s.GetLifecycleRun(ctx, w.owner, w.doc.ComputerID, w.r1.RunID)
	if err != nil {
		t.Fatal(err)
	}
	rec.Metadata["current_author_kind"] = string(types.AuthorAppAgent)
	req := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "stale-author-kind", TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: rec}
	req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
	if _, err := s.ReplaceLifecycleActivation(ctx, req); err != nil {
		t.Fatalf("stale the run: %v", err)
	}
	researchCompletes(t, w)
	commitResearchReport(t, w)
	woken, err := w.handler.ReconcileAgentWake(ctx, w.owner, w.doc.DocID)
	if err != nil || woken == nil || woken.RunID != w.r1.RunID {
		t.Fatalf("report wake = %+v, %v; want %s reactivated", woken, err, w.r1.RunID)
	}
	head, err := s.GetLifecycleDocument(ctx, w.owner, w.doc.ComputerID, w.doc.DocID)
	if err != nil {
		t.Fatal(err)
	}
	headRev, err := s.GetLifecycleRevision(ctx, w.owner, w.doc.ComputerID, head.CurrentRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if got, kind := metadataStringValue(woken.Metadata, "current_revision_id"), metadataStringValue(woken.Metadata, "current_author_kind"); got != headRev.RevisionID || kind != string(headRev.AuthorKind) {
		t.Fatalf("reactivated run names %s by %s; want the head %s by %s", got, kind, headRev.RevisionID, headRev.AuthorKind)
	}
}
