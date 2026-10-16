package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Production by event (event layer V3a, §15). Ways it could fail, written
// before the code (design §15.4, panel round 2):
//   - E10 a Jev outage silently stops production: the flush's captures
//     must reach management on an unresolved record that wakes it;
//   - E11 an event wakes twice for one corroboration, or a growth record
//     repeats members an earlier record named;
//   - B1 (Devin) a crash between a membership write and its record loses
//     the wake: the next flush must fill the gap from the ledger;
//   - E12 a held arrival wakes management or shows in its view as work;
//     growth wakes management (Gemini B2);
//   - E13 Decline of an event record's capture lands on source:event:…;
//   - E15 the aged official wake fires for a non-official source, or
//     twice;
//   - E16 stranded held captures stay invisible: the sweep must release
//     them once, without Jev, and drain everything when the setting is
//     turned off;
//   - the setting off mints nothing.

func setEventSettings(t *testing.T, rt *Runtime, owner, computer string, events, wakes bool, at time.Time) {
	t.Helper()
	for key, on := range map[string]bool{store.SettingIntakeEvents: events, store.SettingIntakeEventWakes: wakes} {
		if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: key, Value: on, UpdatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
}

func managementRecords(t *testing.T, rt *Runtime, owner, computer string) []types.CommitmentRecord {
	t.Helper()
	recs, err := rt.store.ListInboxRecords(context.Background(), owner, computer, agentprofile.Management+":"+owner, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func recordsWithPrefix(recs []types.CommitmentRecord, prefix string) []types.CommitmentRecord {
	var out []types.CommitmentRecord
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, prefix) {
			out = append(out, r)
		}
	}
	return out
}

func itemIDs(rec types.CommitmentRecord) string {
	var ids []string
	for _, item := range rec.Captures.Items {
		ids = append(ids, item.CaptureID)
	}
	return strings.Join(ids, ",")
}

func eurotunnelJudge() *eventJudge {
	return &eventJudge{answers: map[string]eventAnswer{
		"Bus and train collide at Channel Tunnel terminal": {target: "Nineteen injured in Eurotunnel accident"},
		"Eurotunnel crash injures 19":                      {target: "Nineteen injured in Eurotunnel accident"},
	}}
}

func TestCorroboratedEventWakesManagementOnceAndGrowthIsHeld(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
		capItem("c3", "Plex sells movie tickets", "https://arstechnica.com/3", ""),
	}
	report, err := rt.resolveEvents(ctx, owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	recs := managementRecords(t, rt, owner, computer)
	woke := recordsWithPrefix(recs, "event-corroborated:")
	if len(woke) != 1 || len(recs) != 1 {
		t.Fatalf("records after corroboration = %d (%d corroborated), want exactly one", len(recs), len(woke))
	}
	rec := woke[0]
	if !store.RecordWakesReader(rec) || rec.Captures.Held || rec.Captures.Event == nil || rec.Captures.Event.Reason != types.CaptureEventCorroborated ||
		len(rec.Captures.Event.Origins) != 2 || itemIDs(rec) != "c1,c2" {
		t.Fatalf("corroboration record = %+v / %+v", rec.Captures, rec.Captures.Event)
	}
	for _, item := range rec.Captures.Items {
		if item.SourceID != "src-"+item.CaptureID {
			t.Fatalf("E13: item %s names source %q", item.CaptureID, item.SourceID)
		}
	}

	// E11: the same flush again, and a restart, mint nothing new.
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	rt.resetEventIndexForTest()
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	if got := managementRecords(t, rt, owner, computer); len(got) != 1 {
		t.Fatalf("E11: a retry or a restart minted again: %d records", len(got))
	}

	// A third outlet: a held growth record naming only it, which wakes no
	// one but is in management's view.
	third := []eventInput{capItem("c4", "Eurotunnel crash injures 19", "https://www.euronews.com/4", "")}
	again, err := rt.resolveEvents(ctx, owner, computer, third)
	if err != nil {
		t.Fatal(err)
	}
	rt.mintEventWakes(ctx, owner, computer, third, again, nil)
	grew := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-grew:")
	if len(grew) != 1 || !grew[0].Captures.Held || store.RecordWakesReader(grew[0]) || itemIDs(grew[0]) != "c4" ||
		grew[0].Captures.Event.Reason != types.CaptureEventGrew {
		t.Fatalf("E12/E11: growth = %+v", grew)
	}
	view, err := rt.loadCellRecords(ctx, owner, computer, agentprofile.Management+":"+owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.view) != 2 {
		t.Fatalf("management's view = %d records, want the corroboration and the growth", len(view.view))
	}
}

func TestACrashBeforeTheRecordLeavesAGapTheNextFlushFills(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	// The memberships are written; the computer crashes before the
	// record is minted.
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}); err != nil {
		t.Fatal(err)
	}
	rt.resetEventIndexForTest()
	// B1: the next live flush, with nothing new of its own, mints the wake.
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	if woke := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-corroborated:"); len(woke) != 1 || itemIDs(woke[0]) != "c1,c2" {
		t.Fatalf("B1: the gap was not filled: %+v", woke)
	}
}

func TestAnEventCorroboratedBeforeTheSettingIsNotWokenAgain(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}); err != nil {
		t.Fatal(err)
	}
	// Turned on after the event was corroborated in shadow: management
	// saw c1 and c2 on their unheld arrivals.
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(time.Second))
	time.Sleep(1100 * time.Millisecond)
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	if got := managementRecords(t, rt, owner, computer); len(got) != 0 {
		t.Fatalf("an event corroborated before the setting woke again: %d records", len(got))
	}
	third := []eventInput{capItem("c4", "Eurotunnel crash injures 19", "https://www.euronews.com/4", "")}
	report, _ := rt.resolveEvents(ctx, owner, computer, third)
	rt.mintEventWakes(ctx, owner, computer, third, report, nil)
	got := managementRecords(t, rt, owner, computer)
	if len(got) != 1 || !strings.HasPrefix(got[0].RecordID, "event-grew:") || itemIDs(got[0]) != "c4" {
		t.Fatalf("its later member was not named on a held growth record: %+v", got)
	}
}

func TestAJevOutageSendsTheCapturesToManagementUnresolved(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Plex sells movie tickets", "https://arstechnica.com/2", ""),
	}
	rt.mintEventWakes(ctx, owner, computer, inputs, eventResolveReport{}, errors.New("jev: 503"))
	got := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-unresolved:")
	if len(got) != 1 || !store.RecordWakesReader(got[0]) || itemIDs(got[0]) != "c1,c2" || got[0].Captures.Event.Reason != types.CaptureEventUnresolved ||
		got[0].Captures.Items[1].SourceID != "src-c2" {
		t.Fatalf("E10: unresolved record = %+v", got)
	}
	// A replay of the same failure is the same record.
	rt.mintEventWakes(ctx, owner, computer, inputs, eventResolveReport{}, errors.New("jev: 503"))
	if n := len(managementRecords(t, rt, owner, computer)); n != 1 {
		t.Fatalf("a replayed failure minted %d records", n)
	}
}

func TestEventWakesOffMintsNothing(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, false, time.Now().UTC().Add(-time.Hour))
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}
	report, _ := rt.resolveEvents(ctx, owner, computer, inputs)
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	rt.mintEventWakes(ctx, owner, computer, inputs, eventResolveReport{}, errors.New("jev: 503"))
	if n := len(managementRecords(t, rt, owner, computer)); n != 0 {
		t.Fatalf("event_wakes off minted %d records", n)
	}
}

func TestAnOfficialSingleOutletEventWakesOnceWhenAged(t *testing.T) {
	rt, owner, computer := eventRuntime(t, &eventJudge{answers: map[string]eventAnswer{}})
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-3*time.Hour))
	rt.researchDeps = &researchtools.Dependencies{Sources: &sources.Registry{Sources: []sources.Source{
		{ID: "src-o1", Official: true}, {ID: "src-n1"},
	}}}
	old := time.Now().UTC().Add(-time.Hour)
	official := capItem("o1", "Central bank raises rates", "https://www.ecb.europa.eu/press/1", "")
	plain := capItem("n1", "Local fair opens", "https://local.example/1", "")
	official.At, plain.At = old, old
	report, err := rt.resolveEvents(ctx, owner, computer, []eventInput{official, plain})
	if err != nil || report.Minted != 2 {
		t.Fatalf("resolve: %+v, %v", report, err)
	}
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	recs := managementRecords(t, rt, owner, computer)
	aged := recordsWithPrefix(recs, "event-aged:")
	if len(recs) != 1 || len(aged) != 1 || itemIDs(aged[0]) != "o1" || !store.RecordWakesReader(aged[0]) ||
		aged[0].Captures.Event.Reason != types.CaptureEventOfficial {
		t.Fatalf("E15: records = %+v", recs)
	}
}

func TestTheSweepReleasesStrandedHeldCapturesOnceAndDrainsOnRollback(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	// Two held arrivals: c1 was resolved (it waits on its event), c9 was
	// queued when the computer crashed.
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", "")}); err != nil {
		t.Fatal(err)
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:a", []types.CaptureArrival{{CaptureID: "c1", Title: "Nineteen injured in Eurotunnel accident", URL: "https://www.bbc.co.uk/news/1"}}, true); err != nil {
		t.Fatal(err)
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:b", []types.CaptureArrival{{CaptureID: "c9", Title: "Stranded", URL: "https://b.example/9"}}, true); err != nil {
		t.Fatal(err)
	}
	// E12: held arrivals wake no one and are not in the view.
	for _, r := range managementRecords(t, rt, owner, computer) {
		if store.RecordWakesReader(r) {
			t.Fatalf("E12: held arrival %s wakes", r.RecordID)
		}
	}
	if view, _ := rt.loadCellRecords(ctx, owner, computer, agentprofile.Management+":"+owner, ""); len(view.view) != 0 {
		t.Fatalf("E12: held arrivals in the view: %+v", view.view)
	}

	now := time.Now().UTC()
	rt.sweepHeldCaptures(ctx, owner, computer, now) // within the grace: nothing
	if n := len(recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-")); n != 0 {
		t.Fatalf("the sweep released captures inside the grace: %d", n)
	}
	rt.sweepHeldCaptures(ctx, owner, computer, now.Add(eventSweepEvery+eventHeldGrace))
	released := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-unresolved:")
	if len(released) != 1 || itemIDs(released[0]) != "c9" || released[0].Captures.Items[0].SourceID != "rss:b" || !store.RecordWakesReader(released[0]) {
		t.Fatalf("E16: released = %+v", released)
	}
	calls := 0
	if j, ok := rt.candidateJudge.(*eventJudge); ok {
		calls = j.callCount()
	}
	rt.sweepHeldCaptures(ctx, owner, computer, now.Add(2*(eventSweepEvery+eventHeldGrace)))
	if n := len(recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-")); n != 1 {
		t.Fatalf("E16: a second sweep released again: %d records", n)
	}
	if j, ok := rt.candidateJudge.(*eventJudge); ok && j.callCount() != calls {
		t.Fatal("the sweep put a stranded capture to Jev")
	}

	// Rollback: with event_wakes off, the capture waiting on its event is
	// released too.
	setEventSettings(t, rt, owner, computer, true, false, time.Now().UTC())
	rt.sweepHeldCaptures(ctx, owner, computer, now.Add(3*(eventSweepEvery+eventHeldGrace)))
	drained := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-released:")
	if len(drained) != 1 || itemIDs(drained[0]) != "c1" {
		t.Fatalf("rollback drain = %+v", drained)
	}
}

func TestDeclineRoutesAnEventRecordsCapturesToTheirOwnSources(t *testing.T) {
	deskTestWorkerBin(t)
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}
	report, _ := rt.resolveEvents(ctx, owner, computer, inputs)
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	cell := managementCell(t, rt, owner, computer, "run-event-decline")
	out := cell(`choir.Decline([]string{"c1", "c2"}, "a terminal accident with no follow-up")`)
	if strings.Count(out, "rlm:note:") < 2 {
		t.Fatalf("decline did not commit: %s", out)
	}
	for _, src := range []string{"src-c1", "src-c2"} {
		got, _ := rt.store.ListInboxRecords(ctx, owner, computer, types.CaptureArrivalSenderInbox(src), 0, 0)
		if len(got) != 1 || got[0].Disposition == nil || got[0].Disposition.Kind != types.DispositionDecline {
			t.Fatalf("E13: decline to %s = %+v", src, got)
		}
	}
	if got, _ := rt.store.ListInboxRecords(ctx, owner, computer, types.CaptureArrivalSenderInbox(eventCapturesSource(report.Corroborated[0])), 0, 0); len(got) != 0 {
		t.Fatalf("E13: a decline landed on the event's source: %+v", got)
	}
}

func TestWithEventWakesOnAPollsArrivalIsHeldAndWakesNoOne(t *testing.T) {
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
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
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
	if err != nil || len(records) != 1 || records[0].Captures == nil || !records[0].Captures.Held {
		t.Fatalf("arrival = %+v, %v; want one held arrival", records, err)
	}
	pending, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range pending {
		if w.TargetAgentID == management {
			t.Fatalf("E12: a held arrival woke management: %+v", w)
		}
	}
	// V3b-3 (§18): labels are on, but under event wakes the poll makes no
	// per-capture label call (the event flush runs later, off this path).
	if n := rt.candidateJudge.(*eventJudge).callCount(); n != 0 {
		t.Fatalf("labels ran under event wakes: %d calls", n)
	}
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

// Code review round (Devin and Gemini): a failed commit is retried, a
// partly resolved failed flush leaves only its unresolved captures, an
// aged wake survives a restart once, and members past one record's bound
// go on held pages.
func TestAFailedCommitIsRetriedByTheNextFlush(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	failing := true
	rt.commitCaptureArrival = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
		if failing {
			return errors.New("store busy")
		}
		_, err := rt.store.CommitInboxRecords(ctx, req)
		return err
	}
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}
	report, _ := rt.resolveEvents(ctx, owner, computer, inputs)
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	if n := len(managementRecords(t, rt, owner, computer)); n != 0 {
		t.Fatalf("a failed commit committed %d records", n)
	}
	failing = false
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	if woke := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-corroborated:"); len(woke) != 1 {
		t.Fatalf("B1: the next flush did not retry the wake: %+v", woke)
	}
}

func TestAFailedFlushLeavesOnlyCapturesWithoutAMembership(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	resolved := capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", "")
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{resolved}); err != nil {
		t.Fatal(err)
	}
	rt.mintEventWakes(ctx, owner, computer, []eventInput{resolved, capItem("c7", "Plex sells movie tickets", "https://arstechnica.com/7", "")},
		eventResolveReport{}, errors.New("jev: deadline"))
	got := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-unresolved:")
	if len(got) != 1 || itemIDs(got[0]) != "c7" {
		t.Fatalf("B2: unresolved = %+v; want c7 alone", got)
	}
}

func TestAnAgedWakeSurvivesARestartOnce(t *testing.T) {
	rt, owner, computer := eventRuntime(t, &eventJudge{answers: map[string]eventAnswer{}})
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-3*time.Hour))
	rt.researchDeps = &researchtools.Dependencies{Sources: &sources.Registry{Sources: []sources.Source{{ID: "src-o1", Official: true}}}}
	official := capItem("o1", "Central bank raises rates", "https://www.ecb.europa.eu/press/1", "")
	official.At = time.Now().UTC().Add(-time.Hour)
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{official}); err != nil {
		t.Fatal(err)
	}
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	rt.resetEventIndexForTest()
	rt.mintEventWakes(ctx, owner, computer, nil, eventResolveReport{}, nil)
	if aged := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-aged:"); len(aged) != 1 {
		t.Fatalf("E15: aged records after a restart = %d", len(aged))
	}
}

func TestMembersPastOneRecordGoOnHeldPages(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	var inputs []eventInput
	inputs = append(inputs, capItem("m00", "Quake strikes coast", "https://outlet0.example/1", ""))
	for i := 1; i < eventRecordItems+3; i++ {
		title := fmt.Sprintf("Quake report %02d", i)
		judge.answers[title] = eventAnswer{target: "Quake strikes coast"}
		inputs = append(inputs, capItem(fmt.Sprintf("m%02d", i), title, fmt.Sprintf("https://outlet%d.example/1", i), ""))
	}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	report, err := rt.resolveEvents(ctx, owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	recs := managementRecords(t, rt, owner, computer)
	woke, pages := recordsWithPrefix(recs, "event-corroborated:"), recordsWithPrefix(recs, "event-grew:")
	named := 0
	for _, r := range append(woke, pages...) {
		named += len(r.Captures.Items)
	}
	if len(woke) != 1 || len(woke[0].Captures.Items) != eventRecordItems || len(pages) != 1 || !pages[0].Captures.Held || named != len(inputs) {
		t.Fatalf("records = %d corroborated (%d items), %d pages; named %d of %d", len(woke), len(woke[0].Captures.Items), len(pages), named, len(inputs))
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
	post(`{"key":"intake.event_wakes","value":true}`)
	first, err := s.GetComputerSetting(context.Background(), owner, rt.TextureComputerID(), store.SettingIntakeEventWakes)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	post(`{"key":"intake.event_wakes","value":true}`)
	again, _ := s.GetComputerSetting(context.Background(), owner, rt.TextureComputerID(), store.SettingIntakeEventWakes)
	if !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("an unchanged value moved updated_at from %s to %s", first.UpdatedAt, again.UpdatedAt)
	}
}

func TestEventWakesWithoutLabelsShowsTheIntakeSurfaceButNotLabels(t *testing.T) {
	text := runtimeprompts.RLMManagementOverlay(runtimeprompts.RLMManagementOverlayOptions{EventWakes: true})
	if !strings.Contains(text, "Event records:") || !strings.Contains(text, "Delegate capture intake") || strings.Contains(text, "Intake labels:") {
		t.Fatal("the overlay with event wakes and no labels is wrong")
	}
	if off := runtimeprompts.RLMManagementOverlay(); strings.Contains(off, "Event records:") || strings.Contains(off, "Delegate capture intake") {
		t.Fatal("the overlay with neither setting shows the intake surface")
	}
}

func TestTheSweepTallyReachesTheStoriesPage(t *testing.T) {
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	if rt.heldCapturesTally(owner, computer) != nil {
		t.Fatal("a tally before any sweep")
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:b", []types.CaptureArrival{{CaptureID: "c9", Title: "Stranded", URL: "https://b.example/9"}}, true); err != nil {
		t.Fatal(err)
	}
	rt.sweepHeldCaptures(ctx, owner, computer, time.Now().UTC().Add(eventHeldGrace+time.Minute))
	if tally := rt.heldCapturesTally(owner, computer); tally == nil || tally.Released != 1 || tally.Waiting != 0 {
		t.Fatalf("E16: tally = %+v", tally)
	}
}

// V3b (§18): an event record names the stories holding the event's
// earlier reports, and whether management declined them, from the
// disposition index, with no model call.
func TestEventRecordsNameTheStoriesThatHoldTheEvent(t *testing.T) {
	deskTestWorkerBin(t)
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	story := openTestStory(t, rt.store, owner, computer, "Channel Tunnel collision injures 19")
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}
	report, _ := rt.resolveEvents(ctx, owner, computer, inputs)
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	cell := managementCell(t, rt, owner, computer, "run-event-cover")
	if out := cell(`choir.Feed("` + story + `", []string{"c1", "c2"}, "the collision story")`); !strings.Contains(out, "rlm:note:") {
		t.Fatalf("feed: %s", out)
	}
	third := []eventInput{capItem("c4", "Eurotunnel crash injures 19", "https://www.euronews.com/4", "")}
	again, _ := rt.resolveEvents(ctx, owner, computer, third)
	rt.mintEventWakes(ctx, owner, computer, third, again, nil)
	grew := recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-grew:")
	if len(grew) != 1 || strings.Join(grew[0].Captures.Event.Covered, ",") != "texture:"+story || grew[0].Captures.Event.Declined {
		t.Fatalf("growth event = %+v; want it covered by texture:%s", grew[0].Captures.Event, story)
	}

	// Declined: an event whose reports management declined says so.
	judge := rt.candidateJudge.(*eventJudge)
	judge.answers["Plex adds ticketing"] = eventAnswer{target: "Plex sells movie tickets"}
	judge.answers["Plex launches movie tickets"] = eventAnswer{target: "Plex sells movie tickets"}
	plex := []eventInput{capItem("p1", "Plex sells movie tickets", "https://arstechnica.com/p1", ""), capItem("p2", "Plex adds ticketing", "https://www.theverge.com/p2", "")}
	r2, _ := rt.resolveEvents(ctx, owner, computer, plex)
	rt.mintEventWakes(ctx, owner, computer, plex, r2, nil)
	cell = managementCell(t, rt, owner, computer, "run-event-decline-2")
	if out := cell(`choir.Decline([]string{"p1", "p2"}, "a product launch, not news")`); !strings.Contains(out, "rlm:note:") {
		t.Fatalf("decline: %s", out)
	}
	p3 := []eventInput{capItem("p3", "Plex launches movie tickets", "https://techcrunch.com/p3", "")}
	r3, _ := rt.resolveEvents(ctx, owner, computer, p3)
	rt.mintEventWakes(ctx, owner, computer, p3, r3, nil)
	var declined bool
	for _, r := range recordsWithPrefix(managementRecords(t, rt, owner, computer), "event-grew:") {
		if itemIDs(r) == "p3" {
			declined = r.Captures.Event.Declined && len(r.Captures.Event.Covered) == 0
		}
	}
	if !declined {
		t.Fatal("V3b: the growth of a declined event does not say it was declined")
	}
}

// docs/problems/event-records-of-one-event-delegated-apart-2026-10-16.md:
// an event's corroboration and growth records, read in one turn and
// delegated per record, must go to one task, or two tasks open two
// stories of one event.
func TestRecordsOfOneEventAreDelegatedToOneTask(t *testing.T) {
	deskTestWorkerBin(t)
	rt, owner, computer := eventRuntime(t, eurotunnelJudge())
	ctx := context.Background()
	setEventSettings(t, rt, owner, computer, true, true, time.Now().UTC().Add(-time.Hour))
	inputs := []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
	}
	report, _ := rt.resolveEvents(ctx, owner, computer, inputs)
	rt.mintEventWakes(ctx, owner, computer, inputs, report, nil)
	third := []eventInput{capItem("c4", "Eurotunnel crash injures 19", "https://www.euronews.com/4", "")}
	again, _ := rt.resolveEvents(ctx, owner, computer, third)
	rt.mintEventWakes(ctx, owner, computer, third, again, nil)
	cell := managementCell(t, rt, owner, computer, "run-event-delegate")
	if out := cell(delegateAllCell); !strings.Contains(out, "rlm:delegate:") {
		t.Fatalf("delegate: %s", out)
	}
	tasks, err := rt.store.ListManagementTasks(ctx, owner, computer, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("two records of one event made %d tasks, want one", len(tasks))
	}
}
