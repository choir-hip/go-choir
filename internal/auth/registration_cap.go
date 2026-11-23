package auth

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// A first cap on sign-ups, with a waitlist (L4-03; P7 of
// docs/archive/definitions/choir-launch-fp8-image-jailer-2026-10-21.md).
//
// Every registered user gets a computer on Node B at first use, and the only
// limit today is host memory admission, which refuses a start after the
// person has already signed up. AUTH_REGISTRATION_CAP bounds the users who
// hold a passkey (abandoned ceremonies do not count). At the cap a new
// sign-up, or an incomplete one resuming, is refused before any user row is
// written, and the email goes on the waitlist for the owner to invite later.
// The finish re-checks, so ceremonies begun below the cap cannot all land
// above it. A count that cannot be read refuses (fail closed). 0 is no cap.

const registrationWaitlistCode = "registration_waitlist"

// registrationWaitlistMessage is shown to the person as it stands, by the
// site and the phone app (owner, 2026-11-23). It promises only what is true:
// the email is kept on the list; nobody is told a mail will come.
const registrationWaitlistMessage = "Sign-up is full for now. You are on the list."

// WaitlistEntry is one waitlisted email.
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

// CountRegisteredUsersSplit counts users holding a passkey, apart by whether
// their address is on one of testDomains (case-insensitive, lower-case
// domains): people, then test addresses.
func (s *Store) CountRegisteredUsersSplit(testDomains []string) (people, test int, err error) {
	total, err := s.CountRegisteredUsers()
	if err != nil {
		return 0, 0, err
	}
	if len(testDomains) == 0 {
		return total, 0, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(testDomains)), ",")
	args := make([]any, len(testDomains))
	for i, d := range testDomains {
		args[i] = d
	}
	err = s.db.QueryRow(`SELECT COUNT(DISTINCT c.user_id) FROM credentials c JOIN users u ON u.id = c.user_id
		WHERE lower(substr(u.email, instr(u.email, '@') + 1)) IN (`+marks+`)`, args...).Scan(&test)
	if err != nil {
		return 0, 0, err
	}
	return total - test, test, nil
}

// isTestAddress reports whether email is on one of the configured test
// domains. With none configured every address is a person's.
func isTestAddress(email string, testDomains []string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	for _, d := range testDomains {
		if domain == d {
			return true
		}
	}
	return false
}

// AddRegistrationWaitlist records a refused sign-up; a repeat attempt bumps
// the count.
func (s *Store) AddRegistrationWaitlist(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO registration_waitlist (email, created_at, last_seen_at, attempts) VALUES (?, ?, ?, 1)
		ON CONFLICT(email) DO UPDATE SET last_seen_at = excluded.last_seen_at, attempts = attempts + 1`, email, now, now)
	return err
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

// registrationClosed reports whether a sign-up for userID (empty for a new
// user) with address email must be refused. A user who already holds a
// passkey is never closed out. An unreadable count closes registration. A
// test address is bounded by its own cap (RegistrationTestCap), so CI and
// drills neither use the people's room nor bypass it.
func (h *Handler) registrationClosed(userID, email string) (bool, error) {
	if h.config == nil || (h.config.RegistrationCap <= 0 && h.config.RegistrationTestCap <= 0) {
		return false, nil
	}
	if userID != "" {
		creds, err := h.store.GetCredentialsByUserID(userID)
		if err != nil {
			return true, fmt.Errorf("read credentials: %w", err)
		}
		if len(creds) > 0 {
			return false, nil
		}
	}
	people, test, err := h.store.CountRegisteredUsersSplit(h.config.RegistrationTestDomains)
	if err != nil {
		return true, fmt.Errorf("count registered users: %w", err)
	}
	if isTestAddress(email, h.config.RegistrationTestDomains) {
		return h.config.RegistrationTestCap > 0 && test >= h.config.RegistrationTestCap, nil
	}
	return h.config.RegistrationCap > 0 && people >= h.config.RegistrationCap, nil
}

// refuseIfRegistrationClosed answers the request and returns true when the
// cap refuses this sign-up. email is waitlisted when non-empty.
func (h *Handler) refuseIfRegistrationClosed(w http.ResponseWriter, operation, emailHash, userID, email string) bool {
	closed, err := h.registrationClosed(userID, email)
	if !closed {
		return false
	}
	if err != nil {
		log.Printf("[auth] operation=%s email_hash=%s result=error step=registration_cap error=%q", operation, emailHash, err)
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "registration is unavailable right now; please try again later"})
		return true
	}
	if email != "" {
		if err := h.store.AddRegistrationWaitlist(email); err != nil {
			log.Printf("[auth] operation=%s email_hash=%s result=error step=waitlist error=%q", operation, emailHash, err)
		}
	}
	log.Printf("[auth] operation=%s email_hash=%s result=waitlisted cap=%d", operation, emailHash, h.config.RegistrationCap)
	writeJSON(w, http.StatusForbidden, waitlistResponse{Error: registrationWaitlistMessage, Code: registrationWaitlistCode})
	return true
}

type waitlistResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}
