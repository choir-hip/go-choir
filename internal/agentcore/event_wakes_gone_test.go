package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list (W3): production by event (intake.event_wakes) is deleted. It
// failed its false-merge bar three times (7.0%, 9.5%, 15 to 30% against 5%),
// never ran on a production computer, and was the owner's default delete.
//  1. TestEventWakesIsNotASetting: POST /api/computer-settings with
//     intake.event_wakes must be refused as unknown, as intake.events is.
//  2. TestAPollsArrivalIsAnnouncedNotHeld: a source poll's arrival names its
//     captures and is not Held, so management is woken on it, whatever a
//     computer once stored.
//  2b. TestAStoredHeldArrivalIsReadAsAPlainOne: the retired path wrote
//     arrivals with Held set and no Event; a computer that ran it may hold
//     one unread. The reader delivers it once, like any arrival, and the
//     cursor passes it; with no sweep nothing else would ever show it.
//  3. TestSettingAValueItHasKeepsItsTime: the same-value write keeps its
//     updated_at (this test lived with the event-wakes file; it now uses
//     intake.labels).
//  4. TestWorldEventsAPIListsEventsWithMembersForTheOwnerOnly: the events
//     route is unchanged (it lived in the deleted file).

func TestEventWakesIsNotASetting(t *testing.T) {
	rt, _ := testRuntime(t)
	handler := &APIHandler{rt: rt}
	req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(`{"key":"intake.event_wakes","value":true}`))
	req.Header.Set("X-Authenticated-User", "owner-gone")
	rec := httptest.NewRecorder()
	handler.HandleComputerSettings(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown setting") {
		t.Fatalf("intake.event_wakes = %d %s; want it refused as unknown", rec.Code, rec.Body)
	}
}

func TestAPollsArrivalIsAnnouncedNotHeld(t *testing.T) {
	ctx := context.Background()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Held</title>`+
			`<item><title>Dam inspection ordered</title><link>https://held.example/1</link><description>Body held</description><guid>h-1</guid></item>`+
			`</channel></rss>`)
	}))
	t.Cleanup(feed.Close)
	rt, s := testRuntime(t)
	rt.SetCandidateJudge(&eventJudge{answers: map[string]eventAnswer{}})
	owner, computer, sourceID := "owner-held", rt.cfg.ComputerID, "rss:held"
	management := agentprofile.Management + ":" + owner
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: sourceID, Type: sources.SourceTypeRSS, Name: "Held", URL: feed.URL}}}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: &arrivalSink{seen: map[string]string{}}}
	var wakes []armedWake
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	if _, err := rt.StartSourceSchedule(ctx, owner, computer, sourceID, 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleSourcePollDue(ctx, owner, computer, wakes[0].content); err != nil {
		t.Fatal(err)
	}
	records, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	if err != nil || len(records) != 1 || records[0].Captures == nil || records[0].Captures.Held {
		t.Fatalf("arrival = %+v, %v; want one arrival that is not held", records, err)
	}
	if !store.RecordWakesReader(records[0]) && len(records[0].Captures.StartedTasks) == 0 {
		t.Fatalf("the arrival neither wakes management nor started its intake task: %+v", records[0])
	}
}

func TestSettingAValueItHasKeepsItsTime(t *testing.T) {
	rt, s := testRuntime(t)
	owner := "owner-same"
	handler := &APIHandler{rt: rt}
	post := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(body))
		req.Header.Set("X-Authenticated-User", owner)
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("set = %d %s", rec.Code, rec.Body)
		}
	}
	post(`{"key":"intake.labels","value":true}`)
	first, err := s.GetComputerSetting(context.Background(), owner, rt.TextureComputerID(), store.SettingIntakeLabels)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	post(`{"key":"intake.labels","value":true}`)
	again, _ := s.GetComputerSetting(context.Background(), owner, rt.TextureComputerID(), store.SettingIntakeLabels)
	if !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("an unchanged value moved updated_at from %s to %s", first.UpdatedAt, again.UpdatedAt)
	}
}

func eurotunnelJudge() *eventJudge {
	return &eventJudge{answers: map[string]eventAnswer{
		"Bus and train collide at Channel Tunnel terminal": {target: "Nineteen injured in Eurotunnel accident"},
		"Eurotunnel crash injures 19":                      {target: "Nineteen injured in Eurotunnel accident"},
	}}
}

func TestWorldEventsAPIListsEventsWithMembersForTheOwnerOnly(t *testing.T) {
	rt, _, _ := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	owner, computer := "owner-api", rt.TextureComputerID()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
		capItem("c3", "Plex sells movie tickets", "https://arstechnica.com/3", ""),
	}); err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: rt}
	call := func(query string, auth string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodGet, "/api/autopaper/events"+query, nil)
		if auth != "" {
			req.Header.Set("X-Authenticated-User", auth)
		}
		rec := httptest.NewRecorder()
		handler.HandleWorldEvents(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}
	if code, _ := call("", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", code)
	}
	if _, body := call("", owner); body["total"] != float64(2) {
		t.Fatalf("events = %v, want 2", body["total"])
	}
	_, body := call("?corroborated=true", owner)
	events, _ := body["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("corroborated events = %v", body)
	}
	members, _ := events[0].(map[string]any)["member_list"].([]any)
	if len(members) != 2 || members[0].(map[string]any)["counted_origin"] == "" || members[0].(map[string]any)["owner_id"] != "" {
		t.Fatalf("members = %v", members)
	}
	if _, other := call("", "someone-else"); other["total"] != float64(0) {
		t.Fatalf("another owner sees %v events", other["total"])
	}
}

func TestAStoredHeldArrivalIsReadAsAPlainOne(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-stored-held", rt.cfg.ComputerID
	management := agentprofile.Management + ":" + owner
	req, err := capturesRecordRequest(owner, computer, "rss:old", "arrival-held-old", true, nil,
		[]types.CaptureArrival{{CaptureID: "cap-old-1", Title: "Waited on its event", URL: "https://old.example/1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.commitCapturesRecord(ctx, req); err != nil {
		t.Fatal(err)
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	if err != nil || len(recs) != 1 || !recs[0].Captures.Held || recs[0].Captures.Event != nil {
		t.Fatalf("stored record = %+v, %v; want one Held arrival with no Event", recs, err)
	}
	if !store.RecordWakesReader(recs[0]) {
		t.Fatal("a stored held arrival with no event does not count as unread work")
	}
	cr, err := rt.loadCellRecords(ctx, owner, computer, management, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cr.view) != 1 || cr.view[0].Captures == nil || len(cr.view[0].Captures.Items) != 1 || cr.view[0].Captures.Items[0].CaptureID != "cap-old-1" {
		t.Fatalf("the reader's view = %+v; want the held arrival delivered once", cr.view)
	}
}
