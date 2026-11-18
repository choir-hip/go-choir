package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The captured pages' full-text index in corpusd (capture_fulltext.go).
// Failure modes guarded here:
//   1. a fact only in a page's body is not found through the capture search,
//      or comes with no passage and no captured-object to cite;
//   6. a guest of another computer, or another owner, reaches the pages;
//   7. a page captured now is not found at once; a failed index write fails
//      the capture, is not logged with its id, or is not repaired;
//   8. the catch-up is not idempotent, does not follow the projection's
//      pattern, does not yield to searches, or does not log its progress;
//   9. until a page is caught up it is no longer found by title and labels;
//  11. a 2 MB page breaks indexing;
//  12. a deleted, tombstoned, superseded or replaced capture keeps its passages;
//  13. a hit lacks what the research desk needs to cite and read the page.

type ftFixture struct {
	store  *Store
	og     *ObjectGraphStore
	bodies *CaptureBodyStore
	ftDir  string
}

func newFTFixture(t *testing.T, configure bool) *ftFixture {
	t.Helper()
	store, _ := openTestPlatformStore(t)
	f := &ftFixture{store: store, og: NewObjectGraphStore(store), bodies: newCaptureBodyStore(t.TempDir(), nil), ftDir: t.TempDir()}
	if configure {
		f.configure(t)
	}
	return f
}

func (f *ftFixture) configure(t *testing.T) {
	t.Helper()
	if err := f.store.ConfigureCaptureFullText(f.ftDir, f.bodies); err != nil {
		t.Fatal(err)
	}
}

// capture builds a capture object whose text blob holds text.
func (f *ftFixture) capture(t *testing.T, owner, computer, key, title, url, text string, fetched time.Time) objectgraph.Object {
	t.Helper()
	ref, err := f.bodies.PutBody([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: url, CanonicalURL: url, Title: title,
		FetchedAt: fetched.Format(time.RFC3339Nano), ContentBlobID: ref, ExtractedTextBlobID: ref, BlocksBlobID: ref, Extractor: "x1", DocHash: "h-" + key})
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer,
		ContentHash: "h-" + key + fmt.Sprint(len(text)), Metadata: meta, CreatedAt: fetched, UpdatedAt: fetched}
}

func (f *ftFixture) put(t *testing.T, objs ...objectgraph.Object) {
	t.Helper()
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: objs}); err != nil {
		t.Fatalf("PutBatch: %v", err)
	}
}

func (f *ftFixture) search(t *testing.T, owner, computer, query string) CaptureSearchResponse {
	t.Helper()
	resp, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: owner, ComputerID: computer, Query: query, Words: strings.Fields(strings.ToLower(query)), Need: 1, Limit: 50})
	if err != nil {
		t.Fatalf("SearchCaptures %q: %v", query, err)
	}
	return resp
}

func hitIDs(resp CaptureSearchResponse) []string {
	var out []string
	for _, h := range resp.Hits {
		out = append(out, h.CaptureID)
	}
	return out
}

func ftFiller(n int, salt string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "Entry %s%d lists the routine ledger of crates and ropes at the quay, nothing unusual noted.\n\n", salt, i)
	}
	return b.String()
}

// fullTextLog collects the journal lines of a test.
func fullTextLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var mu sync.Mutex
	buf := &bytes.Buffer{}
	w := ftWriterFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) })
	log.SetOutput(w)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

type ftWriterFunc func(p []byte) (int, error)

func (w ftWriterFunc) Write(p []byte) (int, error) { return w(p) }

func TestFullTextFindsABodyOnlyFactAtOnceWithItsPassage(t *testing.T) {
	f := newFTFixture(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	fact := "The quorum of Valdoria ratified the amber treaty after a long night of argument."
	c1 := f.capture(t, "owner-a", "computer-a", "k1", "Weekly digest", "https://example.com/digest", ftFiller(15, "a")+fact+"\n\n"+ftFiller(15, "b"), now)
	c2 := f.capture(t, "owner-a", "computer-a", "k2", "Another digest", "https://example.com/other", ftFiller(20, "c"), now)
	f.put(t, c1, c2)

	// The 12-word question, no word gate: the hit is there at once.
	resp := f.search(t, "owner-a", "computer-a", "what did the quorum of valdoria ratify in the amber treaty")
	if got := hitIDs(resp); len(got) != 1 || got[0] != c1.CanonicalID {
		t.Fatalf("hits %v, want only %s", got, c1.CanonicalID)
	}
	h := resp.Hits[0]
	if !h.BodyMatch || h.TitleMatch || !strings.Contains(h.Snippet, "amber treaty") || h.TextHash == "" || h.Passages < 2 {
		t.Fatalf("hit lacks the passage: %+v", h)
	}
	if resp.FullText == nil || !resp.FullText.Used || resp.FullText.Indexed != 2 || resp.FullText.Captures != 2 {
		t.Fatalf("full-text info %+v", resp.FullText)
	}
	// What the research desk needs to cite and read the page: the capture
	// object (url, title, times) travels with the hit.
	if len(resp.HitObjects) != 1 || resp.HitObjects[0].CanonicalID != c1.CanonicalID {
		t.Fatalf("hit objects %+v", resp.HitObjects)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(resp.HitObjects[0])
	if err != nil || meta.URL != "https://example.com/digest" || meta.Title != "Weekly digest" {
		t.Fatalf("hit object metadata %+v %v", meta, err)
	}
	if h.FetchedAt == "" {
		t.Fatalf("the hit carries no fetched time: %+v", h)
	}
}

func TestFullTextWindowAndOrderThroughTheSearch(t *testing.T) {
	f := newFTFixture(t, true)
	day0 := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	var objs []objectgraph.Object
	for i := 0; i < 6; i++ {
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprint("w", i), "Quay inspection", fmt.Sprintf("https://example.com/q%d", i),
			"The inspector counted the barrels again.\n\n", day0.AddDate(0, 0, i)))
	}
	f.put(t, objs...)
	run := func(since, until, order string) []string {
		resp, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Query: "inspector barrels", Since: since, Until: until, Order: order})
		if err != nil {
			t.Fatal(err)
		}
		return hitIDs(resp)
	}
	if got := run("2026-11-03", "2026-11-05", ""); len(got) != 2 {
		t.Fatalf("window: %v", got)
	}
	recent := run("", "", "recent")
	if len(recent) != 6 || recent[0] != objs[5].CanonicalID || recent[5] != objs[0].CanonicalID {
		t.Fatalf("recent order: %v", recent)
	}
	if _, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Query: "x", Since: "last tuesday"}); err == nil {
		t.Fatal("an unreadable date must be refused")
	}
}

func TestFullTextRoutesKeepTheScopeAndLeaseRules(t *testing.T) {
	f := newFTFixture(t, true)
	now := time.Now().UTC()
	mine := f.capture(t, "owner-a", "computer-a", "m1", "Kestrel census", "https://example.com/k", "kestrel eggs counted on the north cliff.\n\n", now)
	theirs := f.capture(t, "owner-b", "computer-b", "t1", "Kestrel secret", "https://example.com/s", "kestrel eggs counted on the private cliff.\n\n", now)
	sameID := f.capture(t, "owner-other", "computer-a", "t2", "Kestrel elsewhere", "https://example.com/e", "kestrel eggs counted on another owner's cliff.\n\n", now)
	f.put(t, mine, theirs, sameID)
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: f.og}), f.og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "owner-a", ComputerID: "computer-a"},
		"10.200.9.9": {Found: true, UserID: "owner-b", ComputerID: "computer-b"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(from, owner, computer string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(CaptureSearchRequest{OwnerID: owner, ComputerID: computer, Query: "kestrel eggs cliff"})
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/search", bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureSearch(rec, r)
		return rec
	}
	// Another computer's guest naming this computer, or an unbound address.
	for _, c := range []struct{ from, owner, computer string }{
		{"10.200.9.9", "owner-a", "computer-a"},
		{"10.200.1.2", "owner-b", "computer-b"},
		{"10.200.7.7", "owner-a", "computer-a"},
	} {
		if rec := post(c.from, c.owner, c.computer); rec.Code != http.StatusForbidden {
			t.Fatalf("%s searched %s/%s by text: %d %s", c.from, c.owner, c.computer, rec.Code, rec.Body)
		}
	}
	rec := post("10.200.1.2", "owner-a", "computer-a")
	var resp CaptureSearchResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resp) != nil {
		t.Fatalf("bound guest: %d %s", rec.Code, rec.Body)
	}
	if got := hitIDs(resp); len(got) != 1 || got[0] != mine.CanonicalID {
		t.Fatalf("the bound guest saw %v, want only its own page", got)
	}
	for _, o := range resp.HitObjects {
		if o.OwnerID != "owner-a" || o.ComputerID != "computer-a" {
			t.Fatalf("a hit object from another scope: %s %s", o.OwnerID, o.ComputerID)
		}
	}
	// A bad date is the caller's error.
	raw, _ := json.Marshal(CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Query: "kestrel", Since: "soon"})
	r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/search", bytes.NewReader(raw))
	r.RemoteAddr = "10.200.1.2:40000"
	bad := httptest.NewRecorder()
	h.HandleCaptureSearch(bad, r)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("a bad date: %d %s", bad.Code, bad.Body)
	}
}

func TestFullTextWriteFailureDoesNotFailTheCaptureAndTheCatchUpRepairsIt(t *testing.T) {
	f := newFTFixture(t, true)
	buf := fullTextLog(t)
	now := time.Now().UTC()
	text := "The osprey nest on the mast held two chicks this spring.\n\n"
	c := f.capture(t, "owner-a", "computer-a", "fail1", "Osprey report", "https://example.com/osprey", text, now)
	// The text blob is not there when the capture is written (a body lost to
	// disk trouble): the index cannot read it.
	ref := strings.TrimPrefix(textRef(t, c), "sha256:")
	if err := os.Remove(f.bodies.path(ref)); err != nil {
		t.Fatal(err)
	}
	f.put(t, c) // the capture itself is stored
	if _, err := f.og.GetObject(context.Background(), c.CanonicalID); err != nil {
		t.Fatalf("the capture must be stored whatever the index does: %v", err)
	}
	if !strings.Contains(buf.String(), "capture fulltext write failed") || !strings.Contains(buf.String(), "capture="+c.CanonicalID) {
		t.Fatalf("the failure is not logged with the capture id:\n%s", buf.String())
	}
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "osprey nest")); len(got) != 0 {
		t.Fatalf("a page whose text could not be read was found: %v", got)
	}
	// The blob comes back; the catch-up repairs the index.
	if _, err := f.bodies.PutBody([]byte(text)); err != nil {
		t.Fatal(err)
	}
	prog, err := f.store.CatchUpCaptureFullText(context.Background())
	if err != nil || prog.Indexed != 1 || prog.Failed != 0 {
		t.Fatalf("catch-up %+v %v", prog, err)
	}
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "osprey nest")); len(got) != 1 {
		t.Fatalf("after the catch-up: %v", got)
	}
}

// textRef is a capture's text blob id.
func textRef(t *testing.T, obj objectgraph.Object) string {
	t.Helper()
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return meta.ExtractedTextBlobID
}

func TestFullTextCatchUpIsIdempotentBoundedLoggedAndKeepsTheOldWayMeanwhile(t *testing.T) {
	// Captures written before the index existed.
	f := newFTFixture(t, false)
	now := time.Now().UTC()
	var objs []objectgraph.Object
	for i := 0; i < 23; i++ {
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprint("old", i), fmt.Sprintf("Harbor ledger number %d", i), fmt.Sprintf("https://example.com/old%d", i),
			fmt.Sprintf("Barrel count %d recorded; the zanzibar clause was noted on page %d.\n\n", i, i), now.Add(-time.Duration(i)*time.Hour)))
	}
	f.put(t, objs...)
	f.configure(t)
	buf := fullTextLog(t)

	// Not yet caught up: found the old way (title and labels), not by text.
	resp := f.search(t, "owner-a", "computer-a", "zanzibar clause")
	if len(resp.Hits) != 0 || resp.FullText == nil || !resp.FullText.Used || resp.FullText.Indexed != 0 || resp.FullText.Captures != 23 {
		t.Fatalf("before the catch-up: hits %d, info %+v", len(resp.Hits), resp.FullText)
	}
	old, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor", "ledger"}, Need: 2, Candidates: 50})
	if err != nil || len(old.Objects) != 23 {
		t.Fatalf("the title search must go on working: %d objects, err %v", len(old.Objects), err)
	}

	savedBatch, savedDuty, savedYield := captureFullTextBatch, captureFullTextDutyFactor, captureFullTextYield
	t.Cleanup(func() {
		captureFullTextBatch, captureFullTextDutyFactor, captureFullTextYield = savedBatch, savedDuty, savedYield
	})
	captureFullTextBatch, captureFullTextDutyFactor = 5, 0
	yields := 0
	captureFullTextYield = func(context.Context) { yields++ }

	prog, err := f.store.CatchUpCaptureFullText(context.Background())
	if err != nil || prog.Indexed != 23 || prog.Scanned != 23 || prog.Total != 23 || prog.Failed != 0 {
		t.Fatalf("catch-up %+v %v", prog, err)
	}
	if yields < 4 {
		t.Fatalf("a 23-capture pass in batches of 5 must yield between batches, yielded %d times", yields)
	}
	logged := buf.String()
	for _, want := range []string{"capture fulltext catch-up scanned=23 indexed=23", "remaining=0", "rate="} {
		if !strings.Contains(logged, want) {
			t.Fatalf("progress line lacks %q:\n%s", want, logged)
		}
	}
	resp = f.search(t, "owner-a", "computer-a", "zanzibar clause")
	if len(resp.Hits) != 23 || resp.FullText.Indexed != 23 {
		t.Fatalf("after the catch-up: %d hits, info %+v", len(resp.Hits), resp.FullText)
	}
	// Counts match and nothing failed since: a second pass does not walk.
	short, err := f.store.CatchUpCaptureFullText(context.Background())
	if err != nil || short.Scanned != 0 || short.Indexed != 0 {
		t.Fatalf("a pass with matching counts must not walk: %+v %v", short, err)
	}
	// Idempotent: a forced walk finds everything there and writes nothing.
	f.store.fullText.Load().dirty.Store(true)
	again, err := f.store.CatchUpCaptureFullText(context.Background())
	if err != nil || again.Indexed != 0 || again.Skipped != 23 {
		t.Fatalf("second pass %+v %v", again, err)
	}
	// A restart: a new index over the same files needs no catch-up work.
	if err := f.store.ConfigureCaptureFullText(f.ftDir, f.bodies); err != nil {
		t.Fatal(err)
	}
	restarted, err := f.store.CatchUpCaptureFullText(context.Background())
	if err != nil || restarted.Indexed != 0 || restarted.Skipped != 23 { // a new process has no gap to compare, so it walks once
		t.Fatalf("pass after a restart %+v %v", restarted, err)
	}
	if got := f.search(t, "owner-a", "computer-a", "zanzibar"); len(got.Hits) != 23 {
		t.Fatalf("after the restart: %d hits", len(got.Hits))
	}
}

func TestFullTextCatchUpLetsSearchesFinishFirst(t *testing.T) {
	f := newFTFixture(t, true)
	ft := f.store.fullText.Load()
	saved := captureFullTextMaxWaitIdle
	t.Cleanup(func() { captureFullTextMaxWaitIdle = saved })
	captureFullTextMaxWaitIdle = 5 * time.Second
	ft.searching.Add(1)
	go func() { time.Sleep(150 * time.Millisecond); ft.searching.Add(-1) }()
	start := time.Now()
	ft.waitIdle(context.Background())
	if d := time.Since(start); d < 120*time.Millisecond || d > 2*time.Second {
		t.Fatalf("waitIdle returned after %v with a search running for 150ms", d)
	}
	// A search that never ends does not stall the catch-up for ever.
	ft.searching.Add(1)
	defer ft.searching.Add(-1)
	captureFullTextMaxWaitIdle = 60 * time.Millisecond
	start = time.Now()
	ft.waitIdle(context.Background())
	if d := time.Since(start); d < 50*time.Millisecond || d > 2*time.Second {
		t.Fatalf("waitIdle with a stuck search returned after %v", d)
	}
}

func TestFullTextRemovalsAndReplacements(t *testing.T) {
	f := newFTFixture(t, true)
	ctx := context.Background()
	now := time.Now().UTC()
	mk := func(key, title, text string) objectgraph.Object {
		return f.capture(t, "owner-a", "computer-a", key, title, "https://example.com/"+key, text+"\n\n", now)
	}
	deleted := mk("del", "Deleted page", "the walrus colony moved north")
	tomb := mk("tomb", "Tombstoned page", "the narwhal pod moved south")
	sup := mk("sup", "Superseded page", "the manatee herd moved east")
	repl := mk("repl", "Replaced page", "the dugong school moved west")
	orphan := mk("orph", "Orphan page", "the beluga group moved nowhere")
	f.put(t, deleted, tomb, sup, repl, orphan)
	found := func(q string) bool { return len(f.search(t, "owner-a", "computer-a", q).Hits) > 0 }
	for _, q := range []string{"walrus", "narwhal", "manatee", "dugong", "beluga"} {
		if !found(q) {
			t.Fatalf("setup: %s not found", q)
		}
	}

	if err := f.og.DeleteObject(ctx, deleted.CanonicalID); err != nil {
		t.Fatal(err)
	}
	if found("walrus") {
		t.Fatal("a deleted capture is still found by its text")
	}
	tomb.Tombstone = true
	f.put(t, tomb)
	if found("narwhal") {
		t.Fatal("a tombstoned capture is still found by its text")
	}
	sup.SupersededBy = tomb.CanonicalID
	f.put(t, sup)
	if found("manatee") {
		t.Fatal("a superseded capture is still found by its text")
	}
	// Replacement: the same capture, new text.
	newer := f.capture(t, "owner-a", "computer-a", "repl", "Replaced page", "https://example.com/repl", "the sirenian school moved west\n\n", now.Add(time.Minute))
	newer.CanonicalID = repl.CanonicalID
	f.put(t, newer)
	if found("dugong") || !found("sirenian") {
		t.Fatal("a replaced capture must lose its old passages and gain the new")
	}
	// An orphan: the capture row goes without the inline removal (it failed
	// once); the catch-up's sweep removes the page.
	if _, err := f.store.corpus().ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id = ?`, orphan.CanonicalID); err != nil {
		t.Fatal(err)
	}
	scope := capturefts.Scope{Owner: "owner-a", Computer: "computer-a"}
	before, err := f.store.fullText.Load().index.Stats(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	// The search never returns a page whose capture is gone.
	if found("beluga") {
		t.Fatal("a hit whose capture is gone was returned")
	}
	prog, err := f.store.CatchUpCaptureFullText(ctx)
	if err != nil || prog.Removed < 1 {
		t.Fatalf("catch-up %+v %v", prog, err)
	}
	after, err := f.store.fullText.Load().index.Stats(ctx, scope)
	if err != nil || after.Pages != before.Pages-1 {
		t.Fatalf("the sweep must remove the orphan page: %d pages before, %d after (%v)", before.Pages, after.Pages, err)
	}
}

func TestFullTextIndexesA2MBPage(t *testing.T) {
	f := newFTFixture(t, true)
	var b strings.Builder
	for b.Len() < 2<<20 {
		b.WriteString(strings.Repeat("ledger entry about ropes and barrels at the quay today. ", 30) + "\n\n")
	}
	big := "headmarker opens this page.\n\n" + b.String() + "tailmarker closes it.\n\n"
	c := f.capture(t, "owner-a", "computer-a", "big", "Long ledger", "https://example.com/big", big, time.Now().UTC())
	f.put(t, c)
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "headmarker")); len(got) != 1 {
		t.Fatalf("the 2 MB page is not indexed: %v", got)
	}
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "tailmarker")); len(got) != 0 {
		t.Fatalf("text past the per-page cap was indexed: %v", got)
	}
}

// A capture whose record carries an owner-private label is not served to a
// non-custodian (agentcore capture_read_gate.go); its passage must not leave
// through the search either. It is not indexed, a search never returns a
// page whose record became private, and the catch-up removes it.
func TestFullTextNeverIndexesOrReturnsAnOwnerPrivateCapture(t *testing.T) {
	f := newFTFixture(t, true)
	ctx := context.Background()
	now := time.Now().UTC()
	private := f.capture(t, "owner-a", "computer-a", "priv", "Private notes", "https://example.com/p", "the walrus colony moved north.\n\n", now)
	private.Metadata = types.StampMetadataLabels(private.Metadata, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	open := f.capture(t, "owner-a", "computer-a", "open", "Open notes", "https://example.com/o", "the walrus colony moved south.\n\n", now)
	f.put(t, private, open)
	resp := f.search(t, "owner-a", "computer-a", "walrus colony")
	if got := hitIDs(resp); len(got) != 1 || got[0] != open.CanonicalID {
		t.Fatalf("an owner-private capture was indexed or returned: %v", got)
	}
	scope := capturefts.Scope{Owner: "owner-a", Computer: "computer-a"}
	if st, _ := f.store.fullText.Load().index.Stats(ctx, scope); st.Pages != 1 {
		t.Fatalf("pages indexed: %d, want only the open one", st.Pages)
	}

	// A capture indexed while open, then relabelled private in its record.
	later := f.capture(t, "owner-a", "computer-a", "later", "Later notes", "https://example.com/l", "the narwhal pod moved east.\n\n", now)
	f.put(t, later)
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "narwhal pod")); len(got) != 1 {
		t.Fatal("setup: the page should be found while open")
	}
	relabelled := types.StampMetadataLabels(later.Metadata, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	if _, err := f.store.corpus().ExecContext(ctx, `UPDATE og_objects SET metadata = ? WHERE canonical_id = ?`, string(relabelled), later.CanonicalID); err != nil {
		t.Fatal(err)
	}
	if got := hitIDs(f.search(t, "owner-a", "computer-a", "narwhal pod")); len(got) != 0 {
		t.Fatalf("a page whose record became owner-private was returned: %v", got)
	}
	prog, err := f.store.CatchUpCaptureFullText(ctx)
	if err != nil || prog.Removed < 1 {
		t.Fatalf("catch-up %+v %v", prog, err)
	}
	if st, _ := f.store.fullText.Load().index.Stats(ctx, scope); st.Pages != 1 {
		t.Fatalf("the sweep left the relabelled page: %d pages", st.Pages)
	}
}

// A disk-full host must not turn every failing capture into a log line and a
// full walk of every row: failures are counted, logged in a window with a
// count, and the walk is requested once. A later failure also says the index
// is no longer complete.
func TestFullTextWriteFailuresAreCountedNotLoggedOnePerCapture(t *testing.T) {
	f := newFTFixture(t, true)
	buf := fullTextLog(t)
	ft := f.store.fullText.Load()
	if _, err := f.store.CatchUpCaptureFullText(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ft.complete.Load() {
		t.Fatal("setup: an empty catch-up should be complete")
	}
	ft.writeHook = func(objectgraph.Object) error { return fmt.Errorf("no space left on device") }
	now := time.Now().UTC()
	var objs []objectgraph.Object
	for i := 0; i < 60; i++ {
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprint("fail", i), "Page", fmt.Sprintf("https://example.com/f%d", i), "text of the page.\n\n", now))
	}
	f.put(t, objs...)
	if got := ft.failures.Load(); got != 60 {
		t.Fatalf("failures counted %d, want 60", got)
	}
	if n := strings.Count(buf.String(), "capture fulltext write failed"); n > 2 || n == 0 {
		t.Fatalf("%d log lines for 60 failures, want one or two with a count:\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "count=") {
		t.Fatalf("the line carries no count:\n%s", buf.String())
	}
	if got := ft.kicks.Load(); got != 1 {
		t.Fatalf("the catch-up was requested %d times for 60 failures, want 1", got)
	}
	if ft.complete.Load() {
		t.Fatal("a failed write must reset complete")
	}
}

// The cached count of a computer's captures is not re-counted on every search.
func TestFullTextSearchDoesNotCountTheProjectionEveryTime(t *testing.T) {
	f := newFTFixture(t, true)
	now := time.Now().UTC()
	f.put(t, f.capture(t, "owner-a", "computer-a", "c1", "One", "https://example.com/1", "the osprey nest.\n\n", now))
	if got := f.search(t, "owner-a", "computer-a", "osprey").FullText.Captures; got != 1 {
		t.Fatalf("captures %d", got)
	}
	f.put(t, f.capture(t, "owner-a", "computer-a", "c2", "Two", "https://example.com/2", "the osprey eggs.\n\n", now))
	if got := f.search(t, "owner-a", "computer-a", "osprey").FullText.Captures; got != 1 {
		t.Fatalf("a search inside the cache window re-counted: %d", got)
	}
	f.store.fullText.Load().resetCountCache()
	if got := f.search(t, "owner-a", "computer-a", "osprey").FullText.Captures; got != 2 {
		t.Fatalf("after the cache is dropped: %d", got)
	}
}

func TestFullTextAQueryWithNoUsableWordIsNotASearch(t *testing.T) {
	f := newFTFixture(t, true)
	resp, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Query: "!!! ---"})
	if err != nil && !errors.Is(err, ErrCaptureIndexIncomplete) {
		t.Fatal(err)
	}
	if resp.FullText == nil || resp.FullText.Used || !strings.Contains(resp.FullText.Reason, "no searchable words") {
		t.Fatalf("a query with no usable word answered %+v", resp.FullText)
	}
}
