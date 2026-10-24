package agentcore

import (
	"context"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/search"
)

// Search text in a desk's context (docs/reports/autopaper-compute-halving-
// 2026-10-15.md, lever 3). A cell that prints search hits puts about 2k
// tokens (the 8,000-character cell cap) into the run's history, and the
// history is resent on every later call: search output is 29% of a research
// run's input. Exa returns full page text as the snippet.
//
// Failure modes written first:
//   - F1. Trimming cuts a URL or a title: a citation then points nowhere.
//     Only the snippet is shortened.
//   - F2. A snippet is cut inside a multi-byte rune and the cell prints an
//     invalid string.
//   - F3. The model cannot tell a snippet was cut and quotes it as whole.
//     A cut snippet ends in a marker.
//   - F4. A short snippet is padded or altered.
//   - F5. The result cap is applied after the gateway marked the dropped
//     hits "seen" (search_seen), so they never appear again. The cap is the
//     request's max_results, which the gateway honours before it marks.
//   - F6. A bad flag value zeroes the cap (no results) or the snippet (no
//     text). Garbage falls back to the default; 0 turns the limit off.
//   - F7. The limit cannot be turned off if research quality drops.
//     Trimming is opt-in (owner ruling 2026-10-15):
//     CHOIR_DESK_SEARCH_MAX_RESULTS=12, CHOIR_DESK_SEARCH_SNIPPET_CHARS=300.

func TestTrimSearchSnippet(t *testing.T) {
	long := strings.Repeat("é", 900) + "tail"
	got := trimSearchSnippet(long, 500)
	if !utf8.ValidString(got) {
		t.Fatalf("trimmed snippet is not valid UTF-8")
	}
	if n := utf8.RuneCountInString(got); n > 500+utf8.RuneCountInString(searchSnippetCutMarker) {
		t.Fatalf("trimmed to %d runes", n)
	}
	if !strings.HasSuffix(got, searchSnippetCutMarker) || strings.Contains(got, "tail") {
		t.Fatalf("trimmed snippet %q lacks the cut marker or kept the tail", got[len(got)-20:])
	}
	if short := "a short snippet"; trimSearchSnippet(short, 500) != short {
		t.Fatalf("a short snippet changed")
	}
	if trimSearchSnippet(long, 0) != long {
		t.Fatalf("limit 0 must leave the snippet whole")
	}
	if got := trimSearchSnippet("  spaced  ", 500); got != "spaced" {
		t.Fatalf("snippet whitespace = %q", got)
	}
}

func TestDeskSearchLimitsFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name, results, snippet string
		wantResults, wantChars int
	}{
		{name: "defaults", wantResults: defaultDeskSearchMaxResults, wantChars: defaultDeskSearchSnippetChars},
		{name: "opt-in trimming", results: "12", snippet: "300", wantResults: 12, wantChars: 300},
		{name: "garbage falls back", results: "lots", snippet: "-3", wantResults: defaultDeskSearchMaxResults, wantChars: defaultDeskSearchSnippetChars},
		{name: "zero results falls back", results: "0", wantResults: defaultDeskSearchMaxResults, wantChars: defaultDeskSearchSnippetChars},
		{name: "owner ruling: untrimmed by default", wantResults: 40, wantChars: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHOIR_DESK_SEARCH_MAX_RESULTS", tc.results)
			t.Setenv("CHOIR_DESK_SEARCH_SNIPPET_CHARS", tc.snippet)
			if got := deskSearchMaxResults(false); got != tc.wantResults {
				t.Fatalf("max results = %d, want %d", got, tc.wantResults)
			}
			if got := deskSearchSnippetChars(false); got != tc.wantChars {
				t.Fatalf("snippet chars = %d, want %d", got, tc.wantChars)
			}
		})
	}
}

// recordingSearchClient answers at once with one long-snippet hit and
// records the max_results the host asked for.
type recordingSearchClient struct{ maxResults chan int }

func (c recordingSearchClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	c.maxResults <- maxResults
	body := `{"type":"start","query":"q","providers":["exa"],"cooling":[]}
{"type":"provider","provider":"exa","status":"success","latency_ms":1,"results":[{"title":"Full title kept","url":"https://example.test/a?x=1#frag","snippet":"` + strings.Repeat("word ", 400) + `"}]}
{"type":"done","answered":1,"total":1,"cooling":[]}
`
	return search.ReadStream(io.NopCloser(strings.NewReader(body)))
}

func TestDeskSearchAsksForFewerResultsAndTrimsOnlyTheSnippet(t *testing.T) {
	t.Setenv("CHOIR_DESK_SEARCH_MAX_RESULTS", "12")
	t.Setenv("CHOIR_DESK_SEARCH_SNIPPET_CHARS", "300")
	client := recordingSearchClient{maxResults: make(chan int, 1)}
	tool, ctx, _, _ := researchSearchTool(t, client)
	out := evalDeskCell(t, tool, ctx, `s, _ := choir.SearchStart("q")
choir.SearchWait(s.ID, 5000)
r, _ := choir.Searches.Get(s.ID)
for _, h := range r.Results {
	fmt.Println(h.Title, "|", h.URL, "|", len(h.Snippet), "|", strings.HasSuffix(h.Snippet, "…"))
}`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if got := <-client.maxResults; got != 12 {
		t.Fatalf("max_results = %d, want 12", got)
	}
	if !strings.HasPrefix(out.Stdout, "Full title kept | https://example.test/a?x=1#frag | ") || !strings.Contains(out.Stdout, "| true") {
		t.Fatalf("hit = %q", out.Stdout)
	}
}

// E3 end to end: the per-computer setting alone (no env) makes the desk ask
// for 12 results and cut snippets to 300.
func TestDeskSearchCapsSettingAsksForFewerResultsWithoutEnv(t *testing.T) {
	clearSwitchEnv(t)
	client := recordingSearchClient{maxResults: make(chan int, 1)}
	tool, ctx, _, _ := researchSearchToolWith(t, client, func(rt *Runtime) { rt.searchCapsSetting.Store(true) })
	out := evalDeskCell(t, tool, ctx, `s, _ := choir.SearchStart("q")
choir.SearchWait(s.ID, 5000)
r, _ := choir.Searches.Get(s.ID)
for _, h := range r.Results {
	fmt.Println(h.Title, "|", len(h.Snippet), "|", strings.HasSuffix(h.Snippet, "…"))
}`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if got := <-client.maxResults; got != 12 {
		t.Fatalf("max_results = %d, want 12", got)
	}
	if !strings.Contains(out.Stdout, "| true") {
		t.Fatalf("snippet not cut: %q", out.Stdout)
	}
}

func TestDeskSearchCapsSettingOffAsksForFortyWholeSnippets(t *testing.T) {
	clearSwitchEnv(t)
	client := recordingSearchClient{maxResults: make(chan int, 1)}
	tool, ctx, _, _ := researchSearchToolWith(t, client, nil)
	out := evalDeskCell(t, tool, ctx, `s, _ := choir.SearchStart("q")
choir.SearchWait(s.ID, 5000)
r, _ := choir.Searches.Get(s.ID)
for _, h := range r.Results {
	fmt.Println(h.Title, "|", len(h.Snippet), "|", strings.HasSuffix(h.Snippet, "…"))
}`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if got := <-client.maxResults; got != 40 {
		t.Fatalf("max_results = %d, want 40", got)
	}
	if !strings.Contains(out.Stdout, "| false") {
		t.Fatalf("snippet was cut with the setting off: %q", out.Stdout)
	}
}
