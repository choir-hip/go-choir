package agentcore

import (
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/storetables"
	"net/http"
	"net/http/httptest"
	"testing"
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
		if rep.Reading.BudgetMillis != 100000 {
			t.Fatalf("seconds=%s gave a budget of %d ms, want the 100000 ms cap: %s", secs, rep.Reading.BudgetMillis, raw)
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
