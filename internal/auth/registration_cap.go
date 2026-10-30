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

const registrationWaitlistMessage = "Choir is full for now. You're on the waitlist, and we'll email you when there's room."

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
// user) must be refused. A user who already holds a passkey is never closed
// out. An unreadable count closes registration.
func (h *Handler) registrationClosed(userID string) (bool, error) {
	if h.config == nil || h.config.RegistrationCap <= 0 {
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
	n, err := h.store.CountRegisteredUsers()
	if err != nil {
		return true, fmt.Errorf("count registered users: %w", err)
	}
	return n >= h.config.RegistrationCap, nil
}

// refuseIfRegistrationClosed answers the request and returns true when the
// cap refuses this sign-up. email is waitlisted when non-empty.
func (h *Handler) refuseIfRegistrationClosed(w http.ResponseWriter, operation, emailHash, userID, email string) bool {
	closed, err := h.registrationClosed(userID)
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
