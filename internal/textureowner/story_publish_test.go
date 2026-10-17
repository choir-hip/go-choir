package textureowner

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Automatic publishing (Cut 2 2b, W3). Ways it can fail, written before the
// code:
//   - a committed revision of a publish-auto story never publishes, or
//     publishes under a number other than its Texture revision number;
//   - the publish runs on the commit's path, so a slow or failing platform
//     fails or stalls Texture's turn;
//   - a failed publish is retried into a later slot, or the next revision
//     is renumbered to fill the gap;
//   - a story opened without publish auto, or the empty seed, publishes;
//   - a crash boot publishes a revision committed before the crash (a
//     boot-time resumer);
//   - a Write inherits the seed's revision_role (story_seed) through the
//     durable metadata carried forward from the parent revision, so every
//     revision looks like the seed and none publishes (B12, staging
//     c2-paper-3). This only shows when the story lives on the runtime's
//     own computer, as it does deployed: the carry-forward reads the parent
//     by the runtime's computer id.

type recordingStoryPublisher struct {
	mu       sync.Mutex
	versions []agentcore.StoryVersion
	fail     map[int]bool
	done     chan agentcore.StoryVersion
}

func newRecordingStoryPublisher() *recordingStoryPublisher {
	return &recordingStoryPublisher{fail: map[int]bool{}, done: make(chan agentcore.StoryVersion, 16)}
}

func (p *recordingStoryPublisher) PublishVersion(_ context.Context, v agentcore.StoryVersion) error {
	p.mu.Lock()
	p.versions = append(p.versions, v)
	fail := p.fail[v.Revision]
	p.mu.Unlock()
	p.done <- v
	if fail {
		return errors.New("platform unavailable")
	}
	return nil
}

func (p *recordingStoryPublisher) PublishVerdicts(context.Context, types.StoryVerdictsRequest) error {
	return nil
}

func (p *recordingStoryPublisher) next(t *testing.T) agentcore.StoryVersion {
	t.Helper()
	select {
	case v := <-p.done: // hang-guard: bounded by the time.After case below
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("no publish within 10s")
	}
	return agentcore.StoryVersion{}
}

func (p *recordingStoryPublisher) none(t *testing.T, label string) {
	t.Helper()
	select {
	case v := <-p.done: // hang-guard: bounded by the time.After case below
		t.Fatalf("%s published %+v", label, v)
	case <-time.After(300 * time.Millisecond):
	}
}

// openStoryForTexture opens a story the way management does (a Cast to
// texture:new) and projects a Texture records-route run onto it.
func openStoryForTexture(t *testing.T, h *Handler, s *store.Store, suffix string, publish string) types.RunRecord {
	t.Helper()
	return openStoryWithObjective(t, h, s, suffix, publish, "Harbor reopens "+suffix)
}

// openStoryWithObjective opens a story whose opening Cast carries objective.
func openStoryWithObjective(t *testing.T, h *Handler, s *store.Store, suffix, publish, objective string) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	// The story lives on the runtime's own computer, as it does deployed
	// (B12: a story on another computer hid the seed-role carry-forward).
	owner, computer := "owner-story-"+suffix, h.Core.TextureComputerID()
	management := agentprofile.Management + ":" + owner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-open-" + suffix + ":cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: objective},
		Cast:      &types.CommitmentCast{Captures: []string{"cap-court"}, Publish: publish},
		Inbox:     types.StoryOpenInbox, SenderInbox: management, Addressee: types.StoryOpenInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:open-" + suffix, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil || len(receipt.Records) != 1 || !receipt.Records[0].OpenedStory {
		t.Fatalf("open story: %+v, %v", receipt, err)
	}
	textureAgentID := receipt.Records[0].Inbox
	docID := strings.TrimPrefix(textureAgentID, agentprofile.Texture+":")
	doc, err := s.GetLifecycleDocument(ctx, owner, computer, docID)
	if err != nil {
		t.Fatalf("story document: %v", err)
	}
	now := time.Now().UTC()
	caller := types.RunRecord{
		RunID: "texture-run-story-" + suffix, OwnerID: owner, ComputerID: computer,
		AgentID: textureAgentID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		ChannelID: docID, TrajectoryID: doc.TrajectoryID, State: types.RunRunning,
		Metadata:  map[string]any{"doc_id": docID, textureRecordsRouteMetadata: true, "request_source": "inbox_record"},
		CreatedAt: now, UpdatedAt: now,
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "project-story-" + suffix,
		TrajectoryID: doc.TrajectoryID, AgentID: textureAgentID, Run: caller,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project Texture run: %v", err)
	}
	h.createAgentMutationForRun(ctx, &caller)
	return caller
}

func storyWrite(t *testing.T, h *Handler, s *store.Store, rec *types.RunRecord, identity, text string) types.Revision {
	t.Helper()
	base := headOf(t, s, *rec)
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@all\n# Harbor\n\n" + text, Why: "report the harbor",
	}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), rec, turn, identity); err != nil {
		t.Fatalf("Write %s: %v", identity, err)
	}
	head := headOf(t, s, *rec)
	if head.RevisionID == base.RevisionID {
		t.Fatalf("Write %s committed nothing", identity)
	}
	return head
}

// cited appends sentences citing captured sources from two outlets: a
// publish-auto story publishes only revisions whose citations span two
// (docs/problems/capture-and-production-are-coupled-2026-10-16.md).
func cited(text string) string {
	return text + "\n\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}." +
		" The regulator will not appeal {{cite https://wire.example.net/merger \"the regulator said it will not appeal\"}}."
}

// citedOnce cites only the court capture: one outlet.
func citedOnce(text string) string {
	return text + "\n\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}."
}

var wireCapture = texturewrite.Capture{
	Doc: "cap-wire", Version: "doc-hash-wire", URL: "https://wire.example.net/merger", Title: "Regulator response",
	Blocks: []quotematch.Block{{ID: "cid-w1", Text: "The regulator said it will not appeal the ruling."}},
}

// sameOutletCapture is a second page of the court's outlet.
var sameOutletCapture = texturewrite.Capture{
	Doc: "cap-court-2", Version: "doc-hash-court-2", URL: "https://news.example.org/merger-reaction", Title: "Reaction",
	Blocks: []quotematch.Block{{ID: "cid-s1", Text: "Shares rose after the ruling."}},
}

// storyCaptures resolves captures by URL (Write) and by capture id (the
// publish gate reads a citation's Ref.Doc), as the capture store does.
func storyCaptures() fakeCaptureStore {
	court := courtCapture["https://example.org/court"]
	out := fakeCaptureStore{}
	for _, c := range []texturewrite.Capture{court, wireCapture, sameOutletCapture} {
		out[c.URL], out[c.Doc] = c, c
	}
	return out
}

func revisionMetadataString(rev types.Revision, key string) string {
	return metadataStringValue(decodeRevisionMetadata(rev.Metadata), key)
}

func storySetup(t *testing.T) (*agentcore.Runtime, *Handler, *store.Store, *recordingStoryPublisher) {
	t.Helper()
	core, h := testAPISetup(t)
	core.SetTextureCaptures(storyCaptures())
	pub := newRecordingStoryPublisher()
	core.SetStoryPublisher(pub)
	return core, h, core.Store(), pub
}

func TestRevisionOfAPublishAutoStoryPublishesAsItsVersion(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "auto", types.CastPublishAuto)
	seed := headOf(t, s, rec)

	first := storyWrite(t, h, s, &rec, "write-1", cited("The harbor reopens Monday."))
	v := pub.next(t)
	if v.DocID != rec.ChannelID || v.RevisionID != first.RevisionID || v.Revision != types.RevisionNumber(first.VersionNumber) || v.Revision != 2 || v.OwnerID != rec.OwnerID {
		t.Fatalf("published %+v; want revision %s as v2 (the seed is revision 1)", v, first.RevisionID)
	}
	if v.RevisionID == seed.RevisionID {
		t.Fatal("the seed published")
	}
	if role := revisionMetadataString(first, "revision_role"); role == types.StorySeedRole {
		t.Fatalf("Texture's revision %s carries the seed's role %q", first.RevisionID, role)
	}
	second := storyWrite(t, h, s, &rec, "write-2", cited("The harbor reopens Monday. Ferries resume Tuesday."))
	if v := pub.next(t); v.RevisionID != second.RevisionID || v.Revision != 3 {
		t.Fatalf("second publish = %+v; want v3", v)
	}
	pub.none(t, "two revisions")
}

func TestFailedStoryPublishLeavesAGapAndNothingRenumbers(t *testing.T) {
	_, h, s, pub := storySetup(t)
	pub.fail[2] = true
	rec := openStoryForTexture(t, h, s, "gap", types.CastPublishAuto)

	// The Write commits even though its publish fails: publishing is off
	// the commit's path.
	storyWrite(t, h, s, &rec, "write-1", cited("The harbor reopens Monday."))
	if v := pub.next(t); v.Revision != 2 {
		t.Fatalf("first publish = %+v", v)
	}
	third := storyWrite(t, h, s, &rec, "write-2", cited("The harbor reopens Monday. Ferries resume."))
	if v := pub.next(t); v.Revision != 3 || v.RevisionID != third.RevisionID {
		t.Fatalf("after a failed v2, the next revision published as %+v; want v3 (v2 stays a gap)", v)
	}
	pub.none(t, "a failed publish (no retry)")
}

// With research failed, Texture wrote revisions that cite nothing and say
// the claims are unverified, and auto publish published them as the
// story's next version (docs/problems/free-model-429-opens-provider-
// circuit-stub-stories-2026-10-14.md, cause 4). Ways the fix can fail: an
// uncited revision still publishes; a cited one stops publishing; the held
// revision leaves no reason anywhere; the next cited revision is
// renumbered into the held slot; a story without publish auto starts
// publishing.
func TestUncitedRevisionIsHeldAndTheNextCitedOnePublishes(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "uncited", types.CastPublishAuto)

	var logs strings.Builder
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	base := headOf(t, s, rec)
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@all\n# Harbor\n\nThe official page could not be retrieved, so the reopening is unverified.", Why: "research failed",
	}})
	out, err := h.CommitCellTextureAuthor(context.Background(), &rec, turn, "write-uncited")
	if err != nil {
		t.Fatalf("uncited Write: %v", err)
	}
	stub := headOf(t, s, rec)
	if stub.RevisionID == base.RevisionID {
		t.Fatal("the uncited Write committed nothing; it must commit and only not publish")
	}
	pub.none(t, "a revision that cites no captured source")
	if !strings.Contains(logs.String(), "story publish: "+rec.ChannelID+" revision "+stub.RevisionID+" not published: it cites no captured source") {
		t.Fatalf("no reason logged for the held revision; logs:\n%s", logs.String())
	}
	if !strings.Contains(out, `"publish_held"`) || !strings.Contains(out, "cites no captured source") {
		t.Fatalf("turn result %s does not tell Texture the revision was held", out)
	}

	next := storyWrite(t, h, s, &rec, "write-cited", cited("The harbor reopens Monday."))
	v := pub.next(t)
	if v.RevisionID != next.RevisionID || v.Revision != types.RevisionNumber(next.VersionNumber) || v.Revision != 3 {
		t.Fatalf("published %+v; want the cited revision %s as v3 (v2, the uncited one, stays a gap)", v, next.RevisionID)
	}
	pub.none(t, "after the cited revision")
}

func TestStoryWithoutPublishAutoPublishesNothing(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "manual", "")
	storyWrite(t, h, s, &rec, "write-1", cited("The harbor reopens Monday."))
	pub.none(t, "a story without publish auto")
}

func TestCrashBootPublishesNothing(t *testing.T) {
	core, h, s, pub := storySetup(t)
	pub.fail[2] = true
	rec := openStoryForTexture(t, h, s, "crash", types.CastPublishAuto)
	storyWrite(t, h, s, &rec, "write-1", cited("The harbor reopens Monday."))
	if v := pub.next(t); v.Revision != 2 {
		t.Fatalf("publish = %+v", v)
	}
	// A crash: the next boot has no planned-restart marker. Revision 2
	// committed and never published; no boot step publishes it.
	agentcore.WithBootRestart(agentcore.PlannedRestart{}, false)(core)
	core.Start(context.Background())
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("texture owner boot: %v", err)
	}
	pub.none(t, "a crash boot")
}

// Management opened stories with a writing brief as the Cast's objective
// ("Write the story of Switzerland proposing to ban ..."), the store made
// it the title, nothing retitled the story, and every version published
// under it, while Texture's own headline sat unread in the body
// (docs/problems/instruction-text-published-as-story-titles-2026-10-14.md).
// Ways the fix can fail, written first:
//   - a heading-led revision leaves the brief as the title;
//   - a headingless revision (a V1 plan) still publishes, under the brief
//     and with the plan as its dek, or it is held with no reason Texture
//     can read;
//   - a held revision changes the title, or the next heading-led revision
//     is renumbered into the held slot;
//   - a later headline does not retitle the story as it develops, or its
//     marks break the title's spacing;
//   - a document that is not a story is retitled by its heading.
func TestStoryIsTitledByTexturesHeadlineAndAHeadlinelessRevisionIsHeld(t *testing.T) {
	_, h, s, pub := storySetup(t)
	brief := "Write the story of the harbor reopening: who reopens it, when, and what the court ruled."
	rec := openStoryWithObjective(t, h, s, "headline", types.CastPublishAuto, brief)
	ctx := context.Background()
	title := func() string {
		t.Helper()
		doc, err := s.GetLifecycleDocument(ctx, rec.OwnerID, rec.ComputerID, rec.ChannelID)
		if err != nil {
			t.Fatalf("story document: %v", err)
		}
		return doc.Title
	}
	if got := title(); got != brief {
		t.Fatalf("opening title = %q; want the Cast's objective as the working title", got)
	}

	base := headOf(t, s, rec)
	plan := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@all\nThis document will explain the harbor reopening.\n\n" + cited("The court ruling comes first."),
		Why:   "plan the story",
	}})
	out, err := h.CommitCellTextureAuthor(ctx, &rec, plan, "write-plan")
	if err != nil {
		t.Fatalf("headingless Write: %v", err)
	}
	if head := headOf(t, s, rec); head.RevisionID == base.RevisionID {
		t.Fatal("the headingless Write committed nothing; it must commit and only not publish")
	}
	pub.none(t, "a revision that does not lead with a headline")
	if !strings.Contains(out, `"publish_held"`) || !strings.Contains(out, "does not lead with a headline") {
		t.Fatalf("turn result %s does not tell Texture the revision was held for its headline", out)
	}
	if got := title(); got != brief {
		t.Fatalf("a headingless revision retitled the story to %q", got)
	}

	first := storyWrite(t, h, s, &rec, "write-headline", cited("The harbor reopens Monday."))
	if got := title(); got != "Harbor" {
		t.Fatalf("title after a heading-led revision = %q; want Texture's headline %q", got, "Harbor")
	}
	if v := pub.next(t); v.RevisionID != first.RevisionID || v.Revision != 3 {
		t.Fatalf("published %+v; want the heading-led revision %s as v3 (v2, the plan, stays a gap)", v, first.RevisionID)
	}

	head := headOf(t, s, rec)
	developed := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@all\n## Ferries **resume** Tuesday as the harbor reopens\n\n" + cited("The harbor reopened Monday."),
		Why:   "the ferries resumed",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, developed, "write-developed"); err != nil {
		t.Fatalf("developed Write: %v", err)
	}
	if got, want := title(), "Ferries resume Tuesday as the harbor reopens"; got != want {
		t.Fatalf("title after a new headline = %q; want %q", got, want)
	}
	if v := pub.next(t); v.Revision != 4 {
		t.Fatalf("published %+v; want v4", v)
	}
	pub.none(t, "after the developed revision")

	// An owner's document is not a story: its heading does not retitle it.
	dh, ds, docRec := cellAuthorSetup(t, "not-a-story")
	docBase := headOf(t, ds, docRec)
	edit := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: docBase.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b1\n# Merger approved", Why: "retitle the section",
	}})
	if _, err := dh.CommitCellTextureAuthor(ctx, &docRec, edit, "write-doc"); err != nil {
		t.Fatalf("document Write: %v", err)
	}
	owned, err := ds.GetLifecycleDocument(ctx, docRec.OwnerID, docRec.ComputerID, docRec.ChannelID)
	if err != nil || owned.Title != "Cell author not-a-story" {
		t.Fatalf("owner document title = %q, %v; want it unchanged", owned.Title, err)
	}
}

// Owner §24: "We always should have two, ideally more." Ways the gate can
// fail (docs/problems/capture-and-production-are-coupled-2026-10-16.md):
// G1 two captures of one outlet publish; G2 two outlets are held; G3 a
// citation that resolves to no capture counts as an outlet; G4 the hold
// does not tell Texture what is missing.
func TestRevisionCitingOneOutletIsHeldUntilASecondOutletIsCited(t *testing.T) {
	core, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "one-outlet", types.CastPublishAuto)
	ctx := context.Background()
	write := func(identity, edits string) string {
		t.Helper()
		base := headOf(t, s, rec)
		out, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID,
			Write: &yaegikernel.TextureWrite{Edits: edits, Why: "report"}}), identity)
		if err != nil {
			t.Fatalf("Write %s: %v", identity, err)
		}
		return out
	}

	out := write("write-one", "@all\n# Harbor\n\n"+citedOnce("The harbor reopens Monday."))
	pub.none(t, "a revision citing one outlet")
	if !strings.Contains(out, `"publish_held"`) || !strings.Contains(out, "1 outlet") || !strings.Contains(out, "example.org") { // G4
		t.Fatalf("turn result %s does not say the revision cites one outlet", out)
	}

	// G1: a second page of the same publisher is still one outlet.
	write("write-same", "@all\n# Harbor\n\n"+citedOnce("The harbor reopens Monday.")+
		" Shares rose {{cite https://news.example.org/merger-reaction \"shares rose after the ruling\"}}.")
	pub.none(t, "two captures of one outlet")

	// G3: when the store cannot resolve the second capture, it does not count.
	core.SetTextureCaptures(fakeCaptureStore{"https://example.org/court": courtCapture["https://example.org/court"],
		"cap-court": courtCapture["https://example.org/court"], "https://wire.example.net/merger": wireCapture})
	write("write-unresolved", "@all\n# Harbor\n\n"+cited("The harbor reopens Monday."))
	pub.none(t, "a second outlet the store cannot resolve by capture id")

	// G2: two resolvable outlets publish.
	core.SetTextureCaptures(storyCaptures())
	write("write-two", "@all\n# Harbor\n\n"+cited("The harbor reopens Monday. Ferries resume."))
	if v := pub.next(t); v.Revision != 5 {
		t.Fatalf("published %+v; want v5 (v2-v4 held)", v)
	}
	pub.none(t, "after the two-outlet revision")
}

// linkOnlyCapture is a Hacker News feed item: its URL is the linked site,
// its body only the links
// (docs/problems/link-only-capture-counts-as-an-outlet-2026-10-17.md).
var linkOnlyCapture = texturewrite.Capture{
	Doc: "cap-hn", Version: "doc-hash-hn", URL: "https://tool.example.com/", Title: "Show HN: Tool",
	Blocks: []quotematch.Block{{ID: "cid-h1", Text: "Article URL: https://tool.example.com/ Comments URL: https://news.ycombinator.com/item?id=1 Points: 52 # Comments: 21"}},
}

// shortExcerptCapture is a real one-sentence excerpt in Chinese, short in
// letters but the outlet's own words.
var shortExcerptCapture = texturewrite.Capture{
	Doc: "cap-short", Version: "doc-hash-short", URL: "https://zh.example.cn/harbor", Title: "港口",
	Blocks: []quotematch.Block{{ID: "cid-z1", Text: "港口周一重新开放，渡轮恢复运营，官员表示航运将在本周内全面恢复正常。"}},
}

// Ways the link-only rule can fail, written first: L1 a link-only capture
// still counts as an outlet; L2 a short real excerpt in a non-Latin script
// stops counting; L3 two real outlets plus a link-only capture are held;
// L4 the hold does not say a capture held only links.
func TestLinkOnlyCaptureDoesNotCountAsAnOutlet(t *testing.T) {
	core, h, s, pub := storySetup(t)
	captures := storyCaptures()
	for _, c := range []texturewrite.Capture{linkOnlyCapture, shortExcerptCapture} {
		captures[c.URL], captures[c.Doc] = c, c
	}
	core.SetTextureCaptures(captures)
	rec := openStoryForTexture(t, h, s, "link-only", types.CastPublishAuto)
	ctx := context.Background()
	write := func(identity, edits string) string {
		t.Helper()
		base := headOf(t, s, rec)
		out, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID,
			Write: &yaegikernel.TextureWrite{Edits: edits, Why: "report"}}), identity)
		if err != nil {
			t.Fatalf("Write %s: %v", identity, err)
		}
		return out
	}
	linkCite := " The tool is listed {{cite https://tool.example.com/ \"Article URL: https://tool.example.com/\"}}."

	// L1 and L4: one real outlet plus a link-only capture is one outlet.
	out := write("write-link", "@all\n# Harbor\n\n"+citedOnce("The harbor reopens Monday.")+linkCite)
	pub.none(t, "one outlet plus a link-only capture")
	if !strings.Contains(out, `"publish_held"`) || !strings.Contains(out, "1 outlet") || !strings.Contains(out, "only links") {
		t.Fatalf("turn result %s does not say the link-only capture was not counted", out)
	}

	// L2: a short real excerpt in Chinese counts.
	write("write-short", "@all\n# Harbor\n\n"+citedOnce("The harbor reopens Monday.")+
		" Ferries resume {{cite https://zh.example.cn/harbor \"渡轮恢复运营\"}}.")
	if v := pub.next(t); v.Revision != 3 {
		t.Fatalf("published %+v; want v3 (court plus a short Chinese excerpt)", v)
	}

	// L3: two real outlets plus a link-only capture publish.
	write("write-three", "@all\n# Harbor\n\n"+cited("The harbor reopens Monday. Ferries resume.")+linkCite)
	if v := pub.next(t); v.Revision != 4 {
		t.Fatalf("published %+v; want v4 (two outlets plus a link-only capture)", v)
	}
	pub.none(t, "after the three-capture revision")
}
