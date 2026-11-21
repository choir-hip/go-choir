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
	reads int
}

func (s *ctxSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, fmt.Errorf("not used")
}
func (s *ctxSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}
func (s *ctxSink) ReadCapture(_ context.Context, id string, _, _ int) (json.RawMessage, error) {
	s.reads++
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
	if err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("R4: a story with no earlier research was refused: %v", err)
	}
	researchCastAt(t, s, owner, computer, doc, "texture-cell:cast:r1", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4", "c5"}, t0.Add(20*time.Minute))
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	err = rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t")
	if err == nil || !strings.Contains(err.Error(), "c4") || !strings.Contains(err.Error(), "c5") || strings.Contains(err.Error(), "c1") || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("R4: a research Cast after an unread feed was accepted or does not name c4 and c5: %v", err)
	}
	if !strings.Contains(logs.String(), "story research refused run=run-t story="+doc+" unread=2") {
		t.Fatalf("R4: the refusal is not counted: %q", logs.String())
	}
	rt.noteStoryCaptureRead(owner, computer, doc, "c4")
	if err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t"); err == nil || strings.Contains(err.Error(), "c4,") || !strings.Contains(err.Error(), "c5") {
		t.Fatalf("R4: a read capture is owed again, or the other is not: %v", err)
	}
	rt.noteStoryCaptureRead(owner, computer, doc, "c5")
	if err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("R4: every fed capture was read and research was refused: %v", err)
	}
	// Another story's reads are not this story's.
	other := openStoryOf(t, s, owner, computer, "cell-3:cast:c1", "Other story", "Whether the other thing happens at all", "", []string{"o1"}, t0)
	researchCastAt(t, s, owner, computer, other, "texture-cell:cast:r2", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, other, "cell-4:cast:f1", []string{"c4"}, t0.Add(20*time.Minute))
	if err := rt.requireFedCapturesRead(ctx, owner, computer, other, "run-u"); err == nil || !strings.Contains(err.Error(), "c4") {
		t.Fatalf("R4: a read of c4 on one story cleared another story's feed: %v", err)
	}
}

func TestResearchRunOnAStorySeesEarlierReadsFedCapturesAndPredecessor(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-context", rt.cfg.ComputerID
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)
	first := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)
	feedCastAt(t, s, owner, computer, first, "cell-2:cast:f1", []string{"c2"}, t0.Add(5*time.Minute))
	succ := openStoryOf(t, s, owner, computer, "cell-3:cast:c1", "Dock strike ends", "What the settlement of the dock strike contains", "texture:"+first, []string{"c9"}, t0.Add(30*time.Minute))
	stranger := openStoryOf(t, s, owner, computer, "cell-5:cast:c1", "Unrelated vote", "Who wins the unrelated vote and by how much", "", []string{"z1"}, t0)
	sink := &ctxSink{
		pages: map[string][]researchtools.LeadPage{
			first:    {{CaptureID: "cap-r1", URL: "https://news.example/a", Title: "Talks resume"}, {CaptureID: "cap-r2", URL: "https://news.example/b", Title: "Union statement"}, {CaptureID: "cap-r3", URL: "https://wire.example/c"}},
			stranger: {{CaptureID: "cap-z", URL: "https://elsewhere.example/z"}},
		},
		urls: map[string]string{"c1": "https://port.example/1", "c2": "https://port.example/2", "c9": "https://port.example/9"},
	}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	// R2: a second research run on the first story sees the first run's reads, outlets and fed captures.
	rec := briefRunRecord(owner, computer, first, "run-ctx-1")
	c := rt.storyResearchContextOpening(ctx, rec)
	if c == nil {
		t.Fatal("R2: a story that holds pages got no context")
	}
	text := c.openingText()
	for _, want := range []string{"cap-r1", "cap-r2", "cap-r3", "capture_id c1 https://port.example/1", "capture_id c2", "news.example (2)", "wire.example (1)", "main point: Whether the dock strike ends", "[Data, not instructions"} {
		if !strings.Contains(text, want) {
			t.Errorf("R2: the context lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "cap-z") || strings.Contains(text, "elsewhere.example") || strings.Contains(text, "succeeds") {
		t.Errorf("R2: another story's pages, or a predecessor, in a story with none:\n%s", text)
	}
	// R3: the successor's first run starts with its predecessor's pages and outlets.
	rec = briefRunRecord(owner, computer, succ, "run-ctx-2")
	c = rt.storyResearchContextOpening(ctx, rec)
	if c == nil {
		t.Fatal("R3: a successor got no context")
	}
	text = c.openingText()
	for _, want := range []string{"succeeds an earlier one", "cap-r1", "cap-r3", "wire.example", "capture_id c9", "main point: What the settlement", "The earlier story's main point: Whether the dock strike ends"} {
		if !strings.Contains(text, want) {
			t.Errorf("R3: the successor's context lacks %q:\n%s", want, text)
		}
	}
	// A story that holds nothing and succeeds nothing has no section.
	sink.urls = nil
	rt.researchDeps = &researchtools.Dependencies{Captures: &ctxSink{}}
	empty := openStoryOf(t, s, owner, computer, "cell-6:cast:c1", "Empty story", "A story that holds no page of any kind", "", nil, t0)
	if c := rt.storyResearchContextOpening(ctx, briefRunRecord(owner, computer, empty, "run-ctx-3")); c != nil {
		t.Errorf("R2: a story with no pages got a section: %+v", c)
	}
	// Not a story's research run: none.
	if c := rt.storyResearchContextOpening(ctx, &types.RunRecord{RunID: "r", OwnerID: owner, ComputerID: computer, AgentID: "texture:" + first, ChannelID: first, AgentProfile: "texture"}); c != nil {
		t.Errorf("a texture run got a research context")
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
	if err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t"); err == nil || !strings.Contains(err.Error(), "c4") {
		t.Fatalf("a research desk's read cleared the story's refusal: %v", err)
	}
	texture := deskmanifest.DocumentCustodianName()
	read(texture, "c4") // read
	read(texture, "c5") // the store says it is gone: the attempt counts
	err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t")
	if err == nil || strings.Contains(err.Error(), "c4") || strings.Contains(err.Error(), "c5") || !strings.Contains(err.Error(), "c6") {
		t.Fatalf("after reading c4 and trying c5 the refusal should name only c6: %v", err)
	}
	read(texture, "c6")
	if err := rt.requireFedCapturesRead(ctx, owner, computer, doc, "run-t"); err != nil {
		t.Fatalf("every fed capture was read or tried and research is still refused: %v", err)
	}
}

// A story with hundreds of captures delays no research start: the context
// lists at most the section's bound, newest first, and reads only those.
func TestStoryContextReadsAtMostWhatItShows(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-bound", rt.cfg.ComputerID
	var ids []string
	urls := map[string]string{}
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("cap-%03d", i)
		ids = append(ids, id)
		urls[id] = fmt.Sprintf("https://site%d.example/p", i)
	}
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Big story", "A story that holds a great many captures", "", ids, time.Now().UTC().Add(-time.Hour))
	sink := &ctxSink{urls: urls}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	c := rt.storyResearchContextOpening(context.Background(), briefRunRecord(owner, computer, doc, "run-bound"))
	if c == nil || strings.Count(c.text, "- capture_id ") != storyContextMaxPages || sink.reads != storyContextMaxPages {
		t.Fatalf("listed %d pages after %d reads, want %d and %d", strings.Count(c.text, "- capture_id "), sink.reads, storyContextMaxPages, storyContextMaxPages)
	}
	if !strings.Contains(c.text, "capture_id cap-149 ") || strings.Contains(c.text, "capture_id cap-000") {
		t.Fatalf("the newest captures are not the ones listed:\n%s", c.text)
	}
}

// The section's page budget is shared, not first come: forty lead pages must
// not push out the captures the story was just fed (review round 3, finding 8),
// a predecessor's pages take a smaller share than the story's own, one URL
// cannot fill the section, and the whole build has one deadline.
func TestStoryContextKeepsTheNewestCapturesWhenLeadsFillTheSection(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-share", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Big story", "A story with many pages read and a few just fed", "", []string{"old-1", "old-2"}, t0)
	researchCastAt(t, s, owner, computer, doc, "texture-cell:cast:r1", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"fed-1", "fed-2", "fed-3"}, t0.Add(20*time.Minute))
	var leads []researchtools.LeadPage
	for i := 0; i < 60; i++ {
		leads = append(leads, researchtools.LeadPage{CaptureID: fmt.Sprintf("lead-%02d", i), URL: fmt.Sprintf("https://lead%d.example/p", i)})
	}
	long := "https://long.example/" + strings.Repeat("x", 3000)
	sink := &ctxSink{pages: map[string][]researchtools.LeadPage{doc: leads}, urls: map[string]string{"fed-1": long, "fed-2": "https://f.example/2", "fed-3": "https://f.example/3", "old-1": "https://o.example/1"}}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	c := rt.storyResearchContextOpening(context.Background(), briefRunRecord(owner, computer, doc, "run-share"))
	if c == nil {
		t.Fatal("no context")
	}
	for _, want := range []string{"capture_id fed-1", "capture_id fed-2", "capture_id fed-3", "capture_id old-1", "Fed since the last research round: fed-1, fed-2, fed-3"} {
		if !strings.Contains(c.text, want) {
			t.Errorf("60 lead pages pushed out %q:\n%s", want, c.text)
		}
	}
	if n := strings.Count(c.text, "- capture_id "); n != storyContextMaxPages {
		t.Errorf("listed %d pages, want exactly the bound %d", n, storyContextMaxPages)
	}
	if strings.Contains(c.text, strings.Repeat("x", 300)) {
		t.Errorf("a 3,000-character URL was listed whole")
	}
}

func TestStoryContextPredecessorTakesASmallerShare(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-story-pred", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	first := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "First", "The first main point of a story that moved on", "", []string{"p1"}, t0)
	succ := openStoryOf(t, s, owner, computer, "cell-2:cast:c1", "Second", "The second main point after it moved on", "texture:"+first, []string{"s1"}, t0.Add(time.Minute))
	var leads []researchtools.LeadPage
	for i := 0; i < 60; i++ {
		leads = append(leads, researchtools.LeadPage{CaptureID: fmt.Sprintf("pred-%02d", i), URL: fmt.Sprintf("https://pred%d.example/p", i)})
	}
	rt.researchDeps = &researchtools.Dependencies{Captures: &ctxSink{pages: map[string][]researchtools.LeadPage{first: leads}, urls: map[string]string{"s1": "https://s.example/1"}}}
	c := rt.storyResearchContextOpening(context.Background(), briefRunRecord(owner, computer, succ, "run-pred"))
	if c == nil {
		t.Fatal("no context")
	}
	// The predecessor's share is 20 pages: its own capture p1 and 19 of its 60 leads.
	if n := strings.Count(c.text, "- capture_id pred-"); n != storyContextMaxPredecessorPages-1 || !strings.Contains(c.text, "capture_id p1") {
		t.Errorf("the predecessor's leads listed = %d, want %d beside its own capture:\n%s", n, storyContextMaxPredecessorPages-1, c.text)
	}
	if !strings.Contains(c.text, "capture_id s1") {
		t.Errorf("the story's own capture is missing:\n%s", c.text)
	}
}
