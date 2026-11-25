// Package purgeguard is the one rule every destructive step of an account
// deletion checks before it removes anything: the account has a deletion
// record, and the record is at least GracePeriod old.
//
// The auth service owns the record (table account_deletions in auth.db) and
// the runner that executes a purge. The same rule is applied where the data
// lives, in two strengths. vmctl and the retention sweeper read the table
// itself (read-only), so they refuse whatever a caller says. corpusd, maild
// and the gateway cannot open auth.db: they are handed the record's request
// time by the host-internal caller and apply CheckClaim to it, so for them the
// boundary is the host-only route and a caller that names an old enough time;
// a caller on the host that lies about the time is not caught there (a signed,
// single-use capability would close it). The package imports nothing from the
// repo so every service can take it.
package purgeguard

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// GracePeriod is how long an account's deletion waits, access already
// revoked, before the irreversible purge (owner, 2026-11-23: "the
// irreversible purge waits 72 hours and signing in during that time cancels
// it").
const GracePeriod = 72 * time.Hour

// Table is the auth.db table holding one row per pending deletion.
const Table = "account_deletions"

// ErrNoRecord means the account has no deletion record.
var ErrNoRecord = errors.New("purgeguard: the account has no deletion record")

// ErrTooEarly means the record is younger than GracePeriod.
var ErrTooEarly = errors.New("purgeguard: the deletion record is younger than the grace period")

// CheckClaim is the arithmetic of the rule for a caller that was handed the
// record's request time. A zero or future time is refused.
func CheckClaim(requestedAt, now time.Time) error {
	if requestedAt.IsZero() {
		return ErrNoRecord
	}
	if now.Sub(requestedAt) < GracePeriod {
		return fmt.Errorf("%w: requested %s, eligible %s", ErrTooEarly,
			requestedAt.UTC().Format(time.RFC3339), requestedAt.Add(GracePeriod).UTC().Format(time.RFC3339))
	}
	return nil
}

// Eligible reads the record of userID from auth.db (opened read-only or
// read-write) and applies CheckClaim to it.
func Eligible(db *sql.DB, userID string, now time.Time) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrNoRecord
	}
	var requested time.Time
	// A cancelled record (the tombstone a cancel leaves until the takes are
	// served again) is no record: it never makes a purge eligible.
	err := db.QueryRow(`SELECT requested_at FROM `+Table+` WHERE user_id = ? AND state IN ('pending', 'purging')`, userID).Scan(&requested)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoRecord
	}
	if err != nil {
		// A table that does not exist is "no record"; any other failure
		// refuses too (the rule fails closed).
		if strings.Contains(err.Error(), "no such table") {
			return ErrNoRecord
		}
		return fmt.Errorf("purgeguard: read deletion record: %w", err)
	}
	return CheckClaim(requested, now)
}

// PendingUsers is the set of accounts that have a live deletion record
// (pending or purging). Another destroyer of an account's data, such as the
// retention sweeper, asks it and keeps what it names: nothing destroys a
// pending account's data before the purge. A table that does not exist is an
// empty set; any other failure is returned and the caller fails closed.
func PendingUsers(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT user_id FROM ` + Table + ` WHERE state IN ('pending', 'purging')`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("purgeguard: read deletion records: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(id)] = true
	}
	return out, rows.Err()
}

// OpenReadOnly opens auth.db at path for reading only.
func OpenReadOnly(path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("purgeguard: no auth db path configured")
	}
	// A busy timeout: the auth service writes the same file, and a reader that
	// meets a writer waits a moment instead of failing the destroy it guards.
	return sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
}

// Protected computer classes: a computer in one of them is never deleted by
// an account deletion (the paper, the owner's computer, public platform).
const (
	ClassCriticalProtected = "critical_protected"
	ClassPublicPlatform    = "public_platform"
)

// ClassRefused reports whether a computer of this warmness class is outside
// every account deletion.
func ClassRefused(class string) bool {
	switch strings.TrimSpace(class) {
	case ClassCriticalProtected, ClassPublicPlatform:
		return true
	}
	return false
}

// ProtectedComputers is the hard refusal list: the paper's computer and the
// owner's. No account deletion touches either, whatever its class says and
// whoever asks. The tape sweep (internal/platform/tape_purge.go) reads this
// same map, so one list is refused in vmctl, corpusd and the sweep.
var ProtectedComputers = map[string]string{
	"computer-1355b83bced1613eb048ee28d66688a8": "the paper computer",
	"computer-03335285269bdba4f94377e56879f9e6": "the owner computer",
}

// ProtectedComputer reports whether the computer id is on the hard list.
func ProtectedComputer(computerID string) (string, bool) {
	what, ok := ProtectedComputers[strings.TrimSpace(computerID)]
	return what, ok
}
