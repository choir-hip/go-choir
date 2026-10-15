package platform

// Ways the section-filtered front list (?section=, docs/problems/
// autopaper-section-pages-empty-2026-10-15.md) can fail:
//
//  1. The filter returns a story of another section, an unscored story, or
//     drops a story of the section that is older than the first batch.
//  2. Paging repeats or skips a story when a page spans several publication
//     batches, or the cursor does not resume where the scan stopped.
//  3. A rare section makes one request scan without bound: the scan stops
//     at frontSectionScanMax and hands back a cursor that continues.
//  4. A section outside the four is accepted (HTTP 200) instead of 400.
//  5. The unfiltered list changes (it must stay the same code path and
//     output with section empty).
//  6. With no scores file a section page errors instead of coming back empty.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// newSectionFixture holds eight stories, newest first (by latest publish):
// Hotel world, Golf tech_ai, Foxtrot world, Echo (unscored), Delta world,
// Harbor world, Canal business, Bridge world. Scores are written and attached.
func newSectionFixture(t *testing.T) frontFixture {
	t.Helper()
	f := newFrontFixture(t)
	for _, doc := range []string{"doc-delta", "doc-echo", "doc-foxtrot", "doc-golf", "doc-hotel"} {
		title := map[string]string{"doc-delta": "Delta", "doc-echo": "Echo", "doc-foxtrot": "Foxtrot", "doc-golf": "Golf", "doc-hotel": "Hotel"}[doc]
		f.routes[doc] = publishStoryRevision(t, f.svc, doc, 1, title, title+" happened.", nil).RoutePath
	}
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	for i, doc := range []string{"doc-bridge", "doc-canal", "doc-harbor", "doc-delta", "doc-echo", "doc-foxtrot", "doc-golf", "doc-hotel"} {
		setPublicationUpdatedAt(t, f.store, "paper-owner", doc, base.Add(time.Duration(i)*time.Second))
	}
	page, err := f.svc.ListFrontStories(context.Background(), 100, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	if got, want := fmt.Sprint(storyIDs(page)), "[Hotel Golf Foxtrot Echo Delta Harbor reopens Canal dredged Bridge closes]"; got != want {
		t.Fatalf("fixture order = %s, want %s", got, want)
	}
	sectionOf := map[string]string{
		"Harbor reopens": "world", "Canal dredged": "business", "Delta": "world", "Foxtrot": "world",
		"Golf": "tech_ai", "Hotel": "world", "Bridge closes": "world", // Echo is unscored
	}
	scores := map[string]map[string]any{}
	for _, s := range page.Stories {
		if sec, ok := sectionOf[s.Title]; ok {
			scores[s.PublicationID] = map[string]any{"revision": s.LatestRevision, "importance": 5, "section": sec}
		}
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "scores": scores})
	path := filepath.Join(t.TempDir(), "scores.json")
	writeScores(t, path, string(raw))
	f.svc.SetFrontImportancePath(path)
	return f
}

func sectionPage(t *testing.T, svc *Service, limit int, cursor, section string) *FrontPage {
	t.Helper()
	page, err := svc.ListFrontStoriesInSection(context.Background(), limit, cursor, section)
	if err != nil {
		t.Fatalf("section %q: %v", section, err)
	}
	return page
}

// Failure modes 1 and 2: batches of 3 and pages of 2 force a page to span
// batches and the cursor to resume mid-batch.
func TestFrontSectionFiltersAndPagesAcrossBatches(t *testing.T) {
	f := newSectionFixture(t)
	savedBatch := frontSectionBatchSize
	frontSectionBatchSize = 3
	defer func() { frontSectionBatchSize = savedBatch }()

	var got []string
	cursor := ""
	for i := 0; i < 10; i++ {
		page := sectionPage(t, f.svc, 2, cursor, "world")
		for _, s := range page.Stories {
			if s.Section != "world" {
				t.Fatalf("story %q has section %q in the world list", s.Title, s.Section)
			}
			if s.Importance != 5 {
				t.Fatalf("story %q has no attached importance: %+v", s.Title, s)
			}
		}
		got = append(got, storyIDs(page)...)
		if cursor = page.NextCursor; cursor == "" {
			break
		}
	}
	if want := "[Hotel Foxtrot Delta Harbor reopens Bridge closes]"; fmt.Sprint(got) != want {
		t.Fatalf("world stories = %v, want %s", got, want)
	}

	tech := sectionPage(t, f.svc, 20, "", "tech_ai")
	if fmt.Sprint(storyIDs(tech)) != "[Golf]" || tech.NextCursor != "" {
		t.Fatalf("tech_ai = %v next %q", storyIDs(tech), tech.NextCursor)
	}
	// A section whose only story is the 7th publication (older than the first batch).
	business := sectionPage(t, f.svc, 1, "", "business")
	if fmt.Sprint(storyIDs(business)) != "[Canal dredged]" {
		t.Fatalf("business = %v", storyIDs(business))
	}
	if sectionPage(t, f.svc, 20, "", "us_politics").Stories == nil {
		t.Fatalf("a section with no stories must list [] not null")
	}
}

// Failure mode 3.
func TestFrontSectionScanIsBoundedAndContinues(t *testing.T) {
	f := newSectionFixture(t)
	savedBatch, savedMax := frontSectionBatchSize, frontSectionScanMax
	frontSectionBatchSize, frontSectionScanMax = 2, 3
	defer func() { frontSectionBatchSize, frontSectionScanMax = savedBatch, savedMax }()

	// Canal is the 7th publication: scans of 3 find nothing, nothing, then it.
	var pages []string
	cursor := ""
	for i := 0; i < 6; i++ {
		page := sectionPage(t, f.svc, 5, cursor, "business")
		pages = append(pages, fmt.Sprint(storyIDs(page)))
		if cursor = page.NextCursor; cursor == "" {
			break
		}
	}
	if want := "[[] [] [Canal dredged]]"; fmt.Sprint(pages) != want {
		t.Fatalf("business pages = %v, want %s", pages, want)
	}
}

// Failure mode 6.
func TestFrontSectionWithoutScoresIsEmpty(t *testing.T) {
	f := newFrontFixture(t)
	page := sectionPage(t, f.svc, 20, "", "world")
	if len(page.Stories) != 0 || page.NextCursor != "" {
		t.Fatalf("no scores file: page = %+v", page)
	}
}

// Failures 4 and 5, through the handler.
func TestFrontSectionHandler(t *testing.T) {
	f := newSectionFixture(t)
	handler := NewHandler(f.svc)
	get := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/front"+query, nil)
		req.Header.Set("X-Internal-Caller", "true")
		w := httptest.NewRecorder()
		handler.HandleInternalFrontStories(w, req)
		return w
	}
	for _, bad := range []string{"?section=sports", "?section=WORLD", "?section=world,tech_ai"} {
		if w := get(bad); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400", bad, w.Code)
		}
	}
	w := get("?section=tech_ai&limit=1")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var page FrontPage
	if err := json.NewDecoder(w.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(storyIDs(&page)) != "[Golf]" || page.NextCursor == "" {
		t.Fatalf("tech_ai limit 1 = %v next %q", storyIDs(&page), page.NextCursor)
	}

	// Unfiltered is unchanged: every story, scored or not, in order.
	var all FrontPage
	if err := json.NewDecoder(get("").Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprint(storyIDs(&all)), "[Hotel Golf Foxtrot Echo Delta Harbor reopens Canal dredged Bridge closes]"; got != want {
		t.Fatalf("unfiltered = %s, want %s", got, want)
	}
	direct, err := f.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(direct)
	b, _ := json.Marshal(&all)
	if string(a) != string(b) {
		t.Fatalf("handler unfiltered differs from ListFrontStories:\n%s\n%s", a, b)
	}
}
