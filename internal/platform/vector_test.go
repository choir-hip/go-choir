package platform

// The vector index (vector_store.go, vector_worker.go, vector_search.go,
// vector_import.go). A fake embedder gives every text a deterministic vector,
// so nothing here needs the model. Failure modes guarded:
//
//   1. a passage or a public capture has no vector after a pass, or is
//      embedded twice, or a story is embedded after the captures;
//   2. an owner-private capture is embedded, or keeps its vector after it
//      becomes private;
//   3. a restart resumes from memory (it re-derives the gap) or leaves a half
//      written story version;
//   4. a down embedder loses work, is logged on every retry, or never recovers;
//   5. a search returns the wrong order, ignores the window, or returns
//      another computer's captures;
//   6. an import takes another model's file, a file whose sample the embedder
//      here does not reproduce, a row whose text this host does not cut, or a
//      private capture;
//   7. an interrupted index build leaves the search slow for good.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type vecFixture struct {
	svc   *Service
	store *Store
	og    *ObjectGraphStore
	emb   *embedder.Fake
	root  string
}

func newVecFixture(t *testing.T) *vecFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	emb := &embedder.Fake{}
	store.SetEmbedder(emb)
	return &vecFixture{svc: svc, store: store, og: NewObjectGraphStore(store), emb: emb, root: root}
}

// storyText is a story of four paragraphs of about 1,000 runes about tag, so
// capturefts cuts it into four passages.
func storyText(tag string) string {
	var paras []string
	for i := 0; i < 4; i++ {
		var b strings.Builder
		for b.Len() < 1000 {
			fmt.Fprintf(&b, "%s%d reports the %s ledger entry number %d at the quay. ", tag, i, tag, b.Len())
		}
		paras = append(paras, strings.TrimSpace(b.String()))
	}
	return strings.Join(paras, "\n\n")
}

func (f *vecFixture) story(t *testing.T, doc string, rev int, title, content string, updated time.Time) {
	t.Helper()
	publishStoryRevision(t, f.svc, doc, rev, title, content, nil)
	if !updated.IsZero() {
		setPublicationUpdatedAt(t, f.store, "paper-owner", doc, updated)
	}
}

func (f *vecFixture) capture(t *testing.T, owner, computer, key, title, text string, fetched time.Time, private bool) objectgraph.Object {
	t.Helper()
	ref, err := f.svc.captureBodies.PutBody([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: "https://example.com/" + key, CanonicalURL: "https://example.com/" + key, Title: title,
		FetchedAt: fetched.Format(time.RFC3339Nano), ContentBlobID: ref, ExtractedTextBlobID: ref, BlocksBlobID: ref, Extractor: "x1", DocHash: "h-" + key})
	if private {
		meta = types.StampMetadataLabels(meta, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	}
	obj := objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer,
		ContentHash: "h-" + key + fmt.Sprint(len(text)), Metadata: meta, CreatedAt: fetched, UpdatedAt: fetched}
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("PutBatch: %v", err)
	}
	return obj
}

func (f *vecFixture) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := f.store.corpus().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *vecFixture) pass(t *testing.T) VectorPassResult {
	t.Helper()
	res, err := f.svc.RunVectorPass(context.Background())
	if err != nil {
		t.Fatalf("RunVectorPass: %v", err)
	}
	return res
}

// wantStoryRows counts the passages of every eligible version, read the way
// the worker reads them but not through its code path.
func (f *vecFixture) wantStoryRows(t *testing.T) int {
	t.Helper()
	refs, err := f.svc.eligibleStoryVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range refs {
		vt, ok, err := f.svc.loadStoryVersionText(context.Background(), f.og, r.StoryID, r.VersionID)
		if err != nil || !ok {
			t.Fatalf("version %s: ok=%v err=%v", r.VersionID, ok, err)
		}
		n += len(vt.Passages)
	}
	return n
}

func TestVectorPassEmbedsEveryPassageAndPublicCaptureOnce(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), base)
	f.story(t, "doc-a", 2, "Harbor reopens", storyText("harbour"), base)
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), base.Add(time.Second))
	f.capture(t, "o1", "c1", "k1", "Quay notes", "The quay was quiet.\n\nNothing moved.", base, false)
	f.capture(t, "o1", "c1", "k2", "Title only", "", base.Add(time.Minute), false)

	res := f.pass(t)
	want := f.wantStoryRows(t)
	if want != 12 {
		t.Fatalf("fixture has %d passages, want 12", want)
	}
	if got := f.count(t, "vec_story_passages"); got != want || res.StoryPassages != want || res.StoryVersions != 3 {
		t.Fatalf("story rows %d, result %+v, want %d", got, res, want)
	}
	if got := f.count(t, "vec_captures"); got != 2 || res.Captures != 2 {
		t.Fatalf("capture rows %d, result %+v", got, res)
	}
	// Every row carries the model's hash, a day, and the hash of its text.
	var bad int
	if err := f.store.corpus().QueryRow(`SELECT COUNT(*) FROM vec_story_passages WHERE model <> ? OR text_sha = '' OR day IS NULL OR embedded_on IS NULL`, embedder.ModelHash).Scan(&bad); err != nil || bad != 0 {
		t.Fatalf("rows without model, day or text hash: %d %v", bad, err)
	}
	// A second pass finds no gap and embeds nothing.
	calls := f.emb.Inputs.Load()
	res = f.pass(t)
	if res.Worked() || f.emb.Inputs.Load() != calls || res.StoryGap != 0 || res.CaptureGap != 0 {
		t.Fatalf("second pass did work: %+v (inputs %d -> %d)", res, calls, f.emb.Inputs.Load())
	}
	// A new story: only it is embedded.
	f.story(t, "doc-c", 1, "Canal dredged", storyText("canal"), base.Add(2*time.Second))
	res = f.pass(t)
	if res.StoryVersions != 1 || res.StoryPassages != 4 || res.Captures != 0 {
		t.Fatalf("new story pass: %+v", res)
	}
}

func TestVectorPassOrderIsStoriesThenCapturesNewestFirst(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	f.story(t, "doc-old", 1, "Oldstory", storyText("old"), base)
	f.story(t, "doc-new", 1, "Newstory", storyText("new"), base.Add(time.Minute))
	f.capture(t, "o1", "c1", "k-old", "Oldcapture", "old text", base, false)
	f.capture(t, "o1", "c1", "k-new", "Newcapture", "new text", base.Add(time.Hour), false)
	f.pass(t)
	var order []string
	for _, in := range f.emb.Seen() {
		for _, tag := range []string{"Newstory", "Oldstory", "Newcapture", "Oldcapture"} {
			if strings.HasPrefix(in, "title: "+tag+" ") {
				if len(order) == 0 || order[len(order)-1] != tag {
					order = append(order, tag)
				}
			}
		}
	}
	if strings.Join(order, ",") != "Newstory,Oldstory,Newcapture,Oldcapture" {
		t.Fatalf("order %v", order)
	}
}

func TestVectorPassInputIsTheDocumentPromptOfTheLeadPassage(t *testing.T) {
	f := newVecFixture(t)
	text := "First paragraph about the quay.\n\nSecond paragraph."
	f.capture(t, "o1", "c1", "k1", "Quay notes", text, time.Now().UTC().Add(-time.Hour), false)
	f.pass(t)
	spans := capturefts.Passages(text)
	lead := text[spans[0].Start:spans[0].End]
	seen := f.emb.Seen()
	if len(seen) != 1 || seen[0] != "title: Quay notes | text: "+lead {
		t.Fatalf("input %q", seen)
	}
	var sha string
	if err := f.store.corpus().QueryRow(`SELECT text_sha FROM vec_captures`).Scan(&sha); err != nil || sha != embedder.TextSHA(lead) {
		t.Fatalf("text_sha %q %v", sha, err)
	}
}

func TestVectorPassNeverEmbedsAnOwnerPrivateCaptureAndSweepsOneThatTurnsPrivate(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.capture(t, "o1", "c1", "pub", "Public page", "public text", now, false)
	f.capture(t, "o1", "c1", "priv", "Private page", "secret diary of the owner", now, true)
	f.pass(t)
	if got := f.count(t, "vec_captures"); got != 1 {
		t.Fatalf("capture rows %d, want only the public one", got)
	}
	for _, in := range f.emb.Seen() {
		if strings.Contains(in, "secret diary") {
			t.Fatalf("a private capture was sent to the embedder: %q", in)
		}
	}
	// The private capture is not an endless gap either: the next pass does no work.
	if res := f.pass(t); res.Worked() {
		t.Fatalf("second pass worked: %+v", res)
	}
	// A public capture later labelled private loses its vector at the sweep.
	pub, err := f.og.GetObject(context.Background(), mustCaptureID(t, "o1", "pub"))
	if err != nil {
		t.Fatal(err)
	}
	pub.Metadata = types.StampMetadataLabels(pub.Metadata, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	pub.ContentHash += "-private"
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{pub}}); err != nil {
		t.Fatal(err)
	}
	res := f.pass(t)
	if res.Removed != 1 || f.count(t, "vec_captures") != 0 {
		t.Fatalf("sweep: %+v rows %d", res, f.count(t, "vec_captures"))
	}
}

func mustCaptureID(t *testing.T, owner, key string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestVectorSweepRemovesAWithdrawnStory(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), time.Time{})
	f.pass(t)
	ident, _ := publicationIdentityFor("paper-owner", "doc-a")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, ident.CanonicalID); err != nil {
		t.Fatal(err)
	}
	res := f.pass(t)
	if res.Removed != 4 || f.count(t, "vec_story_passages") != 4 {
		t.Fatalf("withdrawn story: %+v rows %d", res, f.count(t, "vec_story_passages"))
	}
}

func TestVectorRestartRederivesTheGapFromTheTables(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-time.Hour)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.capture(t, "o1", "c1", "k1", "One", "one text", base, false)
	f.capture(t, "o1", "c1", "k2", "Two", "two text", base.Add(time.Minute), false)
	f.pass(t)
	// A crash left one version without vectors and one capture without.
	if _, err := f.store.corpus().Exec(`DELETE FROM vec_story_passages`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`DELETE FROM vec_captures WHERE capture_id = ?`, mustCaptureID(t, "o1", "k1")); err != nil {
		t.Fatal(err)
	}
	// A new Service over the same database holds nothing in memory.
	svc2 := NewService(f.store, filepath.Join(f.root, "artifacts"), filepath.Join(f.root, "signing.key"))
	before := f.emb.Inputs.Load()
	res, err := svc2.RunVectorPass(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.StoryVersions != 1 || res.Captures != 1 || f.emb.Inputs.Load()-before != 5 {
		t.Fatalf("restart pass %+v, inputs %d", res, f.emb.Inputs.Load()-before)
	}
	if f.count(t, "vec_story_passages") != 4 || f.count(t, "vec_captures") != 2 {
		t.Fatal("gap not closed")
	}
}

func TestVectorOtherModelsRowsAreTheGapAndAreReplaced(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false)
	f.pass(t)
	if _, err := f.store.corpus().Exec(`UPDATE vec_story_passages SET model = 'old'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE vec_captures SET model = 'old'`); err != nil {
		t.Fatal(err)
	}
	res := f.pass(t)
	var old int
	_ = f.store.corpus().QueryRow(`SELECT (SELECT COUNT(*) FROM vec_story_passages WHERE model='old') + (SELECT COUNT(*) FROM vec_captures WHERE model='old')`).Scan(&old)
	if res.StoryVersions != 1 || res.Captures != 1 || old != 0 || f.count(t, "vec_story_passages") != 4 {
		t.Fatalf("%+v old rows %d", res, old)
	}
}

func TestVectorEmbedderDownDropsNothingLogsOnceAndRecovers(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false)
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.emb.Down.Store(true)
	if _, err := f.svc.RunVectorPass(context.Background()); !errors.Is(err, errEmbedderDown) {
		t.Fatalf("err %v, want embedder down", err)
	}
	if f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
		t.Fatal("rows written with the embedder down")
	}
	logs := fullTextLog(t)
	oStart, oBackoff, oIdle, oMax := vectorStartDelay, vectorBackoffStart, vectorIdleInterval, vectorBackoffMax
	vectorStartDelay, vectorBackoffStart, vectorBackoffMax, vectorIdleInterval = 0, 10*time.Millisecond, 20*time.Millisecond, time.Hour
	t.Cleanup(func() {
		vectorStartDelay, vectorBackoffStart, vectorBackoffMax, vectorIdleInterval = oStart, oBackoff, oMax, oIdle
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.svc.StartVectorWorker(ctx)
	time.Sleep(300 * time.Millisecond) // many retries
	if n := strings.Count(logs.String(), "corpusd vectors embedder down, backing off"); n != 1 {
		t.Fatalf("%d 'embedder down' lines in %d retries, want exactly 1:\n%s", n, 10, logs.String())
	}
	f.emb.Down.Store(false)
	deadline := time.Now().Add(10 * time.Second)
	// The worker says "embedder is back" when the pass that wrote the rows
	// returns, so the rows can be seen a moment before the line: wait for both.
	for f.count(t, "vec_story_passages") != 4 || f.count(t, "vec_captures") != 1 || !strings.Contains(logs.String(), "embedder is back") {
		if time.Now().After(deadline) {
			t.Fatalf("worker did not recover, or did not say so; log:\n%s", logs.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "remaining=") {
		t.Fatalf("log lacks remaining=:\n%s", logs.String())
	}
}

func TestVectorWorkerDoesNotPaceItselfAndSendsOneBatchAtATime(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	for i := 0; i < 6; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("k%d", i), "Page", fmt.Sprintf("text %d", i), time.Now().UTC().Add(-time.Hour), false)
	}
	var inFlight, maxInFlight, maxBatch atomic.Int64
	f.emb.Override = nil
	slow := &countingEmbedder{inner: f.emb, inFlight: &inFlight, maxInFlight: &maxInFlight, maxBatch: &maxBatch}
	f.store.SetEmbedder(slow)
	// A search in flight does not hold the worker.
	start := time.Now()
	f.pass(t)
	if maxInFlight.Load() != 1 {
		t.Fatalf("%d embedder calls at once, want one batch at a time", maxInFlight.Load())
	}
	if maxBatch.Load() > int64(vectorBatch) || vectorBatch != 4 {
		t.Fatalf("batch of %d, vectorBatch %d", maxBatch.Load(), vectorBatch)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the pass was paced")
	}
}

type countingEmbedder struct {
	inner                           embedder.Embedder
	inFlight, maxInFlight, maxBatch *atomic.Int64
}

func (c *countingEmbedder) Health(ctx context.Context) error { return c.inner.Health(ctx) }
func (c *countingEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	n := c.inFlight.Add(1)
	defer c.inFlight.Add(-1)
	for {
		m := c.maxInFlight.Load()
		if n <= m || c.maxInFlight.CompareAndSwap(m, n) {
			break
		}
	}
	if int64(len(in)) > c.maxBatch.Load() {
		c.maxBatch.Store(int64(len(in)))
	}
	return c.inner.Embed(ctx, in)
}

func TestVectorPassLogsTheGapReadersLineAtBatchBoundaries(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	for i := 0; i < 9; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("k%d", i), "Page", fmt.Sprintf("text %d", i), time.Now().UTC().Add(-time.Hour), false)
	}
	f.capture(t, "o1", "c1", "priv", "Private", "secret", time.Now().UTC().Add(-time.Hour), true)
	logs := fullTextLog(t)
	f.pass(t)
	var gap []string
	for _, ln := range strings.Split(logs.String(), "\n") {
		if strings.Contains(ln, "op=embed_gap") {
			gap = append(gap, ln)
		}
	}
	re := regexp.MustCompile(`corpusd retrieval op=embed_gap shape=(stories|captures) ms=[0-9.]+ rows=(\d+) have=(\d+) want=(\d+) remaining=(\d+) model=2188ac1deca4 refused=\d+$`)
	var lastCap, lastStory []string
	nCap := 0
	for _, ln := range gap {
		m := re.FindStringSubmatch(ln[strings.Index(ln, "corpusd"):])
		if m == nil {
			t.Fatalf("line not in the form: %q", ln)
		}
		if m[1] == "captures" {
			lastCap = m
			nCap++
		} else {
			lastStory = m
		}
	}
	// start line + 3 batches of 4 (10 captures: 9 public, 1 private) = 4 lines, not 10.
	if nCap != 4 {
		t.Fatalf("%d capture gap lines, want 4 (start and three batches), not one per row:\n%s", nCap, strings.Join(gap, "\n"))
	}
	if lastCap[3] != "9" || lastCap[4] != "9" || lastCap[5] != "0" {
		t.Fatalf("captures end line %v: want have=9 want=9 remaining=0 (the private one is not wanted)", lastCap)
	}
	if lastStory[3] != "1" || lastStory[4] != "1" || lastStory[5] != "0" {
		t.Fatalf("stories end line %v", lastStory)
	}
}

func TestVectorSearchSaysVectorsAreUnavailableInsteadOfFailing(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	f.emb.Down.Store(true)
	res, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "harbor"})
	if err != nil || res.Unavailable == "" || len(res.Stories) != 0 {
		t.Fatalf("down embedder: %+v %v", res, err)
	}
	f.store.SetEmbedder(nil)
	if res, err = f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "harbor"}); err != nil || res.Unavailable == "" {
		t.Fatalf("no embedder: %+v %v", res, err)
	}
	// A vector needs no embedder and still answers.
	if res, err = f.store.SearchVectors(context.Background(), VectorSearchRequest{Vector: embedder.FakeVector("harbor ledger")}); err != nil || res.Unavailable != "" || len(res.Stories) == 0 {
		t.Fatalf("vector search: %+v %v", res, err)
	}
}

func TestVectorSearchOrderScopeAndWindow(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Truncate(24 * time.Hour).Add(-10 * 24 * time.Hour)
	f.capture(t, "o1", "c1", "near", "Near", "quorum valdoria amber treaty ratified", base.Add(5*24*time.Hour), false)
	f.capture(t, "o1", "c1", "mid", "Mid", "quorum valdoria harvest", base.Add(2*24*time.Hour), false)
	f.capture(t, "o1", "c1", "far", "Far", "tomato basil garden", base.Add(3*24*time.Hour), false)
	f.capture(t, "o2", "c2", "other", "Other", "quorum valdoria amber treaty ratified", base.Add(4*24*time.Hour), false)
	f.story(t, "doc-a", 1, "Treaty", "The quorum of valdoria ratified the amber treaty.\n\n"+strings.Repeat("filler words about nothing. ", 80), time.Time{})
	f.pass(t)
	ctx := context.Background()
	req := VectorSearchRequest{Query: "quorum valdoria amber treaty ratified", Owner: "o1", Computer: "c1"}
	res, err := f.store.SearchVectors(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, h := range res.Captures {
		ids = append(ids, h.CaptureID)
	}
	if len(ids) != 3 || ids[0] != mustCaptureID(t, "o1", "near") || ids[1] != mustCaptureID(t, "o1", "mid") || ids[2] != mustCaptureID(t, "o1", "far") {
		t.Fatalf("capture order %v", ids)
	}
	for i := 1; i < len(res.Captures); i++ {
		if res.Captures[i].Score > res.Captures[i-1].Score {
			t.Fatalf("scores not descending: %+v", res.Captures)
		}
	}
	if res.Captures[0].Score < 0.7 || len(res.Stories) == 0 || res.Stories[0].Ordinal != 0 || res.Stories[0].PublicationID == "" || res.Stories[0].VersionID == "" {
		t.Fatalf("scores or story hits: %+v", res)
	}
	// Another computer's capture is never returned.
	for _, id := range ids {
		if id == mustCaptureID(t, "o2", "other") {
			t.Fatal("another computer's capture returned")
		}
	}
	// No scope: no captures at all.
	req.Owner, req.Computer = "", ""
	if res, _ = f.store.SearchVectors(ctx, req); len(res.Captures) != 0 {
		t.Fatalf("captures without a scope: %+v", res.Captures)
	}
	// The window: since is inclusive, until exclusive, by day.
	req.Owner, req.Computer = "o1", "c1"
	req.Since, req.Until = base.Add(3*24*time.Hour), base.Add(5*24*time.Hour)
	res, err = f.store.SearchVectors(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Captures) != 1 || res.Captures[0].CaptureID != mustCaptureID(t, "o1", "far") {
		t.Fatalf("window hits %+v", res.Captures)
	}
	if len(res.Stories) != 0 {
		t.Fatalf("a story of today is in a window of ten days ago: %+v", res.Stories)
	}
	// A vector instead of a query answers the same.
	req = VectorSearchRequest{Vector: embedder.FakeVector("quorum valdoria amber treaty ratified"), Owner: "o1", Computer: "c1", CaptureLimit: 1}
	if res, err = f.store.SearchVectors(ctx, req); err != nil || len(res.Captures) != 1 || res.Captures[0].CaptureID != mustCaptureID(t, "o1", "near") {
		t.Fatalf("vector search %+v %v", res, err)
	}
	if _, err := f.store.SearchVectors(ctx, VectorSearchRequest{}); err == nil {
		t.Fatal("empty search accepted")
	}
}

func TestVectorSearchUsesTheQueryPromptAndTheJournalLine(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	var lines []string
	f.store.retrieval.logf = func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	if _, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "harbor"}); err != nil {
		t.Fatal(err)
	}
	seen := f.emb.Seen()
	if seen[len(seen)-1] != "task: search result | query: harbor" {
		t.Fatalf("query input %q", seen[len(seen)-1])
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "corpusd retrieval op=vector_search shape=vector_stories ms=") || !strings.Contains(joined, "op=vector_embed") {
		t.Fatalf("journal lines:\n%s", joined)
	}
}

func TestVectorIndexesAreBuiltConcurrentlyAndAnInvalidLeftoverIsRepaired(t *testing.T) {
	f := newVecFixture(t)
	logs := fullTextLog(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "index=idx_vsp_hnsw table=vec_story_passages") || !strings.Contains(logs.String(), "concurrently") {
		t.Fatalf("no concurrent build line:\n%s", logs.String())
	}
	valid := func(name string) bool {
		var v bool
		if err := f.store.corpus().QueryRow(`SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid WHERE c.relname = ?`, name).Scan(&v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	if !valid(vecStoryIndex) || !valid(vecCaptureIndex) {
		t.Fatal("indexes not valid after the build")
	}
	// An interrupted CREATE INDEX CONCURRENTLY leaves the index invalid.
	if _, err := f.store.corpus().Exec(`UPDATE pg_index SET indisvalid = false WHERE indexrelid = 'idx_vsp_hnsw'::regclass`); err != nil {
		t.Fatal(err)
	}
	if valid(vecStoryIndex) {
		t.Fatal("could not invalidate")
	}
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	if !valid(vecStoryIndex) || !strings.Contains(logs.String(), "left invalid by an interrupted build; rebuilding") {
		t.Fatalf("invalid leftover not repaired:\n%s", logs.String())
	}
	// A lock held by another session: the call goes on and builds nothing.
	conn, err := f.store.corpus().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(?)`, vecCaptureIndexLockKey); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE pg_index SET indisvalid = false WHERE indexrelid = 'idx_vc_hnsw'::regclass`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureVectorIndex(ctx, vecCaptureTable, vecCaptureIndex, "capture"); err != nil || valid(vecCaptureIndex) {
		t.Fatalf("built while another process held the lock (err %v)", err)
	}
	_, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock(?)`, vecCaptureIndexLockKey)
	// Writers are not blocked by a build: rows go in while it runs.
	if err := f.store.ensureVectorIndex(ctx, vecCaptureTable, vecCaptureIndex, "capture"); err != nil || !valid(vecCaptureIndex) {
		t.Fatalf("repair after unlock: %v", err)
	}
}

// ---- import ----

func f32ToHalf(f float32) uint16 {
	b := math.Float32bits(f)
	sign := uint16(b>>16) & 0x8000
	exp := int((b>>23)&0xff) - 127 + 15
	frac := b & 0x7fffff
	switch {
	case exp <= 0:
		if exp < -10 {
			return sign
		}
		frac |= 0x800000
		shift := uint(14 - exp)
		h := uint16(frac >> shift)
		if (frac>>(shift-1))&1 == 1 {
			h++
		}
		return sign | h
	case exp >= 31:
		return sign | 0x7bff
	}
	h := sign | uint16(exp)<<10 | uint16(frac>>13)
	if (frac>>12)&1 == 1 {
		h++
	}
	return h
}

func encodeHalfVector(v []float32) string {
	raw := make([]byte, 2*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint16(raw[2*i:], f32ToHalf(x))
	}
	return base64.StdEncoding.EncodeToString(raw)
}

type importRow struct {
	Kind, ID, TextSHA, Model string
	Ordinal, Dim             int
	Vec                      []float32
}

func writeImportFile(t *testing.T, rows []importRow) string {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		line, _ := json.Marshal(map[string]any{"kind": r.Kind, "id": r.ID, "ordinal": r.Ordinal, "text_sha": r.TextSHA, "model": r.Model, "dim": r.Dim, "v": encodeHalfVector(r.Vec)})
		b.Write(line)
		b.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "pass.ndjson")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// macRows are the rows the Mac would make for the fixture: every passage of
// every eligible version and the lead of each named capture, vectors from the
// same fake model.
func (f *vecFixture) macRows(t *testing.T, captureIDs ...string) []importRow {
	t.Helper()
	ctx := context.Background()
	var rows []importRow
	refs, err := f.svc.eligibleStoryVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		vt, ok, err := f.svc.loadStoryVersionText(ctx, f.og, r.StoryID, r.VersionID)
		if err != nil || !ok {
			t.Fatal(err)
		}
		for i, p := range vt.Passages {
			rows = append(rows, importRow{Kind: vectorKindStory, ID: r.VersionID, Ordinal: i, TextSHA: embedder.TextSHA(p), Model: embedder.ModelHash, Dim: 256,
				Vec: embedder.FakeVector(embedder.DocumentInput(vt.Title, p))})
		}
	}
	objs, err := f.store.captureObjectsByIDAny(ctx, captureIDs)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range captureIDs {
		obj := objs[id]
		meta, _ := objectgraph.WebCaptureMetadataFromObject(obj)
		hash := strings.TrimPrefix(meta.ExtractedTextBlobID, "sha256:")
		lead, err := captureLeadPassage(f.svc.captureBodies, hash)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, importRow{Kind: vectorKindCapture, ID: id, Ordinal: 0, TextSHA: embedder.TextSHA(lead), Model: embedder.ModelHash, Dim: 256,
			Vec: embedder.FakeVector(embedder.DocumentInput(meta.Title, lead))})
	}
	return rows
}

func importFixture(t *testing.T) (*vecFixture, []string) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-time.Hour)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), time.Time{})
	a := f.capture(t, "o1", "c1", "k1", "One", "one text about ropes", base, false).CanonicalID
	b := f.capture(t, "o1", "c1", "k2", "Two", "two text about nets", base.Add(time.Minute), false).CanonicalID
	return f, []string{a, b}
}

func TestVectorImportWritesMatchingRowsAndTheWorkerFindsNoGap(t *testing.T) {
	f, caps := importFixture(t)
	path := writeImportFile(t, f.macRows(t, caps...))
	res, err := f.svc.ImportVectorFile(context.Background(), path)
	if err != nil {
		t.Fatalf("import: %v (%+v)", err, res)
	}
	if res.Written != 8+2 || res.Samples == 0 || res.MinCosine < 0.999 || len(res.Rejected) != 0 {
		t.Fatalf("result %+v", res)
	}
	if f.count(t, "vec_story_passages") != 8 || f.count(t, "vec_captures") != 2 {
		t.Fatal("rows missing")
	}
	// The embedder was used only for the sample, and the worker has no gap.
	if f.emb.Inputs.Load() > vectorImportSamples {
		t.Fatalf("%d inputs embedded for an import", f.emb.Inputs.Load())
	}
	if res := f.pass(t); res.Worked() || res.StoryGap != 0 || res.CaptureGap != 0 {
		t.Fatalf("worker after import: %+v", res)
	}
}

func TestVectorImportRefusesAWrongModelOrDimensionWhole(t *testing.T) {
	f, caps := importFixture(t)
	for name, mutate := range map[string]func(r *importRow){
		"model": func(r *importRow) { r.Model = strings.Repeat("0", 64) },
		"dim":   func(r *importRow) { r.Dim = 768 },
		"kind":  func(r *importRow) { r.Kind = "page" },
	} {
		rows := f.macRows(t, caps...)
		mutate(&rows[len(rows)-1]) // the last row only: the whole file is refused
		_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
		if !errors.Is(err, ErrVectorImportRefused) {
			t.Fatalf("%s: err %v", name, err)
		}
		if f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
			t.Fatalf("%s: rows written from a refused file", name)
		}
	}
}

func TestVectorImportRefusesAFileWhoseSampleTheEmbedderHereDoesNotReproduce(t *testing.T) {
	f, caps := importFixture(t)
	rows := f.macRows(t, caps...)
	for i := range rows {
		rows[i].Vec = embedder.FakeVector("something else entirely " + fmt.Sprint(i)) // the right text hashes, other vectors
	}
	res, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, ErrVectorImportRefused) || !strings.Contains(err.Error(), "cosine") {
		t.Fatalf("err %v", err)
	}
	if res.MinCosine >= 0.999 || f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
		t.Fatalf("%+v rows written", res)
	}
}

func TestVectorImportDropsRowsThisHostDoesNotCutPrivateOnesAndPartialVersions(t *testing.T) {
	f, caps := importFixture(t)
	priv := f.capture(t, "o1", "c1", "priv", "Private", "secret diary", time.Now().UTC().Add(-time.Hour), true).CanonicalID
	rows := f.macRows(t, append(append([]string{}, caps...), priv)...)
	var storyRows []int
	for i, r := range rows {
		if r.Kind == vectorKindStory {
			storyRows = append(storyRows, i)
		}
	}
	// One version's passage was cut differently on the Mac: the whole version
	// is dropped, never half written.
	rows[storyRows[1]].TextSHA = embedder.TextSHA("a different cut")
	// A capture's lead differs.
	for i := range rows {
		if rows[i].ID == caps[1] {
			rows[i].TextSHA = embedder.TextSHA("other lead")
		}
	}
	res, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Rejected["text_mismatch"] != 4+1 || res.Rejected["not_public"] != 1 || res.Written != 4+1 {
		t.Fatalf("result %+v", res)
	}
	if f.count(t, "vec_story_passages") != 4 || f.count(t, "vec_captures") != 1 {
		t.Fatalf("rows story %d capture %d", f.count(t, "vec_story_passages"), f.count(t, "vec_captures"))
	}
	var n int
	_ = f.store.corpus().QueryRow(`SELECT COUNT(*) FROM vec_captures WHERE capture_id = ?`, priv).Scan(&n)
	if n != 0 {
		t.Fatal("a private capture was imported")
	}
	// The worker fills exactly what the import left out.
	res2 := f.pass(t)
	if res2.StoryVersions != 1 || res2.Captures != 1 {
		t.Fatalf("worker after partial import: %+v", res2)
	}
	// A version with a passage missing is incomplete and dropped whole.
	f2, caps2 := importFixture(t)
	rows = f2.macRows(t, caps2...)
	rows = append(rows[:1], rows[2:]...)
	res, err = f2.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if err != nil || res.Rejected["incomplete_version"] != 3 || f2.count(t, "vec_story_passages") != 4 {
		t.Fatalf("incomplete version: %+v %v rows %d", res, err, f2.count(t, "vec_story_passages"))
	}
}

func TestVectorImportJobAndRouteRefuseAnUntrustedCaller(t *testing.T) {
	f, caps := importFixture(t)
	path := writeImportFile(t, f.macRows(t, caps...))
	job, err := f.svc.StartVectorImport(path, false)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		st, ok := f.svc.VectorImportStatus(job.JobID)
		if ok && st.State != "running" {
			if st.State != "done" || st.Result == nil || st.Result.Written != 10 {
				t.Fatalf("job %+v", st)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	h := &Handler{service: f.svc}
	rec := httptest.NewRecorder()
	h.HandleVectorImport(rec, httptest.NewRequest("POST", "/internal/platform/vectors/import", strings.NewReader("{}")))
	if rec.Code != 403 {
		t.Fatalf("untrusted caller got %d", rec.Code)
	}
}

// The Mac's export (scripts/embed-pass-export.py) is what made the first
// vectors: a story's title is the manifest's, else the publication's; a
// capture's is its metadata title stripped; a passage is cut on the raw bytes
// and decoded with replacement, not stripped.
func TestVectorInputsMatchTheMacExportsTitlesAndText(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Manifest title", storyText("harbor"), time.Time{})
	ident, _ := publicationIdentityFor("paper-owner", "doc-a")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{title}', '"Publication title"')::text WHERE canonical_id = ?`, ident.CanonicalID); err != nil {
		t.Fatal(err)
	}
	f.capture(t, "o1", "c1", "k1", "  Padded title \n", "Lead \xff\xfe bytes here.\n\nSecond.", time.Now().UTC().Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "k2", "", "Untitled lead.", time.Now().UTC().Add(-2*time.Hour), false)
	f.pass(t)
	var story, caps []string
	for _, in := range f.emb.Seen() {
		if strings.HasPrefix(in, "title: Manifest title | text: ") {
			story = append(story, in)
		}
		if !strings.Contains(in, "harbor") {
			caps = append(caps, in)
		}
	}
	if len(story) != 4 {
		t.Fatalf("story inputs with the manifest title: %d of %q", len(story), f.emb.Seen())
	}
	for _, in := range f.emb.Seen() {
		if strings.Contains(in, "Publication title") {
			t.Fatalf("the publication's title was used over the manifest's: %q", in)
		}
	}
	want := map[string]bool{
		"title: Padded title | text: Lead \uFFFD\uFFFD bytes here.\n\nSecond.": true,
		"title: none | text: Untitled lead.":                                   true,
	}
	for _, in := range caps {
		if !want[in] {
			t.Fatalf("capture input %q, want one of %v", in, want)
		}
		delete(want, in)
	}
	if len(want) != 0 {
		t.Fatalf("capture inputs missing: %v", want)
	}
}

// ---- review round 1: completeness, eligibility, writers, import bounds ----

// captures puts n captures in one batch: texts come from text(i), each fetched
// i hours after the first. It returns their ids in order.
func (f *vecFixture) captures(t *testing.T, owner, computer, tag string, n int, base time.Time, text func(i int) string, step time.Duration) []string {
	t.Helper()
	var objs []objectgraph.Object
	var ids []string
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%s-%d", tag, i)
		body := text(i)
		ref, err := f.svc.captureBodies.PutBody([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		id := mustCaptureID(t, owner, key)
		at := base.Add(time.Duration(i) * step)
		meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: "https://example.com/" + key, CanonicalURL: "https://example.com/" + key, Title: "T " + key,
			FetchedAt: at.Format(time.RFC3339Nano), ContentBlobID: ref, ExtractedTextBlobID: ref, BlocksBlobID: ref, Extractor: "x1", DocHash: "h-" + key})
		objs = append(objs, objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer,
			ContentHash: "h-" + key, Metadata: meta, CreatedAt: at, UpdatedAt: at})
		ids = append(ids, id)
	}
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: objs}); err != nil {
		t.Fatalf("PutBatch: %v", err)
	}
	return ids
}

func forceIndex(t *testing.T) {
	t.Helper()
	old := vectorSearchTxHook
	vectorSearchTxHook = func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`)
		return err
	}
	t.Cleanup(func() { vectorSearchTxHook = old })
}

func TestVectorSearchWithAFilterIsCompleteNotSilentlyShort(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Truncate(24 * time.Hour).Add(-120 * 24 * time.Hour)
	// 400 near the query in one scope, 12 far away in another, a few of the
	// near ones on one day. The index scan reads its candidates by distance;
	// the filter keeps almost none of them.
	f.captures(t, "o1", "big", "near", 400, base, func(i int) string { return fmt.Sprintf("alpha common words number%d", i) }, 6*time.Hour)
	small := f.captures(t, "o1", "small", "far", 12, base, func(i int) string { return fmt.Sprintf("zeta omega unrelated%d", i) }, time.Hour)
	f.pass(t)
	forceIndex(t)
	old := vectorMaxScanTuples
	vectorMaxScanTuples = 20
	oldExact := vectorExactMaxRows
	vectorExactMaxRows = 0 // the index path: the short list is read again exactly
	t.Cleanup(func() { vectorMaxScanTuples, vectorExactMaxRows = old, oldExact })
	ctx := context.Background()

	res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Query: "alpha common words", Owner: "o1", Computer: "small", CaptureLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Captures) != len(small) || res.Partial != "" || !strings.Contains(strings.Join(res.ExactScan, ","), "captures") {
		t.Fatalf("scope: %d of %d captures, exact %v, partial %q", len(res.Captures), len(small), res.ExactScan, res.Partial)
	}
	for i := 1; i < len(res.Captures); i++ {
		if res.Captures[i].Score > res.Captures[i-1].Score {
			t.Fatal("exact scan out of order")
		}
	}

	// A one-day window over the big scope: 4 captures a day, all must come back.
	day := base.Add(40 * 24 * time.Hour)
	res, err = f.store.SearchVectors(ctx, VectorSearchRequest{Query: "alpha common words", Owner: "o1", Computer: "big", Since: day, Until: day.Add(24 * time.Hour), CaptureLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var want int
	if err := f.store.corpus().QueryRow(`SELECT COUNT(*) FROM vec_captures WHERE computer_id = 'big' AND day = ?::date`, day.Format("2006-01-02")).Scan(&want); err != nil || want == 0 {
		t.Fatalf("fixture has %d captures that day (%v)", want, err)
	}
	if len(res.Captures) != want || res.Partial != "" {
		t.Fatalf("window: %d of %d captures (exact %v, partial %q)", len(res.Captures), want, res.ExactScan, res.Partial)
	}

	// A list that fills its limit from the index is not asked again.
	res, err = f.store.SearchVectors(ctx, VectorSearchRequest{Query: "alpha common words", Owner: "o1", Computer: "big", CaptureLimit: 20})
	if err != nil || len(res.Captures) != 20 || strings.Contains(strings.Join(res.ExactScan, ","), "captures") || !strings.Contains(strings.Join(res.Approximate, ","), "captures") {
		t.Fatalf("full list: %d, exact %v, %v", len(res.Captures), res.ExactScan, err)
	}
}

func TestVectorSearchStatementUsesTheHNSWIndex(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "big", "near", 400, time.Now().UTC().Add(-48*time.Hour), func(i int) string { return fmt.Sprintf("alpha common words number%d", i) }, time.Minute)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	lit := vecLiteral(embedder.FakeVector("alpha"))
	none := func(a []any) (string, []any) { return "", a }
	cq, cargs := vectorCaptureQuery(lit, "o1", "big", none, 20)
	sq, sargs := vectorStoryQuery(lit, none, 20)
	for name, q := range map[string]struct {
		sql  string
		args []any
		idx  string
	}{"captures": {cq, cargs, vecCaptureIndex}, "stories": {sq, sargs, vecStoryIndex}} {
		tx, err := f.store.corpus().Begin()
		if err != nil {
			t.Fatal(err)
		}
		// With the alternatives to an ordered index scan priced out, the plan
		// is the HNSW index's (a planner left to itself may rightly prefer the
		// scope's btree and a sort on a small or unanalysed table; the
		// production plan is checked on staging with EXPLAIN ANALYZE).
		_, _ = tx.Exec(`ANALYZE ` + map[string]string{"captures": vecCaptureTable, "stories": vecStoryTable}[name])
		_, _ = tx.Exec(`SET LOCAL enable_seqscan = off`)
		_, _ = tx.Exec(`SET LOCAL enable_sort = off`)
		rows, err := tx.Query(`EXPLAIN `+q.sql, q.args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var plan strings.Builder
		for rows.Next() {
			var l string
			_ = rows.Scan(&l)
			plan.WriteString(l + "\n")
		}
		rows.Close()
		_ = tx.Rollback()
		if !strings.Contains(plan.String(), q.idx) {
			t.Fatalf("%s: the statement does not use %s:\n%s", name, q.idx, plan.String())
		}
	}
}

func TestVectorSearchNeverReturnsATextThatStoppedBeingPublicAndTheSweepNeedsNoEmbedder(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), time.Time{})
	priv := f.capture(t, "o1", "c1", "becomes-private", "Soon private", "harbor quay words", now, false).CanonicalID
	gone := f.capture(t, "o1", "c1", "tombstoned", "Soon gone", "harbor quay words", now, false).CanonicalID
	sup := f.capture(t, "o1", "c1", "superseded", "Soon superseded", "harbor quay words", now, false).CanonicalID
	keep := f.capture(t, "o1", "c1", "kept", "Kept", "harbor quay words", now, false).CanonicalID
	f.pass(t)
	ctx := context.Background()
	search := func() VectorSearchResult {
		res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Vector: embedder.FakeVector("harbor quay words"), Owner: "o1", Computer: "c1", CaptureLimit: 50, StoryLimit: 50})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := search(); len(res.Captures) != 4 || len(res.Stories) != 8 {
		t.Fatalf("before: %d captures %d passages", len(res.Captures), len(res.Stories))
	}
	// The texts stop being public. No pass, no sweep: the very next search is clean.
	p, _ := f.og.GetObject(ctx, priv)
	p.Metadata = types.StampMetadataLabels(p.Metadata, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	p.ContentHash += "-p"
	if err := f.og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{p}}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = '` + gone + `'`,
		`UPDATE og_objects SET superseded_by = 'obj:newer' WHERE canonical_id = '` + sup + `'`,
	} {
		if _, err := f.store.corpus().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	docA, _ := publicationIdentityFor("paper-owner", "doc-a")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, docA.CanonicalID); err != nil {
		t.Fatal(err)
	}
	docB, _ := publicationIdentityFor("paper-owner", "doc-b")
	versions, _ := publishedVersions(ctx, f.og, docB.CanonicalID)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, versions[0].CanonicalID); err != nil {
		t.Fatal(err)
	}
	res := search()
	if len(res.Captures) != 1 || res.Captures[0].CaptureID != keep || len(res.Stories) != 0 {
		t.Fatalf("after: captures %+v, %d passages: a text that is no longer public was returned", res.Captures, len(res.Stories))
	}
	var held int
	_ = f.store.corpus().QueryRow(`SELECT (SELECT COUNT(*) FROM vec_captures) + (SELECT COUNT(*) FROM vec_story_passages)`).Scan(&held)
	if held != 4+8 {
		t.Fatalf("rows %d: the search should not have deleted anything", held)
	}
	// The sweep removes them with the embedder down.
	f.emb.Down.Store(true)
	pass, err := f.svc.RunVectorPass(ctx)
	if !errors.Is(err, errEmbedderDown) {
		t.Fatalf("err %v", err)
	}
	if pass.Removed != 3+8 {
		t.Fatalf("removed %d, want 3 captures and 8 passages", pass.Removed)
	}
	_ = f.store.corpus().QueryRow(`SELECT (SELECT COUNT(*) FROM vec_captures) + (SELECT COUNT(*) FROM vec_story_passages)`).Scan(&held)
	if held != 1 {
		t.Fatalf("rows left %d, want only the kept capture", held)
	}
}

func TestVectorWritersOfOneVersionDoNotAbortEachOther(t *testing.T) {
	f := newVecFixture(t)
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	mk := func(seed string) []storyVectorRow {
		rows := make([]storyVectorRow, 6)
		for i := range rows {
			rows[i] = storyVectorRow{StoryID: "s", VersionID: "v", Ordinal: i, TextSHA: seed, Day: time.Now(), Vec: embedder.FakeVector(fmt.Sprint(seed, i))}
		}
		return rows
	}
	errs := make(chan error, 16)
	for w := 0; w < 8; w++ {
		go func(w int) {
			var err error
			for k := 0; k < 15 && err == nil; k++ {
				err = f.store.writeStoryVersionVectors(context.Background(), "s", "v", mk(fmt.Sprint("w", w)))
			}
			errs <- err
		}(w)
	}
	for w := 0; w < 8; w++ {
		if err := <-errs; err != nil {
			t.Fatalf("a writer failed: %v", err)
		}
	}
	if f.count(t, "vec_story_passages") != 6 {
		t.Fatalf("%d rows, want one whole version", f.count(t, "vec_story_passages"))
	}
}

func TestVectorCaptureLookupReadsOnlyCaptures(t *testing.T) {
	f := newVecFixture(t)
	id := f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false).CanonicalID
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET object_kind = 'choir.note' WHERE canonical_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	objs, err := f.store.captureObjectsByIDAny(context.Background(), []string{id})
	if err != nil || len(objs) != 0 {
		t.Fatalf("a non-capture object was read as a capture: %v %v", objs, err)
	}
}

func TestVectorImportSampleIsRandomStratifiedAndCannotShrinkBelowTheFloor(t *testing.T) {
	// 150 captures; the vectors are right only at the old fixed stride (every
	// fifth row). A random sample of 32 cannot all land on the 30 right rows.
	f := newVecFixture(t)
	ids := f.captures(t, "o1", "c1", "s", 150, time.Now().UTC().Add(-200*time.Hour), func(i int) string { return fmt.Sprintf("sample text number%d", i) }, time.Minute)
	rows := f.macRows(t, ids...)
	for i := range rows {
		if i%5 != 0 {
			rows[i].Vec = embedder.FakeVector(fmt.Sprint("wrong", i))
		}
	}
	res, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, ErrVectorImportRefused) || res.SampleSeed == 0 || !strings.Contains(err.Error(), fmt.Sprint(res.SampleSeed)) {
		t.Fatalf("a fixed-stride file passed or the seed is not recorded: %v (seed %d)", err, res.SampleSeed)
	}
	if f.count(t, "vec_captures") != 0 {
		t.Fatal("rows written from a refused file")
	}
	// Every present kind of row is checked: wrong vectors only on a story's
	// later passages are found.
	f2, caps := importFixture(t)
	rows = f2.macRows(t, caps...)
	for i := range rows {
		if rows[i].Kind == vectorKindStory && rows[i].Ordinal > 0 {
			rows[i].Vec = embedder.FakeVector(fmt.Sprint("wrong", i))
		}
	}
	if _, err := f2.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows)); !errors.Is(err, ErrVectorImportRefused) {
		t.Fatalf("wrong later passages passed: %v", err)
	}
	// A file of which too few rows can be checked is refused: 100 rows, 90 of
	// them with a text this host does not cut.
	f3 := newVecFixture(t)
	ids = f3.captures(t, "o1", "c1", "s", 100, time.Now().UTC().Add(-200*time.Hour), func(i int) string { return fmt.Sprintf("floor text number%d", i) }, time.Minute)
	rows = f3.macRows(t, ids...)
	for i := 10; i < 100; i++ {
		rows[i].TextSHA = embedder.TextSHA("another cut")
	}
	_, err = f3.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, ErrVectorImportRefused) || !strings.Contains(err.Error(), "can be checked") {
		t.Fatalf("a file that could be checked on 10 rows passed: %v", err)
	}
}

func TestVectorImportBoundsALineAndAVersionGroup(t *testing.T) {
	f, caps := importFixture(t)
	path := writeImportFile(t, f.macRows(t, caps...))
	raw, _ := os.ReadFile(path)
	long := append([]byte(strings.Repeat("x", vectorImportMaxLine+10)), '\n')
	bad := filepath.Join(t.TempDir(), "long.ndjson")
	_ = os.WriteFile(bad, append(append([]byte{}, raw...), long...), 0o600)
	if _, err := f.svc.ImportVectorFile(context.Background(), bad); !errors.Is(err, ErrVectorImportRefused) || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("a long line: %v", err)
	}
	if f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
		t.Fatal("rows written from a refused file")
	}
	old := vectorImportMaxGroupRows
	vectorImportMaxGroupRows = 2
	t.Cleanup(func() { vectorImportMaxGroupRows = old })
	res, err := f.svc.ImportVectorFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rejected["oversized_version"] != 8 || res.Written != 2 {
		t.Fatalf("oversized versions: %+v", res)
	}
}

func TestVectorImportRouteRefusesASecondImportBeforeReadingItsBody(t *testing.T) {
	f, caps := importFixture(t)
	f.store.vec.importMu.Lock()
	f.store.vec.jobs = map[string]*VectorImportJob{"j": {JobID: "j", State: "running"}}
	f.store.vec.lastJob = "j"
	f.store.vec.importMu.Unlock()
	_ = caps
	h := &Handler{service: f.svc}
	body := &failReader{t: t}
	req := httptest.NewRequest("POST", "/internal/platform/vectors/import", body)
	req.RemoteAddr = "127.0.0.1:40000"
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleVectorImport(rec, req)
	if rec.Code != 409 || body.read {
		t.Fatalf("status %d, body read %v", rec.Code, body.read)
	}
}

type failReader struct {
	t    *testing.T
	read bool
}

func (r *failReader) Read([]byte) (int, error) { r.read = true; return 0, io.EOF }

func TestVectorWorkerRemovesAStaleSpoolFileAtStart(t *testing.T) {
	f := newVecFixture(t)
	dir := filepath.Join(f.svc.artifactsRoot, vectorImportDirName)
	_ = os.MkdirAll(dir, 0o750)
	stale := filepath.Join(dir, "import-123.ndjson")
	_ = os.WriteFile(stale, []byte("x"), 0o600)
	f.svc.removeStaleVectorSpool()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale spool file kept")
	}
}

func TestVectorSearchOfAFewFilteredRowsIsExactAndALargeOneSaysApproximate(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "c1", "x", 30, time.Now().UTC().Add(-100*time.Hour), func(i int) string { return fmt.Sprintf("alpha text number%d", i) }, time.Minute)
	f.pass(t)
	req := VectorSearchRequest{Query: "alpha text", Owner: "o1", Computer: "c1", CaptureLimit: 10}
	res, err := f.store.SearchVectors(context.Background(), req)
	if err != nil || strings.Join(res.ExactScan, ",") != "stories,captures" || len(res.Approximate) != 0 || len(res.Captures) != 10 {
		t.Fatalf("below the threshold: %+v %v", res, err)
	}
	old := vectorExactMaxRows
	vectorExactMaxRows = 5
	t.Cleanup(func() { vectorExactMaxRows = old })
	res, err = f.store.SearchVectors(context.Background(), req)
	if err != nil || !strings.Contains(strings.Join(res.Approximate, ","), "captures") || len(res.Captures) != 10 {
		t.Fatalf("above the threshold: %+v %v", res, err)
	}
}

func TestVectorStoryWalkSeesEveryPublicationOfOneSecondAcrossABatchBoundary(t *testing.T) {
	f := newVecFixture(t)
	second := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	for i := 0; i < 8; i++ {
		doc := fmt.Sprintf("doc-%d", i)
		f.story(t, doc, 1, "Story "+doc, storyText(fmt.Sprintf("tag%d", i)), time.Time{})
		ident, _ := publicationIdentityFor("paper-owner", doc)
		// Same second, distinct microseconds: the whole-second cursor of the
		// front skips the rows of the second that sort below a batch's last row.
		at := second.Add(time.Duration(100+i*37) * time.Microsecond * 1000)
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, at, ident.CanonicalID); err != nil {
			t.Fatal(err)
		}
	}
	old := vectorStoryBatch
	vectorStoryBatch = 3
	t.Cleanup(func() { vectorStoryBatch = old })
	refs, err := f.svc.eligibleStoryVersions(context.Background())
	if err != nil || len(refs) != 8 {
		t.Fatalf("walk saw %d of 8 publications (%v)", len(refs), err)
	}
}

// vecPublish publishes a one-version story with an access policy and returns
// its publication and version ids.
func (f *vecFixture) vecPublish(t *testing.T, doc, title string, policy json.RawMessage) (pub, version string) {
	t.Helper()
	_, err := f.svc.PublishTexture(context.Background(), PublishTextureRequest{OwnerID: "paper-owner", SourceDocID: doc, SourceRevisionID: doc + "-rev-1", RevisionNumber: 1,
		Title: title, Content: storyText(doc), RequestedBy: "paper-owner", PublishMode: PublishModeAuto, AccessPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	ident, _ := publicationIdentityFor("paper-owner", doc)
	vs, err := publishedVersions(context.Background(), f.og, ident.CanonicalID)
	if err != nil || len(vs) == 0 {
		t.Fatalf("versions: %v %v", vs, err)
	}
	return ident.CanonicalID, vs[0].CanonicalID
}

func TestVectorWorkerWritesOnlyWhatTheOnePredicateKeepsEachClause(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Hour)
	// stories
	_, goodV := f.vecPublish(t, "good", "Good", nil)
	_, privPolV := f.vecPublish(t, "private-policy", "Private policy", json.RawMessage(`{"visibility":"private"}`))
	tombPub, _ := f.vecPublish(t, "tombstoned-pub", "Tomb pub", nil)
	_, tombVer := f.vecPublish(t, "tombstoned-ver", "Tomb ver", nil)
	noRoute, _ := f.vecPublish(t, "no-route", "No route", nil)
	for _, q := range []struct{ sql, id string }{
		{`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, tombPub},
		{`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, tombVer},
		{`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{route_path}', '""')::text WHERE canonical_id = ?`, noRoute},
	} {
		if _, err := f.store.corpus().Exec(q.sql, q.id); err != nil {
			t.Fatal(err)
		}
	}
	// captures
	f.capture(t, "o1", "c1", "ok", "Fine", "fine text", now, false)
	f.capture(t, "o1", "c1", "tomb", "Tomb", "tomb text", now, false)
	f.capture(t, "o1", "c1", "super", "Super", "super text", now, false)
	odd := f.capture(t, "o1", "c1", "oddlabel", "Odd label", "odd text", now, false)
	for _, q := range []string{
		`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = '` + mustCaptureID(t, "o1", "tomb") + `'`,
		`UPDATE og_objects SET superseded_by = 'obj:x' WHERE canonical_id = '` + mustCaptureID(t, "o1", "super") + `'`,
		// a label the Go reader drops (empty suffix is no label) but the one SQL
		// predicate keeps as private: it must not be embedded and swept in turn
		`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{labels}', '["owner_private:"]')::text WHERE canonical_id = '` + odd.CanonicalID + `'`,
	} {
		if _, err := f.store.corpus().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	res := f.pass(t)
	var versions []string
	rows, _ := f.store.corpus().Query(`SELECT DISTINCT version_id FROM vec_story_passages`)
	for rows.Next() {
		var v string
		_ = rows.Scan(&v)
		versions = append(versions, v)
	}
	rows.Close()
	if len(versions) != 1 || versions[0] != goodV {
		t.Fatalf("versions embedded %v (private policy %s), want only %s", versions, privPolV, goodV)
	}
	if f.count(t, "vec_captures") != 1 {
		t.Fatalf("captures embedded %d, want 1", f.count(t, "vec_captures"))
	}
	// A second pass embeds nothing and sweeps nothing: no write-then-sweep loop.
	before := f.emb.Inputs.Load()
	res2 := f.pass(t)
	if res2.Removed != 0 || res2.Captures != 0 || res2.StoryVersions != 0 || f.emb.Inputs.Load() != before {
		t.Fatalf("second pass %+v (first %+v)", res2, res)
	}
	_ = ctx
}

// ---- round 2: deadlines, refused passages, DDL, idle cost, the policy object ----

func TestVectorSearchSaysPartialWhenTheCountOrTheExactScanPassTheDeadline(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "c1", "p", 30, time.Now().UTC().Add(-100*time.Hour), func(i int) string { return fmt.Sprintf("alpha text number%d", i) }, time.Minute)
	f.pass(t)
	old := vectorSearchStageHook
	t.Cleanup(func() { vectorSearchStageHook = old })
	fail := func(stage string) {
		vectorSearchStageHook = func(ctx context.Context, tx *sql.Tx, got string) error {
			if got != stage {
				return nil
			}
			if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '5ms'`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `SELECT pg_sleep(2)`)
			return err
		}
	}
	req := VectorSearchRequest{Query: "alpha text", Owner: "o1", Computer: "c1", CaptureLimit: 10}
	for _, stage := range []string{"exact", "count"} {
		fail(stage)
		res, err := f.store.SearchVectors(context.Background(), req)
		if err != nil {
			t.Fatalf("%s timeout failed the search: %v", stage, err)
		}
		if !strings.Contains(res.Partial, "captures:") || len(res.Captures) != 10 || !strings.Contains(strings.Join(res.Approximate, ","), "captures") {
			t.Fatalf("%s: partial %q, %d captures, approximate %v", stage, res.Partial, len(res.Captures), res.Approximate)
		}
	}
	// And a search with no failure is complete.
	vectorSearchStageHook = nil
	res, err := f.store.SearchVectors(context.Background(), req)
	if err != nil || res.Partial != "" {
		t.Fatalf("clean search: %+v %v", res, err)
	}
}

// refusing refuses any input holding "TOOLONG" with a status error, as the
// server does for an input over its batch, and can fail with a network error.
type refusing struct {
	inner embedder.Embedder
	net   func(in string) bool
	calls atomic.Int64
}

func (r *refusing) Health(ctx context.Context) error { return r.inner.Health(ctx) }
func (r *refusing) Embed(ctx context.Context, in []string) ([][]float32, error) {
	r.calls.Add(int64(len(in)))
	for _, x := range in {
		if strings.Contains(x, "TOOLONG") {
			return nil, &embedder.InputRefusedError{Code: 500, Reason: "input (3000 tokens) is too large to process. increase the physical batch size"}
		}
		if r.net != nil && r.net(x) {
			return nil, errors.New("connection reset by peer")
		}
	}
	return r.inner.Embed(ctx, in)
}

func TestVectorARefusedPassageIsRecordedAndNotAskedAgain(t *testing.T) {
	f := newVecFixture(t)
	text := strings.Replace(storyText("harbor"), "harbor1 reports", "TOOLONG harbor1 reports", 1)
	f.story(t, "doc-a", 1, "Harbor reopens", text, time.Time{})
	f.capture(t, "o1", "c1", "refused-cap", "TOOLONG title", "lead text", time.Now().UTC().Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "fine", "Fine", "harbor lead text", time.Now().UTC().Add(-time.Hour), false)
	r := &refusing{inner: f.emb}
	f.store.SetEmbedder(r)
	logs := fullTextLog(t)
	res := f.pass(t)
	if res.StoryVersions != 1 || res.StoryPassages != 3 || res.Refused != 2 || res.Captures != 1 {
		t.Fatalf("first pass %+v", res)
	}
	var vec, ref int
	_ = f.store.corpus().QueryRow(`SELECT COUNT(*) FILTER (WHERE embedding IS NOT NULL), COUNT(*) FILTER (WHERE refused <> '') FROM vec_story_passages`).Scan(&vec, &ref)
	if vec != 3 || ref != 1 {
		t.Fatalf("story rows: %d vectors, %d refused", vec, ref)
	}
	if !strings.Contains(logs.String(), "refused=1") && !strings.Contains(logs.String(), "refused=2") {
		t.Fatalf("no refused= in the gap line:\n%s", logs.String())
	}
	// The next pass asks the model nothing.
	calls := r.calls.Load()
	res = f.pass(t)
	if r.calls.Load() != calls || res.Worked() || res.StoryGap != 0 || res.CaptureGap != 0 {
		t.Fatalf("second pass asked the model again: %+v, %d calls", res, r.calls.Load()-calls)
	}
	// A search returns the version's other passages and never the refused one.
	got, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Vector: embedder.FakeVector("harbor ledger"), Owner: "o1", Computer: "c1", StoryLimit: 20, CaptureLimit: 20})
	if err != nil || len(got.Stories) != 3 || len(got.Captures) != 1 {
		t.Fatalf("search: %d story hits, %d captures, %v", len(got.Stories), len(got.Captures), err)
	}
	for _, h := range got.Stories {
		if h.Ordinal == 1 {
			t.Fatal("the refused passage was returned")
		}
	}
	// A change of model reopens it.
	if _, err := f.store.corpus().Exec(`UPDATE vec_story_passages SET model = 'old'; UPDATE vec_captures SET model = 'old'`); err != nil {
		t.Fatal(err)
	}
	if res = f.pass(t); res.StoryVersions != 1 || res.Refused != 2 {
		t.Fatalf("after a model change: %+v", res)
	}
}

func TestVectorAnErrorThatIsNotTheServersRefusalRecordsNothing(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	r := &refusing{inner: f.emb, net: func(in string) bool { return strings.Contains(in, "harbor2") }}
	f.store.SetEmbedder(r)
	if res, err := f.svc.RunVectorPass(context.Background()); err != nil || res.Skipped != 1 || res.Refused != 0 {
		t.Fatalf("a dropped connection: %+v %v", res, err)
	}
	if f.count(t, "vec_story_passages") != 0 {
		t.Fatal("a transient failure was recorded as a refusal")
	}
	r.net = nil
	if res := f.pass(t); res.StoryVersions != 1 || res.Refused != 0 {
		t.Fatalf("recovery: %+v", res)
	}
}

func TestVectorEnsureTakesNoLockOnACompleteTableAndSearchRunsNoDDL(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	// A concurrent HNSW build holds ShareUpdateExclusive; a CREATE INDEX IF NOT
	// EXISTS (Share) would wait for it.
	tx, err := f.store.corpus().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`LOCK TABLE vec_story_passages, vec_captures IN SHARE UPDATE EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	if err := f.store.ensureVectorTables(short); err != nil {
		t.Fatalf("ensure while a build holds the lock: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("ensure waited %v on a lock", time.Since(start))
	}
	_ = tx.Rollback()
	// A search with no tables says unavailable and creates nothing.
	if _, err := f.store.corpus().Exec(`DROP TABLE vec_story_passages, vec_captures`); err != nil {
		t.Fatal(err)
	}
	f.store.vec.ready.Store(false)
	res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Query: "harbor"})
	if err != nil || res.Unavailable == "" {
		t.Fatalf("search without tables: %+v %v", res, err)
	}
	var made bool
	_ = f.store.corpus().QueryRow(`SELECT to_regclass('vec_story_passages') IS NOT NULL`).Scan(&made)
	if made {
		t.Fatal("a search created the tables")
	}
}

func TestVectorIndexBuildDoesNotLeaveItsMemorySettingOnTheConnection(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`DROP INDEX idx_vsp_hnsw`); err != nil {
		t.Fatal(err)
	}
	f.store.corpus().SetMaxOpenConns(1)
	f.store.corpus().SetMaxIdleConns(1)
	if err := f.store.ensureVectorIndex(ctx, vecStoryTable, vecStoryIndex, "story"); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := f.store.corpus().QueryRow(`SHOW maintenance_work_mem`).Scan(&got); err != nil || got == "256MB" {
		t.Fatalf("maintenance_work_mem on the pooled connection: %q %v", got, err)
	}
}

func TestVectorIdlePassReadsOneRowAndTwoCountsNotEveryPublication(t *testing.T) {
	f := newVecFixture(t)
	for i := 0; i < 5; i++ {
		f.story(t, fmt.Sprintf("doc-%d", i), 1, fmt.Sprintf("Story %d", i), storyText(fmt.Sprintf("t%d", i)), time.Time{})
	}
	f.pass(t)
	calls := func(op string) int64 { return f.store.RetrievalSnapshot().Ops[op].Calls }
	edges, pubs := calls("list_edges_by_kind"), calls("front_publications")
	res := f.pass(t)
	if res.Worked() || calls("list_edges_by_kind") != edges || calls("front_publications")-pubs != 1 {
		t.Fatalf("idle pass walked: edge reads %d, publication reads %d, %+v", calls("list_edges_by_kind")-edges, calls("front_publications")-pubs, res)
	}
	// A new story, and a new version of an old one, end the idle state.
	f.story(t, "doc-new", 1, "New story", storyText("fresh"), time.Time{})
	if res = f.pass(t); res.StoryVersions != 1 {
		t.Fatalf("new story: %+v", res)
	}
	f.story(t, "doc-0", 2, "Story 0 again", storyText("again"), time.Time{})
	if res = f.pass(t); res.StoryVersions != 1 {
		t.Fatalf("new version: %+v", res)
	}
}

func TestVectorStoryPredicateReadsThePolicyObjectAndAMalformedOneExcludes(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	// distinct policy JSON, so each version has its own content-addressed policy object
	_, vOK := f.vecPublish(t, "ok", "Fine", json.RawMessage(`{"visibility":"public","route":"public","n":1}`))
	_, vPriv := f.vecPublish(t, "became-private", "Private later", json.RawMessage(`{"visibility":"public","route":"public","n":2}`))
	_, vBad := f.vecPublish(t, "bad-policy", "Bad policy", json.RawMessage(`{"visibility":"public","route":"public","n":3}`))
	f.pass(t)
	if f.count(t, "vec_story_passages") != 12 {
		t.Fatalf("rows %d", f.count(t, "vec_story_passages"))
	}
	setPolicy := func(version, accessJSON string) {
		t.Helper()
		res, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_build_object('access_policy_json', ?::text, 'export_policy_json', '{}')::text
			WHERE canonical_id IN (SELECT to_id FROM og_edges WHERE from_id = ? AND kind = 'has_policy')`, accessJSON, version)
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			t.Fatalf("policy objects updated for %s: %d", version, n)
		}
	}
	setPolicy(vPriv, `{"visibility":"private"}`)
	setPolicy(vBad, `{not json`)
	res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Vector: embedder.FakeVector("ledger"), StoryLimit: 50})
	if err != nil {
		t.Fatalf("a malformed policy object failed the search: %v", err)
	}
	seen := map[string]int{}
	for _, h := range res.Stories {
		seen[h.VersionID]++
	}
	// the read route fails closed on a malformed policy, so the vector search does too
	if seen[vPriv] != 0 || seen[vOK] != 4 || seen[vBad] != 0 {
		t.Fatalf("hits by version %v", seen)
	}
	if pass := f.pass(t); pass.Removed != 8 || f.count(t, "vec_story_passages") != 4 {
		t.Fatalf("sweep %+v rows %d", pass, f.count(t, "vec_story_passages"))
	}
}

func TestVectorImportAppliesTheWorkersEmptyTitleRule(t *testing.T) {
	f, caps := importFixture(t)
	empty := f.capture(t, "o1", "c1", "empty", "", "", time.Now().UTC().Add(-time.Hour), false).CanonicalID
	rows := f.macRows(t, caps...)
	rows = append(rows, importRow{Kind: vectorKindCapture, ID: empty, Ordinal: 0, TextSHA: embedder.TextSHA(""), Model: embedder.ModelHash, Dim: 256, Vec: embedder.FakeVector("title: none | text: ")})
	res, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	if res.Rejected["no_text"] != 1 || f.count(t, "vec_captures") != 2 {
		t.Fatalf("%+v captures %d", res, f.count(t, "vec_captures"))
	}
	f.pass(t)
	var vec, refused int
	_ = f.store.corpus().QueryRow(`SELECT COUNT(*) FILTER (WHERE embedding IS NOT NULL), COUNT(*) FILTER (WHERE refused <> '') FROM vec_captures WHERE capture_id = ?`, empty).Scan(&vec, &refused)
	if vec != 0 || refused != 1 {
		t.Fatalf("the worker embedded what the import refused (vectors %d, refused rows %d)", vec, refused)
	}
}

// ---- round 3: an exact gap, one edge query, the pgvector minimum ----

func TestVectorCaptureGapAndWantAreExactAndNeverReReadWhatCannotBeEligible(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("ok%d", i), "Fine", fmt.Sprintf("fine text %d", i), now, false)
	}
	f.capture(t, "o1", "c1", "priv", "Private", "secret", now, true)
	f.capture(t, "o1", "c1", "tomb", "Tomb", "tomb text", now, false)
	f.capture(t, "o1", "c1", "super", "Super", "super text", now, false)
	f.capture(t, "o1", "c1", "empty", "", "", now, false)
	for _, q := range []string{
		`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = '` + mustCaptureID(t, "o1", "tomb") + `'`,
		`UPDATE og_objects SET superseded_by = 'obj:x' WHERE canonical_id = '` + mustCaptureID(t, "o1", "super") + `'`,
	} {
		if _, err := f.store.corpus().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	logs := fullTextLog(t)
	res := f.pass(t)
	if res.CaptureGap != 6 || res.Captures != 5 || res.Refused != 1 {
		t.Fatalf("first pass %+v: the gap is the 5 fine captures and the empty one", res)
	}
	re := regexp.MustCompile(`op=embed_gap shape=captures ms=[0-9.]+ rows=\d+ have=(\d+) want=(\d+) remaining=(\d+) model=\w+ refused=(\d+)`)
	var last []string
	for _, ln := range strings.Split(logs.String(), "\n") {
		if m := re.FindStringSubmatch(ln); m != nil {
			last = m
		}
	}
	if last == nil || last[1] != "6" || last[2] != "6" || last[3] != "0" || last[4] != "1" {
		t.Fatalf("last captures line %v: want have=6 want=6 remaining=0 refused=1 (exact, not an upper bound)", last)
	}
	// Nothing that can never be eligible is a candidate again.
	n, err := f.store.captureGapCount(context.Background(), embedder.ModelHash)
	rows, err2 := f.store.captureGapBatch(context.Background(), embedder.ModelHash, nil, "", 50)
	if err != nil || err2 != nil || n != 0 || len(rows) != 0 {
		t.Fatalf("gap after the pass: count %d, rows %d (%v %v)", n, len(rows), err, err2)
	}
	before := f.emb.Inputs.Load()
	if res := f.pass(t); res.Worked() || res.CaptureGap != 0 || f.emb.Inputs.Load() != before {
		t.Fatalf("second pass %+v", res)
	}
}

func TestVectorStoryWalkReadsEdgesInChunksAndAgreesWithPublishedVersions(t *testing.T) {
	f := newFrontFixture(t)
	svc := f.svc
	ctx := context.Background()
	calls := func() int64 { return svc.store.RetrievalSnapshot().Ops["list_edges_by_kind"].Calls }
	before := calls()
	refs, err := svc.eligibleStoryVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if calls() != before {
		t.Fatalf("the walk made %d per-publication edge reads", calls()-before)
	}
	var want []string
	var after *frontCursor
	og := svc.ogStore()
	for {
		rows, err := svc.frontPublicationBatch(ctx, after, 200)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if !svc.looksSearchable(row) {
				continue
			}
			vs, err := publishedVersions(ctx, og, row.CanonicalID)
			if err != nil {
				t.Fatal(err)
			}
			for i := len(vs) - 1; i >= 0; i-- {
				want = append(want, vs[i].CanonicalID)
			}
		}
		if len(rows) < 200 {
			break
		}
		after = frontCursorAt(rows[len(rows)-1])
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.VersionID)
	}
	if len(got) < 5 || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("one-statement walk %v, publishedVersions walk %v", got, want)
	}
}

func TestVectorRefusesAPgvectorBelowTheMinimumAndTheSearchSaysUnavailable(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	logs := fullTextLog(t)
	if _, err := f.store.corpus().Exec(`UPDATE pg_extension SET extversion = '0.7.4' WHERE extname = 'vector'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.store.corpus().Exec(`UPDATE pg_extension SET extversion = '0.8.7' WHERE extname = 'vector'`)
	})
	err := f.store.ensureVectorTables(context.Background())
	if err == nil || !strings.Contains(err.Error(), "older than the 0.8.0") || !strings.Contains(logs.String(), "corpusd vectors disabled: pgvector 0.7.4") {
		t.Fatalf("ensure on 0.7.4: %v\n%s", err, logs.String())
	}
	if _, perr := f.svc.RunVectorPass(context.Background()); perr == nil || !strings.Contains(perr.Error(), "older than") {
		t.Fatalf("pass on 0.7.4: %v", perr)
	}
	res, serr := f.store.SearchVectors(context.Background(), VectorSearchRequest{Vector: embedder.FakeVector("harbor")})
	if serr != nil || !strings.Contains(res.Unavailable, "0.7.4") {
		t.Fatalf("search on 0.7.4: %+v %v", res, serr)
	}
	if !versionLess("0.7.4", "0.8.0") || versionLess("0.10.1", "0.8.0") || versionLess("0.8.0", "0.8.0") || !versionLess("0.8", "0.8.1") {
		t.Fatal("versionLess")
	}
	if _, err := f.store.corpus().Exec(`UPDATE pg_extension SET extversion = '0.8.7' WHERE extname = 'vector'`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	if res, serr = f.store.SearchVectors(context.Background(), VectorSearchRequest{Vector: embedder.FakeVector("harbor")}); serr != nil || res.Unavailable != "" {
		t.Fatalf("search after restore: %+v %v", res, serr)
	}
}

// ---- round 4: one rule for what a failure may make durable ----

// httpEmbedder serves the llama.cpp shape for a real embedder.Client: /health,
// and /v1/embeddings that answers per behave(inputs).
func httpEmbedder(t *testing.T, behave func(inputs []string) (code int, body string)) *embedder.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		code, body := behave(req.Input)
		if code != 200 {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(body))
			return
		}
		type d struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []d
		for i, in := range req.Input {
			e := make([]float32, 768)
			copy(e, embedder.FakeVector(in))
			data = append(data, d{Index: i, Embedding: e})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	return embedder.NewClient(srv.URL)
}

func TestVectorOnlyAnIdentifiedInputRefusalIsRecordedEveryOtherStatusIsRetried(t *testing.T) {
	for _, c := range []struct {
		code int
		body string
	}{
		{500, `{"error":{"message":"internal error"}}`},
		{503, `{"error":{"message":"Loading model"}}`},
		{429, `too many requests`},
		{404, `not found`},
	} {
		f := newVecFixture(t)
		f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
		f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false)
		bad := true
		f.store.SetEmbedder(httpEmbedder(t, func(in []string) (int, string) {
			if bad {
				return c.code, c.body
			}
			return 200, ""
		}))
		res, err := f.svc.RunVectorPass(context.Background())
		if !errors.Is(err, errEmbedderDown) || res.Refused != 0 {
			t.Fatalf("status %d: err %v, %+v", c.code, err, res)
		}
		if f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
			t.Fatalf("status %d wrote rows", c.code)
		}
		bad = false
		if res := f.pass(t); res.StoryVersions != 1 || res.Captures != 1 || res.Refused != 0 {
			t.Fatalf("status %d: not retried: %+v", c.code, res)
		}
	}
	// The two identified shapes, through the real client, are recorded.
	for _, msg := range []string{
		`{"error":{"message":"input (3000 tokens) is too large to process. increase the physical batch size (current batch size: 2048)"}}`,
		`{"error":{"message":"input (5000 tokens) is larger than the max context size (2048 tokens). skipping"}}`,
	} {
		f := newVecFixture(t)
		f.capture(t, "o1", "c1", "k1", "TOOLONG", "one text", time.Now().UTC().Add(-time.Hour), false)
		f.store.SetEmbedder(httpEmbedder(t, func(in []string) (int, string) {
			for _, x := range in {
				if strings.Contains(x, "TOOLONG") {
					return 500, msg
				}
			}
			return 200, ""
		}))
		if res := f.pass(t); res.Refused != 1 || res.Captures != 0 {
			t.Fatalf("%s: %+v", msg, res)
		}
	}
}

func TestVectorUnreadableMetadataIsRetriedNotRecorded(t *testing.T) {
	f := newVecFixture(t)
	id := f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false).CanonicalID
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = '{"labels":[]}' WHERE canonical_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if res := f.pass(t); res.Skipped != 1 || res.Refused != 0 || f.count(t, "vec_captures") != 0 {
			t.Fatalf("pass %d: %+v rows %d", i, res, f.count(t, "vec_captures"))
		}
	}
	// repaired metadata is embedded on the next pass
	obj := f.capture(t, "o1", "c1", "k1", "One", "one text", time.Now().UTC().Add(-time.Hour), false)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = ? WHERE canonical_id = ?`, string(obj.Metadata), id); err != nil {
		t.Fatal(err)
	}
	if res := f.pass(t); res.Captures != 1 {
		t.Fatalf("after repair: %+v", res)
	}
}

func TestVectorOnePersistentlyFailingCaptureDoesNotStallTheOlderOnes(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-10 * time.Hour)
	var ids []string
	for i := 0; i < 6; i++ {
		title := fmt.Sprintf("Page %d", i)
		if i == 5 { // the newest
			title = "BADONE"
		}
		ids = append(ids, f.capture(t, "o1", "c1", fmt.Sprintf("k%d", i), title, "text", base.Add(time.Duration(i)*time.Hour), false).CanonicalID)
	}
	r := &refusing{inner: f.emb, net: func(in string) bool { return strings.Contains(in, "BADONE") }}
	f.store.SetEmbedder(r)
	logs := fullTextLog(t)
	for i := 0; i < 2; i++ {
		res, err := f.svc.RunVectorPass(context.Background())
		if err != nil || res.Skipped != 1 {
			t.Fatalf("pass %d: %+v %v", i, res, err)
		}
		if i == 0 && res.Captures != 5 {
			t.Fatalf("the older captures stalled behind the failing one: %+v", res)
		}
	}
	if f.count(t, "vec_captures") != 5 || !strings.Contains(logs.String(), "capture="+ids[5]+" could not be embedded now") {
		t.Fatalf("rows %d\\n%s", f.count(t, "vec_captures"), logs.String())
	}
}

// policyCases are access_policy_json values the read route decides on.
var policyCases = []string{
	`{"visibility":"public","route":"public"}`, `{"visibility":"public"}`, `{"visibility":"PUBLIC ","route":" Public"}`,
	`{"visibility":"public","route":"private"}`, `{"visibility":"private"}`, `{"route":"public"}`, `{}`, ``,
	`{not json`, `null`, `[]`, `"public"`, `42`, `{"visibility":"public","route":""}`, `{"visibility":"public"} trailing`,
	`{"visibility":123}`, `{"visibility":"public","route":null}`,
}

func TestVectorSQLPolicyAgreesWithTheReadRoute(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	pub, ver := f.vecPublish(t, "pol", "Policy", json.RawMessage(`{"visibility":"public","route":"public","n":1}`))
	eligible := func() bool {
		ok, err := f.store.sqlEligibleStoryVersions(ctx, []storyVersionRef{{StoryID: pub, VersionID: ver}})
		if err != nil {
			t.Fatal(err)
		}
		return ok[ver]
	}
	if !eligible() {
		t.Fatal("the fixture's own policy is not eligible")
	}
	set := func(metadata string) {
		t.Helper()
		res, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = ? WHERE canonical_id IN (SELECT to_id FROM og_edges WHERE from_id = ? AND kind = 'has_policy')`, metadata, ver)
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			t.Fatalf("policy objects updated: %d", n)
		}
	}
	for _, in := range policyCases {
		meta, _ := json.Marshal(map[string]string{"access_policy_json": in, "export_policy_json": "{}"})
		set(string(meta))
		// the read route: firstNonEmpty(access_policy_json, "{}") through publicationAccessPolicyAllowsPublicRoute
		want := publicationAccessPolicyAllowsPublicRoute(json.RawMessage(firstNonEmpty(in, "{}")))
		if got := eligible(); got != want {
			t.Fatalf("access_policy_json %q: the SQL predicate says %v, the read route says %v", in, got, want)
		}
	}
	// a policy object whose metadata is not JSON, and one with no access key
	for name, meta := range map[string]string{"not json": `{{{`, "no access key": `{"export_policy_json":"{}"}`} {
		set(meta)
		var obj struct {
			AccessPolicyJSON string `json:"access_policy_json"`
		}
		_ = json.Unmarshal([]byte(meta), &obj)
		want := publicationAccessPolicyAllowsPublicRoute(json.RawMessage(firstNonEmpty(obj.AccessPolicyJSON, "{}")))
		if got := eligible(); got != want {
			t.Fatalf("%s: SQL %v, read route %v", name, got, want)
		}
	}
}

func TestVectorIdleMarkSeesATombstoneAndANewVersionThatLeaveTheCountsAlone(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "A", storyText("aaa"), time.Time{})
	f.story(t, "doc-b", 1, "B", storyText("bbb"), time.Time{})
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	m1, err := f.svc.storyMark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	identA, _ := publicationIdentityFor("paper-owner", "doc-a")
	vs, _ := publishedVersions(ctx, f.og, identA.CanonicalID)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, vs[0].CanonicalID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // creation times are whole seconds
	f.story(t, "doc-b", 2, "B again", storyText("bbb2"), time.Time{})
	identB, _ := publicationIdentityFor("paper-owner", "doc-b")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, m1.newest, identB.CanonicalID); err != nil {
		t.Fatal(err)
	}
	m2, err := f.svc.storyMark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m2.pubs != m1.pubs || m2.versions != m1.versions || m2.held != m1.held || !m2.newest.Equal(m1.newest) {
		t.Fatalf("the fixture did not keep the counts: %+v %+v", m1, m2)
	}
	if m2 == m1 {
		t.Fatal("the mark did not change: a tombstone plus a new version would leave the new version unseen")
	}
}

func TestVectorEnsureMigratesATableOfTheEarlierSchema(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`DROP TABLE vec_story_passages, vec_captures`,
		`CREATE TABLE vec_story_passages (story_id TEXT NOT NULL, version_id TEXT NOT NULL, ordinal INTEGER NOT NULL, text_sha TEXT NOT NULL, day DATE NOT NULL, model TEXT NOT NULL, embedding halfvec(256) NOT NULL, embedded_on DATE NOT NULL, PRIMARY KEY (story_id, version_id, ordinal))`,
		`CREATE TABLE vec_captures (capture_id TEXT NOT NULL PRIMARY KEY, owner_id TEXT NOT NULL, computer_id TEXT NOT NULL, text_sha TEXT NOT NULL, day DATE NOT NULL, model TEXT NOT NULL, embedding halfvec(256) NOT NULL, embedded_on DATE NOT NULL)`,
	} {
		if _, err := f.store.corpus().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatalf("ensure on the old schema: %v", err)
	}
	// a refused row (no vector, a reason) now writes
	rows := []storyVectorRow{{StoryID: "s", VersionID: "v", Ordinal: 0, TextSHA: "x", Day: time.Now(), Refused: "too large"}}
	if err := f.store.writeStoryVersionVectors(ctx, "s", "v", rows); err != nil {
		t.Fatalf("a refused row after migration: %v", err)
	}
	// and a second ensure alters nothing (it must not take an exclusive lock each pass)
	tx, _ := f.store.corpus().BeginTx(ctx, nil)
	_, _ = tx.Exec(`LOCK TABLE vec_story_passages IN SHARE UPDATE EXCLUSIVE MODE`)
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := f.store.ensureVectorTables(short); err != nil {
		t.Fatalf("ensure under a build's lock: %v", err)
	}
	_ = tx.Rollback()
}

func TestVectorSearchNeverFailsOnTheIndexReadEither(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "c1", "p", 10, time.Now().UTC().Add(-50*time.Hour), func(i int) string { return fmt.Sprintf("alpha %d", i) }, time.Minute)
	f.pass(t)
	old := vectorSearchStageHook
	t.Cleanup(func() { vectorSearchStageHook = old })
	vectorSearchStageHook = func(ctx context.Context, tx *sql.Tx, stage string) error {
		if stage == "count" || stage == "index" {
			if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '5ms'`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `SELECT pg_sleep(2)`)
			return err
		}
		return nil
	}
	res, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "alpha", Owner: "o1", Computer: "c1"})
	if err != nil || !strings.Contains(res.Partial, "index scan did not finish") {
		t.Fatalf("count and index both past the deadline: %+v %v", res, err)
	}
}

func TestVectorWalkIsTimed(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "A", storyText("aaa"), time.Time{})
	f.pass(t)
	if f.store.RetrievalSnapshot().Ops["published_versions_of"].Calls == 0 {
		t.Fatal("the chunked edge read is not in the retrieval counters")
	}
}

// The capture walk reads the gap a page at a time (vectorGapPage) and feeds the
// embedder vectorBatch at a time from it. Failure list, written before the code:
//
//	h. TestTheCaptureWalkReadsTheGapOncePerPageNotOncePerEmbedBatch: one gap
//	   statement per embed batch of four, each a scan of every capture;
//	i. the same walk still embeds every capture, one batch at a time.
func TestTheCaptureWalkReadsTheGapOncePerPageNotOncePerEmbedBatch(t *testing.T) {
	f := newVecFixture(t)
	const n = 30
	for i := 0; i < n; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("k%d", i), "Page", fmt.Sprintf("text %d", i), time.Now().UTC().Add(-time.Hour), false)
	}
	oldPage, oldHook := vectorGapPage, captureGapBatchHook
	vectorGapPage = 10
	var stmts int
	captureGapBatchHook = func() { stmts++ }
	f.pass(t) // synchronous: no goroutine of the code under test outlives it
	vectorGapPage, captureGapBatchHook = oldPage, oldHook
	if got := f.count(t, "vec_captures"); got != n {
		t.Fatalf("%d captures embedded, want %d", got, n)
	}
	if want := n/10 + 1; stmts > want {
		t.Fatalf("%d gap statements for %d captures, want at most %d (a page of 10), not one per batch of %d", stmts, n, want, vectorBatch)
	}
}
