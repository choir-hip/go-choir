package maild

// Automatic user-number assignment (owner, 2026-10-31: "every new account gets
// an email"; existing real accounts are backfilled, oldest first).
//
// Ways this could fail, written before the code:
//   - double assignment under concurrency: the allocation and the alias insert
//     share one transaction, and the partial unique index on
//     (target_id) WHERE canonical_number IS NOT NULL AND target_type='user'
//     refuses a second numbered alias for the same account;
//   - gaps on failure: allocateNumber's UPDATE rolls back with the transaction,
//     so a failed insert burns nothing (only an occupied-number skip burns one,
//     and that is bounded and logged);
//   - the root getting a number: the root alias (000) is a numbered 'user'
//     alias for RootOwnerID, so the existing-number check returns it; the HTTP
//     handler also refuses RootOwnerID outright, and the store refuses to
//     assign at all until the root alias exists (schema seeded);
//   - test accounts consuming user numbers: auth filters test/ephemeral sign-in
//     domains before it calls; the user sequence never issues a number in the
//     test range (allocateNumber jumps over it); tests use ReserveTestNumber;
//   - reconcile running before maild's schema repair: the route is served only
//     after EnsureSchema (cmd/maild), and AssignUserNumber returns
//     ErrNumberSchemaNotReady if the root alias is absent.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// AuthAssignNumbersPath is the host-internal route the auth service calls with
// an ordered list of account ids to give numbers to. Same transport gates as
// the recovery-link route: loopback host-service callers only.
const AuthAssignNumbersPath = "/internal/maild/auth/assign-numbers"

const authAssignNumbersMaxOwners = 500

var ErrNumberSchemaNotReady = errors.New("numbered-address schema not ready: root alias missing")

// UserNumberAssignment is one account's number.
type UserNumberAssignment struct {
	OwnerID string `json:"owner_id"`
	Number  int64  `json:"number"`
	Created bool   `json:"created"`
}

// AssignUserNumber gives ownerID the next user-range number as a numbered
// alias on the public receive policy. Idempotent: an account that already has a
// numbered alias (disabled included) keeps it and nothing is allocated.
func (s *Store) AssignUserNumber(ctx context.Context, domain, ownerID string, now time.Time) (UserNumberAssignment, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	ownerID = strings.TrimSpace(ownerID)
	if domain == "" || ownerID == "" {
		return UserNumberAssignment{}, fmt.Errorf("domain and owner are required")
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return UserNumberAssignment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var rootRows int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM email_aliases WHERE canonical_number = 0`).Scan(&rootRows); err != nil {
		return UserNumberAssignment{}, err
	}
	if rootRows == 0 {
		return UserNumberAssignment{}, ErrNumberSchemaNotReady
	}
	var existing sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT canonical_number FROM email_aliases
		WHERE target_type = 'user' AND target_id = ? AND canonical_number IS NOT NULL`, ownerID).Scan(&existing)
	if err == nil {
		return UserNumberAssignment{OwnerID: ownerID, Number: existing.Int64}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UserNumberAssignment{}, err
	}
	stamp := now.UTC().Format(numberTimeFormat)
	for attempt := 0; attempt < reserveAttempts; attempt++ {
		n, err := allocateNumber(ctx, tx, numberRangeUser)
		if err != nil {
			return UserNumberAssignment{}, err
		}
		localPart := canonicalNumberString(n)
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO email_aliases (
			id, domain, local_part, canonical_number, target_type, target_id,
			visibility, receive_policy_id, created_at
		) VALUES (?, ?, ?, ?, 'user', ?, 'public', ?, ?)`,
			aliasRowID(domain, localPart), domain, localPart, n, ownerID, DefaultPublicPolicyID, stamp)
		if err != nil {
			return UserNumberAssignment{}, fmt.Errorf("insert numbered alias: %w", err)
		}
		if c, _ := res.RowsAffected(); c != 1 {
			log.Printf("maild: user number %s is occupied; skipping", localPart)
			continue
		}
		if err := tx.Commit(); err != nil {
			return UserNumberAssignment{}, err
		}
		return UserNumberAssignment{OwnerID: ownerID, Number: n, Created: true}, nil
	}
	return UserNumberAssignment{}, fmt.Errorf("no free user number after %d attempts", reserveAttempts)
}

// AssignTestUserNumber gives a test or ephemeral-domain account a number from
// the test range [10,000,000, 100,000,000) as a numbered alias on the public
// receive policy, in one step (reserve and bind together: the account is
// known, so no reservation waits). Idempotent like AssignUserNumber: an
// account that already has a numbered alias keeps it. Every account has a
// numbered address (owner, 2026-11-04: "every vm has its own email, even test
// accounts"); before this, nothing bound a test account to the test range.
func (s *Store) AssignTestUserNumber(ctx context.Context, domain, ownerID string, now time.Time) (UserNumberAssignment, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	ownerID = strings.TrimSpace(ownerID)
	if domain == "" || ownerID == "" {
		return UserNumberAssignment{}, fmt.Errorf("domain and owner are required")
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return UserNumberAssignment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var existing sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT canonical_number FROM email_aliases
		WHERE target_type = 'user' AND target_id = ? AND canonical_number IS NOT NULL`, ownerID).Scan(&existing)
	if err == nil {
		return UserNumberAssignment{OwnerID: ownerID, Number: existing.Int64}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UserNumberAssignment{}, err
	}
	stamp := now.UTC().Format(numberTimeFormat)
	for attempt := 0; attempt < reserveAttempts; attempt++ {
		n, err := allocateNumber(ctx, tx, numberRangeTest)
		if err != nil {
			return UserNumberAssignment{}, err
		}
		localPart := canonicalNumberString(n)
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO email_aliases (
			id, domain, local_part, canonical_number, target_type, target_id,
			visibility, receive_policy_id, created_at
		) VALUES (?, ?, ?, ?, 'user', ?, 'public', ?, ?)`,
			aliasRowID(domain, localPart), domain, localPart, n, ownerID, DefaultPublicPolicyID, stamp)
		if err != nil {
			return UserNumberAssignment{}, fmt.Errorf("insert test numbered alias: %w", err)
		}
		if c, _ := res.RowsAffected(); c != 1 {
			log.Printf("maild: test number %s is occupied; skipping", localPart)
			continue
		}
		if err := tx.Commit(); err != nil {
			return UserNumberAssignment{}, err
		}
		return UserNumberAssignment{OwnerID: ownerID, Number: n, Created: true}, nil
	}
	return UserNumberAssignment{}, fmt.Errorf("no free test number after %d attempts", reserveAttempts)
}

// PlannedUserNumber is one line of the dry-run plan.
type PlannedUserNumber struct {
	OwnerID  string `json:"owner_id"`
	Number   int64  `json:"number"`
	Existing bool   `json:"existing"`
}

// PlanUserNumbers reports, without writing, the number each owner (in the given
// order) has or would get. Owners equal to rootOwnerID are omitted.
func (s *Store) PlanUserNumbers(ctx context.Context, rootOwnerID string, owners []string) ([]PlannedUserNumber, error) {
	var next int64
	if err := s.routingDB.QueryRowContext(ctx, `SELECT next FROM email_number_sequence WHERE range_name = 'user'`).Scan(&next); err != nil {
		return nil, err
	}
	have := map[string]int64{}
	taken := map[int64]bool{}
	rows, err := s.routingDB.QueryContext(ctx, `SELECT canonical_number, target_type, target_id FROM email_aliases WHERE canonical_number IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n int64
		var tt, tid string
		if err := rows.Scan(&n, &tt, &tid); err != nil {
			return nil, err
		}
		taken[n] = true
		if tt == "user" {
			have[tid] = n
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var plan []PlannedUserNumber
	seen := map[string]bool{}
	for _, o := range owners {
		o = strings.TrimSpace(o)
		if o == "" || o == rootOwnerID || seen[o] {
			continue
		}
		seen[o] = true
		if n, ok := have[o]; ok {
			plan = append(plan, PlannedUserNumber{OwnerID: o, Number: n, Existing: true})
			continue
		}
		for {
			n := next
			if next >= TestNumberStart && next < UserNumberBlockTwoStart {
				n = UserNumberBlockTwoStart
				next = n
			}
			next++
			if !taken[n] {
				taken[n] = true
				plan = append(plan, PlannedUserNumber{OwnerID: o, Number: n})
				break
			}
		}
	}
	return plan, nil
}

type authAssignNumbersRequest struct {
	OwnerIDs []string `json:"owner_ids"`
	// TestOwnerIDs are accounts on test or ephemeral sign-in domains: they
	// get numbers from the test range, never the user sequence.
	TestOwnerIDs []string `json:"test_owner_ids,omitempty"`
}

type authAssignNumbersResponse struct {
	Assigned int `json:"assigned"`
	Existing int `json:"existing"`
}

// HandleAuthAssignNumbers assigns numbers in the order given and stops at the
// first failure, so the oldest accounts always hold the lowest numbers.
func (h *Handler) HandleAuthAssignNumbers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !isMaildLoopbackHost(maildRemoteAddrHost(r)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "host service caller required"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
		return
	}
	if !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
		return
	}
	if strings.TrimSpace(r.Header.Get("X-Authenticated-User")) != "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available to user requests"})
		return
	}
	var in authAssignNumbersRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	if len(in.OwnerIDs)+len(in.TestOwnerIDs) == 0 || len(in.OwnerIDs) > authAssignNumbersMaxOwners || len(in.TestOwnerIDs) > authAssignNumbersMaxOwners {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner_ids and test_owner_ids hold 1 to 500 ids"})
		return
	}
	var out authAssignNumbersResponse
	for _, id := range in.OwnerIDs {
		id = strings.TrimSpace(id)
		if id == "" || id == h.cfg.RootOwnerID {
			continue
		}
		a, err := h.store.AssignUserNumber(r.Context(), h.cfg.PrimaryDomain, id, time.Now())
		if err != nil {
			log.Printf("maild: operation=assign_user_number result=error error=%q", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "assignment failed", "assigned": out.Assigned, "existing": out.Existing})
			return
		}
		if a.Created {
			out.Assigned++
			log.Printf("maild: operation=assign_user_number owner_id=%s number=%s result=assigned", id, canonicalNumberString(a.Number))
		} else {
			out.Existing++
		}
	}
	for _, id := range in.TestOwnerIDs {
		id = strings.TrimSpace(id)
		if id == "" || id == h.cfg.RootOwnerID {
			continue
		}
		a, err := h.store.AssignTestUserNumber(r.Context(), h.cfg.PrimaryDomain, id, time.Now())
		if err != nil {
			log.Printf("maild: operation=assign_test_number result=error error=%q", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "assignment failed", "assigned": out.Assigned, "existing": out.Existing})
			return
		}
		if a.Created {
			out.Assigned++
			log.Printf("maild: operation=assign_test_number owner_id=%s number=%s result=assigned", id, canonicalNumberString(a.Number))
		} else {
			out.Existing++
		}
	}
	writeJSON(w, http.StatusOK, out)
}
