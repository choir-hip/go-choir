package textureowner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
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
//     boot-time resumer).

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
	case v := <-p.done:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("no publish within 10s")
	}
	return agentcore.StoryVersion{}
}

func (p *recordingStoryPublisher) none(t *testing.T, label string) {
	t.Helper()
	select {
	case v := <-p.done:
		t.Fatalf("%s published %+v", label, v)
	case <-time.After(300 * time.Millisecond):
	}
}

// openStoryForTexture opens a story the way management does (a Cast to
// texture:new) and projects a Texture records-route run onto it.
func openStoryForTexture(t *testing.T, h *Handler, s *store.Store, suffix string, publish string) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	owner, computer := "owner-story-"+suffix, "computer-story"
	management := agentprofile.Management + ":" + owner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-open-" + suffix + ":cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Harbor reopens " + suffix},
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

func storySetup(t *testing.T) (*agentcore.Runtime, *Handler, *store.Store, *recordingStoryPublisher) {
	t.Helper()
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	pub := newRecordingStoryPublisher()
	core.SetStoryPublisher(pub)
	return core, h, core.Store(), pub
}

func TestRevisionOfAPublishAutoStoryPublishesAsItsVersion(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "auto", types.CastPublishAuto)
	seed := headOf(t, s, rec)

	first := storyWrite(t, h, s, &rec, "write-1", "The harbor reopens Monday.")
	v := pub.next(t)
	if v.DocID != rec.ChannelID || v.RevisionID != first.RevisionID || v.Revision != types.RevisionNumber(first.VersionNumber) || v.Revision != 2 || v.OwnerID != rec.OwnerID {
		t.Fatalf("published %+v; want revision %s as v2 (the seed is revision 1)", v, first.RevisionID)
	}
	if v.RevisionID == seed.RevisionID {
		t.Fatal("the seed published")
	}
	second := storyWrite(t, h, s, &rec, "write-2", "The harbor reopens Monday. Ferries resume Tuesday.")
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
	storyWrite(t, h, s, &rec, "write-1", "The harbor reopens Monday.")
	if v := pub.next(t); v.Revision != 2 {
		t.Fatalf("first publish = %+v", v)
	}
	third := storyWrite(t, h, s, &rec, "write-2", "The harbor reopens Monday. Ferries resume.")
	if v := pub.next(t); v.Revision != 3 || v.RevisionID != third.RevisionID {
		t.Fatalf("after a failed v2, the next revision published as %+v; want v3 (v2 stays a gap)", v)
	}
	pub.none(t, "a failed publish (no retry)")
}

func TestStoryWithoutPublishAutoPublishesNothing(t *testing.T) {
	_, h, s, pub := storySetup(t)
	rec := openStoryForTexture(t, h, s, "manual", "")
	storyWrite(t, h, s, &rec, "write-1", "The harbor reopens Monday.")
	pub.none(t, "a story without publish auto")
}

func TestCrashBootPublishesNothing(t *testing.T) {
	core, h, s, pub := storySetup(t)
	pub.fail[2] = true
	rec := openStoryForTexture(t, h, s, "crash", types.CastPublishAuto)
	storyWrite(t, h, s, &rec, "write-1", "The harbor reopens Monday.")
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
