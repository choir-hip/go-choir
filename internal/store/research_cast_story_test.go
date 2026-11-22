package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The research desk is not the custodian of Texture's documents and cannot read them
// (objectgraph reader gate X1); the story's text reaches it inside the Cast the
// Texture's own turn commits. Failure list, written before the code: a Cast to research
// carries no account of the story; the text is the head the turn replaced instead of the
// revision the turn wrote; a `story` typed by a cell is trusted; the labels are lost.
func castToResearch(inbox, texture string) []types.CommitmentRecord {
	return []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:cast:tray-story", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover chips", Body: `{"kind":"question","summary":"chips"}`},
		// A cell's own claim about the story: overwritten, never trusted.
		Cast:  &types.CommitmentCast{Story: &types.CastStory{Text: "forged by a cell", MainPoint: "forged"}, Hypotheses: []types.CastHypothesis{{Statement: "a new chip shipped", Probability: 0.6}}},
		Inbox: inbox, SenderInbox: texture, Addressee: inbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: texture},
	}}
}

const (
	fixtureMainPoint   = "Whether the harbor strike ends and on what terms"
	fixturePredecessor = "document-earlier-strike"
	fixtureOldMain     = "Why the earlier harbor strike began"
)

// storyFixture is the turn fixture as a STORY: its trajectory carries a main point and a
// predecessor, and the predecessor's trajectory carries its own, so the Cast's account of
// the story can be asserted equal to what the trajectories hold (an empty main point must
// fail, as it did in the deployed context).
func storyFixture(t *testing.T) (*Store, types.StartLifecycleRequest, types.RunRecord) {
	t.Helper()
	s := openTestStore(t)
	s, start, caller, _ := setupLifecycleTextureTargetFixtureWith(t, s, func(r *types.StartLifecycleRequest) {
		r.Kind = types.TrajectoryKindStory
		r.SubjectRefs[types.StoryRefMainPoint] = fixtureMainPoint
		r.SubjectRefs[types.StoryRefSucceeds] = deskmanifest.DocumentCustodianName() + ":" + fixturePredecessor
	})
	// The predecessor: its own story trajectory and document.
	pre := lifecycleStartFixture()
	pre.OwnerID, pre.ComputerID = start.OwnerID, start.ComputerID
	pre.CommandID, pre.TrajectoryID = "command-start-predecessor", "trajectory-predecessor"
	pre.Kind = types.TrajectoryKindStory
	pre.InitialWork.WorkItemID = "work-predecessor"
	pre.InitialDocument.DocID, pre.InitialRevision.RevisionID = fixturePredecessor, "revision-predecessor"
	pre.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(fixturePredecessor, "revision-predecessor", "Initial artifact")
	pre.Agent.AgentID, pre.Agent.ChannelID = "texture:"+fixturePredecessor, fixturePredecessor
	pre.SubjectRefs = map[string]string{"artifact": "texture://" + fixturePredecessor, "doc_id": fixturePredecessor, types.StoryRefMainPoint: fixtureOldMain}
	pre.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(pre)
	if _, err := s.StartLifecycle(context.Background(), pre); err != nil {
		t.Fatalf("start the predecessor story: %v", err)
	}
	return s, start, caller
}

func TestACastToResearchCarriesTheRevisionTheTurnWrote(t *testing.T) {
	s, start, caller := storyFixture(t)
	ctx := context.Background()
	owner, computer, doc := start.OwnerID, start.ComputerID, start.InitialDocument.DocID
	textureInbox, researchInbox := start.Agent.AgentID, "research:"+doc

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	req.CommandID = "story-turn-revision"
	rev := start.InitialRevision
	rev.RevisionID, rev.AuthorKind, rev.AuthorLabel, rev.CreatedAt = "story-rev-2", types.AuthorAppAgent, "appagent", time.Time{}
	rev.ParentRevisionID = req.ExpectedHeadRevisionID
	rev.BodyDoc, _ = json.Marshal(userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, "The harbor strike ended on Tuesday."))
	rev.Content = "The harbor strike ended on Tuesday."
	rev.Provenance = json.RawMessage(`{"schema_version":1,"authoring_model":{"provider":"chatgpt","model":"gpt-6-luna"},"authored_at":"2026-10-15T00:00:00Z"}`)
	req.Revision = rev
	req.InboxRecords = castToResearch(researchInbox, textureInbox)
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, req); err != nil {
		t.Fatalf("turn with a revision and a research Cast: %v", err)
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, researchInbox, 0, 0)
	if err != nil || len(recs) != 1 || recs[0].Cast == nil || recs[0].Cast.Story == nil {
		t.Fatalf("the Cast carries no story: %+v %v", recs, err)
	}
	story := recs[0].Cast.Story
	if story.Text != "The harbor strike ended on Tuesday." || story.RevisionID != "story-rev-2" {
		t.Fatalf("the Cast carries the text of %q (%s), want the revision this turn wrote", story.Text, story.RevisionID)
	}
	if story.MainPoint != fixtureMainPoint {
		t.Fatalf("main point = %q, want the trajectory's %q (a cell's own claim is never trusted)", story.MainPoint, fixtureMainPoint)
	}
	if strings.Contains(story.Text, "forged") {
		t.Fatalf("a story typed by a cell was trusted: %+v", story)
	}
	if want := deskmanifest.DocumentCustodianName() + ":" + fixturePredecessor; story.Succeeds != want || story.SucceedsMainPoint != fixtureOldMain {
		t.Fatalf("succeeds = %q (%q), want %q (%q)", story.Succeeds, story.SucceedsMainPoint, want, fixtureOldMain)
	}
	if len(story.Labels) != 1 || story.Labels[0] != types.OwnerPrivateLabel("document") {
		t.Fatalf("labels = %v, want the document's own owner_private:document", story.Labels)
	}
}

func TestACastToResearchInATurnThatWroteNothingCarriesTheCurrentHead(t *testing.T) {
	s, start, caller := storyFixture(t)
	ctx := context.Background()
	owner, computer, doc := start.OwnerID, start.ComputerID, start.InitialDocument.DocID
	textureInbox, researchInbox := start.Agent.AgentID, "research:"+doc

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "story-turn-wait"
	req.InboxRecords = castToResearch(researchInbox, textureInbox)
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, req); err != nil {
		t.Fatalf("turn with a research Cast: %v", err)
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, researchInbox, 0, 0)
	if err != nil || len(recs) != 1 || recs[0].Cast == nil || recs[0].Cast.Story == nil {
		t.Fatalf("the Cast carries no story: %+v %v", recs, err)
	}
	if got, want := recs[0].Cast.Story.RevisionID, start.InitialRevision.RevisionID; got != want {
		t.Fatalf("revision = %s, want the head the turn left unchanged (%s)", got, want)
	}
	if strings.Contains(recs[0].Cast.Story.Text, "forged") || recs[0].Cast.Story.Text != start.InitialRevision.Content {
		t.Fatalf("text = %q, want the head's %q", recs[0].Cast.Story.Text, start.InitialRevision.Content)
	}
	if recs[0].Cast.Story.MainPoint != fixtureMainPoint {
		t.Fatalf("main point = %q, want %q", recs[0].Cast.Story.MainPoint, fixtureMainPoint)
	}
}

// A turn that gives the story a new headline sends the NEW title in its Cast: the story
// account is built from the result document, not the document the turn started from.
func TestACastToResearchCarriesTheTitleTheTurnLeaves(t *testing.T) {
	s, start, caller := storyFixture(t)
	ctx := context.Background()
	textureInbox, researchInbox := start.Agent.AgentID, "research:"+start.InitialDocument.DocID
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	req.CommandID = "story-turn-retitle"
	rev := start.InitialRevision
	rev.RevisionID, rev.AuthorKind, rev.AuthorLabel, rev.CreatedAt = "story-rev-title", types.AuthorAppAgent, "appagent", time.Time{}
	rev.ParentRevisionID = req.ExpectedHeadRevisionID
	sd := userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, "Harbor strike ends after nine days")
	sd.Doc.Content[0].Type = "heading"
	sd.Doc.Content[0].Attrs["level"] = 1
	rev.BodyDoc, _ = json.Marshal(sd)
	rev.Content = "# Harbor strike ends after nine days"
	rev.Provenance = json.RawMessage(`{"schema_version":1,"authoring_model":{"provider":"chatgpt","model":"gpt-6-luna"},"authored_at":"2026-10-15T00:00:00Z"}`)
	req.Revision = rev
	req.InboxRecords = castToResearch(researchInbox, textureInbox)
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	res, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("turn: %v", err)
	}
	recs, err := s.ListInboxRecords(ctx, start.OwnerID, start.ComputerID, researchInbox, 0, 0)
	if err != nil || len(recs) != 1 || recs[0].Cast == nil || recs[0].Cast.Story == nil {
		t.Fatalf("the Cast carries no story: %+v %v", recs, err)
	}
	if res.Document == nil || res.Document.Title == start.InitialDocument.Title {
		t.Fatalf("the turn did not retitle the story (%+v): the test would prove nothing", res.Document)
	}
	if got := recs[0].Cast.Story.Title; got != res.Document.Title {
		t.Fatalf("the Cast carries the title %q, want the result document's %q", got, res.Document.Title)
	}
}
