// Package unprovedpurge deletes the accounts whose address was never proved
// (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md; the owner,
// 2026-11-24: "delete unproved accounts"). It is an operator tool and nothing
// else: no service, boot step or timer calls it. Black class: it deletes
// accounts for good.
//
// The rule it keeps: an account is purged only if its address is not proved and
// it is not the operator's bootstrap account, holds no owner key, owns no
// computer now (vmctl's list), and has no row in a store that exists only for an
// account that used a computer. Anything else is REFUSED and printed (hashed),
// never deleted. The dry run prints every table and its count and a digest of
// the list; the real run needs that digest, so the list that was shown is the
// list that goes.
//
// Every table of every service keyed by a user is in Registry, with its key.
// The tool also asks each database which tables carry a user-keyed column and
// stops when one is not in Registry: a table added after this list was written
// is a refusal to run, not a table left behind.
package unprovedpurge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// Key is one table and the column that names a user in it.
type Key struct {
	Table  string
	Column string
	// By is "id" (the column holds a user id) or "email" (it holds the address).
	By string
	// Where narrows the rows (an extra SQL condition on the same table).
	Where string
	// Clear sets the column to '' instead of deleting the row (a row that other accounts share).
	Clear bool
	// Proof: any row here means the account used a computer, so it is refused, not purged.
	Proof bool
}

// Service is one database and the keys in it.
type Service struct {
	Name string
	DB   *sql.DB
	Kind dbx.Kind
	Keys []Key
	// MailboxRoot, when set, is the directory holding users/<id> (the per-account mailboxes and attachments), removed after the database commit.
	MailboxRoot string
}

// Registry: every user-keyed table, by service. Order is deletion order (children first, users last).
var (
	AuthKeys = []Key{
		{Table: "credentials", Column: "user_id", By: "id"},
		{Table: "challenge_state", Column: "user_id", By: "id"},
		{Table: "refresh_sessions", Column: "user_id", By: "id"},
		{Table: "desktop_exchange_codes", Column: "user_id", By: "id"},
		{Table: "api_keys", Column: "user_id", By: "id"},
		{Table: "recovery_tokens", Column: "user_id", By: "id"},
		{Table: "recovery_tokens", Column: "email", By: "email"},
		{Table: "email_signin_attempts", Column: "user_id", By: "id"},
		{Table: "device_authorizations", Column: "user_id", By: "id"},
		{Table: "account_set_member", Column: "user_id", By: "id"},
		{Table: "account_set", Column: "active_user_id", By: "id", Clear: true},
		{Table: "owner_root_key_wraps", Column: "owner_id", By: "id"},
		{Table: "owner_root_keys", Column: "owner_id", By: "id", Proof: false},
		{Table: "pending_registrations", Column: "user_id", By: "id"},
		{Table: "pending_registrations", Column: "email", By: "email"},
		{Table: "registration_waitlist", Column: "email", By: "email"},
		{Table: "users", Column: "id", By: "id"},
	}
	MaildKeys = []Key{
		{Table: "email_sender_whitelist", Column: "owner_id", By: "id"},
		{Table: "email_provider_message_index", Column: "owner_id", By: "id"},
		{Table: "email_approval_token_index", Column: "owner_id", By: "id"},
		{Table: "email_owner_addresses", Column: "owner_id", By: "id"},
		{Table: "email_aliases", Column: "target_id", By: "id", Where: "target_type = 'user'"},
		{Table: "digest_sent", Column: "owner_id", By: "id"},
		{Table: "digest_runs", Column: "owner_id", By: "id"},
		{Table: "digest_subscriptions", Column: "owner_id", By: "id"},
	}
	// PlatformKeys are the PostgreSQL tables (the control and graph databases) keyed by an owner; a table
	// is looked for by name in each database and absent ones are skipped. Rows in any of them mean the
	// account used a computer (Proof), so they refuse the account.
	PlatformKeys = []Key{
		{Table: "events", Column: "owner_id", By: "id", Proof: true},
		{Table: "choir_events", Column: "owner_id", By: "id", Proof: true},
		{Table: "run_memory_entries", Column: "owner_id", By: "id", Proof: true},
		{Table: "wire_publish_debounce_entries", Column: "owner_id", By: "id", Proof: true},
		{Table: "wire_publish_debounce_state", Column: "owner_id", By: "id", Proof: true},
		{Table: "co_super_slots", Column: "owner_id", By: "id", Proof: true},
		{Table: "media_progress", Column: "owner_id", By: "id", Proof: true},
		{Table: "media_recents", Column: "owner_id", By: "id", Proof: true},
		{Table: "user_preferences", Column: "owner_id", By: "id", Proof: true},
		{Table: "desktop_workspaces", Column: "owner_id", By: "id", Proof: true},
		{Table: "desktop_sessions", Column: "owner_id", By: "id", Proof: true},
		{Table: "desktop_app_instances", Column: "owner_id", By: "id", Proof: true},
		{Table: "desktop_window_placements", Column: "owner_id", By: "id", Proof: true},
		{Table: "texture_revisions", Column: "owner_id", By: "id", Proof: true},
		{Table: "texture_source_entities", Column: "owner_id", By: "id", Proof: true},
		{Table: "texture_source_refs", Column: "owner_id", By: "id", Proof: true},
		{Table: "texture_document_aliases", Column: "owner_id", By: "id", Proof: true},
		{Table: "texture_agent_mutations", Column: "owner_id", By: "id", Proof: true},
		{Table: "agent_evidence", Column: "owner_id", By: "id", Proof: true},
		{Table: "og_objects", Column: "owner_id", By: "id", Proof: true},
		{Table: "platform_texture_documents", Column: "owner_id", By: "id", Proof: true},
		{Table: "platform_texture_revisions", Column: "owner_id", By: "id", Proof: true},
		{Table: "proj_capture_resolution", Column: "owner_id", By: "id", Proof: true},
		{Table: "proj_capture_index", Column: "owner_id", By: "id", Proof: true},
		{Table: "proj_capture_terms", Column: "owner_id", By: "id", Proof: true},
		{Table: "vec_captures", Column: "owner_id", By: "id", Proof: true},
		{Table: "search_leads", Column: "owner_id", By: "id", Proof: true},
		{Table: "owner_act_receipts", Column: "owner_user_id", By: "id", Proof: true},
	}
	// userColumns are the column names that name a user; a table with one that is not in a service's keys stops the run.
	userColumns = map[string]bool{"user_id": true, "owner_id": true, "owner_user_id": true, "account_id": true, "active_user_id": true, "target_id": true}
	// notUserKeyed are tables whose such column is not a user (reviewed): none today.
	notUserKeyed = map[string]bool{}
)

// Candidate is an account whose address is not proved.
type Candidate struct{ ID, Email string }

// Refusal is a candidate that is not purged, and why.
type Refusal struct{ IDHash, Reason string }

// Count is the rows a purge would delete (or clear) in one table.
type Count struct {
	Service, Table, Column string
	Rows                   int64
	Clear                  bool
}

// Plan is what a run would do.
type Plan struct {
	Purge     []Candidate
	Refused   []Refusal
	Counts    []Count
	Mailboxes int
	Digest    string
}

// Hash names an id in output without printing it.
func Hash(id string) string {
	s := sha256.Sum256([]byte("purge-unproved:" + id))
	return hex.EncodeToString(s[:6])
}

// Digest is the digest of a list of ids: what the real run must be given.
func Digest(ids []string) string {
	c := append([]string(nil), ids...)
	sort.Strings(c)
	s := sha256.Sum256([]byte(strings.Join(c, "\n")))
	return hex.EncodeToString(s[:])
}

// Owners answers the user ids that own a computer in vmctl's registry now, in any state.
type Owners func(ctx context.Context) (map[string]bool, error)

// ErrUnregistered: a table carries a user-keyed column the registry does not name.
var ErrUnregistered = errors.New("a table is keyed by a user and not in the purge registry")

func ph(kind dbx.Kind, n int) string {
	if kind == dbx.Postgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func inList(kind dbx.Kind, from int, n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = ph(kind, from+i)
	}
	return strings.Join(p, ",")
}

func chunks(ss []string, n int) [][]string {
	var out [][]string
	for len(ss) > 0 {
		k := n
		if k > len(ss) {
			k = len(ss)
		}
		out = append(out, ss[:k])
		ss = ss[k:]
	}
	return out
}

func args(ss []string) []any {
	a := make([]any, len(ss))
	for i, s := range ss {
		a[i] = s
	}
	return a
}

// columnsOf lists a table's columns.
func columnsOf(ctx context.Context, db *sql.DB, kind dbx.Kind, table string) ([]string, error) {
	var q string
	switch kind {
	case dbx.SQLite:
		q = fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, strings.ReplaceAll(table, "'", ""))
	default:
		q = `SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1`
	}
	var rows *sql.Rows
	var err error
	if kind == dbx.Postgres {
		rows, err = db.QueryContext(ctx, q, table)
	} else {
		rows, err = db.QueryContext(ctx, q)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// present: the keys of the service whose table exists in the database; and the check that no other table is user-keyed.
func (s *Service) present(ctx context.Context) ([]Key, error) {
	tables, err := dbx.Tables(ctx, s.DB, s.Kind)
	if err != nil {
		return nil, fmt.Errorf("%s: list tables: %w", s.Name, err)
	}
	have := map[string]bool{}
	for _, t := range tables {
		have[t] = true
	}
	known := map[string]map[string]bool{}
	var out []Key
	for _, k := range s.Keys {
		if known[k.Table] == nil {
			known[k.Table] = map[string]bool{}
		}
		known[k.Table][k.Column] = true
		if have[k.Table] {
			out = append(out, k)
		}
	}
	var unknown []string
	for _, t := range tables {
		cols, err := columnsOf(ctx, s.DB, s.Kind, t)
		if err != nil {
			return nil, fmt.Errorf("%s: columns of %s: %w", s.Name, t, err)
		}
		for _, c := range cols {
			if userColumns[strings.ToLower(c)] && !known[t][c] && !notUserKeyed[t+"."+c] {
				unknown = append(unknown, t+"."+c)
			}
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%w: %s: %s", ErrUnregistered, s.Name, strings.Join(unknown, ", "))
	}
	return out, nil
}

func (k Key) where(kind dbx.Kind, n int) string {
	col := k.Column
	if k.By == "email" {
		col = "lower(" + k.Column + ")"
	}
	w := fmt.Sprintf("%s IN (%s)", col, inList(kind, 1, n))
	if k.Where != "" {
		w = "(" + k.Where + ") AND " + w
	}
	return w
}

func (s *Service) count(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, k Key, vals []string) (int64, error) {
	var total int64
	for _, c := range chunks(vals, 400) {
		var n int64
		if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", k.Table, k.where(s.Kind, len(c))), args(c)...).Scan(&n); err != nil {
			return 0, fmt.Errorf("%s: count %s.%s: %w", s.Name, k.Table, k.Column, err)
		}
		total += n
	}
	return total, nil
}

// BuildPlan reads and counts; it writes nothing.
func BuildPlan(ctx context.Context, authDB *sql.DB, owners Owners, services []Service) (*Plan, error) {
	rows, err := authDB.QueryContext(ctx, `SELECT id, email FROM users WHERE email_verified_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list unproved accounts: %w", err)
	}
	var cands []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.Email); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = rows.Close()

	refused := map[string]string{}
	refuse := func(id, why string) {
		if _, ok := refused[id]; !ok {
			refused[id] = why
		}
	}
	rootRows, err := authDB.QueryContext(ctx, `SELECT owner_id FROM owner_root_keys`)
	if err != nil {
		return nil, fmt.Errorf("owner keys: %w", err)
	}
	hasRoot := map[string]bool{}
	for rootRows.Next() {
		var id string
		if err := rootRows.Scan(&id); err != nil {
			_ = rootRows.Close()
			return nil, err
		}
		hasRoot[id] = true
	}
	_ = rootRows.Close()
	own, err := owners(ctx)
	if err != nil {
		return nil, fmt.Errorf("computer owners (vmctl): %w", err)
	}
	for _, c := range cands {
		switch {
		case c.ID == auth.BootstrapAdminAPIKeyUserID:
			refuse(c.ID, "the bootstrap admin")
		case hasRoot[c.ID]:
			refuse(c.ID, "holds an owner key")
		case own[c.ID]:
			refuse(c.ID, "owns or owned a computer (vmctl registry)")
		}
	}

	// The rows of the stores that exist only for an account that used a computer refuse it too.
	type svcKeys struct {
		svc  *Service
		keys []Key
	}
	var all []svcKeys
	for i := range services {
		keys, err := services[i].present(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, svcKeys{&services[i], keys})
	}
	ids := make([]string, len(cands))
	for i, c := range cands {
		ids[i] = c.ID
	}
	for _, sk := range all {
		for _, k := range sk.keys {
			if !k.Proof {
				continue
			}
			for _, chunk := range chunks(ids, 400) {
				rs, err := sk.svc.DB.QueryContext(ctx, fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s", k.Column, k.Table, k.where(sk.svc.Kind, len(chunk))), args(chunk)...)
				if err != nil {
					return nil, fmt.Errorf("%s: %s.%s: %w", sk.svc.Name, k.Table, k.Column, err)
				}
				for rs.Next() {
					var id string
					if err := rs.Scan(&id); err != nil {
						_ = rs.Close()
						return nil, err
					}
					refuse(id, fmt.Sprintf("has rows in %s.%s (it used a computer)", sk.svc.Name, k.Table))
				}
				_ = rs.Close()
			}
		}
	}

	p := &Plan{}
	var purgeIDs, purgeEmails []string
	for _, c := range cands {
		if why, no := refused[c.ID]; no {
			p.Refused = append(p.Refused, Refusal{IDHash: Hash(c.ID), Reason: why})
			continue
		}
		p.Purge = append(p.Purge, c)
		purgeIDs = append(purgeIDs, c.ID)
		purgeEmails = append(purgeEmails, strings.ToLower(strings.TrimSpace(c.Email)))
	}
	p.Digest = Digest(purgeIDs)
	for _, sk := range all {
		for _, k := range sk.keys {
			vals := purgeIDs
			if k.By == "email" {
				vals = purgeEmails
			}
			n, err := sk.svc.count(ctx, sk.svc.DB, k, vals)
			if err != nil {
				return nil, err
			}
			p.Counts = append(p.Counts, Count{Service: sk.svc.Name, Table: k.Table, Column: k.Column, Rows: n, Clear: k.Clear})
		}
		if sk.svc.MailboxRoot != "" {
			for _, id := range purgeIDs {
				if st, err := os.Stat(filepath.Join(sk.svc.MailboxRoot, "users", id)); err == nil && st.IsDir() {
					p.Mailboxes++
				}
			}
		}
	}
	return p, nil
}

// Apply deletes what BuildPlan listed, if its digest is the one given: one transaction per service,
// the auth service last (it holds the accounts). Nothing is deleted on any error.
func Apply(ctx context.Context, authDB *sql.DB, owners Owners, services []Service, digest string) (*Plan, error) {
	plan, err := BuildPlan(ctx, authDB, owners, services)
	if err != nil {
		return nil, err
	}
	if digest == "" || digest != plan.Digest {
		return plan, fmt.Errorf("the digest given is not the digest of the list now (%s): run the dry run again and give its digest", plan.Digest)
	}
	var ids, emails []string
	for _, c := range plan.Purge {
		ids = append(ids, c.ID)
		emails = append(emails, strings.ToLower(strings.TrimSpace(c.Email)))
	}
	if len(ids) == 0 {
		return plan, nil
	}
	// The other services first, the accounts (auth) last: a failure before the end leaves the accounts in
	// place, so the run can be made again; the reverse would leave data nobody can find.
	order := append([]Service(nil), services...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].Name != "auth" && order[j].Name == "auth" })
	for i := range order {
		keys, err := order[i].present(ctx)
		if err != nil {
			return plan, err
		}
		if err := order[i].delete(ctx, keys, ids, emails, order[i].Name == "auth"); err != nil {
			return plan, err
		}
	}
	for _, s := range services {
		if s.MailboxRoot == "" {
			continue
		}
		for _, id := range ids {
			if err := os.RemoveAll(filepath.Join(s.MailboxRoot, "users", id)); err != nil {
				return plan, fmt.Errorf("remove mailbox of %s: %w", Hash(id), err)
			}
		}
	}
	return plan, nil
}

func (s *Service) delete(ctx context.Context, keys []Key, ids, emails []string, isAuth bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", s.Name, err)
	}
	defer func() { _ = tx.Rollback() }()
	if s.Kind == dbx.SQLite {
		if _, err := tx.ExecContext(ctx, "PRAGMA defer_foreign_keys = ON"); err != nil {
			return fmt.Errorf("%s: defer foreign keys: %w", s.Name, err)
		}
	} else {
		_, _ = tx.ExecContext(ctx, "SET CONSTRAINTS ALL DEFERRED")
	}
	if isAuth {
		// Inside the write transaction: every account is still unproved, or nothing goes.
		var still int
		for _, c := range chunks(ids, 400) {
			var n int
			if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM users WHERE email_verified_at IS NULL AND id IN (%s)", inList(s.Kind, 1, len(c))), args(c)...).Scan(&n); err != nil {
				return fmt.Errorf("%s: re-check: %w", s.Name, err)
			}
			still += n
		}
		if still != len(ids) {
			return fmt.Errorf("%s: %d of %d accounts changed since the plan (one was proved): nothing was deleted", s.Name, len(ids)-still, len(ids))
		}
	}
	for _, k := range keys {
		vals := ids
		if k.By == "email" {
			vals = emails
		}
		for _, c := range chunks(vals, 400) {
			var q string
			if k.Clear {
				q = fmt.Sprintf("UPDATE %s SET %s = '' WHERE %s", k.Table, k.Column, k.where(s.Kind, len(c)))
			} else {
				q = fmt.Sprintf("DELETE FROM %s WHERE %s", k.Table, k.where(s.Kind, len(c)))
			}
			if _, err := tx.ExecContext(ctx, q, args(c)...); err != nil {
				return fmt.Errorf("%s: %s.%s: %w", s.Name, k.Table, k.Column, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", s.Name, err)
	}
	return nil
}
