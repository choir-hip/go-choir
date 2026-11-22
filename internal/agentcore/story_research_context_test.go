package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A story's research trajectory (story_research_context.go). Failure list,
// written before the code:
//
//  R1  captures opened as one batch make more than one story, or the story's
//      material lacks one of them (a story's research must hold every capture
//      of its batch, as one trajectory);
//  R2  a research run on a story does not see the pages earlier runs on that
//      story read, the outlets they hold, or the captures fed since; or sees
//      another story's;
//  R3  a successor story's research does not start with its predecessor's
//      pages, outlets and main point (through its succeeds ref);
//  R4  a Texture's Cast to research is accepted while captures fed since its
//      last research Cast are unread by it; or a story with no earlier research
//      is refused; or a capture it read (or tried to read) is owed again; or
//      the refusal does not name the captures to read; or no line counts it;
//  R5  a research child's web search files its leads under no story.

type ctxSink struct {
	pages map[string][]researchtools.LeadPage
	urls  map[string]string
	gone  map[string]bool
}

func (s *ctxSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, fmt.Errorf("not used")
}
func (s *ctxSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}
func (s *ctxSink) ReadCapture(_ context.Context, id string, _, _ int) (json.RawMessage, error) {
	if s.gone[id] {
		return nil, fmt.Errorf("capture %s not found", id)
	}
	return json.Marshal(map[string]any{"url": s.urls[id]})
}
func (s *ctxSink) SearchLeadPages(_ context.Context, _, _, conv string, _ int) ([]researchtools.LeadPage, error) {
	return s.pages[conv], nil
}

func commitStoryRecord(t *testing.T, s *store.Store, owner, computer, id string, rec types.CommitmentRecord, at time.Time) {
	t.Helper()
	rec.SchemaID, rec.RecordID = types.CommitmentRecordSchemaV1, id
	rec.Kind = types.CommitmentKindDirective
	rec.EmitterKind, rec.Discrepancy = types.CommitmentEmitterAgent, types.DiscrepancyUnresolved
	rec.Provenance = types.CommitmentProvenance{AgentID: rec.SenderInbox, CommittedAt: at.UTC().Format(time.RFC3339Nano)}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:" + id, Records: []types.CommitmentRecord{rec}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("commit %s: %v", id, err)
	}
}

// openStoryOf opens a story with a Cast naming captures, a main point and a
// successor, and returns its doc id.
func openStoryOf(t *testing.T, s *store.Store, owner, computer, id, headline, point, succeeds string, captures []string, at time.Time) string {
	t.Helper()
	management := "management:" + owner
	rec := types.CommitmentRecord{
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: headline},
		Cast:      &types.CommitmentCast{Publish: types.CastPublishAuto, Captures: captures, MainPoint: point, Succeeds: succeeds},
		Inbox:     types.StoryOpenInbox(), SenderInbox: management,
	}
	commitStoryRecord(t, s, owner, computer, id, rec, at)
	docs, _ := s.ListStoryDocuments(context.Background(), owner, computer, time.Time{}, "", 0)
	for _, d := range docs {
		if d.Title == headline {
			return d.DocID
		}
	}
	t.Fatalf("story %q not found among %d", headline, len(docs))
	return ""
}

func researchCastAt(t *testing.T, s *store.Store, owner, computer, doc, id string, at time.Time) {
	t.Helper()
	rec := types.CommitmentRecord{
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "find sources"},
		Cast:      &types.CommitmentCast{}, Inbox: "research:" + doc, SenderInbox: "texture:" + doc, Addressee: "research:" + doc,
	}
	commitStoryRecord(t, s, owner, computer, id, rec, at)
}

func feedCastAt(t *testing.T, s *store.Store, owner, computer, doc, id string, captures []string, at time.Time) {
	t.Helper()
	rec := types.CommitmentRecord{
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "more on the story"},
		Cast:      &types.CommitmentCast{Captures: captures}, Inbox: "texture:" + doc, SenderInbox: "management:" + owner,
	}
	commitStoryRecord(t, s, owner, computer, id, rec, at)
}

func TestStoryMaterialHoldsTheWholeBatchAndFeedsSinceResearch(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-material", rt.cfg.ComputerID
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1", "c2", "c3"}, t0)
	docs, _ := s.ListStoryDocuments(ctx, owner, computer, time.Time{}, "", 0)
	if len(docs) != 1 {
		t.Fatalf("R1: three captures judged one beat made %d stories", len(docs))
	}
	m, err := rt.storyMaterialOf(ctx, owner, computer, doc)
	if err != nil || strings.Join(m.Captures, ",") != "c1,c2,c3" || m.HasResearch {
		t.Fatalf("R1: material = %+v, %v; want c1,c2,c3 and no research yet", m, err)
	}
	// No earlier research: nothing is refused, the opening captures are its first material.
	if err := requireFedRead(rt, ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("R4: a story with no earlier research was refused: %v", err)
	}
	researchCastAt(t, s, owner, computer, doc, "texture-cell:cast:r1", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4", "c5"}, t0.Add(20*time.Minute))
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	err = requireFedRead(rt, ctx, owner, computer, doc, "run-t")
	if err == nil || !strings.Contains(err.Error(), "c4") || !strings.Contains(err.Error(), "c5") || strings.Contains(err.Error(), "c1") || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("R4: a research Cast after an unread feed was accepted or does not name c4 and c5: %v", err)
	}
	if !strings.Contains(logs.String(), "story research refused run=run-t story="+doc+" unread=2") {
		t.Fatalf("R4: the refusal is not counted: %q", logs.String())
	}
	rt.noteStoryCaptureRead(owner, computer, doc, "c4")
	if err := requireFedRead(rt, ctx, owner, computer, doc, "run-t"); err == nil || strings.Contains(err.Error(), "c4,") || !strings.Contains(err.Error(), "c5") {
		t.Fatalf("R4: a read capture is owed again, or the other is not: %v", err)
	}
	rt.noteStoryCaptureRead(owner, computer, doc, "c5")
	if err := requireFedRead(rt, ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("R4: every fed capture was read and research was refused: %v", err)
	}
	// Another story's reads are not this story's.
	other := openStoryOf(t, s, owner, computer, "cell-3:cast:c1", "Other story", "Whether the other thing happens at all", "", []string{"o1"}, t0)
	researchCastAt(t, s, owner, computer, other, "texture-cell:cast:r2", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, other, "cell-4:cast:f1", []string{"c4"}, t0.Add(20*time.Minute))
	if err := requireFedRead(rt, ctx, owner, computer, other, "run-u"); err == nil || !strings.Contains(err.Error(), "c4") {
		t.Fatalf("R4: a read of c4 on one story cleared another story's feed: %v", err)
	}
}

func TestResearchChildLeadsAreFiledUnderTheStory(t *testing.T) {
	ctx := context.WithValue(context.Background(), researchLineageKey{}, researchLineage{channelID: "doc-story"})
	if got := leadConversationOf(ctx, toolregistry.ExecutionContext{}); got != "doc-story" {
		t.Fatalf("R5: a child's search leads are filed under %q, want the story", got)
	}
	if got := leadConversationOf(ctx, toolregistry.ExecutionContext{ChannelID: "own"}); got != "own" {
		t.Fatalf("R5: a desk's own channel lost to the lineage: %q", got)
	}
	if got := leadConversationOf(context.Background(), toolregistry.ExecutionContext{}); got != "" {
		t.Fatalf("R5: %q", got)
	}
}

// The read mark is written where a story Texture's ReadCapture goes through
// deskEgress, and the refusal reads it: the two derive the story the same way
// (from the desk's agent id, never the channel, which can be a run id), and a
// read the gate or the store refuses still counts, so a capture that cannot be
// read never blocks research for ever. Failure list for this change (seat 1,
// 4a358b4d9): a mark under another identifier than the refusal's; a refused
// read not counted; another desk's read counted.
func TestStoryReadThroughDeskEgressClearsTheRefusal(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-egress", rt.cfg.ComputerID
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)
	researchCastAt(t, s, owner, computer, doc, "texture-cell:cast:r1", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4", "c5", "c6"}, t0.Add(20*time.Minute))
	rt.researchDeps = &researchtools.Dependencies{Captures: &ctxSink{urls: map[string]string{"c4": "https://a.example/4"}, gone: map[string]bool{"c5": true}}}
	// The run's channel is a run id, not the doc, as runtime.go sets one when none is.
	exec := toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer, RunID: "run-texture-1", AgentID: "texture:" + doc, ChannelID: "run-texture-1"}
	read := func(role, id string) {
		payload, _ := json.Marshal(map[string]any{"capture_id": id})
		_, _ = rt.deskEgress(ctx, role, exec, yaegikernel.ActionReadCapture, payload)
	}
	// Another desk's read of c4 is no read by the story's Texture.
	read("research", "c4")
	if err := requireFedRead(rt, ctx, owner, computer, doc, "run-t"); err == nil || !strings.Contains(err.Error(), "c4") {
		t.Fatalf("a research desk's read cleared the story's refusal: %v", err)
	}
	texture := deskmanifest.DocumentCustodianName()
	read(texture, "c4") // read
	read(texture, "c5") // the store says it is gone: the attempt counts
	err := requireFedRead(rt, ctx, owner, computer, doc, "run-t")
	if err == nil || strings.Contains(err.Error(), "c4") || strings.Contains(err.Error(), "c5") || !strings.Contains(err.Error(), "c6") {
		t.Fatalf("after reading c4 and trying c5 the refusal should name only c6: %v", err)
	}
	read(texture, "c6")
	if err := requireFedRead(rt, ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("every fed capture was read or tried and research is still refused: %v", err)
	}
}

// requireFedRead is the refusal the reducer makes (rlm_reduce.go): the unread fed captures
// of a story as an error, nil when there are none.
func requireFedRead(rt *Runtime, ctx context.Context, owner, computer, doc, runID string) error {
	unread := rt.unreadFedCaptures(ctx, owner, computer, doc, runID)
	if len(unread) == 0 {
		return nil
	}
	return fmt.Errorf("%s (nothing was sent)", unreadFedMessage(unread))
}
