package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AccountSetIdleTTL is how long a set may go unused before the hygiene sweep
// deletes it (the choir_accounts cookie lives the same 30 days).
const AccountSetIdleTTL = 30 * 24 * time.Hour

// ErrRotationLost means the session being rotated was already rotated or
// deleted by a concurrent request.
var ErrRotationLost = errors.New("refresh session already rotated")

// AccountSet is one browser's set of signed-in accounts.
type AccountSet struct {
	ID           string
	HandleHash   string
	ActiveUserID string
	CreatedAt    time.Time
	LastUsedAt   time.Time
}

// AccountMember is one account in a set, tied to its refresh session.
type AccountMember struct {
	SetID            string
	UserID           string
	RefreshSessionID string
	AddedAt          time.Time
}

// AccountListing is one account as the list endpoint shows it.
type AccountListing struct {
	UserID   string
	Email    string
	Active   bool
	SignedIn bool
}

// rotateTx replaces oldID with ns inside tx: delete old (ErrRotationLost when
// it is already gone), insert new with rotated_from = old, and repoint any
// account-set member from old to new.
func rotateTx(tx *sql.Tx, oldID string, ns *RefreshSession) error {
	res, err := tx.Exec("DELETE FROM refresh_sessions WHERE id = ?", oldID)
	if err != nil {
		return fmt.Errorf("rotate refresh session: delete old: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRotationLost
	}
	if _, err := tx.Exec(
		"INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at, rotated_from, device_info, auth_method) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		ns.ID, ns.UserID, ns.TokenHash, ns.CreatedAt, ns.ExpiresAt, oldID, ns.DeviceInfo, normalizeAuthMethod(ns.AuthMethod),
	); err != nil {
		return fmt.Errorf("rotate refresh session: insert new: %w", err)
	}
	if _, err := tx.Exec("UPDATE account_set_member SET refresh_session_id = ? WHERE refresh_session_id = ?", ns.ID, oldID); err != nil {
		return fmt.Errorf("rotate refresh session: repoint member: %w", err)
	}
	return nil
}

// RotateRefreshSession inserts the new session (rotated_from = old id),
// repoints any account-set member from old to new, and deletes old, in one
// transaction. It returns ErrRotationLost when old no longer exists.
func (s *Store) RotateRefreshSession(oldID string, ns *RefreshSession) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("rotate refresh session: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := rotateTx(tx, oldID, ns); err != nil {
		return err
	}
	return tx.Commit()
}

// RotateAndActivate is RotateRefreshSession plus setting the set's active
// member, in the same transaction (the switch).
func (s *Store) RotateAndActivate(setID, userID, oldID string, ns *RefreshSession) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("switch: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := rotateTx(tx, oldID, ns); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE account_set SET active_user_id = ?, last_used_at = ? WHERE id = ?", userID, time.Now().UTC(), setID); err != nil {
		return fmt.Errorf("switch: set active: %w", err)
	}
	return tx.Commit()
}

// SetActiveAccount sets the set's active member and touches last_used_at.
func (s *Store) SetActiveAccount(setID, userID string) error {
	_, err := s.db.Exec("UPDATE account_set SET active_user_id = ?, last_used_at = ? WHERE id = ?", userID, time.Now().UTC(), setID)
	if err != nil {
		return fmt.Errorf("set active account: %w", err)
	}
	return nil
}

// AddAccountToSet get-or-creates the set (by the presented handle hash, else
// the new one), upserts (set, user, session) on the unique key, makes the
// user active, and deletes the replaced member session, all in one
// transaction. created reports that newHash became the set's handle.
func (s *Store) AddAccountToSet(presentedHash, newHash, userID, sessionID string) (setID string, created bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", false, fmt.Errorf("add account: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	if presentedHash != "" {
		err = tx.QueryRow("SELECT id FROM account_set WHERE handle_hash = ?", presentedHash).Scan(&setID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("add account: find set: %w", err)
		}
	}
	if setID == "" {
		setID = uuid.NewString()
		created = true
		if _, err = tx.Exec("INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES (?, ?, ?, ?, ?)",
			setID, newHash, userID, now, now); err != nil {
			return "", false, fmt.Errorf("add account: create set: %w", err)
		}
	}
	var oldSession string
	err = tx.QueryRow("SELECT refresh_session_id FROM account_set_member WHERE set_id = ? AND user_id = ?", setID, userID).Scan(&oldSession)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err = tx.Exec("INSERT INTO account_set_member (set_id, user_id, refresh_session_id, added_at) VALUES (?, ?, ?, ?)",
			setID, userID, sessionID, now); err != nil {
			return "", false, fmt.Errorf("add account: insert member: %w", err)
		}
	case err != nil:
		return "", false, fmt.Errorf("add account: read member: %w", err)
	default:
		if _, err = tx.Exec("UPDATE account_set_member SET refresh_session_id = ? WHERE set_id = ? AND user_id = ?", sessionID, setID, userID); err != nil {
			return "", false, fmt.Errorf("add account: replace member: %w", err)
		}
		if oldSession != "" && oldSession != sessionID {
			if _, err = tx.Exec("DELETE FROM refresh_sessions WHERE id = ?", oldSession); err != nil {
				return "", false, fmt.Errorf("add account: delete replaced session: %w", err)
			}
		}
	}
	if _, err = tx.Exec("UPDATE account_set SET active_user_id = ?, last_used_at = ? WHERE id = ?", userID, now, setID); err != nil {
		return "", false, fmt.Errorf("add account: set active: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", false, err
	}
	return setID, created, nil
}

// GetAccountSetByHandleHash returns the set for a handle hash, or sql.ErrNoRows.
func (s *Store) GetAccountSetByHandleHash(hash string) (*AccountSet, error) {
	as := &AccountSet{}
	err := s.db.QueryRow("SELECT id, handle_hash, active_user_id, created_at, last_used_at FROM account_set WHERE handle_hash = ?", hash).
		Scan(&as.ID, &as.HandleHash, &as.ActiveUserID, &as.CreatedAt, &as.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return as, nil
}

// GetAccountMember returns a set's member for a user, or sql.ErrNoRows.
func (s *Store) GetAccountMember(setID, userID string) (*AccountMember, error) {
	m := &AccountMember{}
	err := s.db.QueryRow("SELECT set_id, user_id, refresh_session_id, added_at FROM account_set_member WHERE set_id = ? AND user_id = ?", setID, userID).
		Scan(&m.SetID, &m.UserID, &m.RefreshSessionID, &m.AddedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetAccountMemberBySession returns the member of a set tied to a refresh
// session, or sql.ErrNoRows.
func (s *Store) GetAccountMemberBySession(setID, sessionID string) (*AccountMember, error) {
	m := &AccountMember{}
	err := s.db.QueryRow("SELECT set_id, user_id, refresh_session_id, added_at FROM account_set_member WHERE set_id = ? AND refresh_session_id = ?", setID, sessionID).
		Scan(&m.SetID, &m.UserID, &m.RefreshSessionID, &m.AddedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ListAccountSet lists a set's members, active first, then by age. A member
// whose session is gone or expired reads signed_in=false and is deleted after
// it is listed, so "signed out" shows once and then goes.
func (s *Store) ListAccountSet(set *AccountSet) ([]AccountListing, error) {
	rows, err := s.db.Query(`SELECT m.user_id, u.email,
		EXISTS(SELECT 1 FROM refresh_sessions r WHERE r.id = m.refresh_session_id AND r.expires_at > ?)
		FROM account_set_member m JOIN users u ON u.id = m.user_id
		WHERE m.set_id = ? ORDER BY m.added_at, m.user_id`, time.Now().UTC(), set.ID)
	if err != nil {
		return nil, fmt.Errorf("list account set: %w", err)
	}
	var all []AccountListing
	for rows.Next() {
		var l AccountListing
		if err := rows.Scan(&l.UserID, &l.Email, &l.SignedIn); err != nil {
			_ = rows.Close()
			return nil, err
		}
		l.Active = l.UserID == set.ActiveUserID
		all = append(all, l)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	out := make([]AccountListing, 0, len(all))
	for _, l := range all {
		if l.Active {
			out = append(out, l)
		}
	}
	for _, l := range all {
		if !l.Active {
			out = append(out, l)
		}
	}
	for _, l := range all {
		if !l.SignedIn {
			_, _ = s.db.Exec("DELETE FROM account_set_member WHERE set_id = ? AND user_id = ?", set.ID, l.UserID)
		}
	}
	return out, nil
}

// RemoveAccountMember deletes a member and, when it was the last, the set.
// If the removed member was active, the next live member becomes active.
// nextLive names a live member to switch to ("" if none); setDeleted reports
// the empty-set case.
func (s *Store) RemoveAccountMember(setID, userID string) (nextLive string, setDeleted bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", false, fmt.Errorf("remove member: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec("DELETE FROM account_set_member WHERE set_id = ? AND user_id = ?", setID, userID); err != nil {
		return "", false, fmt.Errorf("remove member: %w", err)
	}
	var remaining int
	if err = tx.QueryRow("SELECT COUNT(*) FROM account_set_member WHERE set_id = ?", setID).Scan(&remaining); err != nil {
		return "", false, err
	}
	if remaining == 0 {
		if _, err = tx.Exec("DELETE FROM account_set WHERE id = ?", setID); err != nil {
			return "", false, err
		}
		return "", true, tx.Commit()
	}
	var active string
	if err = tx.QueryRow("SELECT active_user_id FROM account_set WHERE id = ?", setID).Scan(&active); err != nil {
		return "", false, err
	}
	if active == userID || active == "" {
		var live []string
		rows, qerr := tx.Query(`SELECT m.user_id FROM account_set_member m
			JOIN refresh_sessions r ON r.id = m.refresh_session_id AND r.expires_at > ?
			WHERE m.set_id = ? ORDER BY m.added_at DESC, m.user_id`, time.Now().UTC(), setID)
		if qerr != nil {
			return "", false, qerr
		}
		for rows.Next() {
			var u string
			if err = rows.Scan(&u); err != nil {
				_ = rows.Close()
				return "", false, err
			}
			live = append(live, u)
		}
		_ = rows.Close()
		next := ""
		if len(live) > 0 {
			next = live[0]
			nextLive = live[0]
		} else if err = tx.QueryRow("SELECT user_id FROM account_set_member WHERE set_id = ? ORDER BY added_at DESC LIMIT 1", setID).Scan(&next); err != nil {
			// Only dead members remain; keep a valid active pointer.
			return "", false, err
		}
		if _, err = tx.Exec("UPDATE account_set SET active_user_id = ? WHERE id = ?", next, setID); err != nil {
			return "", false, err
		}
	}
	return nextLive, false, tx.Commit()
}

// ListAccountMembers returns every member of a set.
func (s *Store) ListAccountMembers(setID string) ([]AccountMember, error) {
	rows, err := s.db.Query("SELECT set_id, user_id, refresh_session_id, added_at FROM account_set_member WHERE set_id = ? ORDER BY added_at", setID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AccountMember
	for rows.Next() {
		var m AccountMember
		if err := rows.Scan(&m.SetID, &m.UserID, &m.RefreshSessionID, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteAccountSet removes a set and (by cascade) its members.
func (s *Store) DeleteAccountSet(setID string) error {
	_, err := s.db.Exec("DELETE FROM account_set WHERE id = ?", setID)
	return err
}

// DeleteAccountMembersForUser removes a user's member rows from every set
// (used when all of that user's sessions are revoked on every device).
func (s *Store) DeleteAccountMembersForUser(userID string) error {
	_, err := s.db.Exec("DELETE FROM account_set_member WHERE user_id = ?", userID)
	return err
}

// SweepAccountSets deletes members whose session row is gone or expired and
// sets with no members or idle past AccountSetIdleTTL.
func (s *Store) SweepAccountSets(now time.Time) (members, sets int64, err error) {
	res, err := s.db.Exec(`DELETE FROM account_set_member WHERE NOT EXISTS
		(SELECT 1 FROM refresh_sessions r WHERE r.id = account_set_member.refresh_session_id AND r.expires_at > ?)`, now.UTC())
	if err != nil {
		return 0, 0, fmt.Errorf("sweep account members: %w", err)
	}
	members, _ = res.RowsAffected()
	res, err = s.db.Exec(`DELETE FROM account_set WHERE last_used_at < ? OR NOT EXISTS
		(SELECT 1 FROM account_set_member m WHERE m.set_id = account_set.id)`, now.UTC().Add(-AccountSetIdleTTL))
	if err != nil {
		return members, 0, fmt.Errorf("sweep account sets: %w", err)
	}
	sets, _ = res.RowsAffected()
	return members, sets, nil
}
