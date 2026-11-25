package researchtools

// The corpus verbs' host tools. Failure list, written first:
//
//   T1 a tool takes its scope from an argument or a fallback, not the run;
//   T2 a page through corpusd's real route is not what the tool returns, or a
//      cursor does not continue it;
//   T3 corpus_texts asks for a scope from its arguments, or reads by the older
//      read by id (which serves any capture id to any bound guest);
//   T4 corpus_near routes a capture id, a publication id and a text to the
//      wrong request field.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/objectgraph/gatefixture"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type fakeCorpusSink struct {
	CaptureSink
	posts []string
}

func (f *fakeCorpusSink) CorpusCall(_ context.Context, route string, body any) (json.RawMessage, error) {
	raw, _ := json.Marshal(body)
	f.posts = append(f.posts, route+" "+string(raw))
	return json.RawMessage(`{}`), nil
}

func runCtx(owner, computer string) context.Context {
	return toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer})
}

func TestCorpusTextsAskCorpusdUnderTheRunsScope(t *testing.T) {
	sink := &fakeCorpusSink{}
	tool := newCorpusTextsTool(Dependencies{Captures: sink})
	raw, _ := json.Marshal(map[string]any{"ids": []string{"a", "b"}, "max_chars": 50, "owner_id": "evil", "computer_id": "evil"})
	if _, err := tool.Func(runCtx("o", "c"), raw); err != nil { // T3
		t.Fatal(err)
	}
	if len(sink.posts) != 1 || !strings.HasPrefix(sink.posts[0], "texts ") || !strings.Contains(sink.posts[0], `"owner_id":"o"`) || !strings.Contains(sink.posts[0], `"computer_id":"c"`) ||
		strings.Contains(sink.posts[0], "evil") || !strings.Contains(sink.posts[0], `"ids":["a","b"]`) {
		t.Fatalf("posts %v", sink.posts)
	}
	if _, err := tool.Func(context.Background(), raw); err == nil { // T1
		t.Fatal("a call with no run scope was served")
	}
	many, _ := json.Marshal(map[string]any{"ids": make([]string, 201)})
	if _, err := tool.Func(runCtx("o", "c"), many); err == nil {
		t.Fatal("201 ids were accepted")
	}
}

func TestCorpusCallRefusesAnAnswerThatReachesTheBoundInsteadOfReadingItShort(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"captures":["` + strings.Repeat("x", corpusReplResponseBytes) + `"]}`))
	}))
	defer big.Close()
	sink := NewCorpusdCaptureSink(big.URL)
	if out, err := sink.CorpusCall(context.Background(), "page", map[string]any{}); err == nil {
		t.Fatalf("a truncated answer was returned: %d bytes", len(out))
	}
}

func TestCorpusNearRoutesItsSubject(t *testing.T) {
	sink := &fakeCorpusSink{}
	tool := newCorpusNearTool(Dependencies{Captures: sink})
	for _, of := range []string{"obj:choir.web_capture:o:abc", "pub-0123456789abcdef0123456789abcdef", "pub-nothex harbor talks"} {
		raw, _ := json.Marshal(map[string]any{"of": of, "k": 5, "owner_id": "evil", "computer_id": "evil"})
		if _, err := tool.Func(runCtx("o", "c"), raw); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{`"capture_id":"obj:choir.web_capture:o:abc"`, `"story_id":"pub-0123456789abcdef0123456789abcdef"`, `"text":"pub-nothex harbor talks"`} // T4
	for i, w := range want {
		if !strings.Contains(sink.posts[i], w) || !strings.Contains(sink.posts[i], `"owner_id":"o"`) || !strings.Contains(sink.posts[i], `"computer_id":"c"`) || strings.Contains(sink.posts[i], "evil") {
			t.Fatalf("post %d: %s", i, sink.posts[i])
		}
	}
}

func TestCorpusCapturesThroughCorpusdPagesWithACursor(t *testing.T) {
	gatefixture.Observe(t)
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
	sink := NewCorpusdCaptureSink(hs.URL)
	sink.Client = &http.Client{Transport: internalCaller{http.DefaultTransport}}

	now := time.Now().UTC()
	var items []sourcegraph.CaptureItem
	for i := 0; i < 5; i++ {
		src := "feed:a"
		if i%2 == 1 {
			src = sourcegraph.ReadSourceID
		}
		items = append(items, sourcegraph.CaptureItem{
			Item:      sources.Item{ID: fmt.Sprintf("i%d", i), SourceID: src, Title: fmt.Sprintf("Page %d", i), URL: fmt.Sprintf("https://example.com/p%d", i), FetchedAt: now.Add(-time.Duration(i) * time.Hour), BodyKind: sources.BodyKindSourceBody},
			Extractor: "x1",
			Blocks:    []sourcefetch.Block{{ID: "b1", Kind: "paragraph", Text: fmt.Sprintf("alpha text %d", i)}, {ID: "b2", Kind: "paragraph", Text: "beta"}}})
	}
	res, err := sink.WriteCaptures(ctx, owner, computer, items)
	if err != nil || len(res.CaptureIDs) != 5 {
		t.Fatalf("write %+v %v", res, err)
	}
	deps := Dependencies{Captures: sink}
	page := newCorpusCapturesTool(deps)
	call := func(args map[string]any) (ids []string, next string) {
		t.Helper()
		raw, _ := json.Marshal(args)
		out, err := page.Func(runCtx(owner, computer), raw)
		if err != nil {
			t.Fatal(err)
		}
		var p struct {
			Captures []struct {
				CaptureID string `json:"capture_id"`
				Kind      string `json:"kind"`
			} `json:"captures"`
			Next string `json:"next"`
		}
		if err := json.Unmarshal([]byte(out), &p); err != nil {
			t.Fatal(err)
		}
		for _, c := range p.Captures {
			ids = append(ids, c.CaptureID)
		}
		return ids, p.Next
	}
	first, next := call(map[string]any{"limit": 2})
	if len(first) != 2 || next == "" { // T2
		t.Fatalf("first page %v next %q", first, next)
	}
	rest, end := call(map[string]any{"limit": 10, "cursor": next})
	if len(rest) != 3 || end != "" || rest[0] == first[1] {
		t.Fatalf("rest %v end %q", rest, end)
	}
	reads, _ := call(map[string]any{"kind": "read"})
	if len(reads) != 2 {
		t.Fatalf("reads %v", reads)
	}
	// corpus_texts through the real read route: the capture is this computer's.
	texts := newCorpusTextsTool(deps)
	raw, _ := json.Marshal(map[string]any{"ids": append(first, "obj:choir.web_capture:x:nothere")})
	out, err := texts.Func(runCtx(owner, computer), raw)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Texts []struct {
			Blocks []json.RawMessage `json:"blocks"`
			Error  string            `json:"error"`
		} `json:"texts"`
	}
	_ = json.Unmarshal([]byte(out), &got)
	if len(got.Texts) != 3 || len(got.Texts[0].Blocks) != 2 || got.Texts[2].Error == "" {
		t.Fatalf("texts %s", out)
	}
	// Another computer's run is answered, with nothing of this computer's in
	// it (the guest binding itself is pinned in corpusd's route tests).
	out, err = page.Func(runCtx(owner, "computer-other"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("another computer's page: %v", err)
	}
	var other struct {
		Captures []json.RawMessage `json:"captures"`
	}
	if err := json.Unmarshal([]byte(out), &other); err != nil || len(other.Captures) != 0 {
		t.Fatalf("another computer's run was served %d captures: %s %v", len(other.Captures), out, err)
	}
}
