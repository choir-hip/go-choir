package auth

import (
	"crypto/subtle"
	"database/sql"
	"errors"
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
// The finish stores the passkey and counts in one transaction, so ceremonies
// begun below the cap cannot all land above it. The operator's own accounts
// (a secret the server checks) are not counted. A count that cannot be read refuses (fail closed). 0 is no cap.

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

// OperatorHeader carries the operator secret on a registration made by the
// operator's own tools (CI, drills). See Config.OperatorSecret.
const OperatorHeader = "X-Choir-Operator"

// ErrUserGone is a finish for an account row that no longer exists (swept or
// deleted while its ceremony was in flight): the person starts again.
var ErrUserGone = errors.New("sign-up no longer exists")

// ErrRegistrationClosed is the cap refusing a passkey at the moment it would
// be stored.
var ErrRegistrationClosed = errors.New("registration is closed")

// normalizeEmail is the one spelling of an address the register handlers
// store and look up: surrounding space removed and the domain lower-cased
// (the local part keeps its case, as sign-in has always matched it).
func normalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	// The first "@", the one SQLite's instr finds in Store.GetUserByEmail.
	if at := strings.Index(email, "@"); at >= 0 {
		return email[:at+1] + strings.ToLower(email[at+1:])
	}
	return email
}

// operatorRequest reports whether r carries the operator secret. With no
// secret configured nothing is an operator.
func (h *Handler) operatorRequest(r *http.Request) bool {
	secret := h.config.OperatorSecret
	if secret == "" {
		return false
	}
	got := r.Header.Get(OperatorHeader)
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(secret)) == 1
}

// CountRegisteredUsers counts the accounts holding at least one passkey that
// are not the operator's: each gets a computer, which is what binds.
func (s *Store) CountRegisteredUsers() (int, error) {
	return countRegistered(s.db)
}

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func countRegistered(q queryRower) (int, error) {
	var n int
	err := q.QueryRow(`SELECT COUNT(DISTINCT c.user_id) FROM credentials c JOIN users u ON u.id = c.user_id WHERE u.operator = 0`).Scan(&n)
	return n, err
}

// RefreshSignUp restarts the day an unfinished sign-up may stay: a person who
// begins again is in a ceremony, and the sweep must not take the row.
func (s *Store) RefreshSignUp(userID string) error {
	_, err := s.db.Exec(`UPDATE users SET created_at = ? WHERE id = ? AND NOT EXISTS (SELECT 1 FROM credentials WHERE user_id = ?)`, time.Now().UTC(), userID, userID)
	return err
}

// MarkOperatorAccount marks an account as the operator's own.
func (s *Store) MarkOperatorAccount(userID string) error {
	_, err := s.db.Exec(`UPDATE users SET operator = 1 WHERE id = ?`, userID)
	return err
}

// IsOperatorAccount reports whether the account is marked as the operator's.
func (s *Store) IsOperatorAccount(userID string) (bool, error) {
	var v int
	if err := s.db.QueryRow(`SELECT operator FROM users WHERE id = ?`, userID).Scan(&v); err != nil {
		return false, err
	}
	return v != 0, nil
}

// CreateCredentialWithinCap stores a passkey, refusing with
// ErrRegistrationClosed when the user is a new holder, not the operator's,
// and cap holders already exist (cap 0: no cap). The count and the insert
// are one transaction on the store's one connection, so concurrent finishes
// cannot overshoot.
func (s *Store) CreateCredentialWithinCap(c *Credential, cap int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if cap > 0 {
		var operator, held int
		if err := tx.QueryRow(`SELECT operator FROM users WHERE id = ?`, c.UserID).Scan(&operator); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserGone
			}
			return fmt.Errorf("read account: %w", err)
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM credentials WHERE user_id = ?`, c.UserID).Scan(&held); err != nil {
			return fmt.Errorf("read credentials: %w", err)
		}
		if operator == 0 && held == 0 {
			n, err := countRegistered(tx)
			if err != nil {
				return fmt.Errorf("count registered users: %w", err)
			}
			if n >= cap {
				return ErrRegistrationClosed
			}
		}
	}
	if cap <= 0 {
		var one int
		if err := tx.QueryRow(`SELECT 1 FROM users WHERE id = ?`, c.UserID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserGone
			}
			return fmt.Errorf("read account: %w", err)
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO credentials (id, user_id, public_key, attestation_type, transport, sign_count, aaguid, flags, name, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.UserID, c.PublicKey, c.AttestationType, c.Transport, c.SignCount, c.AAGUID, c.Flags, c.Name, c.CreatedAt,
	); err != nil {
		return fmt.Errorf("create credential %q: %w", c.ID, err)
	}
	return tx.Commit()
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
// user) must be refused. The operator's accounts and a user who already holds
// a passkey are never closed out. An unreadable count closes registration.
func (h *Handler) registrationClosed(userID string, operator bool) (bool, error) {
	if h.config == nil || h.config.RegistrationCap <= 0 || operator {
		return false, nil
	}
	if userID != "" {
		marked, err := h.store.IsOperatorAccount(userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return true, fmt.Errorf("read account: %w", err)
		}
		if marked {
			return false, nil
		}
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
func (h *Handler) refuseIfRegistrationClosed(w http.ResponseWriter, operation, emailHash, userID, email string, operator bool) bool {
	closed, err := h.registrationClosed(userID, operator)
	if !closed {
		return false
	}
	if err != nil {
		log.Printf("[auth] operation=%s email_hash=%s result=error step=registration_cap error=%q", operation, emailHash, err)
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "registration is unavailable right now; please try again later"})
		return true
	}
	h.writeWaitlisted(w, operation, emailHash, email)
	return true
}

// writeWaitlisted keeps the email on the list and says so, as it stands.
func (h *Handler) writeWaitlisted(w http.ResponseWriter, operation, emailHash, email string) {
	if email != "" {
		if err := h.store.AddRegistrationWaitlist(email); err != nil {
			log.Printf("[auth] operation=%s email_hash=%s result=error step=waitlist error=%q", operation, emailHash, err)
		}
	}
	log.Printf("[auth] operation=%s email_hash=%s result=waitlisted cap=%d", operation, emailHash, h.config.RegistrationCap)
	writeJSON(w, http.StatusForbidden, waitlistResponse{Error: registrationWaitlistMessage, Code: registrationWaitlistCode})
}

type waitlistResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}
