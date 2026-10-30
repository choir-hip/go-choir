package agentcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// The fixture guard (L2a R2-7,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Ways it can fail,
// written before the code:
//   - the fixture's registry entry is missing, or its tos_class is not
//     test_fixture, so nothing can tell it from a real feed;
//   - a schedule on the fixture starts without being marked a fixture run,
//     so a paper computer polls test data 24/7 (standing schedules survive
//     crashes);
//   - the marked request is refused, so the fixture cannot run at all;
//   - the fixture's max_items is capped below 100, so a fixture longer
//     than the cap hides the deferred-items defect;
//   - the product API offers no way to mark the run.

func TestFixtureRegistryEntryIsATestFixtureOfUpToAHundredItems(t *testing.T) {
	reg, err := sources.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var found *sources.Source
	for i, s := range reg.Sources {
		if s.URL == "https://raw.githubusercontent.com/choir-hip/go-choir/main/testdata/fixtures/l2a-multilingual-2026-10-14.xml" {
			found = &reg.Sources[i]
		}
	}
	if found == nil {
		t.Fatal("the L2a fixture feed has no registry entry")
	}
	if found.TOSClass != sources.TOSClassTestFixture || found.Type != sources.SourceTypeRSS || found.MaxItemsPerPoll != 100 {
		t.Fatalf("fixture entry = %+v; want rss, tos_class test_fixture, max_items_per_poll 100", found)
	}
	if got := researchtools.EffectivePollMaxItems(100); got != 100 {
		t.Fatalf("a schedule's max_items 100 is capped to %d", got)
	}
}

func TestStartSourceScheduleRefusesATestFixtureUnlessMarked(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:real", nil)
	fixtureID := "fixture:l2a_test"
	reg := rt.researchDeps.Sources
	reg.Sources = append(reg.Sources, sources.Source{ID: fixtureID, Type: sources.SourceTypeRSS, Name: "Fixture", URL: "https://fixture.example/feed.xml", TOSClass: sources.TOSClassTestFixture, MaxItemsPerPoll: 100})
	owner, computer := "owner-fixture", rt.cfg.ComputerID

	if _, err := rt.StartSourceSchedule(ctx, owner, computer, fixtureID, 10*time.Minute, 100); err == nil || !strings.Contains(err.Error(), "test_fixture") {
		t.Fatalf("an unmarked schedule on a test fixture = %v; want refused", err)
	}
	if len(*wakes) != 0 {
		t.Fatalf("a refused fixture schedule armed a poll: %+v", *wakes)
	}
	if _, err := rt.store.GetSourceSchedule(ctx, owner, fixtureID); err == nil {
		t.Fatal("a refused fixture schedule was stored")
	}
	sched, err := rt.StartSourceScheduleRun(ctx, owner, computer, fixtureID, 10*time.Minute, 100, true)
	if err != nil {
		t.Fatalf("a marked fixture run was refused: %v", err)
	}
	if !sched.FixtureRun || sched.MaxItems != 100 {
		t.Fatalf("fixture schedule = %+v; want marked, max_items 100", sched)
	}
	// A real source needs no mark.
	if _, err := rt.StartSourceSchedule(ctx, owner, computer, "rss:real", 10*time.Minute, 20); err != nil {
		t.Fatalf("a real source was refused: %v", err)
	}

	// Through the product API: fixture_run marks the request.
	handler := &APIHandler{rt: rt}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/autopaper/sources/schedules", strings.NewReader(body))
		req.Header.Set("X-Authenticated-User", "owner-fixture-api")
		rec := httptest.NewRecorder()
		handler.HandleSourceSchedules(rec, req)
		return rec
	}
	if rec := post(`{"source_id":"` + fixtureID + `","max_items":100}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "test_fixture") {
		t.Fatalf("unmarked API request = %d %s", rec.Code, rec.Body)
	}
	if rec := post(`{"source_id":"` + fixtureID + `","max_items":100,"fixture_run":true}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"fixture_run":true`) {
		t.Fatalf("marked API request = %d %s", rec.Code, rec.Body)
	}
}
