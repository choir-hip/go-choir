package agentcore

import (
	"encoding/json"
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
