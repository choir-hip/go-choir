package autoputer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The derived-column operation's endpoint
// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md).
// Ways it can fail, written before the code:
//
//  1. A caller other than the host peer (loopback: the guest's own
//     children) reads or starts it.
//  2. A GET, or an unknown action, starts a migration.
//  3. A second migrate while one runs is accepted instead of refused.
//  4. Status is not machine-readable for the operator's poll.

func derivedColumnsRequest(method, rawURL, remote string) *http.Request {
	req := httptest.NewRequest(method, rawURL, nil)
	req.RemoteAddr = remote
	return req
}

func TestDerivedColumnsEndpoint(t *testing.T) {
	db, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := derivedColumnsHandler(db)
	const host = "192.0.2.44:40000"

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		handler(rec, derivedColumnsRequest(method, "/internal/store/derived-columns?action=migrate", "127.0.0.1:40000"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("loopback %s = %d, want 403", method, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodGet, "/internal/store/derived-columns?action=migrate", host))
	var status choirstore.DerivedColumnMigrationStatus
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &status) != nil || status.State != choirstore.DerivedColumnMigrationIdle {
		t.Fatalf("GET = %d %s, want 200 idle (a GET never starts it)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodPost, "/internal/store/derived-columns?action=rebuild", host))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown action = %d, want 400", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodDelete, "/internal/store/derived-columns", host))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE = %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodPost, "/internal/store/derived-columns?action=migrate", host))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("migrate = %d %s, want 202", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		rec = httptest.NewRecorder()
		handler(rec, derivedColumnsRequest(http.MethodGet, "/internal/store/derived-columns", host))
		status = choirstore.DerivedColumnMigrationStatus{}
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("status body %q: %v", rec.Body.String(), err)
		}
		if status.State != choirstore.DerivedColumnMigrationRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("migration did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.State != choirstore.DerivedColumnMigrationDone || len(status.Steps) != 0 {
		t.Fatalf("fresh store migration status %+v, want done with no steps", status)
	}

	rec = httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodPost, "/internal/store/derived-columns?action=cancel", host))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel = %d %s", rec.Code, rec.Body.String())
	}
}

// The restore action (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md,
// "The index diet", rollback): host peer only, POST only, idempotent, and
// refused for an unknown index name. Ways it can fail, written before the code:
//
//  1. A loopback caller (the guest's own children) rebuilds indexes.
//  2. A GET starts a restore.
//  3. An unknown index name is accepted.
//  4. The report is not machine-readable, or a second call reports a build.
func TestDerivedColumnsRestoreEndpoint(t *testing.T) {
	db, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := derivedColumnsHandler(db)
	const host = "192.0.2.44:40000"
	const url = "/internal/store/derived-columns?action=restore_run_memory_indexes"

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		handler(rec, derivedColumnsRequest(method, url, "127.0.0.1:40000"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("loopback %s restore = %d, want 403", method, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodGet, url, host))
	var status choirstore.DerivedColumnMigrationStatus
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &status) != nil || status.State != choirstore.DerivedColumnMigrationIdle {
		t.Fatalf("GET restore = %d %s, want the idle status (a GET never restores)", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler(rec, derivedColumnsRequest(http.MethodPost, url+"&indexes=idx_nope", host))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown index = %d %s, want 400", rec.Code, rec.Body.String())
	}

	// A fresh store has none of the three (the diet's state): the first restore
	// builds them in order, the second finds them.
	type report struct {
		Steps []struct {
			Name   string `json:"name"`
			Action string `json:"action"`
			HoldMS int64  `json:"hold_ms"`
		} `json:"steps"`
		TotalHoldMS int64 `json:"total_hold_ms"`
	}
	post := func() report {
		rec := httptest.NewRecorder()
		handler(rec, derivedColumnsRequest(http.MethodPost, url, host))
		if rec.Code != http.StatusOK {
			t.Fatalf("restore = %d %s, want 200", rec.Code, rec.Body.String())
		}
		var r report
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatalf("report body %q: %v", rec.Body.String(), err)
		}
		return r
	}
	first := post()
	want := []string{"idx_run_memory_entries_owner_agent_created", "idx_run_memory_entries_owner_loop_seq", "idx_run_memory_entries_loop_seq"}
	if len(first.Steps) != 3 {
		t.Fatalf("first restore steps = %+v", first.Steps)
	}
	for i, st := range first.Steps {
		if st.Name != want[i] || st.Action != "created" {
			t.Fatalf("first restore step %d = %+v, want %s created", i, st, want[i])
		}
	}
	for _, st := range post().Steps {
		if st.Action != "present" {
			t.Fatalf("second restore step %+v, want present", st)
		}
	}
}
