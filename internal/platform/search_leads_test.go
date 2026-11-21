package platform

// Search leads (search_leads.go; landing L1 of
// docs/definitions/corpus-first-search-one-verb-2026-11-19.md). Every web
// search result a computer is handed becomes a row corpusd holds. Failure
// list, written before the code:
//
//   L1  an arrival's results are not one row per distinct URL key of the
//       owner and computer (a trailing slash makes a second row; two results
//       of one arrival make a conflict error), or a row lacks its title,
//       snippet, url, provider date, first and last time, times returned, or
//       the query key and conversation id that returned it
//   L2  a second return of a URL does not add one to times_returned, move
//       last_returned_at, keep first_returned_at and the provider date, put
//       the newest query and conversation first without repeats, bound both
//       lists at eight, or an empty snippet erases a held one
//   L3  another computer's (or owner's) lead of the same URL is the same row,
//       or one computer's read returns another's leads
//   L4  a lead is embedded, or the vector worker reads the leads table, or a
//       full-text match on title and snippet misses a lead of the computer,
//       finds another computer's, or a lookup by URL misses a trailing-slash
//       variant
//   L5  a write or read before the table is made panics, creates the table on
//       the request path, is not a plain error, or the table's creation needs
//       the vector tables (or a failed one disables anything but lead writes)
//   L6  marking a lead tried does not match a URL by the captures' key
//       (trailing slash), moves tried_at when set already, or touches another
//       computer's lead, or fails on a URL that is no lead
//   L7  a capture stored for a URL (its URL or its canonical URL) does not
//       set capture_id and tried_at of the matching lead, or sets another
//       computer's
//   L8  a lead not returned for the retention is kept, or one returned
//       within it is deleted, or the prune deletes more than its bound
//   L9  a write of more results, or a tried call of more URLs, than the bound
//       is not a 400 (a guest could turn one request into thousands of rows)
//   L10 the tried-call or write bound refuses a legitimate search's results
//   L11 a guest bound to another owner or computer writes or marks leads
//       through the routes; a bound guest cannot write its own
//   L12 a lead is found by the capture search, or enters vec_captures

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func leadFixture(t *testing.T) *vecFixture {
	t.Helper()
	f := newVecFixture(t)
	if err := f.store.ensureSearchLeads(context.Background()); err != nil {
		t.Fatalf("ensureSearchLeads: %v", err)
	}
	return f
}

func leadWrite(owner, computer, query, conv string, hits ...SearchLeadHit) SearchLeadsWrite {
	return SearchLeadsWrite{OwnerID: owner, ComputerID: computer, Query: query, ConversationID: conv, Results: hits}
}

func readLeads(t *testing.T, f *vecFixture, owner, computer string) map[string]SearchLead {
	t.Helper()
	got, err := f.store.ReadSearchLeads(context.Background(), owner, computer, 1000)
	if err != nil {
		t.Fatalf("ReadSearchLeads: %v", err)
	}
	out := map[string]SearchLead{}
	for _, l := range got {
		out[l.URL] = l
	}
	return out
}

func TestSearchLeadsOneRowPerDistinctURLKeyWithEverythingTheArrivalCarried(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	n, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "  Harbor   REOPENS ", "conv-1",
		SearchLeadHit{URL: "https://example.com/harbor", Title: "Harbor reopens", Snippet: "The harbor reopened.", PublishedAt: "2026-11-18T10:00:00Z"},
		SearchLeadHit{URL: "https://example.com/harbor/", Title: "dup with slash", Snippet: "x"},
		SearchLeadHit{URL: "https://example.com/other", Title: "Other", Snippet: ""},
		SearchLeadHit{URL: "  ", Title: "no url"},
	))
	if err != nil {
		t.Fatalf("WriteSearchLeads: %v", err)
	}
	if n != 2 {
		t.Fatalf("rows written = %d, want 2 (a slash twin and a blank URL are no rows)", n)
	}
	got := readLeads(t, f, "o1", "c1")
	if len(got) != 2 {
		t.Fatalf("leads = %d, want 2: %+v", len(got), got)
	}
	h := got["https://example.com/harbor"]
	if h.Title != "Harbor reopens" || h.Snippet != "The harbor reopened." || h.TimesReturned != 1 {
		t.Fatalf("harbor lead: %+v", h)
	}
	if h.PublishedAt == nil || !h.PublishedAt.Equal(time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("provider date: %v", h.PublishedAt)
	}
	if !h.FirstReturnedAt.Equal(h.LastReturnedAt) || h.FirstReturnedAt.IsZero() {
		t.Fatalf("first/last: %v %v", h.FirstReturnedAt, h.LastReturnedAt)
	}
	if len(h.QueryKeys) != 1 || h.QueryKeys[0] != "harbor reopens" || len(h.ConversationIDs) != 1 || h.ConversationIDs[0] != "conv-1" {
		t.Fatalf("query keys %v conversations %v", h.QueryKeys, h.ConversationIDs)
	}
	if h.TriedAt != nil || h.CaptureID != "" {
		t.Fatalf("a new lead is untried and uncaptured: %+v", h)
	}
	if h.URLKey != captureURLKey("https://example.com/harbor") {
		t.Fatalf("url key is not the capture side's key: %s", h.URLKey)
	}
}

func TestSearchLeadsSecondReturnCountsMergesAndBounds(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	hit := SearchLeadHit{URL: "https://example.com/a", Title: "A", Snippet: "first snippet", PublishedAt: "2026-11-18T10:00:00Z"}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q0", "conv0", hit)); err != nil {
		t.Fatal(err)
	}
	first := readLeads(t, f, "o1", "c1")[hit.URL]
	time.Sleep(20 * time.Millisecond)
	// Same query again, then ten more queries and conversations: the lists keep
	// the newest eight, newest first, no repeats.
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q0", "conv0", SearchLeadHit{URL: hit.URL + "/", Title: "A", Snippet: ""})); err != nil {
		t.Fatal(err)
	}
	mid := readLeads(t, f, "o1", "c1")[hit.URL]
	if mid.TimesReturned != 2 || len(mid.QueryKeys) != 1 || len(mid.ConversationIDs) != 1 {
		t.Fatalf("a repeat of the same query and conversation: %+v", mid)
	}
	if mid.Snippet != "first snippet" {
		t.Fatalf("an empty snippet erased the held one: %q", mid.Snippet)
	}
	if mid.PublishedAt == nil {
		t.Fatal("a return without a provider date erased it")
	}
	for i := 1; i <= 10; i++ {
		q := "q" + string(rune('0'+i%10)) + string(rune('a'+i))
		if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", q, "conv"+q, SearchLeadHit{URL: hit.URL, Title: "A2", Snippet: "second snippet"})); err != nil {
			t.Fatal(err)
		}
	}
	last := readLeads(t, f, "o1", "c1")[hit.URL]
	if last.TimesReturned != 12 {
		t.Fatalf("times returned = %d, want 12", last.TimesReturned)
	}
	if !last.FirstReturnedAt.Equal(first.FirstReturnedAt) || !last.LastReturnedAt.After(first.LastReturnedAt) {
		t.Fatalf("first %v->%v last %v->%v", first.FirstReturnedAt, last.FirstReturnedAt, first.LastReturnedAt, last.LastReturnedAt)
	}
	if len(last.QueryKeys) != 8 || len(last.ConversationIDs) != 8 {
		t.Fatalf("lists are bounded at eight: %v %v", last.QueryKeys, last.ConversationIDs)
	}
	if last.QueryKeys[0] != "q0k" || last.QueryKeys[7] != "q3d" {
		t.Fatalf("newest first: %v", last.QueryKeys)
	}
	if last.Title != "A2" || last.Snippet != "second snippet" {
		t.Fatalf("latest text wins: %q %q", last.Title, last.Snippet)
	}
}

func TestSearchLeadsAreInvisibleAcrossComputersAndOwners(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	hit := SearchLeadHit{URL: "https://example.com/shared", Title: "Shared", Snippet: "s"}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "cA", "q", "conv", hit)); err != nil {
		t.Fatal(err)
	}
	if got := readLeads(t, f, "o1", "cB"); len(got) != 0 {
		t.Fatalf("computer B read computer A's leads: %+v", got)
	}
	if got := readLeads(t, f, "o2", "cA"); len(got) != 0 {
		t.Fatalf("owner 2 read owner 1's leads: %+v", got)
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "cB", "q", "conv", hit)); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "search_leads") != 2 {
		t.Fatalf("the same URL on two computers is two rows, got %d", f.count(t, "search_leads"))
	}
	if got := readLeads(t, f, "o1", "cA"); got[hit.URL].TimesReturned != 1 {
		t.Fatalf("B's return counted on A: %+v", got[hit.URL])
	}
	if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", "cB", []string{hit.URL}, "cap-b"); err != nil {
		t.Fatal(err)
	}
	if a := readLeads(t, f, "o1", "cA")[hit.URL]; a.TriedAt != nil || a.CaptureID != "" {
		t.Fatalf("B's read marked A's lead: %+v", a)
	}
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{ComputerID: "cA", Results: []SearchLeadHit{hit}}); err == nil {
		t.Fatal("a write with no owner must be refused")
	}
}

func TestSearchLeadsBeforeTheVectorTablesExistAreAPlainError(t *testing.T) {
	f := newVecFixture(t) // no ensureSearchLeads: the table is not made yet
	ctx := context.Background()
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "c", SearchLeadHit{URL: "https://example.com/x", Title: "x"})); !errors.Is(err, errSearchLeadsNotReady) {
		t.Fatalf("write err = %v, want errSearchLeadsNotReady", err)
	}
	if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", []string{"https://example.com/x"}, ""); !errors.Is(err, errSearchLeadsNotReady) {
		t.Fatalf("mark err = %v, want errSearchLeadsNotReady", err)
	}
	if _, err := f.store.ReadSearchLeads(ctx, "o1", "c1", 10); !errors.Is(err, errSearchLeadsNotReady) {
		t.Fatalf("read err = %v, want errSearchLeadsNotReady", err)
	}
	var exists bool
	if err := f.store.corpus().QueryRow(`SELECT to_regclass('search_leads') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("the request path made the table: exists=%v err=%v", exists, err)
	}
	// A capture stored with the tables absent stores, and leaves no error.
	f.store.markSearchLeadsCaptured(ctx, []objectgraph.Object{testCapture(t, "o1", "c1", "k", "T", "https://example.com/x", time.Now())})
}

func TestMarkSearchLeadsTriedMatchesTheCapturesKeyKeepsTheFirstTimeAndToleratesNoLead(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "conv",
		SearchLeadHit{URL: "https://example.com/t", Title: "T", Snippet: "s"},
		SearchLeadHit{URL: "https://example.com/u", Title: "U", Snippet: "s"})); err != nil {
		t.Fatal(err)
	}
	n, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", []string{"https://example.com/t/", "https://example.com/not-a-lead"}, "")
	if err != nil || n != 1 {
		t.Fatalf("mark: n=%d err=%v", n, err)
	}
	got := readLeads(t, f, "o1", "c1")
	t1 := got["https://example.com/t"].TriedAt
	if t1 == nil || got["https://example.com/u"].TriedAt != nil || got["https://example.com/t"].CaptureID != "" {
		t.Fatalf("tried: %+v", got)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", []string{"https://example.com/t"}, "capture-1"); err != nil {
		t.Fatal(err)
	}
	got = readLeads(t, f, "o1", "c1")
	if !got["https://example.com/t"].TriedAt.Equal(*t1) || got["https://example.com/t"].CaptureID != "capture-1" {
		t.Fatalf("the first tried time is kept and the capture id set: %+v", got["https://example.com/t"])
	}
	if n, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", nil, ""); err != nil || n != 0 {
		t.Fatalf("no urls: n=%d err=%v", n, err)
	}
}

func TestACaptureStoredForALeadsURLSetsItsCaptureIDAndTriedAt(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	if err := f.store.ensureCaptureIndexTables(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"c1", "c2"} {
		if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", c, "q", "conv",
			SearchLeadHit{URL: "https://example.com/page", Title: "P", Snippet: "s"},
			SearchLeadHit{URL: "https://example.com/redirected", Title: "R", Snippet: "s"})); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	byURL := testCapture(t, "o1", "c1", "k1", "P", "https://example.com/page/", now)
	// A capture whose own URL differs but whose canonical URL is the lead's.
	canon := testCapture(t, "o1", "c1", "k2", "R", "https://example.com/short", now)
	meta, _ := objectgraph.WebCaptureMetadataFromObject(canon)
	meta.CanonicalURL = "https://example.com/redirected"
	canon.Metadata, _ = json.Marshal(meta)
	if err := f.og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{byURL, canon}}); err != nil {
		t.Fatal(err)
	}
	c1 := readLeads(t, f, "o1", "c1")
	for u, want := range map[string]string{"https://example.com/page": byURL.CanonicalID, "https://example.com/redirected": canon.CanonicalID} {
		if c1[u].CaptureID != want || c1[u].TriedAt == nil {
			t.Fatalf("lead %s: capture=%q tried=%v, want %s", u, c1[u].CaptureID, c1[u].TriedAt, want)
		}
	}
	for u, l := range readLeads(t, f, "o1", "c2") {
		if l.CaptureID != "" || l.TriedAt != nil {
			t.Fatalf("a capture of computer 1 marked computer 2's lead %s: %+v", u, l)
		}
	}
}

func TestSearchLeadRoutesFollowTheGuestWriteRule(t *testing.T) {
	f := leadFixture(t)
	og := f.og
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "owner-a", ComputerID: "computer-a"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(handler http.HandlerFunc, path, from string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		hostauth.Middleware(handler).ServeHTTP(rec, r)
		return rec
	}
	hit := SearchLeadHit{URL: "https://example.com/route", Title: "Route", Snippet: "s"}
	own := SearchLeadsWrite{OwnerID: "owner-a", ComputerID: "computer-a", Query: "q", ConversationID: "c", Results: []SearchLeadHit{hit}}
	for _, other := range []struct{ owner, computer string }{{"owner-a", "computer-b"}, {"owner-b", "computer-a"}} {
		bad := own
		bad.OwnerID, bad.ComputerID = other.owner, other.computer
		if rec := post(h.HandleSearchLeads, "/internal/platform/search-leads", "10.200.1.2", bad); rec.Code != http.StatusForbidden {
			t.Fatalf("a bound guest wrote %s/%s leads: %d %s", other.owner, other.computer, rec.Code, rec.Body)
		}
		if rec := post(h.HandleSearchLeadsTried, "/internal/platform/search-leads/tried", "10.200.1.2", map[string]any{"owner_id": other.owner, "computer_id": other.computer, "urls": []string{hit.URL}}); rec.Code != http.StatusForbidden {
			t.Fatalf("a bound guest marked %s/%s leads: %d %s", other.owner, other.computer, rec.Code, rec.Body)
		}
	}
	if rec := post(h.HandleSearchLeads, "/internal/platform/search-leads", "10.200.9.9", own); rec.Code != http.StatusForbidden {
		t.Fatalf("an unbound caller wrote leads: %d", rec.Code)
	}
	rec := post(h.HandleSearchLeads, "/internal/platform/search-leads", "10.200.1.2", own)
	var out struct {
		Rows int `json:"rows"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Rows != 1 {
		t.Fatalf("bound guest write: %d %s", rec.Code, rec.Body)
	}
	rec = post(h.HandleSearchLeadsTried, "/internal/platform/search-leads/tried", "10.200.1.2", map[string]any{"owner_id": "owner-a", "computer_id": "computer-a", "urls": []string{hit.URL}, "capture_id": "cap-9"})
	if rec.Code != http.StatusOK {
		t.Fatalf("bound guest mark: %d %s", rec.Code, rec.Body)
	}
	if l := readLeads(t, f, "owner-a", "computer-a")[hit.URL]; l.TriedAt == nil || l.CaptureID != "cap-9" {
		t.Fatalf("lead after the route marks: %+v", l)
	}
	// Nothing returns a lead to a guest in L1: the routes only write.
	for _, path := range []string{"/internal/platform/search-leads", "/internal/platform/search-leads/tried"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleSearchLeads)).ServeHTTP(rec, r)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s answered %d, want 405", path, rec.Code)
		}
	}
}

func TestALeadIsNeverFoundByTheCaptureSearch(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "conv", SearchLeadHit{URL: "https://example.com/zebra", Title: "Zebra migration", Snippet: "zebra zebra zebra"})); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureCaptureIndexTables(ctx); err != nil {
		t.Fatal(err)
	}
	f.store.retrievalState.captureIndexComplete.Store(true)
	resp, err := f.store.SearchCaptures(ctx, CaptureSearchRequest{OwnerID: "o1", ComputerID: "c1", Words: []string{"zebra"}, Need: 5, Query: "zebra migration"})
	if err != nil {
		t.Fatalf("SearchCaptures: %v", err)
	}
	if len(resp.Objects) != 0 || len(resp.Hits) != 0 || (resp.Ranked != nil && len(resp.Ranked.Entries) != 0) {
		t.Fatalf("a lead is evidence in a capture search: %+v", resp)
	}
}

func TestSearchLeadsTableNeedsNoVectorTablesAndLeavesTheirReadinessAlone(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureSearchLeads(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureSearchLeads(ctx); err != nil {
		t.Fatalf("not idempotent: %v", err)
	}
	var vecMade bool
	if err := f.store.corpus().QueryRow(`SELECT to_regclass('vec_captures') IS NOT NULL OR to_regclass('vec_story_passages') IS NOT NULL`).Scan(&vecMade); err != nil || vecMade {
		t.Fatalf("making the leads table made vector tables: %v %v", vecMade, err)
	}
	if f.store.VectorsReady() {
		t.Fatal("making the leads table marked the vector index ready")
	}
	// The other way: the vector tables' presence check does not name leads.
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().ExecContext(ctx, `DROP TABLE search_leads`); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.store.vectorTablesPresent(ctx); err != nil || !ok {
		t.Fatalf("vector readiness depends on search_leads: %v %v", ok, err)
	}
}

func TestSearchLeadsAreFoundByURLKeyAndByTheWordsOfTheirTitleAndSnippet(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "conv",
		SearchLeadHit{URL: "https://example.com/harbor", Title: "Harbor reopens", Snippet: "The port authority said ships return Monday."},
		SearchLeadHit{URL: "https://example.com/other", Title: "Quay repairs", Snippet: "Cranes arrive."})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c2", "q", "conv", SearchLeadHit{URL: "https://example.com/elsewhere", Title: "Harbor elsewhere", Snippet: "x"})); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.SearchLeadsByText(ctx, "o1", "c1", "harbor ships", 10)
	if err != nil || len(got) != 1 || got[0].URL != "https://example.com/harbor" {
		t.Fatalf("by text: %v %+v", err, got)
	}
	if got, _ := f.store.SearchLeadsByText(ctx, "o1", "c1", "harbor", 10); len(got) != 1 {
		t.Fatalf("another computer's lead matched: %+v", got)
	}
	if got, err := f.store.SearchLeadsByText(ctx, "o1", "c1", "  !! ", 10); err != nil || len(got) != 0 {
		t.Fatalf("a query of no words matched: %v %+v", err, got)
	}
	if got, _ := f.store.SearchLeadsByText(ctx, "o1", "c1", "zebra", 10); len(got) != 0 {
		t.Fatalf("a word no lead has matched: %+v", got)
	}
	got, err = f.store.SearchLeadsByURL(ctx, "o1", "c1", []string{"https://example.com/harbor/", "https://example.com/none", " "}, 10)
	if err != nil || len(got) != 1 || got[0].Title != "Harbor reopens" {
		t.Fatalf("by url: %v %+v", err, got)
	}
	if got, _ := f.store.SearchLeadsByURL(ctx, "o1", "c2", []string{"https://example.com/harbor"}, 10); len(got) != 0 {
		t.Fatalf("by url crossed computers: %+v", got)
	}
}

func TestPruneSearchLeadsDeletesOnlyThoseNotReturnedWithinTheRetentionAndRespectsItsBound(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	var hits []SearchLeadHit
	for i := 0; i < 5; i++ {
		hits = append(hits, SearchLeadHit{URL: "https://example.com/p" + string(rune('a'+i)), Title: "t", Snippet: "s"})
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "conv", hits...)); err != nil {
		t.Fatal(err)
	}
	// Four are older than the retention; one was returned just now.
	if _, err := f.store.corpus().ExecContext(ctx, `UPDATE search_leads SET last_returned_at = now() - interval '31 days' WHERE url <> 'https://example.com/pe'`); err != nil {
		t.Fatal(err)
	}
	n, err := f.store.PruneSearchLeads(ctx, searchLeadRetention, 3)
	if err != nil || n != 3 {
		t.Fatalf("a pass with bound 3 deleted %d (%v), want 3", n, err)
	}
	n, err = f.store.PruneSearchLeads(ctx, searchLeadRetention, 3)
	if err != nil || n != 1 {
		t.Fatalf("the next pass deleted %d (%v), want 1", n, err)
	}
	if left := readLeads(t, f, "o1", "c1"); len(left) != 1 || left["https://example.com/pe"].URL == "" {
		t.Fatalf("the live lead was deleted or an old one kept: %+v", left)
	}
	if n, _ := f.store.PruneSearchLeads(ctx, searchLeadRetention, 3); n != 0 {
		t.Fatalf("a third pass deleted %d", n)
	}
}

func TestSearchLeadRoutesRefuseMoreThanTheBoundWithAPlain400AndTakeTheBound(t *testing.T) {
	f := leadFixture(t)
	og := f.og
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": {Found: true, UserID: "o", ComputerID: "c"}}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(raw))
		r.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		hostauth.Middleware(handler).ServeHTTP(rec, r)
		return rec
	}
	mk := func(n int) []SearchLeadHit {
		out := make([]SearchLeadHit, n)
		for i := range out {
			out[i] = SearchLeadHit{URL: fmt.Sprintf("https://example.com/%d", i), Title: "t"}
		}
		return out
	}
	rec := post(h.HandleSearchLeads, SearchLeadsWrite{OwnerID: "o", ComputerID: "c", Results: mk(searchLeadMaxResults + 1)})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too many results") {
		t.Fatalf("over the results bound: %d %s", rec.Code, rec.Body)
	}
	urls := make([]string, searchLeadMaxURLs+1)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/%d", i)
	}
	rec = post(h.HandleSearchLeadsTried, map[string]any{"owner_id": "o", "computer_id": "c", "urls": urls})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too many urls") {
		t.Fatalf("over the urls bound: %d %s", rec.Code, rec.Body)
	}
	rec = post(h.HandleSearchLeadsTried, map[string]any{"owner_id": "o", "computer_id": "c", "urls": []string{"https://example.com/0"}, "capture_id": strings.Repeat("x", searchLeadCaptureIDRunes+1)})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "capture_id too long") {
		t.Fatalf("a long capture id: %d %s", rec.Code, rec.Body)
	}
	if n, err := f.store.WriteSearchLeads(context.Background(), SearchLeadsWrite{OwnerID: "o", ComputerID: "c", Results: mk(searchLeadMaxResults)}); err != nil || n != searchLeadMaxResults {
		t.Fatalf("a write at the bound: %d %v", n, err)
	}
	if searchLeadMaxResults < 4*40 {
		t.Fatalf("the bound %d would refuse a search of four providers of 40 results", searchLeadMaxResults)
	}
}

func TestMarkSearchLeadsTriedClipsALongCaptureIDAndAMegabyteURLStillKeysLikeTheWrite(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	long := "https://example.com/" + strings.Repeat("a", 5000)
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "conv", SearchLeadHit{URL: long, Title: "t"})); err != nil {
		t.Fatal(err)
	}
	n, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", []string{long}, strings.Repeat("c", 100000))
	if err != nil || n != 1 {
		t.Fatalf("a long url did not mark the lead the write made: %d %v", n, err)
	}
	var id string
	if err := f.store.corpus().QueryRow(`SELECT capture_id FROM search_leads WHERE owner_id='o1'`).Scan(&id); err != nil || len(id) != searchLeadCaptureIDRunes {
		t.Fatalf("stored capture id length %d (%v), want %d", len(id), err, searchLeadCaptureIDRunes)
	}
}

// Landing A reads the stories whose research searches returned a capture's
// page. Failure list, written before the code: a capture no lead made is
// listed; another computer's or owner's conversations are returned; a lead
// returned by several conversations names only one; the route answers a caller
// bound to another computer; it returns a lead's text instead of conversation ids.
func TestSearchLeadConversationsByCaptureAreScopedAndConversationsOnly(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	a, b := "https://example.com/read-a", "https://example.com/unread-b"
	for _, conv := range []string{"doc-one", "doc-two"} {
		if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", conv, SearchLeadHit{URL: a, Title: "A", Snippet: "secret snippet"}, SearchLeadHit{URL: b, Title: "B"})); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c2", "q", "doc-other-computer", SearchLeadHit{URL: a, Title: "A"})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c1", []string{a}, "cap-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", "c2", []string{a}, "cap-1"); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.SearchLeadConversationsByCapture(ctx, "o1", "c1", []string{"cap-1", "cap-none", ""})
	if err != nil {
		t.Fatal(err)
	}
	convs := append([]string(nil), got["cap-1"]...)
	sort.Strings(convs)
	if len(got) != 1 || strings.Join(convs, ",") != "doc-one,doc-two" {
		t.Fatalf("conversations = %v, want only cap-1: doc-one, doc-two (not the other computer's, not an unread lead)", got)
	}
	if got, err := f.store.SearchLeadConversationsByCapture(ctx, "o1", "c1", nil); err != nil || len(got) != 0 {
		t.Fatalf("no captures: %v, %v", got, err)
	}

	og := f.og
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": {Found: true, UserID: "o1", ComputerID: "c1"}}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(from string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/search-leads/by-capture", bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleSearchLeadsByCapture)).ServeHTTP(rec, r)
		return rec
	}
	if rec := post("10.200.1.2", map[string]any{"owner_id": "o1", "computer_id": "c2", "capture_ids": []string{"cap-1"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("a bound guest read another computer's conversations: %d %s", rec.Code, rec.Body)
	}
	rec := post("10.200.1.2", map[string]any{"owner_id": "o1", "computer_id": "c1", "capture_ids": []string{"cap-1"}})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "secret snippet") || !strings.Contains(rec.Body.String(), "doc-one") {
		t.Fatalf("route answer: %d %s", rec.Code, rec.Body)
	}
}

// A story's later research starts from what its earlier runs read: the pages
// read from leads that the story's conversation's searches returned.
func TestSearchLeadPagesOfConversationAreReadPagesOfThatConversationOnly(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	a, b, c := "https://example.com/read-a", "https://example.com/unread-b", "https://example.com/read-c"
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "doc-one", SearchLeadHit{URL: a, Title: "A"}, SearchLeadHit{URL: b, Title: "B"})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c1", "q", "doc-two", SearchLeadHit{URL: c, Title: "C"})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.WriteSearchLeads(ctx, leadWrite("o1", "c2", "q", "doc-one", SearchLeadHit{URL: a, Title: "A elsewhere"})); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ computer, url, capture string }{{"c1", a, "cap-a"}, {"c1", c, "cap-c"}, {"c2", a, "cap-a2"}} {
		if _, err := f.store.MarkSearchLeadsTried(ctx, "o1", m.computer, []string{m.url}, m.capture); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.store.SearchLeadPagesOfConversation(ctx, "o1", "c1", "doc-one", 10)
	if err != nil || len(got) != 1 || got[0].CaptureID != "cap-a" || got[0].URL != a || got[0].Title != "A" {
		t.Fatalf("pages = %+v, %v; want only the read lead of doc-one on this computer", got, err)
	}
	if got, _ := f.store.SearchLeadPagesOfConversation(ctx, "o1", "c1", "", 10); len(got) != 0 {
		t.Fatalf("an empty conversation listed %+v", got)
	}
	og := f.og
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": {Found: true, UserID: "o1", ComputerID: "c1"}}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(computer string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]any{"owner_id": "o1", "computer_id": computer, "conversation_id": "doc-one"})
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/search-leads/by-conversation", bytes.NewReader(raw))
		r.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleSearchLeadsByConversation)).ServeHTTP(rec, r)
		return rec
	}
	if rec := post("c2"); rec.Code != http.StatusForbidden {
		t.Fatalf("a bound guest read another computer's pages: %d", rec.Code)
	}
	if rec := post("c1"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "cap-a") || strings.Contains(rec.Body.String(), "snippet") {
		t.Fatalf("route answer: %d %s", rec.Code, rec.Body)
	}
}
