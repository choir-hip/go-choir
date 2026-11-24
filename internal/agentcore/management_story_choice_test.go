package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The feed-or-open choice, logged (search-reuse commit 4, from
// docs/problems/research-does-not-start-from-the-corpus-2026-11-18.md,
// "Remaining error"). Failure list, written before the code:
//  1. The choice management makes at a commit (a task to an existing story,
//     texture:<doc>, or texture:new) leaves no line with the run id, the
//     choice, the existing story's id when fed, and how many candidate
//     stories the run was shown.
//  2. The shown count adds a page twice, or carries one run's count into
//     another's, or is never forgotten (one entry per run, for ever).
//  3. The line carries a title or a lede (ids and counts only).
func TestStoriesShownCountsPerRunAndIsForgotten(t *testing.T) {
	rt := &Runtime{}
	rt.noteStoriesShown("run-1", 20)
	rt.noteStoriesShown("run-1", 5)
	rt.noteStoriesShown("run-2", 3)
	if got := rt.storiesShownCount("run-1"); got != 25 {
		t.Fatalf("run-1 shown = %d, want 25", got)
	}
	if got := rt.storiesShownCount("run-2"); got != 3 {
		t.Fatalf("run-2 shown = %d, want 3", got)
	}
	rt.forgetStoriesShown("run-1")
	if got := rt.storiesShownCount("run-1"); got != 0 {
		t.Fatalf("run-1 shown after forget = %d, want 0", got)
	}
}

func TestManagementStoryChoiceLine(t *testing.T) {
	if got := managementStoryChoiceLine("run-9", "feed", "doc-4", 50, 25, choiceDetail{}); got != "management story choice run=run-9 choice=feed story=doc-4 opening_shown=50 verb_shown=25 candidates_shown=75 window=- page_in_window=0 page_left_out=0 offered=- answers=- captures=0 sources=near:0,lead:0 main_point=0 succeeds=- capture_ids=-" {
		t.Fatalf("feed line = %q", got)
	}
	if got := managementStoryChoiceLine("run-9", "open", "", 0, 0, choiceDetail{}); got != "management story choice run=run-9 choice=open story=- opening_shown=0 verb_shown=0 candidates_shown=0 window=- page_in_window=0 page_left_out=0 offered=- answers=- captures=0 sources=near:0,lead:0 main_point=0 succeeds=- capture_ids=-" {
		t.Fatalf("open line = %q", got)
	}
}

// Defect 2 (recurring-agent-failures record, 2026-11-19): the line counted only
// choir.Stories calls, not the page the task's opening delivered.
func TestOpeningStoriesShownIsCountedSeparately(t *testing.T) {
	rt := &Runtime{}
	rt.noteOpeningOffer("run-1", &openingOffer{Shown: 50})
	rt.noteOpeningOffer("run-2", nil)
	rt.noteStoriesShown("run-1", 7)
	if got := rt.openingShownCount("run-1"); got != 50 {
		t.Fatalf("opening shown = %d, want 50", got)
	}
	if got := rt.storiesShownCount("run-1"); got != 7 {
		t.Fatalf("verb shown = %d, want 7", got)
	}
	if got := rt.openingShownCount("run-2"); got != 0 {
		t.Fatalf("unreadable page counted %d", got)
	}
	rt.forgetStoriesShown("run-1")
	if rt.openingShownCount("run-1") != 0 || rt.storiesShownCount("run-1") != 0 {
		t.Fatal("forget left a count behind")
	}
}

// Defect 1: a cursor the host issued must pass the broker's own bound, at its
// longest (nine fractional digits, the longest document id).
func TestStoriesCursorFitsTheBrokerBound(t *testing.T) {
	at := time.Date(2026, 11, 19, 3, 39, 12, 123456789, time.UTC)
	for _, doc := range []string{strings.Repeat("a", 36), strings.Repeat("a", 64)} {
		next := storiesCursor(at, doc)
		payload, _ := json.Marshal(yaegikernel.StoriesPayload{Limit: 50, Before: next})
		if err := yaegikernel.ValidatePayload(yaegikernel.ActionStories, payload); err != nil {
			t.Errorf("a cursor of %d bytes the host issued is refused: %v", len(next), err)
		}
	}
	junk, _ := json.Marshal(yaegikernel.StoriesPayload{Before: strings.Repeat("x", yaegikernel.MaxStoriesBeforeBytes+1)})
	if yaegikernel.ValidatePayload(yaegikernel.ActionStories, junk) == nil {
		t.Error("an oversize before was accepted")
	}
}

// Each Cast a story's Texture sends to its research desk leaves one line with
// its cause decided at the source; nothing else does (another desk's Cast, a
// Cast to another inbox). A successor is a first round of a story whose
// trajectory names a succeeds ref.
func TestStoryResearchCastLineCarriesItsCauseAtTheSource(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-research-cause", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	plain := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Plain story", "Whether the plain thing happens at all", "", []string{"c1"}, t0)
	succ := openStoryOf(t, s, owner, computer, "cell-2:cast:c1", "Next story", "What the plain thing led to afterwards", "texture:"+plain, []string{"c2"}, t0)
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	rec := func(inbox string, opened bool, round int) types.InboxRecordReceipt {
		return types.InboxRecordReceipt{Inbox: inbox, OpenedRound: opened, Round: round}
	}
	ctx := context.Background()
	tex := deskmanifest.DocumentCustodianName()
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(succ), true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), true, 3))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), false, 3))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec("management:o", true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-m", "management", rec(deskmanifest.ResearchInbox(plain), true, 1))
	lines := []string{}
	for _, l := range strings.Split(out.String(), "\n") {
		if i := strings.Index(l, "story research cast"); i >= 0 {
			lines = append(lines, l[i:])
		}
	}
	want := []string{
		"story research cast run=run-t story=" + plain + " round=1 opened=true cause=open",
		"story research cast run=run-t story=" + succ + " round=1 opened=true cause=successor",
		"story research cast run=run-t story=" + plain + " round=3 opened=true cause=feed",
		"story research cast run=run-t story=" + plain + " round=3 opened=false cause=continue",
	}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q\nwant   %q", lines, want)
	}
}

// A feed sent as a Cast (choir.Cast("texture:<doc>", ...)) carries its captures
// in the spec, not in the intent's Captures: the line counts them from the
// packet, and names the candidates offered for them (review round 3, finding 2:
// it logged captures=0 and every candidate).
func TestFeedSentAsACastLogsItsCapturesFromThePacket(t *testing.T) {
	rt := &Runtime{}
	rt.noteOpeningOffer("run-f", &openingOffer{
		ByCapture: map[string][]string{"c1": {"docA"}, "c2": {"docA", "docB"}, "c9": {"docZ"}},
		Found:     map[string]map[string][]string{"c1": {"docA": {"near:1"}}, "c2": {"docA": {"lead"}}},
	})
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	rec := types.InboxRecordReceipt{Inbox: deskmanifest.DocumentInbox("docA")}
	rt.logManagementStoryChoice("run-f", deskmanifest.AdmitterName(), rec, yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentCast, ToDesk: deskmanifest.DocumentInbox("docA"), Statement: `{"captures":["c1","c2"]}`})
	line := out.String()
	for _, want := range []string{"choice=feed", "story=docA", "captures=2", "offered=docA,docB", "sources=near:1,lead:1"} {
		if !strings.Contains(line, want) {
			t.Errorf("a feed sent as a Cast: the line lacks %q: %q", want, line)
		}
	}
	if strings.Contains(line, "docZ") {
		t.Errorf("a candidate offered for a capture the feed does not name is in the line: %q", line)
	}
	// choir.Feed carries its captures on the intent and no packet.
	out.Reset()
	rt.logManagementStoryChoice("run-f", deskmanifest.AdmitterName(), rec, yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentFeed, Captures: []string{"c1"}})
	if !strings.Contains(out.String(), "captures=1") {
		t.Errorf("a Feed verb's captures are not counted: %q", out.String())
	}
}

// R0: the story-choice line gains its capture ids, and the host writes one
// decision-record row per capture from the choice it observed, with the
// candidates offered for that capture alone. Ways this fails: the line lacks
// the ids; a row carries another capture's candidates; a decline or open
// writes no rows; the row names management as having written it (the decider
// is observed, the writer is the decisions service); a non-management profile
// writes rows.
func TestChoiceLineNamesCaptureIDsAndRowsAreOnePerCapture(t *testing.T) {
	rt := &Runtime{}
	rt.noteOpeningOffer("run-r", &openingOffer{ByCapture: map[string][]string{"c1": {"docA"}, "c2": {"docB", "docC"}}})
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	p := 0.8
	rec := types.InboxRecordReceipt{Inbox: deskmanifest.DocumentInbox("docA")}
	c := rt.logManagementStoryChoice("run-r", deskmanifest.AdmitterName(), rec, yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentFeed, Captures: []string{"c1", "c2", "c1"}, Req: &types.RequestFields{PSuccess: &p}})
	if c == nil || !strings.Contains(out.String(), "capture_ids=c1,c2") {
		t.Fatalf("line = %q, choice = %+v", out.String(), c)
	}
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	rows := rt.decisionRows("o1", "comp1", "run-r", "cmd-1", *c, at)
	if len(rows) != 2 || rows[0].CaptureID != "c1" || rows[1].CaptureID != "c2" {
		t.Fatalf("rows = %+v", rows)
	}
	if got := rows[0].Candidates; len(got) != 1 || got[0] != "docA" {
		t.Errorf("c1 candidates = %v", got)
	}
	if got := rows[1].Candidates; len(got) != 2 || got[0] != "docB" {
		t.Errorf("c2 candidates = %v", got)
	}
	r := rows[0]
	if r.Stage != types.DecisionStageManagementRoute || r.Decision != "feed" || r.Target != "docA" || r.DeciderKind != types.DeciderDesk || r.Decider != deskmanifest.AdmitterName() || r.CommandID != "cmd-1" || r.RunID != "run-r" || r.Probability == nil || *r.Probability != 0.8 || !r.At.Equal(at) {
		t.Errorf("row = %+v", r)
	}
	d := rt.logManagementStoryChoice("run-r", deskmanifest.AdmitterName(), types.InboxRecordReceipt{}, yaegikernel.StagedIntent{Kind: yaegikernel.IntentDecline, Captures: []string{"c2"}})
	if d == nil || d.Choice != "decline" || d.Story != "" || len(rt.decisionRows("o1", "comp1", "run-r", "cmd-2", *d, at)) != 1 || rt.decisionRows("o1", "comp1", "run-r", "cmd-2", *d, at)[0].Probability != nil {
		t.Errorf("decline = %+v", d)
	}
	// An open whose captures are on the intent and not in the packet still names them.
	o := rt.logManagementStoryChoice("run-r", deskmanifest.AdmitterName(), types.InboxRecordReceipt{OpenedStory: true, Inbox: deskmanifest.DocumentInbox("docN")},
		yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, Captures: []string{"c9"}, Statement: "{}"})
	if o == nil || o.Choice != types.DecisionOpen || len(o.Captures) != 1 || o.Captures[0] != "c9" {
		t.Errorf("open = %+v, want the intent's capture c9", o)
	}
	if got := rt.logManagementStoryChoice("run-r", "texture", rec, yaegikernel.StagedIntent{Kind: yaegikernel.IntentFeed, Captures: []string{"c1"}}); got != nil {
		t.Errorf("a non-management profile made a choice: %+v", got)
	}
}

// decisionLogBuf is a log sink safe for a concurrent writer.
type decisionLogBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (d *decisionLogBuf) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.b.Write(p)
}
func (d *decisionLogBuf) String() string { d.mu.Lock(); defer d.mu.Unlock(); return d.b.String() }

// R0: rows reach the store through appendDecisionRows and are read back by
// the route; a failure to write never panics or returns an error to the
// decision (the record is observation); the route refuses an unauthenticated
// caller and a bad window.
func TestDecisionRowsReachTheStoreAndTheRoute(t *testing.T) {
	rt, st := testRuntime(t)
	rt.cfg.ComputerID = "computer-d1"
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	rt.appendDecisionRows(context.Background(), []types.DecisionRecord{{OwnerID: "owner-d1", ComputerID: "computer-d1", CaptureID: "cap-1", Stage: types.DecisionStageManagementRoute,
		Decision: "open", Target: "doc-9", DeciderKind: types.DeciderDesk, Decider: "management", RunID: "r", CommandID: "cmd", At: at}})
	got, err := st.ListDecisionRecords(context.Background(), "owner-d1", "computer-d1", types.DecisionFilter{CaptureID: "cap-1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("store rows = %v, %v", got, err)
	}
	// An invalid row is logged, not returned.
	logged := &decisionLogBuf{}
	log.SetOutput(logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	rt.appendDecisionRows(context.Background(), []types.DecisionRecord{{OwnerID: "owner-d1"}})
	log.SetOutput(os.Stderr)
	if !strings.Contains(logged.String(), "decision record: 1 rows not written") {
		t.Fatalf("an invalid row was not logged: %q", logged.String())
	}
	if again, _ := st.ListDecisionRecords(context.Background(), "owner-d1", "computer-d1", types.DecisionFilter{}); len(again) != 1 {
		t.Fatalf("an invalid row changed the store: %d rows", len(again))
	}
	// A cell cancelled right after its commit still gets its rows written.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	rt.appendDecisionRows(cancelled, []types.DecisionRecord{{OwnerID: "owner-d1", ComputerID: "computer-d1", CaptureID: "cap-2", Stage: types.DecisionStageManagementRoute,
		Decision: "decline", DeciderKind: types.DeciderDesk, Decider: "management", RunID: "r", CommandID: "cmd", At: at}})
	if got2, _ := st.ListDecisionRecords(context.Background(), "owner-d1", "computer-d1", types.DecisionFilter{CaptureID: "cap-2"}); len(got2) != 1 {
		t.Fatalf("a cancelled cell lost its rows: %d", len(got2))
	}
	(*Runtime)(nil).appendDecisionRows(context.Background(), got)

	h := &APIHandler{rt: rt}
	get := func(query string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/decisions"+query, nil)
		if auth {
			req.Header.Set("X-Authenticated-User", "owner-d1")
		}
		rec := httptest.NewRecorder()
		h.HandleDecisions(rec, req)
		return rec
	}
	if rec := get("", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", rec.Code)
	}
	if rec := get("?since=yesterday", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad since = %d", rec.Code)
	}
	rec := get("?capture=cap-1&stage=management_route", true)
	var out struct {
		Decisions []types.DecisionRecord `json:"decisions"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Decisions) != 1 || out.Decisions[0].Target != "doc-9" {
		t.Fatalf("route = %d %s", rec.Code, rec.Body)
	}
	if rec := get("?capture=cap-none", true); !strings.Contains(rec.Body.String(), `"decisions":[]`) {
		t.Fatalf("an unknown capture answered %s", rec.Body)
	}
}
