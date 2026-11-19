package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The verb states what it searched. Three review rounds of the research brief
// found the same defect in three forms: emptiness inferred from the absence of
// failure notes in prose ("held nothing" printed when a half never ran). The
// result now carries a typed coverage object, set only on the path where a
// search returned without error; its zero value means not searched.
//
// Failure list, written before the code:
//  1. a result has no coverage object, or one that says a part ran when it did
//     not (no capture store, no page-text answer, the window fallback, a failed
//     or capped published search, Jev's labels unreadable);
//  2. a path nobody set reads as searched (the zero value must be "not").

// noSearchSink is a capture store that cannot search: only the window of the
// newest captures can be listed.
type noSearchSink struct{}

func (noSearchSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, nil
}
func (noSearchSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, errors.New("not used")
}
func (noSearchSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, nil
}

// indexSink answers the index and the published search as configured.
type indexSink struct {
	noSearchSink
	text      *CaptureTextInfo
	termsOpen bool
	published PublishedAnswer
	pubErr    error
}

func (s indexSink) SearchCaptures(context.Context, CaptureSearchQuery) (CaptureSearchAnswer, error) {
	return CaptureSearchAnswer{FullText: s.text, TermsIncomplete: s.termsOpen}, nil
}
func (s indexSink) SearchPublished(context.Context, string, int) (PublishedAnswer, error) {
	return s.published, s.pubErr
}

func coverageOf(t *testing.T, captures CaptureSink) SearchCoverage {
	t.Helper()
	return coverageOfQuery(t, captures, "harbor strike")
}

func coverageOfQuery(t *testing.T, captures CaptureSink, query string) SearchCoverage {
	t.Helper()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: captures})
	// A desk reads the labels as itself; the kernel reader stands for it here.
	ctx := objectgraph.WithReader(toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "o", ComputerID: "c"}), objectgraph.WriterKernel)
	payload, _ := json.Marshal(map[string]string{"query": query})
	out, err := tool.Func(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Coverage *SearchCoverage `json:"coverage"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatal(err)
	}
	if body.Coverage == nil {
		t.Fatalf("the result has no coverage object: %s", out)
	}
	return *body.Coverage
}

func TestCoverageNamesEachPartThatRan(t *testing.T) {
	whole := &CaptureTextInfo{Used: true, Indexed: 5, Captures: 5}
	cases := []struct {
		name string
		sink CaptureSink
		want SearchCoverage
	}{
		{"no capture store", nil, SearchCoverage{}},
		{"index answered, page text did not", indexSink{}, SearchCoverage{Titles: true, Labels: true, Published: true}},
		{"window fallback", noSearchSink{}, SearchCoverage{Window: true}},
		{"title and label projection still building", indexSink{text: whole, termsOpen: true}, SearchCoverage{PageText: true, PageTextIndexed: 5, Window: true, Published: true}},
		{"page text index covers part", indexSink{text: &CaptureTextInfo{Used: true, Indexed: 3, Captures: 5}}, SearchCoverage{PageText: true, PageTextIndexed: 3, Titles: true, Labels: true, Published: true}},
		{"page text refused", indexSink{text: &CaptureTextInfo{Used: false, Reason: "index not ready"}}, SearchCoverage{Titles: true, Labels: true, Published: true}},
		{"published search failed", indexSink{text: whole, pubErr: errors.New("corpusd 503")}, SearchCoverage{PageText: true, PageTextIndexed: 5, Titles: true, Labels: true}},
		// A capped search ran and returned pieces: it is Published, and says it was capped.
		{"published search capped", indexSink{text: whole, published: PublishedAnswer{Capped: true}}, SearchCoverage{PageText: true, PageTextIndexed: 5, Titles: true, Labels: true, Published: true, PublishedCapped: true}},
		{"every part ran", indexSink{text: whole}, SearchCoverage{PageText: true, PageTextIndexed: 5, Titles: true, Labels: true, Published: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coverageOf(t, tc.sink); got != tc.want {
				t.Fatalf("coverage = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Jev's labels are read from the store separately from the index answer. When
// that read fails, the titles were searched and the labels were not.
func TestCoverageLabelsAreFalseWhenTheLabelStoreCannotBeRead(t *testing.T) {
	for name, prepare := range map[string]func(*store.Store, context.Context) context.Context{
		// The store is closed: the read errors.
		"store closed": func(s *store.Store, ctx context.Context) context.Context {
			_ = s.Close()
			return objectgraph.WithReader(ctx, objectgraph.WriterKernel)
		},
		// The call declares no reader: the object graph refuses the read.
		"no reader declared": func(s *store.Store, ctx context.Context) context.Context { return ctx },
	} {
		t.Run(name, func(t *testing.T) {
			guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = guest.Close() })
			tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: indexSink{text: &CaptureTextInfo{Used: true, Indexed: 5, Captures: 5}}})
			ctx := prepare(guest, toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "o", ComputerID: "c"}))
			out, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`))
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Coverage SearchCoverage `json:"coverage"`
			}
			if err := json.Unmarshal([]byte(out), &body); err != nil {
				t.Fatal(err)
			}
			if !body.Coverage.Titles || body.Coverage.Labels {
				t.Fatalf("coverage = %+v, want titles searched and labels not", body.Coverage)
			}
		})
	}
}

func TestCoverageZeroValueIsNotSearched(t *testing.T) {
	var c SearchCoverage
	if c.PageText || c.Titles || c.Labels || c.Window || c.Published || c.PageTextIndexed != 0 {
		t.Fatalf("zero coverage reads as searched: %+v", c)
	}
}

// A thread of stop words alone has no word to match titles on: nothing in the
// title half is searched, so titles and labels are not claimed.
func TestCoverageTitlesAreFalseWhenTheQueryHasNoSearchableWord(t *testing.T) {
	got := coverageOfQuery(t, indexSink{text: &CaptureTextInfo{Used: true, Indexed: 5, Captures: 5}}, "What is the latest?")
	want := SearchCoverage{PageText: true, PageTextIndexed: 5, NoTitleWords: true, Published: true}
	if got != want {
		t.Fatalf("coverage = %+v, want %+v", got, want)
	}
}
