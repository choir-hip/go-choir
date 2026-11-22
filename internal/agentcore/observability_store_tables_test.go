package agentcore

import (
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/storecomposition"
	"github.com/yusefmosiah/go-choir/internal/storetables"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The store_tables section of guest observability: absent until asked for,
// read only with ?store_tables=detail (a request shape, not a switch), and
// shown as the last reading on a plain read afterwards.

func readStoreTables(t *testing.T, rt *Runtime, query string) (json.RawMessage, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability"+query, nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	raw, ok := obs["store_tables"]
	return raw, ok
}

func TestObservabilityReadsTheStoreTablesOnRequest(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-tables")

	if raw, ok := readStoreTables(t, rt, ""); ok {
		t.Fatalf("a plain read with no reading showed store_tables: %s", raw)
	}

	raw, ok := readStoreTables(t, rt, "?store_tables=detail&store_tables_seconds=30")
	if !ok {
		t.Fatal("?store_tables=detail returned no store_tables")
	}
	var rep struct {
		Reading struct {
			Truncated bool `json:"truncated"`
			Header    struct {
				PageCount int64 `json:"page_count"`
				PageSize  int64 `json:"page_size"`
			} `json:"header"`
			Trees []struct {
				Name     string `json:"name"`
				Complete bool   `json:"complete"`
			} `json:"trees"`
			Tables []struct {
				Table      string `json:"table"`
				IndexBytes int64  `json:"index_bytes"`
			} `json:"tables"`
		} `json:"reading"`
	}
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Reading.Header.PageCount == 0 || rep.Reading.Header.PageSize == 0 || rep.Reading.Truncated {
		t.Fatalf("reading = %s", raw)
	}
	var objects, runMemory bool
	for _, tt := range rep.Reading.Tables {
		objects = objects || tt.Table == "og_objects"
		runMemory = runMemory || tt.Table == "run_memory_entries"
	}
	if !objects || !runMemory {
		t.Fatalf("the guest's own tables are not in the reading (og_objects %v, run_memory_entries %v): %s", objects, runMemory, raw)
	}

	// A plain read now shows the last reading instead of reading again.
	if again, ok := readStoreTables(t, rt, ""); !ok || string(again) != string(raw) {
		t.Fatalf("a plain read after a reading does not show it: %s", again)
	}
}

// A huge store_tables_seconds must clamp to the cap, not wrap the duration.
func TestObservabilityStoreTablesSecondsCannotOverflow(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-tables-huge")
	for _, secs := range []string{"9223372036854775807", "9223372036", "99999999999"} {
		raw, ok := readStoreTables(t, rt, "?store_tables=detail&store_tables_seconds="+secs)
		if !ok {
			t.Fatalf("seconds=%s returned no store_tables", secs)
		}
		var rep struct {
			Reading struct {
				BudgetMillis int64 `json:"budget_ms"`
			} `json:"reading"`
		}
		if err := json.Unmarshal(raw, &rep); err != nil {
			t.Fatal(err)
		}
		want, _ := storeTablesBudget(storetables.MaxBudget*2, 0, server.WriteTimeoutFromEnv())
		if want != storetables.MaxBudget && want != 95*time.Second {
			t.Fatalf("unexpected ceiling %s", want)
		}
		if d := want.Milliseconds() - rep.Reading.BudgetMillis; d < 0 || d > 2000 { // handler time already spent comes off the budget
			t.Fatalf("seconds=%s gave a budget of %d ms, want the %s cap: %s", secs, rep.Reading.BudgetMillis, want, raw)
		}
	}
}

type storeTablesOnlyReading struct {
	Reading struct {
		Only    []string `json:"only"`
		Unknown []string `json:"unknown"`
		Dropped int      `json:"only_dropped"`
		Trees   []struct {
			Name     string `json:"name"`
			Complete bool   `json:"complete"`
		} `json:"trees"`
	} `json:"reading"`
}

// store_tables_only names the trees to read: parsed from a comma list, blanks
// dropped, unknown names reported, the list capped, and a restricted reading
// is not what a later plain read shows.
func TestObservabilityStoreTablesOnlyNamesTheTrees(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-tables-only")
	raw, ok := readStoreTables(t, rt, "?store_tables=detail&store_tables_only=og_objects,%20,no_such_tree,,og_objects")
	if !ok {
		t.Fatal("no store_tables")
	}
	var rep storeTablesOnlyReading
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Reading.Only) != 2 || rep.Reading.Only[0] != "og_objects" || rep.Reading.Only[1] != "no_such_tree" {
		t.Fatalf("only = %v: %s", rep.Reading.Only, raw)
	}
	if len(rep.Reading.Unknown) != 1 || rep.Reading.Unknown[0] != "no_such_tree" {
		t.Fatalf("unknown = %v", rep.Reading.Unknown)
	}
	if len(rep.Reading.Trees) != 1 || rep.Reading.Trees[0].Name != "og_objects" || !rep.Reading.Trees[0].Complete {
		t.Fatalf("trees = %+v", rep.Reading.Trees)
	}
	if again, ok := readStoreTables(t, rt, ""); ok {
		t.Fatalf("a plain read after only a restricted reading showed one: %s", again)
	}
}

func TestObservabilityStoreTablesOnlyIsCappedAndTolerant(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-tables-only-cap")
	names := ""
	for i := 0; i < 20; i++ {
		names += fmt.Sprintf("x%d,", i)
	}
	for _, q := range []string{names, ",,,", "%00", "a%2Cb"} {
		raw, ok := readStoreTables(t, rt, "?store_tables=detail&store_tables_seconds=-5&store_tables_only="+q)
		if !ok {
			t.Fatalf("only=%q returned no store_tables", q)
		}
		var rep storeTablesOnlyReading
		if err := json.Unmarshal(raw, &rep); err != nil {
			t.Fatal(err)
		}
		if len(rep.Reading.Only) > storetables.MaxOnly {
			t.Fatalf("only=%q kept %d names", q, len(rep.Reading.Only))
		}
		if q == names && (len(rep.Reading.Only) != 16 || rep.Reading.Dropped != 4) {
			t.Fatalf("cap: %d kept, %d dropped", len(rep.Reading.Only), rep.Reading.Dropped)
		}
	}
}

// One request must end under the server's write timeout: the store budget is
// what the handler's overall deadline leaves, and nothing is read when nothing
// is left. The handler's clock is injected.
func TestObservabilityStoreTablesBudgetFitsTheWriteTimeout(t *testing.T) {
	t.Setenv("SERVER_WRITE_TIMEOUT", "120s")
	reserve := storetables.HeaderBudget + toolLayerReadBudget + time.Second + 4*observabilityReadBudget + 5*time.Second
	ceiling := 120*time.Second - reserve
	run := func(elapsed time.Duration) (budgetMs int64, skipped string, hasReading bool) {
		start := time.Now()
		calls := 0
		old := observabilityNow
		observabilityNow = func() time.Time {
			calls++
			if calls == 1 {
				return start
			}
			return start.Add(elapsed)
		}
		defer func() { observabilityNow = old }()
		rt := telemetryRuntime(t, fmt.Sprintf("computer-store-tables-deadline-%d", elapsed))
		raw, ok := readStoreTables(t, rt, "?store_tables=detail&store_tables_seconds=100")
		if !ok {
			t.Fatal("no store_tables")
		}
		var rep struct {
			Skipped string `json:"skipped"`
			Reading *struct {
				BudgetMillis int64 `json:"budget_ms"`
			} `json:"reading"`
		}
		if err := json.Unmarshal(raw, &rep); err != nil {
			t.Fatal(err)
		}
		if rep.Reading != nil {
			budgetMs = rep.Reading.BudgetMillis
		}
		return budgetMs, rep.Skipped, rep.Reading != nil
	}
	if ms, _, _ := run(0); ms != min(ceiling, storetables.MaxBudget).Milliseconds() {
		t.Fatalf("no pre-read: budget %d ms, ceiling %s", ms, ceiling)
	}
	if ms, _, _ := run(ceiling - 10*time.Second); ms != 10000 {
		t.Fatalf("after a slow pre-read: budget %d ms, want 10000", ms)
	}
	if ms, skipped, has := run(ceiling + time.Second); has || ms != 0 || skipped == "" {
		t.Fatalf("with nothing left: reading %v budget %d skipped %q", has, ms, skipped)
	}
}

// The run_memory_retirement block of guest observability is always present:
// empty before the first retirement pass, the last pass's census after it.
func TestObservabilityServesTheRunMemoryRetirementCensus(t *testing.T) {
	rt := telemetryRuntime(t, "computer-run-memory-census")
	read := func() struct {
		Passes       int64      `json:"passes"`
		FinishedAt   *time.Time `json:"finished_at"`
		Dispositions []struct {
			Disposition string `json:"disposition"`
			Runs        int64  `json:"runs"`
		} `json:"dispositions"`
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs struct {
			Census *struct {
				Passes       int64      `json:"passes"`
				FinishedAt   *time.Time `json:"finished_at"`
				Dispositions []struct {
					Disposition string `json:"disposition"`
					Runs        int64  `json:"runs"`
				} `json:"dispositions"`
			} `json:"run_memory_retirement"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
		}
		if obs.Census == nil {
			t.Fatalf("no run_memory_retirement in %s", rec.Body.String())
		}
		return *obs.Census
	}
	if c := read(); c.Passes != 0 || c.FinishedAt != nil || len(c.Dispositions) != 0 {
		t.Fatalf("before a pass: %+v", c)
	}
	if _, err := rt.store.RetireRunMemory(t.Context(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	c := read()
	if c.Passes != 1 || c.FinishedAt == nil || len(c.Dispositions) == 0 {
		t.Fatalf("after a pass: %+v", c)
	}
}

// The store_composition section: absent on a plain read; ?store_composition=start
// begins a background reading and answers at once; ?store_composition=1 reads
// its progress and then its finished report.
func TestObservabilityStartsAndReadsTheStoreComposition(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-composition")
	read := func(query string) (state string, tables []string, ok bool) {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability"+query, nil)
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs struct {
			C *struct {
				State  string `json:"state"`
				Tables []struct {
					Table string `json:"table"`
				} `json:"tables"`
			} `json:"store_composition"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
		}
		if obs.C == nil {
			return "", nil, false
		}
		for _, tt := range obs.C.Tables {
			tables = append(tables, tt.Table)
		}
		return obs.C.State, tables, true
	}
	if _, _, ok := read(""); ok {
		t.Fatal("a plain read showed store_composition")
	}
	if _, _, ok := read("?store_composition=1"); ok {
		t.Fatal("a read with no reading showed one")
	}
	if state, _, ok := read("?store_composition=start"); !ok || (state != "running" && state != "done") {
		t.Fatalf("start answered state %q ok=%v", state, ok)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		state, tables, ok := read("?store_composition=1")
		if ok && state == "done" {
			have := map[string]bool{}
			for _, n := range tables {
				have[n] = true
			}
			if !have["og_objects"] || !have["run_memory_entries"] {
				t.Fatalf("the guest's own tables are not in the reading: %v", tables)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the reading did not finish: %q", state)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The route refuses a request with no authenticated user, and starts nothing.
func TestObservabilityStoreCompositionNeedsAnAuthenticatedUser(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-composition-auth")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability?store_composition=start", nil)
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no user: %d %s", rec.Code, rec.Body.String())
	}
	if rt.storeComposition != nil {
		t.Fatal("a refused request opened the scanner")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/runtime/observability?store_composition=start", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec = httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rt.storeComposition != nil {
		t.Fatalf("POST: %d", rec.Code)
	}
}

// Stop ends a running composition reading before the store closes, and a
// start after Stop begins nothing.
func TestRuntimeStopEndsTheStoreCompositionReading(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-composition-stop")
	rt.storeCompositionOnce.Do(func() {
		rt.storeComposition = storecomposition.Open("", storecomposition.Config{BatchRows: 1, MinYield: 200 * time.Millisecond})
	})
	if _, started := rt.storeComposition.Start(rt.store.ReadDB); !started {
		t.Fatal("reading did not start")
	}
	if st := rt.storeComposition.Status(); st == nil || st.State != "running" {
		t.Fatalf("status = %+v", st)
	}
	t0 := time.Now()
	rt.stopStoreComposition()
	if time.Since(t0) > 5*time.Second {
		t.Fatalf("stop took %v", time.Since(t0))
	}
	if st := rt.storeComposition.Status(); st == nil || st.State != "interrupted" || st.Reason != "cancelled" {
		t.Fatalf("after stop = %+v", st)
	}
	if _, started := rt.storeComposition.Start(rt.store.ReadDB); started {
		t.Fatal("a reading started after stop")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability?store_composition=start", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	if strings.Contains(rec.Body.String(), `"store_composition"`) {
		t.Fatalf("the route answered for a stopped runtime: %s", rec.Body.String())
	}
}

// A report a dead process left "running" is rewritten "interrupted" when the
// runtime is built, before anything asks for it.
func TestRuntimeBootRewritesARunningCompositionReport(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "runtime.db")
	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	file := filepath.Join(dir, "store-composition.json")
	running := `{"state":"running","started_at":"2026-11-22T00:00:00Z","progress":{"rows_walked":5,"batches":1,"tables_done":0,"tables_total":3},"store":{},"scope":"` + storecomposition.Scope + `","reading_note":"` + storecomposition.ReadingNote + `"}`
	if err := os.WriteFile(file, []byte(running), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := New(provideriface.Config{ComputerID: "computer-boot-rewrite", StorePath: dbPath}, s, events.NewEventBus(), nil)
	t.Cleanup(rt.Stop)
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), `"interrupted"`) || strings.Contains(string(data), `"running"`) {
		t.Fatalf("file after boot: %s %v", data, err)
	}
}
