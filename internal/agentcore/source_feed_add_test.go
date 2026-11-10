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

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Lane F: add a source by asking (docs/problems/lane-f-asking-failure-list-2026-11-10.md,
// S1 to S12). The tests were written before the code.

// feedAddOwner is the owner ownerScheduleRequest authenticates as.
const feedAddOwner = "owner-stamp"

func rssBody(items int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>Harbor Gazette</title>`)
	for i := 0; i < items; i++ {
		fmt.Fprintf(&b, `<item><title>Item %d</title><link>https://harbor.example/%d</link><description>Body %d</description><guid>h-%d</guid></item>`, i, i, i, i)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

// feedAddFixture is a runtime whose registry has no custom source, with the
// private-network guard off (the test servers are loopback) and a feed server
// the test controls.
func feedAddFixture(t *testing.T, serve http.HandlerFunc) (*Runtime, *APIHandler, string, *[]armedWake) {
	t.Helper()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	srv := httptest.NewServer(serve)
	t.Cleanup(srv.Close)
	rt, _ := testRuntime(t)
	rt.cfg.ComputerID = "computer-reader"
	reg := &sources.Registry{UserAgent: "test"}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: &recordingSink{}}
	var wakes []armedWake
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	return rt, &APIHandler{rt: rt}, srv.URL, &wakes
}

func goodFeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/rss+xml")
	fmt.Fprint(w, rssBody(3))
}

func postFeed(h *APIHandler, body string, inProcess bool) *httptest.ResponseRecorder {
	req := ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", body)
	if inProcess {
		req.RemoteAddr = ""
	}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, req)
	return rec
}

func scheduleCount(t *testing.T, rt *Runtime) int {
	t.Helper()
	all, err := rt.store.ListSourceSchedules(scheduleCustodian(context.Background()), feedAddOwner)
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

func decodeSchedule(t *testing.T, rec *httptest.ResponseRecorder) store.SourceSchedule {
	t.Helper()
	var out struct {
		Schedule store.SourceSchedule `json:"schedule"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Schedule
}

func TestAddFeedStartsOwnerStampedScheduleWithNoRegistryEntry(t *testing.T) {
	rt, h, url, wakes := feedAddFixture(t, goodFeed)
	rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, url+"/feed.xml"), false)
	if rec.Code != http.StatusOK {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	s := decodeSchedule(t, rec)
	if !strings.HasPrefix(s.SourceID, "custom:rss:") || s.FeedURL != url+"/feed.xml" || s.FeedKind != "rss" || s.FeedName != "Harbor Gazette" {
		t.Fatalf("schedule = %+v", s)
	}
	if s.CreatedByOwnerAct == "" || s.State != store.SourceScheduleActive || len(*wakes) != 1 {
		t.Fatalf("owner stamp / state / first poll not armed: %+v wakes=%d", s, len(*wakes))
	}
	// S9: the poll resolves the definition from the schedule.
	if err := rt.HandleSourcePollDue(context.Background(), feedAddOwner, "computer-reader", (*wakes)[0].content); err != nil {
		t.Fatal(err)
	}
	got := getStoredSchedule(t, rt, s.SourceID)
	if got.Totals.Polls != 1 || got.Totals.Errors != 0 || got.Totals.New != 3 {
		t.Fatalf("custom poll totals = %+v, recent %+v", got.Totals, got.Recent)
	}
}

func TestAddFeedRefusals(t *testing.T) {
	huge := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Big</title>`)
		chunk := strings.Repeat("<item><title>x</title><description>"+strings.Repeat("y", 1000)+"</description></item>", 100)
		for i := 0; i < 200; i++ { // about 20 MB
			fmt.Fprint(w, chunk)
		}
		fmt.Fprint(w, `</channel></rss>`)
	}
	cases := []struct {
		name  string
		serve http.HandlerFunc
		want  string
	}{
		{"not reachable", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", 500) }, "feed_not_reachable"},
		{"html is not a feed", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html><body>hello</body></html>") }, "feed_not_parsed"},
		{"json is not a feed", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"a":1}`) }, "feed_not_parsed"},
		{"empty feed", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, rssBody(0)) }, "feed_empty"},
		{"oversize", huge, "feed_too_large"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rt, h, url, wakes := feedAddFixture(t, c.serve)
			rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, url+"/f"), false)
			if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), c.want) {
				t.Fatalf("add = %d %s, want 422 naming %s", rec.Code, rec.Body, c.want)
			}
			if scheduleCount(t, rt) != 0 || len(*wakes) != 0 {
				t.Fatal("a refused feed left a schedule or a wake")
			}
		})
	}
}

func TestAddFeedRefusesUnsafeURLs(t *testing.T) {
	rt, h, _, wakes := feedAddFixture(t, goodFeed)
	sourcefetch.SetAllowPrivateNetworkForTests(false) // S1: the real guard
	for _, u := range []string{
		"file:///etc/passwd", "ftp://example.com/feed", "http://127.0.0.1/feed", "http://169.254.169.254/latest/meta-data",
		"http://user:pw@example.com/feed", "http://localhost/feed", "not a url", "",
	} {
		rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, u), false)
		if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
			t.Errorf("%q: %d %s, want a refusal", u, rec.Code, rec.Body)
		}
	}
	if scheduleCount(t, rt) != 0 || len(*wakes) != 0 {
		t.Fatal("an unsafe URL left a schedule")
	}
}

func TestAddFeedIsAnOwnerActNotADesksS6(t *testing.T) {
	rt, h, url, _ := feedAddFixture(t, goodFeed)
	rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, url), true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "owner act") {
		t.Fatalf("desk add = %d %s, want 403 naming the owner act", rec.Code, rec.Body)
	}
	if scheduleCount(t, rt) != 0 {
		t.Fatal("a desk added a feed")
	}
}

func TestAddFeedOneOrTheOtherS12(t *testing.T) {
	_, h, url, _ := feedAddFixture(t, goodFeed)
	rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q,"source_id":"rss:x"}`, url), false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("both = %d %s", rec.Code, rec.Body)
	}
}

func TestAddFeedDuplicateIsTheSameScheduleS8(t *testing.T) {
	rt, h, url, _ := feedAddFixture(t, goodFeed)
	body := fmt.Sprintf(`{"feed_url":%q}`, url+"/a.xml")
	for i := 0; i < 2; i++ {
		if rec := postFeed(h, body, false); rec.Code != http.StatusOK {
			t.Fatalf("add %d = %d %s", i, rec.Code, rec.Body)
		}
	}
	if scheduleCount(t, rt) != 1 {
		t.Fatalf("schedules = %d, want 1", scheduleCount(t, rt))
	}
}

func TestAddFeedCapPerOwnerS7(t *testing.T) {
	rt, h, url, _ := feedAddFixture(t, goodFeed)
	for i := 0; i < maxCustomFeedsPerOwner; i++ {
		if rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, fmt.Sprintf("%s/f%d.xml", url, i)), false); rec.Code != http.StatusOK {
			t.Fatalf("add %d = %d %s", i, rec.Code, rec.Body)
		}
	}
	rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, url+"/one-too-many.xml"), false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "feed_cap") {
		t.Fatalf("over the cap = %d %s", rec.Code, rec.Body)
	}
	if scheduleCount(t, rt) != maxCustomFeedsPerOwner {
		t.Fatal("the refused feed was written")
	}
}

func TestCustomFeedSurvivesACrashBootAndAStopStaysStoppedS10S11(t *testing.T) {
	ctx := context.Background()
	rt, h, url, wakes := feedAddFixture(t, goodFeed)
	rec := postFeed(h, fmt.Sprintf(`{"feed_url":%q}`, url+"/keep.xml"), false)
	if rec.Code != http.StatusOK {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	id := decodeSchedule(t, rec).SourceID
	markPollInFlight(t, rt, feedAddOwner, id, time.Now().UTC().Add(-time.Second))
	crashBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getStoredSchedule(t, rt, id)
	if got.State != store.SourceScheduleActive || got.FeedURL != url+"/keep.xml" || len(*wakes) != 2 {
		t.Fatalf("after a crash boot = %+v wakes=%d; want the standing feed kept", got, len(*wakes))
	}
	// The next tick polls the custom feed with no registry entry.
	if err := rt.HandleSourcePollDue(ctx, feedAddOwner, "computer-reader", (*wakes)[1].content); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.StopSourceSchedule(ctx, feedAddOwner, id, "owner stopped"); err != nil {
		t.Fatal(err)
	}
	n := len(*wakes)
	crashBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	if got := getStoredSchedule(t, rt, id); got.State != store.SourceScheduleStopped || len(*wakes) != n {
		t.Fatalf("a stopped feed came back: %+v", got)
	}
}
