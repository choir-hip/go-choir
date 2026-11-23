package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The research digest (research_digest.go). Failure list, written before the code:
//  D1 a research run on a story opens with pointers (capture ids and URLs) it must
//     fetch: no text of the story, nothing of what earlier rounds found, no passage of
//     a capture the story holds;
//  D2 a Report's summary and its "adds:" / "missing:" notes are not kept on the record,
//     so the next run cannot be shown what an earlier round found or what was missing;
//  D3 the digest shows another owner's story, packets or captures;
//  D4 a slow capture read fails the whole digest, or delays the run past its bound;
//  D5 the digest is not bounded, or pads (it must be as long as what exists);
//  D6 a run's console line says nothing of the digest (tokens, items by kind, ms);
//  D7 a web search with untried pages is refused (the read-first refusals are gone).

type digestSink struct {
	*ctxSink
	text map[string]string
	slow map[string]bool
	// neighbours is what the capture neighbours request answers (by seed capture id);
	// queries records every request, so a test can see the owner and computer it carried.
	neighbours map[string][]researchtools.CaptureNeighbour
	queries    *[]researchtools.CaptureNeighboursQuery
	// labels gives a capture its labels (a private one is dropped); readers records the
	// owner and computer in the context of every read, so a test sees what the capture
	// reader would be bound to.
	labels  map[string][]string
	readers *[][2]string
}

// digestSinkMu guards what the sinks record: the digest reads captures from up to eight
// goroutines, and a sink is copied by value, so the lock is shared.
var digestSinkMu sync.Mutex

func (s digestSink) CaptureNeighbours(_ context.Context, q researchtools.CaptureNeighboursQuery) (researchtools.CaptureNeighboursAnswer, error) {
	if s.queries != nil {
		digestSinkMu.Lock()
		*s.queries = append(*s.queries, q)
		digestSinkMu.Unlock()
	}
	out := map[string][]researchtools.CaptureNeighbour{}
	for _, id := range q.CaptureIDs {
		out[id] = s.neighbours[id]
	}
	return researchtools.CaptureNeighboursAnswer{Neighbours: out}, nil
}

func (s digestSink) ReadCapture(ctx context.Context, id string, from, max int) (json.RawMessage, error) {
	if s.readers != nil {
		exec := toolregistry.ExecutionContextFrom(ctx)
		digestSinkMu.Lock()
		*s.readers = append(*s.readers, [2]string{exec.OwnerID, exec.ComputerID})
		digestSinkMu.Unlock()
	}
	if s.slow[id] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.gone[id] {
		return nil, fmt.Errorf("capture %s not found", id)
	}
	return json.Marshal(map[string]any{"url": s.urls[id], "title": "title of " + id, "labels": s.labels[id],
		"blocks": []map[string]string{{"kind": "paragraph", "text": s.text[id]}}})
}

func commitReport(t *testing.T, s *store.Store, owner, computer, doc, id string, report types.CommitmentReport, final bool, at time.Time) {
	t.Helper()
	commitReportLabelled(t, s, owner, computer, doc, id, report, final, at, nil)
}

// commitReportLabelled commits a Report carrying the labels its run was marked with
// (Provenance.Labels).
func commitReportLabelled(t *testing.T, s *store.Store, owner, computer, doc, id string, report types.CommitmentReport, final bool, at time.Time, labels []string) {
	t.Helper()
	rec := types.CommitmentRecord{
		Report: &report, Inbox: "texture:" + doc, SenderInbox: "research:" + doc, Addressee: "texture:" + doc, Final: final,
	}
	rec.SchemaID, rec.RecordID = types.CommitmentRecordSchemaV1, id
	rec.Kind = types.CommitmentKindReport
	rec.EmitterKind, rec.Discrepancy = types.CommitmentEmitterAgent, types.DiscrepancyUnresolved
	rec.Provenance = types.CommitmentProvenance{AgentID: rec.SenderInbox, CommittedAt: at.UTC().Format(time.RFC3339Nano), Labels: labels}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:" + id, Records: []types.CommitmentRecord{rec}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("commit %s: %v", id, err)
	}
}

// commitCastWithStory commits a Texture's Cast to its research desk carrying the story's own
// account, as the Texture's turn does (store.attachStoryToResearchCasts).
func commitCastWithStory(t *testing.T, s *store.Store, owner, computer, doc, id string, story *types.CastStory, at time.Time) {
	t.Helper()
	rec := types.CommitmentRecord{
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "find sources"},
		Cast:      &types.CommitmentCast{Story: story}, Inbox: "research:" + doc, SenderInbox: "texture:" + doc, Addressee: "research:" + doc,
	}
	commitStoryRecord(t, s, owner, computer, id, rec, at)
}

// buildDigest and buildBrief run the build with the object-graph gates in ENFORCE, as the
// product ships them (TestMain runs this package's fixtures in observe): a digest that
// read a Texture document as the research desk would be refused and lose its story.
func buildDigest(t *testing.T, rt *Runtime, rec *types.RunRecord) *researchDigest {
	t.Helper()
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	return rt.researchDigestOpening(context.Background(), rec)
}

func buildBrief(t *testing.T, rt *Runtime, rec *types.RunRecord) *researchBrief {
	t.Helper()
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	return rt.researchBriefOpening(context.Background(), rec)
}

func digestFixture(t *testing.T) (rt *Runtime, s *store.Store, owner, computer, doc string, sink digestSink) {
	rt, s = testRuntime(t)
	owner, computer = "owner-digest", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc = openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1", "c2"}, t0)
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c3"}, t0.Add(5*time.Minute))
	commitCastWithStory(t, s, owner, computer, doc, "cell-3:cast:r1", &types.CastStory{
		Title: "Dock strike", MainPoint: "Whether the dock strike ends and on what terms", RevisionID: "rev-7",
		Text: "Dockworkers struck on Monday over a wage floor; talks are set for Tuesday.", Labels: []string{types.OwnerPrivateLabel("document")},
	}, t0.Add(10*time.Minute))
	commitReport(t, s, owner, computer, doc, "cell-4:report:1", types.CommitmentReport{
		Summary: "Talks resumed on Tuesday; the union dropped its demand for a wage floor.",
		Notes:   []string{"adds: the union's changed demand, from the capture c1", "missing: the employer's side of the offer (the corpus had only union sources)"},
		Claims:  []types.ReportClaim{{Text: "The union dropped its wage-floor demand.", Sources: []string{"s1"}}},
	}, false, t0.Add(20*time.Minute))
	sink = digestSink{ctxSink: &ctxSink{urls: map[string]string{"c1": "https://port.example/1", "c2": "https://port.example/2", "c3": "https://port.example/3"}},
		text: map[string]string{"c1": "The union said on Tuesday it no longer demands a wage floor.", "c2": "Port officials confirmed the talks.", "c3": "A fed capture about the employer."}}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	return rt, s, owner, computer, doc, sink
}

func TestADigestHoldsTheStorysTextWhatEarlierRoundsFoundAndAPassageOfEachCapture(t *testing.T) {
	rt, _, owner, computer, doc, _ := digestFixture(t)
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-d1"))
	if d == nil || strings.TrimSpace(d.text) == "" {
		t.Fatal("a story that holds material got no digest")
	}
	for _, want := range []string{
		"[This story]", "Title: Dock strike", "Main point: Whether the dock strike ends and on what terms", "Dockworkers struck on Monday over a wage floor", "revision rev-7",
		"[Earlier research on this story]", "Talks resumed on Tuesday; the union dropped its demand for a wage floor.",
		"missing: the employer's side of the offer", "adds: the union's changed demand",
		"capture_id c1 https://port.example/1", ": The union said on Tuesday it no longer demands a wage floor.",
		"capture_id c2", ": Port officials confirmed the talks.", "capture_id c3", ": A fed capture about the employer.",
	} {
		if !strings.Contains(d.text, want) {
			t.Errorf("the digest lacks %q:\n%s", want, d.text)
		}
	}
	if d.kinds["story"] != 1 || d.kinds["reports"] != 1 || d.kinds["passages"] != 3 || d.kinds["pointers"] != 0 {
		t.Errorf("kinds = %v", d.kinds)
	}
	if len(d.captures) != 3 {
		t.Errorf("captures = %v, want the three the story holds", d.captures)
	}
	if d.tokens <= 0 || d.tokens > researchDigestMaxTokens {
		t.Errorf("tokens = %d, want within 1..%d", d.tokens, researchDigestMaxTokens)
	}
	if line := d.digestLogFields(); !strings.Contains(line, "digest_passages=3") || !strings.Contains(line, "digest_reports=1") || !strings.Contains(line, " digest_tokens=") {
		t.Errorf("log fields = %q", line)
	}
	// It does not pad: a story with no earlier round and no capture has a short digest.
	if len(d.text) > 6000 {
		t.Errorf("a small story gave a %d byte digest", len(d.text))
	}
	t.Logf("fixture digest: %d bytes, %d tokens (est.), %d ms; kinds %v", len(d.text), d.tokens, d.ms, d.kinds)
}

func TestADigestNeverCrossesOwners(t *testing.T) {
	rt, s, owner, computer, doc, _ := digestFixture(t)
	other := "owner-stranger"
	// A run of another owner naming this owner's story: the store reads are scoped by
	// owner and computer, so nothing of the story comes back.
	d := buildDigest(t, rt, briefRunRecord(other, computer, doc, "run-x"))
	if d != nil && strings.TrimSpace(d.text) != "" {
		t.Fatalf("another owner's run was handed this story's digest:\n%s", d.text)
	}
	// A run of this owner on another computer naming this story: scoped by computer too.
	d = buildDigest(t, rt, briefRunRecord(owner, "another-computer", doc, "run-z"))
	if d != nil && strings.TrimSpace(d.text) != "" {
		t.Fatalf("another computer's run was handed this story's digest:\n%s", d.text)
	}
	// And the other owner's own story shows none of this owner's text.
	t0 := time.Now().UTC().Add(-time.Hour)
	otherDoc := openStoryOf(t, s, other, computer, "cell-9:cast:o1", "Unrelated vote", "Who wins the unrelated vote and by how much", "", nil, t0)
	d = buildDigest(t, rt, briefRunRecord(other, computer, otherDoc, "run-y"))
	if d == nil || strings.Contains(d.text, "Dock strike") || strings.Contains(d.text, "wage floor") || strings.Contains(d.text, "port.example") {
		t.Fatalf("the other owner's digest holds this owner's material (owner %s):\n%v", owner, d)
	}
}

func TestADigestDegradesBySectionWhenACaptureReadIsSlow(t *testing.T) {
	rt, _, owner, computer, doc, sink := digestFixture(t)
	sink.slow = map[string]bool{"c1": true}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	buf := briefCaptureLog(t)
	start := time.Now()
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-d4"))
	if time.Since(start) > researchBriefDeadline+time.Second {
		t.Fatalf("the build ran %s, past the brief's %s deadline", time.Since(start), researchBriefDeadline)
	}
	if d == nil || !strings.Contains(d.text, "[This story]") || !strings.Contains(d.text, "Title: Dock strike") || !strings.Contains(d.text, "Talks resumed on Tuesday") {
		t.Fatalf("a slow capture read cost the story's own sections:\n%v", d)
	}
	if !strings.Contains(d.text, "capture_id c1") || strings.Contains(d.text, "capture_id c1 https://port.example/1: ") || d.kinds["pointers"] != 1 || d.kinds["passages"] != 2 {
		t.Fatalf("the slow capture must keep its pointer and the others their passages: %v\n%s", d.kinds, d.text)
	}
	if !strings.Contains(buf.String(), "research digest: capture read timed out capture=c1") && !strings.Contains(buf.String(), "story research context: capture read timed out") {
		t.Fatalf("the timeout is not logged: %q", buf.String())
	}
}

func TestABriefCarriesTheDigestAndItsLogLine(t *testing.T) {
	rt, _, owner, computer, doc, _ := digestFixture(t)
	buf := briefCaptureLog(t)
	b := buildBrief(t, rt, briefRunRecord(owner, computer, doc, "run-d6"))
	if b == nil || b.digest == nil {
		t.Fatal("the run's opening carries no digest")
	}
	text := b.openingText()
	if !strings.Contains(text, "Talks resumed on Tuesday") || !strings.Contains(text, "capture_id c1") {
		t.Fatalf("the opening text lacks the digest:\n%s", text)
	}
	b.finish(true)
	line := buf.String()
	for _, want := range []string{"research brief run=run-d6", " digest_tokens=", " digest_story=1", " digest_reports=1", " digest_passages=3", " digest_ms=", " digest_captures="} {
		if !strings.Contains(line, want) {
			t.Errorf("the brief line lacks %q: %s", want, line)
		}
	}
}

func TestAReportKeepsItsSummaryAndNotes(t *testing.T) {
	packet := `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"Found the vote.","notes":["adds: the vote count","missing: the second outlet"],"claims":[{"claim_id":"k1","text":"A won","source_ids":["s1"]}],"work_disposition":"completed"}`
	report, final := reportBodyFromPacket(packet, "")
	if !final || report.Summary != "Found the vote." || len(report.Notes) != 2 || report.Notes[1] != "missing: the second outlet" {
		t.Fatalf("report = %+v final %v", report, final)
	}
}

func TestAResearchReportLineCountsCorpusAndRunCapturedSourcesAndTheGap(t *testing.T) {
	packet := `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x",
"notes":["adds: a thing","missing: the employer's side"],
"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1","capture_id":"cap-corpus"}},
{"source_id":"s2","kind":"web_source","target":{"uri":"https://b.example/2","capture_id":"cap-run"}},
{"source_id":"s3","kind":"web_source","target":{"uri":"https://c.example/3"}}],
"claims":[{"text":"a","source_ids":["s1"]},{"text":"b","source_ids":["s2"]}]}`
	researchtools.NoteCaptureMadeByRunForTest("run-rl", "cap-run")
	line := researchReportLine("run-rl", "research:doc-1", packet, true)
	want := "research report run=run-rl final=true sources=3 from_corpus=1 by_run=1 no_capture=1 claims=2 adds=true gap=true"
	if line != want {
		t.Fatalf("line = %q\nwant   %q", line, want)
	}
	if researchReportLine("run-rl", "texture:doc-1", packet, true) != "" {
		t.Fatal("only a research desk's Report is counted")
	}
}

// The vector neighbourhood (owner, 2026-11-22: "we can run captures through vector search,
// then for each match we can do graph traversal. this is then sampled and provided with
// metadata and full data accessible in repl"): one hop, one corpusd request, scoped to the
// run's owner and computer; sampled by distinct outlet with the rest counted.
func TestADigestAddsTheVectorNeighbourhoodSampledByOutletAndScoped(t *testing.T) {
	rt, _, owner, computer, doc, sink := digestFixture(t)
	var queries []researchtools.CaptureNeighboursQuery
	sink.queries = &queries
	sink.neighbours = map[string][]researchtools.CaptureNeighbour{
		"c1": {{CaptureID: "n1", Score: 0.91}, {CaptureID: "n2", Score: 0.88}, {CaptureID: "n3", Score: 0.80}},
		"c2": {{CaptureID: "n4", Score: 0.77}, {CaptureID: "c3", Score: 0.99}},
	}
	for id, url := range map[string]string{"n1": "https://wire.example/1", "n2": "https://wire.example/2", "n3": "https://other.example/3", "n4": "https://third.example/4"} {
		sink.urls[id] = url
		sink.text[id] = "passage of " + id
	}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-nb"))
	if d == nil {
		t.Fatal("no digest")
	}
	if len(queries) != 1 || queries[0].OwnerID != owner || queries[0].ComputerID != computer || len(queries[0].CaptureIDs) != 3 {
		t.Fatalf("one neighbours request scoped to the run's owner and computer, seeded by the story's captures: %+v", queries)
	}
	for _, want := range []string{"[Nearby in the graph]", "capture_id n1 https://wire.example/1", "[vector 0.91, near c1]: passage of n1", "capture_id n3", "capture_id n4"} {
		if !strings.Contains(d.text, want) {
			t.Errorf("the digest lacks %q:\n%s", want, d.text)
		}
	}
	// n2 is the second capture of an outlet already shown; c3 is the story's own.
	if strings.Contains(d.text, "capture_id n2") || strings.Count(d.text, "capture_id c3") != 1 {
		t.Errorf("sampling by outlet / dedupe failed:\n%s", d.text)
	}
	if d.kinds["neighbours"] != 3 || d.more != 1 || !strings.Contains(d.text, "1 read and left out") {
		t.Errorf("kinds %v more %d: the one left out must be counted and said", d.kinds, d.more)
	}
}

func TestAPrivateNeighbourIsDroppedAndEveryReadIsBoundToTheRunsOwnerAndComputer(t *testing.T) {
	rt, _, owner, computer, doc, sink := digestFixture(t)
	var readers [][2]string
	var queries []researchtools.CaptureNeighboursQuery
	sink.readers, sink.queries = &readers, &queries
	sink.neighbours = map[string][]researchtools.CaptureNeighbour{"c1": {{CaptureID: "private-n", Score: 0.95}, {CaptureID: "public-n", Score: 0.90}}}
	sink.urls["private-n"], sink.text["private-n"] = "https://secret.example/1", "an owner-private page"
	sink.urls["public-n"], sink.text["public-n"] = "https://open.example/2", "a public page"
	sink.labels = map[string][]string{"private-n": {types.OwnerPrivateLabel("capture")}}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-scope"))
	if d == nil || strings.Contains(d.text, "private-n") || strings.Contains(d.text, "an owner-private page") || strings.Contains(d.text, "secret.example") {
		t.Fatalf("a private neighbour reached the run:\n%v", d)
	}
	if !strings.Contains(d.text, "capture_id public-n") {
		t.Fatalf("the public neighbour is missing:\n%s", d.text)
	}
	if len(readers) == 0 {
		t.Fatal("no capture was read")
	}
	for _, r := range readers {
		if r[0] != owner || r[1] != computer {
			t.Fatalf("a capture read ran as owner %q computer %q, not the run's %q %q", r[0], r[1], owner, computer)
		}
	}
	for _, q := range queries {
		if q.OwnerID != owner || q.ComputerID != computer {
			t.Fatalf("the neighbours request carried owner %q computer %q", q.OwnerID, q.ComputerID)
		}
	}
	if d.reads == 0 || d.requests == 0 || !strings.Contains(d.digestLogFields(), " digest_reads=") || !strings.Contains(d.digestLogFields(), " digest_requests=") {
		t.Fatalf("the digest's cost is not counted: reads %d requests %d", d.reads, d.requests)
	}
	// Each capture is read once: the story's three and the two neighbours.
	if d.reads != len(readers) || len(readers) > 6 {
		t.Fatalf("%d capture reads recorded %d: each capture must be read once", d.reads, len(readers))
	}
	// The digest holds the story's own text, private to the owner: the run is tainted.
	if !d.labels.Has(types.OwnerPrivateLabel("document")) {
		t.Fatalf("labels = %v: the story's text is owner_private:document", d.labels)
	}
}

func TestAReportRecordBoundsItsSummaryAndNotesAtWrite(t *testing.T) {
	var notes []string
	for i := 0; i < 30; i++ {
		notes = append(notes, "note "+strings.Repeat("x", 5000))
	}
	raw, _ := json.Marshal(map[string]any{"schema_version": "coagent_source_packet.v1", "kind": "evidence_update", "summary": strings.Repeat("s", 9000), "notes": notes})
	report, _ := reportBodyFromPacket(string(raw), "")
	if len(report.Notes) != reportNotesMax || len([]rune(report.Summary)) != reportSummaryRunes {
		t.Fatalf("notes %d summary %d: want %d and %d", len(report.Notes), len([]rune(report.Summary)), reportNotesMax, reportSummaryRunes)
	}
	for _, n := range report.Notes {
		if len([]rune(n)) > reportNoteRunes {
			t.Fatalf("a note of %d runes was kept", len([]rune(n)))
		}
	}
}

func TestADigestNamesThePagesItDoesNotListAndUnionsTheLabelsOfWhatItShows(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-many", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	var ids []string
	sink := digestSink{ctxSink: &ctxSink{urls: map[string]string{}}, text: map[string]string{}, labels: map[string][]string{}}
	for i := 1; i <= 45; i++ {
		id := fmt.Sprintf("m%02d", i)
		ids = append(ids, id)
		sink.urls[id], sink.text[id] = fmt.Sprintf("https://m%02d.example/p", i), "passage of "+id
	}
	sink.labels["m45"] = []string{types.LabelPublisherPrefix + "example"}
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Many pages", "A story that holds more pages than the digest lists", "", ids, t0)
	commitCastWithStory(t, s, owner, computer, doc, "cell-3:cast:r1", &types.CastStory{Title: "Many pages", Text: "text", Labels: []string{types.OwnerPrivateLabel("document")}}, t0.Add(time.Minute))
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-many"))
	if d == nil || !strings.Contains(d.text, "(5 more pages are held and not listed here") {
		t.Fatalf("the five pages past the 40 listed are not named:\n%v", d)
	}
	if d.kinds["passages"] != storyContextMaxPages {
		t.Fatalf("passages = %d, want %d (newest fed first)", d.kinds["passages"], storyContextMaxPages)
	}
	if !strings.Contains(d.text, "capture_id m45") || strings.Contains(d.text, "capture_id m01 ") {
		t.Fatalf("the newest pages must be listed and the oldest left to the count:\n%s", d.text)
	}
	if !d.labels.Has(types.OwnerPrivateLabel("document")) || !d.labels.Has(types.LabelPublisherPrefix+"example") || !d.labels.Has(types.LabelWorldUntrusted) {
		t.Fatalf("labels = %v: the union of the document's, the shown captures' and outside text's", d.labels)
	}
}
