package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// Sign-up closes when the host cannot hold one more computer (host_room.go).
// Every person who makes an account gets a computer, so the limit is the
// host's disk, never a count of accounts. An account that has a passkey is
// never closed out of signing in; recovery adds a passkey to an account and
// is not a sign-up. The decision at the mail and at the begin saves the
// person a ceremony; the decision that counts is made inside the transaction
// that makes the account, at the finish (Store.CreateAccountWithCredential).
//
// A refused sign-up leaves its address on a waitlist for the owner: the
// normalised address, the first and last time seen, how many times, and a
// SHA-256 hash of the network source address that asked (unkeyed, so an IPv4
// address could be recovered by guessing; kept only to bound what one source
// can add). Nothing else; it is deleted after waitlistTTL.

// ErrRegistrationClosed is the host having no room for another computer.
var ErrRegistrationClosed = errors.New("registration is closed")

const registrationWaitlistCode = "registration_waitlist"

const registrationWaitlistMessage = "Choir is full for now. You're on the waitlist, and we'll email you when there's room."

// registrationWaitlistFullMessage is said when the list could not keep the
// person (the source or the list is at its bound): they are not on it.
const registrationWaitlistFullMessage = "Choir is full for now, and the waitlist is full too. Please try again later."

const (
	// waitlistPerSourceMax is how many addresses one source may keep on the
	// list; waitlistTotalMax bounds the table; waitlistTTL is how long an
	// address stays. A stranger can add rows at the sign-up rate limit and
	// no faster than these allow.
	waitlistPerSourceMax = 5
	waitlistTotalMax     = 1000
	waitlistTTL          = 30 * 24 * time.Hour
)

// WaitlistEntry is one waitlisted address.
type WaitlistEntry struct {
	Email      string
	CreatedAt  time.Time
	LastSeenAt time.Time
	Attempts   int
}

// CountRegisteredUsers counts users holding at least one passkey.
func (s *Store) CountRegisteredUsers() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM credentials").Scan(&n)
	return n, err
}

// AddRegistrationWaitlist records a refused sign-up; a repeat attempt of an
// address already listed bumps its count. A new address is stored only while
// its source and the table are under their bounds. Rows past waitlistTTL are
// deleted first. The key is the normalised address (normalizeEmailAddress,
// the one the address proof uses).
//
// It reports whether the address is on the list afterwards: false when the
// source or the table is at its bound and the address was not kept.
func (s *Store) AddRegistrationWaitlist(email, sourceHash string, now time.Time) (bool, error) {
	email = normalizeEmailAddress(email)
	now = now.UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM registration_waitlist WHERE last_seen_at < ?`, now.Add(-waitlistTTL)); err != nil {
		return false, err
	}
	res, err := tx.Exec(`UPDATE registration_waitlist SET last_seen_at = ?, attempts = attempts + 1 WHERE email = ?`, now, email)
	if err != nil {
		return false, err
	}
	recorded := true
	if n, _ := res.RowsAffected(); n == 0 {
		var fromSource, total int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM registration_waitlist WHERE source_hash = ?`, sourceHash).Scan(&fromSource); err != nil {
			return false, err
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM registration_waitlist`).Scan(&total); err != nil {
			return false, err
		}
		if fromSource >= waitlistPerSourceMax || total >= waitlistTotalMax {
			recorded = false
		} else if _, err := tx.Exec(`INSERT INTO registration_waitlist (email, created_at, last_seen_at, attempts, source_hash) VALUES (?, ?, ?, 1, ?)`, email, now, now, sourceHash); err != nil {
			return false, err
		}
	}
	return recorded, tx.Commit()
}

// SweepRegistrationWaitlist deletes the addresses last seen more than
// waitlistTTL before now, whether or not anyone is being refused now.
func (s *Store) SweepRegistrationWaitlist(now time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM registration_waitlist WHERE last_seen_at < ?`, now.UTC().Add(-waitlistTTL))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// waitlistSource is the unit a stranger's waitlist allowance is counted in: a
// SHA-256 hash of an IPv4 address, or of the /64 of an IPv6 one (appSource's
// rule), so rotating the interface id inside one allocation is one source.
func waitlistSource(addr string) string {
	return hashIP(appSource(addr))
}

// ListRegistrationWaitlist returns the waitlist, oldest first.
func (s *Store) ListRegistrationWaitlist() ([]WaitlistEntry, error) {
	rows, err := s.db.Query("SELECT email, created_at, last_seen_at, attempts FROM registration_waitlist ORDER BY created_at, email")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WaitlistEntry
	for rows.Next() {
		var e WaitlistEntry
		if err := rows.Scan(&e.Email, &e.CreatedAt, &e.LastSeenAt, &e.Attempts); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// refuseIfRegistrationClosed answers the request and returns true when the
// host has no room for another computer, or the reading that says so is
// missing. It is the early check; the finish decides in its transaction.
func (h *Handler) refuseIfRegistrationClosed(w http.ResponseWriter, r *http.Request, operation, emailHash, email string) bool {
	return h.refuseRoomError(w, r, operation, emailHash, email, h.room.admit(h.store.db))
}

// refuseRoomError answers a room decision: no error is no refusal.
func (h *Handler) refuseRoomError(w http.ResponseWriter, r *http.Request, operation, emailHash, email string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrRegistrationClosed):
		message := registrationWaitlistMessage
		if email != "" {
			recorded, werr := h.store.AddRegistrationWaitlist(email, waitlistSource(clientIP(r)), time.Now().UTC())
			if werr != nil {
				log.Printf("[auth] operation=%s email_hash=%s result=error step=waitlist error=%q", operation, emailHash, werr)
			}
			if !recorded {
				message = registrationWaitlistFullMessage
			}
		}
		log.Printf("[auth] operation=%s email_hash=%s result=waitlisted", operation, emailHash)
		writeJSON(w, http.StatusForbidden, waitlistResponse{Error: message, Code: registrationWaitlistCode})
	default:
		log.Printf("[auth] operation=%s email_hash=%s result=error step=room error=%q", operation, emailHash, strings.TrimSpace(err.Error()))
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "registration is unavailable right now; please try again later"})
	}
	return true
}

type waitlistResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}
