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
//  1. a result has no coverage object, or one that says "searched" for a half
//     that did not run (no capture store, no page-text answer, the window
//     fallback, a failed or capped published search);
//  2. a path nobody set reads as searched (the zero value must be "not");
//  3. the page text counts as whole while the index covers fewer pages than
//     the computer holds;
//  4. Full is true when any one of page text, labels or published is missing.

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
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: captures})
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "o", ComputerID: "c"})
	out, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`))
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

func TestCoverageIsFullOnlyWhenEveryHalfRan(t *testing.T) {
	whole := &CaptureTextInfo{Used: true, Indexed: 5, Captures: 5}
	cases := []struct {
		name string
		sink CaptureSink
		want SearchCoverage
		full bool
	}{
		{"no capture store", nil, SearchCoverage{}, false},
		{"index answered, page text did not", indexSink{}, SearchCoverage{Labels: true, Published: true}, false},
		{"window fallback", noSearchSink{}, SearchCoverage{Window: true}, false},
		{"title and label projection still building", indexSink{text: whole, termsOpen: true}, SearchCoverage{PageText: true, PageTextIndexed: 5, Pages: 5, Window: true, Published: true}, false},
		{"page text index catching up", indexSink{text: &CaptureTextInfo{Used: true, Indexed: 3, Captures: 5}}, SearchCoverage{PageText: true, PageTextIndexed: 3, Pages: 5, Labels: true, Published: true}, false},
		{"page text refused", indexSink{text: &CaptureTextInfo{Used: false, Reason: "index not ready"}}, SearchCoverage{Labels: true, Published: true}, false},
		{"published search failed", indexSink{text: whole, pubErr: errors.New("corpusd 503")}, SearchCoverage{PageText: true, PageTextIndexed: 5, Pages: 5, Labels: true}, false},
		{"published search capped", indexSink{text: whole, published: PublishedAnswer{Capped: true}}, SearchCoverage{PageText: true, PageTextIndexed: 5, Pages: 5, Labels: true}, false},
		{"every half ran", indexSink{text: whole}, SearchCoverage{PageText: true, PageTextIndexed: 5, Pages: 5, Labels: true, Published: true}, true},
		{"every half ran on an empty computer", indexSink{text: &CaptureTextInfo{Used: true}}, SearchCoverage{PageText: true, Labels: true, Published: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := coverageOf(t, tc.sink)
			if got != tc.want || got.Full() != tc.full {
				t.Fatalf("coverage = %+v (full %v), want %+v (full %v)", got, got.Full(), tc.want, tc.full)
			}
		})
	}
}

func TestCoverageZeroValueIsNotSearched(t *testing.T) {
	if (SearchCoverage{}).Full() {
		t.Fatal("a result nobody set coverage for reads as fully searched")
	}
	for _, c := range []SearchCoverage{{PageText: true, Labels: true}, {PageText: true, Published: true}, {Labels: true, Published: true}} {
		if c.Full() {
			t.Fatalf("%+v is full with a half missing", c)
		}
	}
}
