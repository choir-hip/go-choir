package textureowner

// Station TE (docs/design/texture-evidence-embeds-te-2026-10-25.md): a
// frozen candidate's captures are embedded in its trajectory's Texture
// document by Texture's own package. These tests drive the write on the real
// store with a seeded lifecycle document; the listing is faked at the seam
// (the listing itself is tested in agentcore). Ways it could fail, written
// before the code:
//   - the embeds land in another document than the operation's trajectory
//     names, or in a document that names another trajectory;
//   - a digest the listing does not hold is embedded (membership), or more
//     captures than a bundle can carry (E4);
//   - the candidate identity that changes (bundle digest, frontend build,
//     capture receipt, resolvable) is stored in the revision;
//   - a blank capture is not marked blank (E16);
//   - a second ask writes a second set (E10);
//   - the digests are not pinned to the revision, or the pin comes after
//     the commit (E17);
//   - the revision is not Texture's (author kind and label), or does not
//     validate, or loses the document's blocks;
//   - the Texture desk's next Write against the old head commits over it
//     (E9), or a Write on the new head drops the embeds (E8);
//   - a capture that finishes after the owner's revise continued the settled
//     document onto a new trajectory is dropped, though it was admitted
//     before the continue (F1; the embed job is async and settlement does
//     not wait for it);
//   - the owner route lets a key add a capsule_evidence embed the parent
//     does not hold (E7), or refuses keeping or deleting one.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type fakeEvidenceSource struct {
	listing agentcore.VisualEvidenceListing
	err     error
	calls   int
	// anyComputer answers for any computer, as a broken resolver would.
	anyComputer bool
}

func (f *fakeEvidenceSource) CandidateVisualEvidence(_ context.Context, computerID, operationID string) (agentcore.VisualEvidenceListing, error) {
	f.calls++
	if f.err != nil {
		return agentcore.VisualEvidenceListing{}, f.err
	}
	if (!f.anyComputer && computerID != f.listing.ComputerID) || operationID != f.listing.OperationID {
		return agentcore.VisualEvidenceListing{}, errors.New("self-development operation not found")
	}
	return f.listing, nil
}

func evidenceDigest(c byte) string { return strings.Repeat(string(c), 64) }

func evidenceListing(rec types.RunRecord) agentcore.VisualEvidenceListing {
	return agentcore.VisualEvidenceListing{
		OperationID: "selfdev-op-te", TrajectoryID: rec.TrajectoryID, ComputerID: rec.ComputerID, State: "frozen",
		BundleDigest: evidenceDigest('b'), ManifestSHA256: evidenceDigest('m'), FrontendSHA256: evidenceDigest('f'),
		CaptureReceipt: "capsule-exec:sha256:" + evidenceDigest('r'),
		Captures: []agentcore.VisualEvidenceCapture{
			{Artifact: "artifact:sha256:" + evidenceDigest('1'), Digest: evidenceDigest('1'), Route: "/reader", Width: 1280, Height: 800, Bytes: 10, Resolvable: true},
			{Artifact: "artifact:sha256:" + evidenceDigest('2'), Digest: evidenceDigest('2'), Route: "/", Width: 1280, Height: 800, Bytes: 9, Blank: true, Resolvable: false},
		},
	}
}

func evidenceSetup(t *testing.T, suffix string, shape ...func(*types.Revision)) (*Handler, *agentcore.Runtime, types.RunRecord, *fakeEvidenceSource) {
	t.Helper()
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	rec := seedCellAuthorLifecycleOn(t, core.Store(), core.TextureComputerID(), "owner-evidence", suffix, "# Reader change\n\nThe candidate froze.", shape...)
	h.createAgentMutationForRun(context.Background(), &rec)
	source := &fakeEvidenceSource{listing: evidenceListing(rec)}
	h.evidenceSource = source
	return h, core, rec, source
}

func bodyOf(t *testing.T, rev types.Revision) texturedoc.StructuredTextureDoc {
	t.Helper()
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(rev.BodyDoc, &doc); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return doc
}

func TestFrozenCandidateCapturesAreEmbeddedByTexture(t *testing.T) {
	h, core, rec, source := evidenceSetup(t, "embed")
	ctx := context.Background()
	s := core.Store()
	base := headOf(t, s, rec)

	receipt, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	head := headOf(t, s, rec)
	if head.RevisionID == base.RevisionID || head.ParentRevisionID != base.RevisionID || !strings.Contains(receipt, head.RevisionID) {
		t.Fatalf("no evidence revision on the base: head %s parent %s receipt %s", head.RevisionID, head.ParentRevisionID, receipt)
	}
	if head.AuthorKind != types.AuthorAppAgent || head.AuthorLabel != "texture" {
		t.Fatalf("author = %s/%s; want Texture's", head.AuthorKind, head.AuthorLabel)
	}
	doc := bodyOf(t, head)
	if err := texturedoc.Validate(doc, nil); err != nil {
		t.Fatalf("evidence revision does not validate: %v", err)
	}
	// The base is the owner's text; Texture reads it as its markdown blocks,
	// as a desk's Write does.
	baseDoc, _, err := texturewrite.Base(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Doc.Content) != len(baseDoc.Doc.Content)+2 {
		t.Fatalf("blocks %d, want the base's %d and two embeds", len(doc.Doc.Content), len(baseDoc.Doc.Content))
	}
	for i := range baseDoc.Doc.Content {
		before, _ := json.Marshal(baseDoc.Doc.Content[i])
		after, _ := json.Marshal(doc.Doc.Content[i])
		if string(before) != string(after) {
			t.Fatalf("block %d changed: %s -> %s", i, before, after)
		}
	}
	embeds := texturedoc.Embeds(doc)
	if len(embeds) != 2 {
		t.Fatalf("embeds = %+v", embeds)
	}
	for i, e := range embeds {
		row := source.listing.Captures[i]
		if e.Kind != texturedoc.EmbedKindCapsuleEvidence || e.Digest != row.Digest || e.OperationID != "selfdev-op-te" ||
			e.ComputerID != rec.ComputerID || e.Route != row.Route || e.Width != 1280 || e.Height != 800 || e.Blank != row.Blank {
			t.Fatalf("embed %d = %+v, want capture %+v", i, e, row)
		}
	}
	if !strings.Contains(embeds[0].Caption, "/reader") || strings.Contains(strings.ToLower(embeds[0].Caption), "blank") ||
		!strings.Contains(embeds[1].Caption, "Blank screenshot") || embeds[1].Alt == "" {
		t.Fatalf("captions = %q / %q, alt %q", embeds[0].Caption, embeds[1].Caption, embeds[1].Alt)
	}
	for _, unstored := range []string{evidenceDigest('b'), evidenceDigest('f'), evidenceDigest('r'), "resolvable", "bundle_digest", "frontend_sha256", "capture_receipt"} {
		if strings.Contains(string(head.BodyDoc), unstored) {
			t.Fatalf("the revision stores %q, which re-resolves live", unstored)
		}
	}
	if !strings.Contains(head.Content, "[evidence: ") {
		t.Fatalf("projection lost the evidence: %q", head.Content)
	}
	var meta map[string]any
	_ = json.Unmarshal(head.Metadata, &meta)
	assets, _ := meta[textureEmbeddedAssetsMetadataKey].([]any)
	if len(assets) != 2 || assets[0] != evidenceDigest('1') {
		t.Fatalf("metadata %s = %v", textureEmbeddedAssetsMetadataKey, meta[textureEmbeddedAssetsMetadataKey])
	}
	for _, digest := range []string{evidenceDigest('1'), evidenceDigest('2')} {
		pins, err := core.AssetStore().Pins(digest)
		if err != nil || len(pins) != 1 || pins[0] != "texture-revision:"+rec.ChannelID+":"+head.RevisionID {
			t.Fatalf("pins of %s = %v, %v", digest[:4], pins, err)
		}
	}

	// A second ask writes nothing.
	if _, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("second ask: %v", err)
	}
	if again := headOf(t, s, rec); again.RevisionID != head.RevisionID {
		t.Fatalf("a second ask wrote revision %s", again.RevisionID)
	}

	// The Texture desk's Write against the old head is refused; on the new
	// head it writes around the embeds.
	stale := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@insert end\nA note.", Why: "note"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, stale, "stale-1"); err == nil || !strings.Contains(err.Error(), "changed since your frame") {
		t.Fatalf("a Write on the old head: %v", err)
	}
	fresh := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@insert after b2\nThe screenshots follow.", Why: "introduce the evidence"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, fresh, "fresh-1"); err != nil {
		t.Fatalf("a Write around the embeds: %v", err)
	}
	if got := texturedoc.Embeds(bodyOf(t, headOf(t, s, rec))); len(got) != 2 || got[0] != embeds[0] || got[1] != embeds[1] {
		t.Fatalf("embeds after the desk's Write = %+v", got)
	}
}

// An embed job outlives the trajectory it was started on: the episode
// settles, the owner's next revise continues the document, and the capture
// lands afterwards. It is committed on the document's current trajectory.
func TestEvidenceEmbedLandsAfterTheDocumentContinued(t *testing.T) {
	h, core, rec, _ := evidenceSetup(t, "continued")
	ctx := context.Background()
	s := core.Store()
	term := types.TerminalizeRunRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "terminalize-evidence-run", TrajectoryID: rec.TrajectoryID,
		AgentID: rec.AgentID, RunID: rec.RunID, TerminalState: types.RunCompleted, Reason: "done",
	}
	term.CommandDigest, _ = store.ComputeTerminalizeRunDigest(term)
	if _, err := s.TerminalizeRun(ctx, term); err != nil {
		t.Fatalf("terminalize run: %v", err)
	}
	settleDocumentTrajectory(t, s, rec.OwnerID, rec.ComputerID, rec.ChannelID, rec.TrajectoryID)
	settledDoc, err := s.GetLifecycleDocument(ctx, rec.OwnerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	settledSnapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	current, _, err := h.continueSettledDocumentForOwner(ctx, rec.OwnerID, settledDoc, settledSnapshot)
	if err != nil || current.TrajectoryID == rec.TrajectoryID {
		t.Fatalf("continue: trajectory %q err=%v", current.TrajectoryID, err)
	}
	base := headOf(t, s, rec)

	// The listing still names the trajectory the candidate was frozen on.
	if _, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("embed after the continue: %v", err)
	}
	head := headOf(t, s, rec)
	if head.RevisionID == base.RevisionID || head.ParentRevisionID != base.RevisionID || head.TrajectoryID != current.TrajectoryID {
		t.Fatalf("head %s parent %s trajectory %s; want an evidence revision on %s over %s", head.RevisionID, head.ParentRevisionID, head.TrajectoryID, current.TrajectoryID, base.RevisionID)
	}
	if got := texturedoc.Embeds(bodyOf(t, head)); len(got) != 2 {
		t.Fatalf("embeds = %+v", got)
	}
}

func TestEvidenceEmbedRefusals(t *testing.T) {
	ctx := context.Background()
	cases := map[string]func(*fakeEvidenceSource){
		"listing refused": func(f *fakeEvidenceSource) { f.err = errors.New("the candidate carries no visual evidence") },
		"other trajectory": func(f *fakeEvidenceSource) {
			f.listing.TrajectoryID = "trajectory-someone-else"
		},
		"malformed digest": func(f *fakeEvidenceSource) { f.listing.Captures[0].Digest = "not-a-digest" },
		"too many captures": func(f *fakeEvidenceSource) {
			for len(f.listing.Captures) <= 6 {
				f.listing.Captures = append(f.listing.Captures, f.listing.Captures[0])
			}
		},
		"no captures": func(f *fakeEvidenceSource) { f.listing.Captures = nil },
		"another computer's listing": func(f *fakeEvidenceSource) {
			f.listing.ComputerID = "computer-elsewhere"
			f.anyComputer = true
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h, core, rec, source := evidenceSetup(t, "refuse-"+strings.ReplaceAll(name, " ", "-"))
			mutate(source)
			base := headOf(t, core.Store(), rec)
			if _, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err == nil {
				t.Fatal("embedded")
			}
			if head := headOf(t, core.Store(), rec); head.RevisionID != base.RevisionID {
				t.Fatalf("a refused embed wrote %s", head.RevisionID)
			}
		})
	}
}

func TestOwnerRouteCannotAddEvidenceEmbeds(t *testing.T) {
	h, core, rec, _ := evidenceSetup(t, "owner-route")
	ctx := context.Background()
	s := core.Store()
	if _, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatal(err)
	}
	post := func(key string, mutate func(*texturedoc.StructuredTextureDoc)) *httptest.ResponseRecorder {
		t.Helper()
		head := headOf(t, s, rec)
		doc := bodyOf(t, head)
		mutate(&doc)
		projection, err := texturedoc.Project(doc, nil)
		if err != nil {
			t.Fatalf("test document invalid: %v", err)
		}
		snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]any{
			"content": projection.Text, "body_doc": doc, "parent_revision_id": head.RevisionID,
			"idempotency_key": key, "expected_lifecycle_version": snapshot.Trajectory.LifecycleVersion,
		})
		w := httptest.NewRecorder()
		req := authenticatedRequest(http.MethodPost, "/api/texture/documents/"+rec.ChannelID+"/revisions", string(body), rec.OwnerID)
		h.HandleTextureRevisions(w, req)
		return w
	}
	forged := func(doc *texturedoc.StructuredTextureDoc) {
		n := doc.Doc.Content[len(doc.Doc.Content)-1]
		raw, _ := json.Marshal(n)
		var copyNode texturedoc.Node
		_ = json.Unmarshal(raw, &copyNode)
		copyNode.Attrs["id"] = "embed-forged"
		copyNode.Attrs["target"].(map[string]any)["digest"] = evidenceDigest('9')
		doc.Doc.Content = append(doc.Doc.Content, copyNode)
	}
	if w := post("forge-1", forged); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "embed-forged") {
		t.Fatalf("a forged evidence embed: %d %s", w.Code, w.Body.String())
	}
	image := func(doc *texturedoc.StructuredTextureDoc) {
		doc.Doc.Content = append(doc.Doc.Content, texturedoc.Node{Type: "embed", Attrs: map[string]any{
			"id": "embed-image", "kind": texturedoc.EmbedKindImage, "target": map[string]any{"digest": evidenceDigest('8')}, "alt": "Any stored image",
		}})
	}
	if w := post("image-1", image); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "embed-image") {
		t.Fatalf("an owner-added image embed: %d %s", w.Code, w.Body.String())
	}
	altered := func(doc *texturedoc.StructuredTextureDoc) {
		doc.Doc.Content[len(doc.Doc.Content)-1].Attrs["caption"] = "Screenshot of a page the candidate never rendered"
	}
	if w := post("alter-1", altered); w.Code != http.StatusBadRequest {
		t.Fatalf("an altered evidence embed: %d %s", w.Code, w.Body.String())
	}
	moved := func(doc *texturedoc.StructuredTextureDoc) {
		n := len(doc.Doc.Content)
		doc.Doc.Content[n-1], doc.Doc.Content[n-2] = doc.Doc.Content[n-2], doc.Doc.Content[n-1]
	}
	if w := post("move-1", moved); w.Code != http.StatusCreated {
		t.Fatalf("an owner moving an embed: %d %s", w.Code, w.Body.String())
	}
	keep := func(doc *texturedoc.StructuredTextureDoc) {
		doc.Doc.Content[1].Content = []texturedoc.Node{{Type: "text", Text: "The owner edited this line."}}
	}
	if w := post("keep-1", keep); w.Code != http.StatusCreated {
		t.Fatalf("an owner edit keeping the embeds: %d %s", w.Code, w.Body.String())
	}
	drop := func(doc *texturedoc.StructuredTextureDoc) {
		doc.Doc.Content = doc.Doc.Content[:len(doc.Doc.Content)-1]
	}
	if w := post("drop-1", drop); w.Code != http.StatusCreated {
		t.Fatalf("an owner deleting an embed: %d %s", w.Code, w.Body.String())
	}
	if got := texturedoc.Embeds(bodyOf(t, headOf(t, s, rec))); len(got) != 1 {
		t.Fatal(fmt.Sprintf("embeds after the owner's delete = %d, want 1", len(got)))
	}
}

// Review bunch 231 (Devin): a route Texture interpolates into its words must
// not trip the authoring rules (a "[12]" is a legacy citation there), or the
// write refuses and the evidence is lost.
func TestABracketedRouteIsEmbeddedWithSafeWords(t *testing.T) {
	h, core, rec, source := evidenceSetup(t, "bracketed")
	source.listing.Captures[0].Route = "/items[12]/{{source:x}}"
	if _, err := h.EmbedCandidateEvidence(context.Background(), rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("a bracketed route: %v", err)
	}
	embeds := texturedoc.Embeds(bodyOf(t, headOf(t, core.Store(), rec)))
	if len(embeds) != 2 || embeds[0].Route != "/items[12]/{{source:x}}" {
		t.Fatalf("embeds = %+v; want the route kept as captured", embeds)
	}
	for _, words := range []string{embeds[0].Caption, embeds[0].Alt} {
		if strings.ContainsAny(words, "[]{}") || !strings.Contains(words, "/items") {
			t.Fatalf("Texture's words %q carry markup from the route", words)
		}
	}
}

// Review bunch 231 (DeepSeek): a publish-auto story whose document holds
// evidence is held with the reason, and the platform is never asked.
func TestAStoryHoldingEvidenceIsHeldNotPublished(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "evidence", types.CastPublishAuto)
	storyWrite(t, h, s, &rec, "write-1", cited("The harbor reopens Monday."))
	pub.next(t)
	h.evidenceSource = &fakeEvidenceSource{listing: evidenceListing(rec)}
	if _, err := h.EmbedCandidateEvidence(context.Background(), rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("embed: %v", err)
	}
	head := headOf(t, s, rec)
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@insert end\n" + cited("Ferries resume Tuesday."), Why: "add the ferries",
	}})
	receipt, err := h.CommitCellTextureAuthor(context.Background(), &rec, turn, "write-2")
	if err != nil {
		t.Fatalf("Write on the evidence revision: %v", err)
	}
	if !strings.Contains(receipt, "publish_held") || !strings.Contains(receipt, "private embed") {
		t.Fatalf("receipt %s; want the publish held for the evidence", receipt)
	}
	pub.none(t, "a story holding evidence")
}
