package platform

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// A serialization conflict that outlives the host's retries is a retryable
// 503 with no database text, never the 400 a refusal gets (review of the
// cutover, 2026-10-30: a 40001 reached the guest as 400 with the raw
// PostgreSQL message). Ways this could fail:
//  1. a wrapped 40001 or 40P01 maps to 400;
//  2. the response carries the driver's message or the SQLSTATE;
//  3. the CAS conflict and the admission refusal lose their 409 and 403;
//  4. a plain refusal stops being a 400.
func TestComputerEventAppendErrorStatus(t *testing.T) {
	pg := &pgconn.PgError{Code: "40001", Message: "could not serialize access due to read/write dependencies among transactions"}
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"serialization", fmt.Errorf("computer event CAS: commit: %w", pg), http.StatusServiceUnavailable},
		{"deadlock", fmt.Errorf("computer event CAS: update head: %w", &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}), http.StatusServiceUnavailable},
		{"cas conflict", ErrComputerEventCASConflict, http.StatusConflict},
		{"admission", fmt.Errorf("x: %w", ErrEffectAdmissionRefused), http.StatusForbidden},
		{"refusal", errors.New("computer event CAS: guest projection mismatch"), http.StatusBadRequest},
	}
	for _, tc := range cases {
		status, message := computerEventAppendError(tc.err)
		if status != tc.status {
			t.Errorf("%s: status %d, want %d", tc.name, status, tc.status)
		}
		if status == http.StatusServiceUnavailable && (strings.Contains(message, "serialize") || strings.Contains(message, "deadlock") || strings.Contains(message, "40")) {
			t.Errorf("%s: message %q carries database text", tc.name, message)
		}
	}
}
