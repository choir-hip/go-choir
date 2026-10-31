package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Numbered-address ranges (owner, 2026-10-31). The boundaries live here and
// nowhere else.
//
//	user  1 .. 9,999,999, then 110,000,000 upward with no ceiling
//	      (000 is the owner's, seeded; the user sequence never issues a number
//	      in [TestNumberStart, UserNumberBlockTwoStart))
//	test  [10,000,000, 100,000,000), refused at the ceiling, never wrapped
//	      and never spilled
const (
	UserNumberStart         int64 = 1
	UserNumberBlockOneLast  int64 = 9_999_999
	TestNumberStart         int64 = 10_000_000
	TestNumberEnd           int64 = 100_000_000 // exclusive
	UserNumberBlockTwoStart int64 = 110_000_000
	// MaxNumberDigits caps the digit length of a numeric local-part base
	// before it is parsed.
	MaxNumberDigits = 12
	// NumberReservationTTL is how long a reserved test number waits for its bind.
	NumberReservationTTL = 24 * time.Hour

	numberRangeUser = "user"
	numberRangeTest = "test"
)

var (
	ErrTestRangeExhausted = errors.New("test number range exhausted (ceiling 100,000,000); refusing, never wrapping or spilling")
	ErrNumberNotReserved  = errors.New("address is not a reserved test number")
	ErrNumberExpired      = errors.New("reserved test number expired unbound")
	ErrNumberBound        = errors.New("number is already bound to a different account")
)

// NumberReservation is a reserved, not yet bound test number.
type NumberReservation struct {
	Address   string    `json:"address"`
	LocalPart string    `json:"local_part"`
	ExpiresAt time.Time `json:"expires_at"`
}

// numberedLocalPart classifies a local part against the numbered scheme.
type numberedLocalPart struct {
	Numeric   bool // the base (before "+") is nonempty and all digits
	Canonical bool // base equals the canonical form of its own value (within the digit cap)
	Number    int64
	Base      string
	HasPlus   bool
}

func canonicalNumberString(n int64) string { return fmt.Sprintf("%03d", n) }

func parseNumberedLocalPart(localPart string) numberedLocalPart {
	lp := strings.TrimSpace(localPart)
	base, _, hasPlus := strings.Cut(lp, "+")
	out := numberedLocalPart{Base: base, HasPlus: hasPlus}
	if base == "" {
		return out
	}
	for _, r := range base {
		if r < '0' || r > '9' {
			return out
		}
	}
	out.Numeric = true
	if len(base) > MaxNumberDigits {
		return out
	}
	var n int64
	for _, r := range base {
		n = n*10 + int64(r-'0')
	}
	out.Number = n
	out.Canonical = base == canonicalNumberString(n)
	return out
}

// canonicalNumberFromLocalPart returns the number of a bare canonical numeric
// local part. ok is false for non-numeric, plus-tagged, non-canonical and
// over-long forms, so 000 (0, true) is distinct from "abc" (0, false).
func canonicalNumberFromLocalPart(localPart string) (int, bool) {
	p := parseNumberedLocalPart(localPart)
	if !p.Numeric || p.HasPlus || !p.Canonical {
		return 0, false
	}
	return int(p.Number), true
}

// allocateNumber takes the next number from a range's sequence in one atomic
// statement. Never max+1; numbers are never reused.
func allocateNumber(ctx context.Context, tx *sql.Tx, rangeName string) (int64, error) {
	var next int64
	var err error
	switch rangeName {
	case numberRangeTest:
		err = tx.QueryRowContext(ctx, `UPDATE email_number_sequence SET next = next + 1
			WHERE range_name = 'test' AND next >= ? AND next < ? RETURNING next - 1`,
			TestNumberStart, TestNumberEnd).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTestRangeExhausted
		}
	case numberRangeUser:
		// A next inside [10M, 110M) jumps to 110M; the issued number is
		// always new-1.
		err = tx.QueryRowContext(ctx, `UPDATE email_number_sequence SET next =
			CASE WHEN next >= ? AND next < ? THEN ? ELSE next + 1 END
			WHERE range_name = 'user' RETURNING next - 1`,
			TestNumberStart, UserNumberBlockTwoStart, UserNumberBlockTwoStart+1).Scan(&next)
	default:
		return 0, fmt.Errorf("unknown number range %q", rangeName)
	}
	if err != nil {
		return 0, fmt.Errorf("allocate %s number: %w", rangeName, err)
	}
	return next, nil
}

func ensureNumberSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS email_number_sequence (
			range_name text primary key,
			next integer not null
		)`); err != nil {
		return fmt.Errorf("number schema: %w", err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO email_number_sequence (range_name, next) VALUES ('user', ?), ('test', ?)`,
		UserNumberStart, TestNumberStart); err != nil {
		return fmt.Errorf("seed number sequences: %w", err)
	}
	var has int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('email_aliases') WHERE name = 'reserved_until'`).Scan(&has); err != nil {
		return err
	}
	if has == 0 {
		if _, err := db.Exec(`ALTER TABLE email_aliases ADD COLUMN reserved_until text`); err != nil {
			return fmt.Errorf("add reserved_until: %w", err)
		}
	}
	// Repair the ambiguous canonical_number 0: only a bare canonical numeric
	// local part carries a number; everything else is NULL.
	repairs := []string{
		`UPDATE email_aliases SET canonical_number = NULL
			WHERE canonical_number IS NOT NULL AND (
				local_part GLOB '*[^0-9]*' OR length(local_part) > 12
				OR printf('%03d', CAST(local_part AS INTEGER)) != local_part)`,
		`UPDATE email_aliases SET canonical_number = CAST(local_part AS INTEGER)
			WHERE canonical_number IS NULL AND target_type = 'user'
				AND local_part NOT GLOB '*[^0-9]*' AND length(local_part) <= 12
				AND printf('%03d', CAST(local_part AS INTEGER)) = local_part`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_email_aliases_canonical_number
			ON email_aliases(canonical_number) WHERE canonical_number IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_email_aliases_numbered_target
			ON email_aliases(target_id) WHERE canonical_number IS NOT NULL AND target_type = 'user'`,
	}
	for _, stmt := range repairs {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("number index migration: %w", err)
		}
	}
	return nil
}

// ReserveTestNumber atomically takes the next test number and creates its
// alias on the public numeric inbound policy with no owner yet. The alias
// routes nowhere until BindNumber, and the reservation expires after ttl.
func (s *Store) ReserveTestNumber(ctx context.Context, domain string, now time.Time, ttl time.Duration) (NumberReservation, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return NumberReservation{}, fmt.Errorf("domain is required")
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return NumberReservation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	stamp := now.UTC().Format(time.RFC3339Nano)
	// Sweep: an unbound reservation past its time is disabled (its number
	// stays burned: the sequence never goes back).
	if _, err := tx.ExecContext(ctx, `UPDATE email_aliases SET disabled_at = ?
		WHERE target_type = 'reserved' AND disabled_at IS NULL AND reserved_until <= ?`, stamp, stamp); err != nil {
		return NumberReservation{}, fmt.Errorf("sweep expired reservations: %w", err)
	}
	n, err := allocateNumber(ctx, tx, numberRangeTest)
	if err != nil {
		return NumberReservation{}, err
	}
	localPart := canonicalNumberString(n)
	expires := now.UTC().Add(ttl)
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_aliases (
		id, domain, local_part, canonical_number, target_type, target_id,
		visibility, receive_policy_id, created_at, reserved_until
	) VALUES (?, ?, ?, ?, 'reserved', '', 'public', ?, ?, ?)`,
		aliasRowID(domain, localPart), domain, localPart, n, DefaultPublicPolicyID, stamp,
		expires.Format(time.RFC3339Nano)); err != nil {
		return NumberReservation{}, fmt.Errorf("insert reserved alias: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return NumberReservation{}, err
	}
	return NumberReservation{Address: localPart + "@" + domain, LocalPart: localPart, ExpiresAt: expires}, nil
}

// BindNumber binds a reserved test number to an account. Binding the same
// owner again is a no-op; a different owner is refused.
func (s *Store) BindNumber(ctx context.Context, domain, localPart, ownerID string, now time.Time) (EmailAlias, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	localPart = strings.ToLower(strings.TrimSpace(localPart))
	ownerID = strings.TrimSpace(ownerID)
	if domain == "" || localPart == "" || ownerID == "" {
		return EmailAlias{}, fmt.Errorf("domain, number and owner are required")
	}
	p := parseNumberedLocalPart(localPart)
	if !p.Numeric || p.HasPlus || !p.Canonical || p.Number < TestNumberStart || p.Number >= TestNumberEnd {
		return EmailAlias{}, ErrNumberNotReserved
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return EmailAlias{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var id, targetType, targetID string
	var reservedUntil, disabled sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, target_type, target_id, reserved_until, disabled_at
		FROM email_aliases WHERE domain = ? AND local_part = ?`, domain, localPart).
		Scan(&id, &targetType, &targetID, &reservedUntil, &disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return EmailAlias{}, ErrNumberNotReserved
	}
	if err != nil {
		return EmailAlias{}, err
	}
	switch targetType {
	case "user":
		if targetID == ownerID && !disabled.Valid {
			_ = tx.Rollback()
			return s.ResolveAlias(ctx, domain, localPart)
		}
		return EmailAlias{}, ErrNumberBound
	case "reserved":
	default:
		return EmailAlias{}, ErrNumberNotReserved
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	until, perr := time.Parse(time.RFC3339Nano, reservedUntil.String)
	if disabled.Valid || perr != nil || !now.UTC().Before(until) {
		if !disabled.Valid {
			_, _ = tx.ExecContext(ctx, `UPDATE email_aliases SET disabled_at = ? WHERE id = ?`, stamp, id)
			_ = tx.Commit()
		}
		return EmailAlias{}, ErrNumberExpired
	}
	var other string
	err = tx.QueryRowContext(ctx, `SELECT local_part FROM email_aliases
		WHERE target_type = 'user' AND target_id = ? AND canonical_number IS NOT NULL AND disabled_at IS NULL`, ownerID).Scan(&other)
	if err == nil {
		return EmailAlias{}, fmt.Errorf("account already has a numbered address")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EmailAlias{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_aliases SET target_type = 'user', target_id = ?, reserved_until = NULL
		WHERE id = ?`, ownerID, id); err != nil {
		return EmailAlias{}, fmt.Errorf("bind number: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailAlias{}, err
	}
	return s.ResolveAlias(ctx, domain, localPart)
}
