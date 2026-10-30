package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Corpus search over all history, end to end: the guest's corpus search
// and URL citation lookup, through CorpusdCaptureSink, against corpusd's
// real routes on a real (embedded) Dolt Store B, with the guest's real
// label store. Failure modes guarded (problem record
// docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
//   8. the index path loses a match the window path finds, or ranks
//      differently (label-only matches included);
//   9. short tokens, CJK and attached-prefix scripts are not found;
//  10. a search crosses computers or owners;
//  11. the rollback switches do not restore the window path.

type guestBinding struct{ owner, computer string }

type internalCaller struct{ next http.RoundTripper }

func (c internalCaller) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-Internal-Caller", "true")
	return c.next.RoundTrip(r)
}

func (g guestBinding) LookupGuestContext(context.Context, string) (*vmctl.GuestOwnership, error) {
	return &vmctl.GuestOwnership{Found: true, UserID: g.owner, ComputerID: g.computer}, nil
}

func openEmbeddedPlatformStore(t *testing.T) *platform.Store {
	t.Helper()
	control, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := platform.NewStore(control, graph)
	if err := s.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateRetrieval(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

type fixtureCapture struct {
	key, title, url, source, lang string
	age                           time.Duration
	owner, computer               string
}

func captureObject(t *testing.T, c fixtureCapture, now time.Time) objectgraph.Object {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, c.owner, objectgraph.StableSuffixFromKey(c.key))
	if err != nil {
		t.Fatal(err)
	}
	at := now.Add(-c.age)
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: c.url, CanonicalURL: c.url,
		Title: c.title, FetchedAt: at.Format(time.RFC3339Nano), ContentBlobID: "sha256:raw-" + c.key, ExtractedTextBlobID: "sha256:text-" + c.key,
		BlocksBlobID: "sha256:blocks-" + c.key, Extractor: "x1", DocHash: "h-" + c.key, SourceID: c.source, Language: c.lang, BlockCount: 3})
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: c.owner, ComputerID: c.computer,
		ContentHash: "h-" + c.key, Metadata: meta, CreatedAt: at, UpdatedAt: at}
}

func TestCorpusSearchOverAllHistory(t *testing.T) {
	ctx := context.Background()
	owner, computer := "owner-paper", "computer-paper"
	pstore := openEmbeddedPlatformStore(t)
	og := platform.NewObjectGraphStore(pstore)
	h := platform.NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	if err := h.ConfigureGuestBinding(guestBinding{owner, computer}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureBodies(t.TempDir())
	h.ConfigureCaptureIndex(pstore)
	srv := server.NewServer("corpusd-test", "0")
	platform.RegisterObjectGraphRoutes(srv, h)
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)

	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })

	// Ages in hours: the window below holds the newest 6 captures of this
	// computer; everything older is reachable only through the index.
	day := 24 * time.Hour
	fixtures := []fixtureCapture{
		{key: "fresh-ai", title: "New AI model release: EU regulators weigh in", url: "https://www.reuters.com/tech/ai-model", source: "rss:reuters-tech", lang: "en", age: time.Hour},
		{key: "fresh-harbor", title: "Harbor reopens to cargo ships", url: "https://portnews.example/harbor", source: "rss:portnews", lang: "en", age: 2 * time.Hour},
		{key: "fresh-filler-1", title: "Markets close higher", url: "https://example.com/m1", source: "rss:example", lang: "en", age: 3 * time.Hour},
		{key: "fresh-filler-2", title: "Weather service warns of storms", url: "https://example.com/w1", source: "rss:example", lang: "en", age: 4 * time.Hour},
		{key: "fresh-aid", title: "Aid convoy arrives after talks", url: "https://example.com/aid", source: "rss:example", lang: "en", age: 5 * time.Hour},
		{key: "fresh-filler-3", title: "Parliament passes budget", url: "https://example.com/b1", source: "rss:example", lang: "en", age: 6 * time.Hour},
		{key: "old-ai", title: "EU AI Act enforcement begins", url: "https://www.politico.eu/ai-act", source: "rss:politico", lang: "en", age: 3 * day},
		{key: "old-cjk", title: "中国发布新的人工智能模型", url: "https://news.cn/ai", source: "rss:xinhua-zh", lang: "zh", age: 10 * day},
		{key: "old-arabic", title: "والمصريون يحتفلون بالعيد", url: "https://ahram.example/eid", source: "rss:ahram", lang: "ar", age: 40 * day},
		{key: "old-covid", title: "COVID-19 cases rise again", url: "https://www.who.int/covid", source: "rss:who", lang: "en", age: 400 * day},
		{key: "old-harbor", title: "Harbor closed by storm", url: "https://portnews.example/harbor-closed", source: "rss:portnews", lang: "en", age: 8 * day},
		{key: "old-labelled", title: "بندر دوباره باز شد", url: "https://www.irna.ir/news/1", source: "rss:irna-fa", lang: "fa", age: 9 * day},
	}
	var objs []objectgraph.Object
	now := time.Now().UTC().Truncate(time.Second)
	for _, f := range fixtures {
		f.owner, f.computer = owner, computer
		objs = append(objs, captureObject(t, f, now))
	}
	// Another computer and another owner hold matching captures: never
	// returned (failure mode 10).
	objs = append(objs,
		captureObject(t, fixtureCapture{key: "other-computer", title: "Harbor reopens elsewhere AI", url: "https://example.org/a", owner: owner, computer: "computer-other", age: time.Hour}, now),
		captureObject(t, fixtureCapture{key: "other-owner", title: "Harbor reopens for another owner AI", url: "https://example.org/b", owner: "owner-other", computer: computer, age: time.Hour}, now))
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: objs}); err != nil {
		t.Fatalf("PutBatch: %v", err)
	}
	labelled := objs[11].CanonicalID
	if _, err := guest.AppendCaptureLabel(ctx, store.CaptureLabel{OwnerID: owner, ComputerID: computer, CaptureID: labelled, Title: "بندر دوباره باز شد",
		Outlet: "Islamic Republic News Agency", Language: "fa", Script: "Arabic", Event: store.CaptureLabelEventNew, TopicHeadline: "Harbor reopens after the storm"}); err != nil {
		t.Fatal(err)
	}

	sink := NewCorpusdCaptureSink(hs.URL)
	// A host-internal caller (the guest-binding rule is pinned in the
	// platform route tests).
	sink.Client = &http.Client{Transport: internalCaller{http.DefaultTransport}}
	deps := Dependencies{Store: guest, Captures: sink}
	search := func(query string) ([]CorpusCapture, string) {
		t.Helper()
		got, note, err := searchCaptures(ctx, deps, owner, computer, corpusQueryTerms(query), corpusMaxLimit)
		if err != nil {
			t.Fatalf("searchCaptures %q: %v", query, err)
		}
		return got, note
	}
	captureIDs := func(cs []CorpusCapture) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.CaptureID)
		}
		return strings.Join(out, ",")
	}
	queries := []string{"AI", "harbor reopens", "EU AI", "人工智能", "مصر", "covid-19", "reuters", "storm", "portnews harbor", "budget", "cargo ships harbor storm"}

	// Ground truth: the old matcher over every capture (a window wider than
	// the corpus), the path today's code takes.
	saved := corpusCaptureScan
	t.Cleanup(func() { corpusCaptureScan = saved })
	corpusCaptureScan = 1000
	t.Setenv(corpusSearchModeEnv, "window")
	truth := map[string]string{}
	for _, q := range queries {
		got, _ := search(q)
		truth[q] = captureIDs(got)
	}

	nonEmpty := 0
	for _, q := range queries {
		if truth[q] != "" {
			nonEmpty++
		}
	}
	if nonEmpty < len(queries)-1 {
		t.Fatalf("fixture: only %d of %d queries match anything: %v", nonEmpty, len(queries), truth)
	}

	// The index path over all history returns exactly that, ranked the
	// same, with no window bound (failure modes 8, 9).
	corpusCaptureScan = 6
	t.Setenv(corpusSearchModeEnv, "")
	// The one named deviation (captureterms.MinPrefixRunes): the old
	// matcher let "ai" match "Aid"; the index finds "AI" as a word.
	aid := objs[4].CanonicalID
	if !strings.Contains(truth["AI"], aid) {
		t.Fatalf("fixture: the old matcher no longer matches ai in Aid: %s", truth["AI"])
	}
	truth["AI"] = strings.Trim(strings.ReplaceAll(strings.ReplaceAll(truth["AI"], aid, ""), ",,", ","), ",")
	for _, q := range queries {
		got, note := search(q)
		if captureIDs(got) != truth[q] {
			t.Errorf("query %q:\n index %s\n truth %s\n note %q", q, captureIDs(got), truth[q], note)
		}
		if strings.Contains(note, "newest") && strings.Contains(note, "not searched") {
			t.Errorf("query %q: index path reports a window: %q", q, note)
		}
		for _, c := range got {
			if strings.Contains(c.URL, "example.org") {
				t.Errorf("query %q returned another computer's or owner's capture %s", q, c.URL)
			}
		}
	}
	// The label-only match (a Persian title found by its label's English
	// topic) came through the ids the guest sent.
	got, _ := search("harbor reopens")
	if !strings.Contains(captureIDs(got), labelled) {
		t.Fatalf("label-only match lost: %s", captureIDs(got))
	}

	// The window path, today's behaviour, misses what is older than the
	// window: the recall the index buys.
	t.Setenv(corpusSearchModeEnv, "window")
	win, note := search("人工智能")
	if len(win) != 0 || !strings.Contains(note, "older captures were not searched") {
		t.Fatalf("window path found %s (%q); the fixture's CJK capture should be outside the window", captureIDs(win), note)
	}
	t.Setenv(corpusSearchModeEnv, "")
	if idx, _ := search("人工智能"); len(idx) != 1 {
		t.Fatalf("index path: 人工智能 = %s, want the old CJK capture", captureIDs(idx))
	}

	// URL citation lookup over all history; the window misses it.
	tc := NewTextureCaptures(sink, computer)
	savedWin := textureCaptureWindow
	t.Cleanup(func() { textureCaptureWindow = savedWin })
	textureCaptureWindow = 6
	id, err := tc.newestCaptureOf(ctx, owner, "https://www.who.int/covid/")
	if err != nil || id != objs[9].CanonicalID {
		t.Fatalf("URL lookup through the index: %q %v", id, err)
	}
	t.Setenv(corpusSearchModeEnv, "window")
	if _, err := tc.newestCaptureOf(ctx, owner, "https://www.who.int/covid"); !errors.Is(err, texturewrite.ErrNotCaptured) {
		t.Fatalf("URL lookup through the window: err %v, want not captured", err)
	}
	t.Setenv(corpusSearchModeEnv, "")

	// Host rollback switch: corpusd refuses, the guest falls back to the
	// window and says so (failure mode 11).
	t.Setenv("CORPUSD_CAPTURE_SEARCH", "off")
	off, note := search("人工智能")
	if len(off) != 0 || !strings.Contains(note, "capture index not used") {
		t.Fatalf("CORPUSD_CAPTURE_SEARCH=off: %s %q", captureIDs(off), note)
	}
	t.Setenv("CORPUSD_CAPTURE_SEARCH", "")

	// Every call was timed and counted on corpusd.
	snap := pstore.RetrievalSnapshot()
	if snap.Ops["capture_search"].Calls == 0 || snap.Ops["capture_by_url"].Calls == 0 || !snap.Live["capture_index_complete"] {
		t.Fatalf("retrieval snapshot: %+v", snap)
	}
}
