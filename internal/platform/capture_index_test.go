package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The capture projection (problem record
// docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
// Failure modes guarded here, written before the code:
//   5. the migration is not idempotent, or a re-run repeats work;
//   7. a failed projection write, a lost table, or captures written before
//      the migration leave the projection behind its authority for good,
//      or an unprojectable capture forces a full pass on every check;
//  10. the routes answer a guest for another computer;
//  11. an incomplete projection or the host switch still answers.

func testCapture(t *testing.T, owner, computer, key, title, url string, at time.Time) objectgraph.Object {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: url, CanonicalURL: url, Title: title,
		FetchedAt: at.Format(time.RFC3339Nano), ContentBlobID: "sha256:r", ExtractedTextBlobID: "sha256:t", BlocksBlobID: "sha256:b", Extractor: "x1", DocHash: "h-" + key})
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer,
		ContentHash: "h-" + key, Metadata: meta, CreatedAt: at, UpdatedAt: at}
}

func TestCaptureProjectionBackfillsReconcilesAndRepairs(t *testing.T) {
	saved := captureRecountDelay
	captureRecountDelay = 0
	t.Cleanup(func() { captureRecountDelay = saved })
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()

	// Captures written before the projection existed.
	store.retrievalState.captureIndexReady.Store(false)
	store.retrievalState.captureIndexComplete.Store(false)
	for _, table := range []string{captureIndexTable, captureTermsTable} {
		if _, err := store.corpus().ExecContext(ctx, "DROP TABLE "+table); err != nil {
			t.Fatal(err)
		}
	}
	before := testCapture(t, "owner-a", "computer-a", "before", "Harbor reopens", "https://example.com/harbor", now.Add(-48*time.Hour))
	unprojectable := testCapture(t, strings.Repeat("o", 200), "computer-a", "long-owner", "Harbor too", "https://example.com/long", now)
	for _, obj := range []objectgraph.Object{before, unprojectable} {
		if _, err := og.InsertObjectIfAbsent(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.SearchCaptures(ctx, CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1}); !errors.Is(err, ErrCaptureIndexIncomplete) {
		t.Fatalf("search before the projection: err %v, want incomplete", err)
	}

	if err := store.MigrateRetrieval(ctx); err != nil {
		t.Fatalf("MigrateRetrieval: %v", err)
	}
	resp, err := store.SearchCaptures(ctx, CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1})
	if err != nil || len(resp.Objects) != 1 || resp.Objects[0].CanonicalID != before.CanonicalID {
		t.Fatalf("backfilled search: %+v %v", resp, err)
	}
	// Re-running finds nothing to do and no full pass is forced by the
	// capture that cannot be projected.
	if err := store.MigrateRetrieval(ctx); err != nil {
		t.Fatalf("MigrateRetrieval again: %v", err)
	}
	shapes := store.RetrievalSnapshot().Ops[retrievalCaptureIndexSync].Shapes
	// (The store opened with one check of the empty projection.)
	if shapes["backfilled"] != 1 || shapes["checked"] != 2 {
		t.Fatalf("reconcile shapes %v: want one backfill then one check", shapes)
	}

	// An inline projection write fails (its table is gone): the capture
	// is still written, the store is marked dirty, and the reconciler
	// repairs it once the table is back.
	if _, err := store.corpus().ExecContext(ctx, "DROP TABLE "+captureTermsTable); err != nil {
		t.Fatal(err)
	}
	after := testCapture(t, "owner-a", "computer-a", "after", "Harbor storm damage", "https://example.com/storm", now)
	if inserted, err := og.InsertObjectIfAbsent(ctx, after); err != nil || !inserted {
		t.Fatalf("capture write with a broken projection: %v %v", inserted, err)
	}
	if !store.retrievalState.captureIndexDirty.Load() {
		t.Fatal("failed projection write did not mark the projection dirty")
	}
	if err := store.ensureCaptureIndexTables(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileCaptureIndex(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	resp, err = store.SearchCaptures(ctx, CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"storm"}, Need: 1})
	if err != nil || len(resp.Objects) != 1 || resp.Objects[0].CanonicalID != after.CanonicalID {
		t.Fatalf("repaired search: %+v %v", resp, err)
	}

	// Delete removes the projection.
	if err := og.DeleteObject(ctx, after.CanonicalID); err != nil {
		t.Fatal(err)
	}
	if res, err := store.CaptureByURL(ctx, "owner-a", "computer-a", "https://example.com/storm", true); err != nil || res.Found {
		t.Fatalf("deleted capture still found by URL: %+v %v", res, err)
	}
}

func TestCaptureIndexRoutesScopeAndSwitch(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()
	capture := testCapture(t, "owner-a", "computer-a", "k1", "Harbor reopens", "https://example.com/harbor", now)
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{capture}}); err != nil {
		t.Fatal(err)
	}
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "owner-a", ComputerID: "computer-a"},
		"10.200.9.9": {Found: true, UserID: "owner-b", ComputerID: "computer-b"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(store)
	post := func(from string, req CaptureSearchRequest) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(req)
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/search", bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureSearch(rec, r)
		return rec
	}
	byURL := func(from, owner, computer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/by-url?owner="+owner+"&computer="+computer+"&url=https://example.com/harbor/&blocks=1", nil)
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureByURL(rec, r)
		return rec
	}
	req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1}

	if rec := post("10.200.9.9", req); rec.Code != http.StatusForbidden {
		t.Fatalf("another computer's guest searched: %d %s", rec.Code, rec.Body)
	}
	if rec := byURL("10.200.9.9", "owner-a", "computer-a"); rec.Code != http.StatusForbidden {
		t.Fatalf("another computer's guest looked up a URL: %d %s", rec.Code, rec.Body)
	}
	rec := post("10.200.1.2", req)
	var resp CaptureSearchResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resp) != nil || len(resp.Objects) != 1 || resp.Searched != 1 {
		t.Fatalf("bound guest search: %d %s", rec.Code, rec.Body)
	}
	rec = byURL("10.200.1.2", "owner-a", "computer-a")
	var found CaptureByURLResult
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &found) != nil || !found.Found || found.CaptureID != capture.CanonicalID {
		t.Fatalf("bound guest URL lookup: %d %s", rec.Code, rec.Body)
	}

	t.Setenv(captureSearchEnv, "off")
	if rec := post("10.200.1.2", req); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("switch off: search answered %d", rec.Code)
	}
	if rec := byURL("10.200.1.2", "owner-a", "computer-a"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("switch off: URL lookup answered %d", rec.Code)
	}
	t.Setenv(captureSearchEnv, "")
	store.retrievalState.captureIndexComplete.Store(false)
	if rec := post("10.200.1.2", req); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("incomplete projection: search answered %d", rec.Code)
	}
}

// TestCaptureIndexRoutesFollowTheGuestListRule pins the new routes to the
// rule main applies to GET /internal/platform/objects
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F5): a
// bound guest names exactly the owner and computer vmctl binds it to, and
// a loopback caller needs the host credential once hostauth enforces. The
// SQL scope holds for a host caller too: owner B's search never returns
// owner A's capture on the same computer id.
func TestCaptureIndexRoutesFollowTheGuestListRule(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()
	capture := testCapture(t, "owner-a", "computer-a", "scope-1", "Harbor reopens", "https://example.com/harbor", now)
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{capture}}); err != nil {
		t.Fatal(err)
	}
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "owner-a", ComputerID: "computer-a"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(store)
	search := func(from, owner, computer string, mark func(*http.Request)) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(CaptureSearchRequest{OwnerID: owner, ComputerID: computer, Words: []string{"harbor"}, Need: 1})
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/search", bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		if mark != nil {
			mark(r)
		}
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleCaptureSearch)).ServeHTTP(rec, r)
		return rec
	}
	byURL := func(from, owner, computer string, mark func(*http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/by-url?owner="+owner+"&computer="+computer+"&url=https://example.com/harbor", nil)
		r.RemoteAddr = from + ":40000"
		if mark != nil {
			mark(r)
		}
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleCaptureByURL)).ServeHTTP(rec, r)
		return rec
	}

	// A bound guest naming any owner or computer but its own is refused.
	for _, c := range []struct{ owner, computer string }{
		{"owner-a", "computer-other"},
		{"owner-other", "computer-a"},
		{"owner-other", "computer-other"},
	} {
		if rec := search("10.200.1.2", c.owner, c.computer, nil); rec.Code != http.StatusForbidden {
			t.Fatalf("bound guest searched %s/%s: %d %s", c.owner, c.computer, rec.Code, rec.Body)
		}
		if rec := byURL("10.200.1.2", c.owner, c.computer, nil); rec.Code != http.StatusForbidden {
			t.Fatalf("bound guest looked up a URL for %s/%s: %d %s", c.owner, c.computer, rec.Code, rec.Body)
		}
	}
	// An unbound tap address is refused even for a real owner and computer.
	if rec := search("10.200.7.7", "owner-a", "computer-a", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("unbound guest searched: %d %s", rec.Code, rec.Body)
	}

	// Loopback with the marker but without the host credential: refused in
	// enforce; with it: answered, and scoped by SQL to the named owner.
	const token = "capture-index-scope-token-0123456789abcdef0123"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	marker := func(r *http.Request) { r.Header.Set("X-Internal-Caller", "true") }
	withCredential := func(r *http.Request) { marker(r); r.Header.Set(hostauth.Header, token) }
	if rec := search("127.0.0.1", "owner-a", "computer-a", marker); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback without credential searched in enforce: %d %s", rec.Code, rec.Body)
	}
	if rec := byURL("127.0.0.1", "owner-a", "computer-a", marker); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback without credential looked up a URL in enforce: %d %s", rec.Code, rec.Body)
	}
	var resp CaptureSearchResponse
	if rec := search("127.0.0.1", "owner-a", "computer-a", withCredential); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resp) != nil || len(resp.Objects) != 1 {
		t.Fatalf("host caller with credential: %d %s", rec.Code, rec.Body)
	}
	resp = CaptureSearchResponse{}
	if rec := search("127.0.0.1", "owner-other", "computer-a", withCredential); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resp) != nil || len(resp.Objects) != 0 {
		t.Fatalf("host search for another owner on the same computer id returned owner-a's capture: %d %s", rec.Code, rec.Body)
	}
	var found CaptureByURLResult
	if rec := byURL("127.0.0.1", "owner-other", "computer-a", withCredential); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &found) != nil || found.Found {
		t.Fatalf("host URL lookup for another owner found owner-a's capture: %d %s", rec.Code, rec.Body)
	}
}

func TestCaptureSearchQueryPlans(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()
	var objs []objectgraph.Object
	for i, title := range []string{"New AI model", "EU AI Act", "Harbor reopens", "中国发布新的人工智能模型", "Storm hits harbor", "Ukraine talks", "AI chips", "Election results"} {
		objs = append(objs, testCapture(t, "owner-a", "computer-a", title, title, "https://example.com/"+string(rune('a'+i)), now.Add(-time.Duration(i)*time.Hour)))
	}
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: objs}); err != nil {
		t.Fatal(err)
	}
	db := store.corpus()
	assertPlanIndex(t, "term exact", explainPlan(t, db, `SELECT capture_id, 0 AS qt, fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND term = ?`, "owner-a", "computer-a", "ai"),
		"proj_capture_terms_pkey")
	assertPlanIndex(t, "term prefix", explainPlan(t, db, `SELECT capture_id, 0 AS qt, fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND (term >= ? AND term < ?)`, "owner-a", "computer-a", "harb", "harb\U0010FFFF"),
		"proj_capture_terms_pkey")
	assertPlanIndex(t, "url key", explainPlan(t, db, `SELECT capture_id, fetched_at, updated_at FROM proj_capture_index WHERE owner_id = ? AND computer_id = ? AND url_key = ? AND has_blocks = TRUE ORDER BY fetched_at DESC NULLS LAST LIMIT 20`, "owner-a", "computer-a", captureURLKey("https://example.com/a")),
		"idx_pci_url")
	// The whole candidate query for a CJK word and a short word reads the
	// term table through its primary key only.
	plan := explainPlan(t, db, `WITH hits AS (SELECT capture_id, 0 AS qt, MAX(fetched_at) AS fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND (term = ? OR term = ?) GROUP BY capture_id HAVING COUNT(DISTINCT term) >= 2 UNION ALL SELECT capture_id, 1 AS qt, fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND term = ?)
		SELECT capture_id, COUNT(DISTINCT qt) AS words, MAX(fetched_at) AS fetched FROM hits GROUP BY capture_id HAVING COUNT(DISTINCT qt) >= ?`,
		"owner-a", "computer-a", "人工", "智能", "owner-a", "computer-a", "ai", 1)
	assertPlanIndex(t, "candidate query", plan, "proj_capture_terms_pkey")
	resp, err := store.SearchCaptures(ctx, CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"人工智能", "ai"}, Need: 1})
	if err != nil || len(resp.Objects) != 4 {
		t.Fatalf("search 人工智能 OR ai: %d objects %v", len(resp.Objects), err)
	}
}
