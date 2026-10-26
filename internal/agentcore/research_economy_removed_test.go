package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The research economy experiments are removed (owner ruling 2026-10-26,
// docs/problems/research-economy-switches-removed-2026-10-26.md): the
// research.run_memory_elision and research.search_caps settings, their env
// twins, the elision and the snippet trimming. Failure modes of the deletion,
// written before the code:
//   - R1. A removed key is still a known setting, so GET lists it or POST
//     stores it. POST answers the ordinary "unknown setting" 400.
//   - R2. A guest still exporting the old env twins changes behavior: the run
//     context must carry no elision stub and a desk search must ask for 40
//     results with whole snippets.
//   - R3. The run memory pressure estimate follows anything but the plain
//     context.

var removedResearchEconomyKeys = []string{"research.run_memory_elision", "research.search_caps"}

func TestRemovedResearchEconomyKeysAreUnknownSettings(t *testing.T) {
	rt, _ := testRuntime(t)
	for _, key := range removedResearchEconomyKeys {
		if _, known := store.KnownComputerSettings[key]; known {
			t.Fatalf("%s is still a known computer setting", key)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":%q,"value":true}`, key)))
		req.Header.Set("X-Authenticated-User", "owner-removed-economy")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown setting "+key) {
			t.Fatalf("POST %s = %d %s, want the ordinary unknown-setting 400", key, rec.Code, rec.Body)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/computer-settings", nil)
	req.Header.Set("X-Authenticated-User", "owner-removed-economy")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	for _, key := range removedResearchEconomyKeys {
		if strings.Contains(rec.Body.String(), key) {
			t.Fatalf("GET lists the removed key %s", key)
		}
	}
}

// R2/R3: CHOIR_RUN_MEMORY_ELISION=1 is inert; the context a research run sends
// is the plain context.
func TestRunMemoryContextIgnoresTheRemovedElisionEnv(t *testing.T) {
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "1")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_KEEP", "1")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_BATCH", "1")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_MIN_CHARS", "1")
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := &types.RunRecord{
		RunID: "run-removed-elision", AgentID: "research:doc", AgentProfile: agentprofile.Research,
		OwnerID: "owner-removed-economy", ComputerID: "autoputer-test", State: types.RunRunning,
		Prompt: "research", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, *rec); err != nil {
		t.Fatal(err)
	}
	add := func(role string, msg any) {
		raw, _ := json.Marshal(msg)
		if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: rec.RunID, OwnerID: rec.OwnerID, AgentID: rec.AgentID, Kind: types.RunMemoryEntryMessage, Role: role, Message: raw}); err != nil {
			t.Fatal(err)
		}
	}
	add("user", map[string]any{"role": "user", "content": []any{map[string]string{"type": "text", "text": "assignment"}}})
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("call_%d", i)
		add("assistant", map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": "desk_go_eval", "input": map[string]string{"source": "fmt.Println(1)"}},
		}})
		add("user", map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": fmt.Sprintf("result %03d ", i) + strings.Repeat("x", 3000)},
		}})
	}
	entries, err := s.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	manager := newRunMemoryManager(s, rec, provideriface.Config{}, nil)
	sent, err := manager.contextMessages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plain := buildRunMemoryContext(entries)
	if len(sent) != len(plain) {
		t.Fatalf("sent %d messages, plain %d", len(sent), len(plain))
	}
	for i := range sent {
		if string(sent[i]) != string(plain[i]) {
			t.Fatalf("message %d differs from the plain context with the removed env set", i)
		}
		if strings.Contains(string(sent[i]), "Earlier tool output elided") {
			t.Fatalf("message %d carries an elision stub", i)
		}
	}
}

// R2: the removed search env knobs are inert too.
func TestDeskSearchIgnoresTheRemovedCapsEnv(t *testing.T) {
	t.Setenv("CHOIR_DESK_SEARCH_MAX_RESULTS", "12")
	t.Setenv("CHOIR_DESK_SEARCH_SNIPPET_CHARS", "300")
	client := economySearchClient{maxResults: make(chan int, 1)}
	tool, ctx, _, _ := researchSearchTool(t, client)
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
	if !strings.Contains(out.Stdout, "| false") || strings.Contains(out.Stdout, "| 301 ") {
		t.Fatalf("snippet was cut: %q", out.Stdout)
	}
}

// economySearchClient answers at once with one long-snippet hit and records
// the max_results the host asked for.
type economySearchClient struct{ maxResults chan int }

func (c economySearchClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	c.maxResults <- maxResults
	body := `{"type":"start","query":"q","providers":["exa"],"cooling":[]}
{"type":"provider","provider":"exa","status":"success","latency_ms":1,"results":[{"title":"Full title kept","url":"https://example.test/a","snippet":"` + strings.Repeat("word ", 400) + `"}]}
{"type":"done","answered":1,"total":1,"cooling":[]}
`
	return search.ReadStream(io.NopCloser(strings.NewReader(body)))
}
