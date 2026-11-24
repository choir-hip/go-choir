// Package purgeguard is the one rule every destructive step of an account
// deletion checks before it removes anything: the account has a deletion
// record, and the record is at least GracePeriod old.
//
// The auth service owns the record (table account_deletions in auth.db) and
// the runner that executes a purge. The same rule is read by the services
// that hold the data (vmctl reads the table read-only; corpusd and maild
// receive the record's times in the request and apply CheckClaim), so a
// destructive call that names no old enough record is refused where the data
// lives, not only where the call is made. The package imports nothing from
// the repo so every service can take it.
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
	err := db.QueryRow(`SELECT requested_at FROM `+Table+` WHERE user_id = ?`, userID).Scan(&requested)
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

// OpenReadOnly opens auth.db at path for reading only.
func OpenReadOnly(path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("purgeguard: no auth db path configured")
	}
	return sql.Open("sqlite", "file:"+path+"?mode=ro")
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
