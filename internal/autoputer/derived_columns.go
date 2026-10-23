package autoputer

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/server"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// derivedColumnsHandler is the explicit, per-computer operation that adds
// og_objects' missing derived columns and indexes
// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md).
// Boot never adds them to a table with rows; the operator does, from the
// host, at a time they choose:
//
//	GET  /internal/store/derived-columns                 status and plan
//	POST /internal/store/derived-columns?action=migrate  start (202; 409 while one runs)
//	POST /internal/store/derived-columns?action=cancel   stop before the next step
//
// After the og_objects steps it also builds the run_memory_entries kind index
// (internal/store/run_memory_kind_index.go) when it is missing, reported as
// run_memory_kind_index in the status. Each step holds the engine for one ALTER or CREATE INDEX, so the
// computer's store work waits during a step and runs between steps; the
// listener and health stay up. Only the host peer may call it: the guest's
// own children (loopback) are refused.
func derivedColumnsHandler(db *choirstore.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !server.HostPeerCaller(r) {
			http.Error(w, "host peer required", http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeDerivedColumnsJSON(w, http.StatusOK, db.DerivedColumnMigrationStatus(r.Context()))
		case http.MethodPost:
			switch r.URL.Query().Get("action") {
			case "migrate":
				status, err := db.StartDerivedColumnMigration()
				switch {
				case errors.Is(err, choirstore.ErrDerivedColumnMigrationRunning):
					writeDerivedColumnsJSON(w, http.StatusConflict, status)
				case err != nil:
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
				default:
					writeDerivedColumnsJSON(w, http.StatusAccepted, status)
				}
			case "cancel":
				cancelled := db.CancelDerivedColumnMigration()
				writeDerivedColumnsJSON(w, http.StatusOK, map[string]any{
					"cancelled": cancelled,
					"status":    db.DerivedColumnMigrationStatus(r.Context()),
				})
			default:
				http.Error(w, "action must be migrate or cancel", http.StatusBadRequest)
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func writeDerivedColumnsJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
