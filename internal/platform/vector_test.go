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
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	old := vectorSweepEvery
	vectorSweepEvery = 0
	t.Cleanup(func() { vectorSweepEvery = old })
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
	if res.Removed != 1 || f.count(t, "vec_story_passages") != 4 {
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
	for f.count(t, "vec_story_passages") != 4 || f.count(t, "vec_captures") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("worker did not recover; log:\n%s", logs.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "embedder is back") || !strings.Contains(logs.String(), "remaining=") {
		t.Fatalf("log lacks recovery or remaining=:\n%s", logs.String())
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
	re := regexp.MustCompile(`corpusd retrieval op=embed_gap shape=(stories|captures) ms=[0-9.]+ rows=(\d+) have=(\d+) want=(\d+) remaining=(\d+) model=2188ac1deca4$`)
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
