// Package unprovedpurge deletes the accounts whose address was never proved
// (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md; the owner,
// 2026-11-24: "delete unproved accounts"). It is an operator tool and nothing
// else: no service, boot step or timer calls it. Black class: it deletes
// accounts for good.
//
// The rule it keeps: an account is purged only if its address is not proved and
// it is not the operator's bootstrap account, holds no owner key, owns no
// computer in vmctl's registry now, and has no row in a store that exists only
// for an account that used a computer. Anything else is REFUSED and printed
// (hashed), never deleted. The dry run prints every table and its count and a
// digest of the list and of each account's state; the real run needs that
// digest, so the list that was shown is the list that goes.
//
// The real run holds auth's write lock from before the first service is touched
// until auth commits last, and checks inside that lock that every listed account
// is still unproved and unchanged, so no proof can land between the check and
// the deletion. The order is: lock auth, check, delete the other services (one
// transaction each), remove the mailbox directories, then delete the accounts
// and commit auth. A failure before the end leaves the accounts in place and
// the run can be made again; what was already deleted stays deleted, and the
// result says which services committed.
//
// Every user-keyed table of every service is in the registry, with its key.
// Each database is also asked which tables carry a user-keyed column (by name
// or by suffix) and a table that is not in the registry stops the run: a table
// added after this list was written is a refusal to run, not a table left
// behind.
package unprovedpurge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
	// LikePrefix: the column holds a composite id "<prefix><user id>:..." (a control route slot,
	// "computer:<owner>:<computer>"); rows are matched by prefix and id, one statement per id.
	LikePrefix string
	// Proof: any row here means the account used a computer or the platform, so it is refused, not purged.
	Proof bool
	// Exists: the rows match when this condition holds, "{list}" standing for the placeholders of the ids: for a
	// webhook event, that the mail index lists its provider and message id for one of the accounts.
	Exists string
}

// Service is one database and the keys in it.
type Service struct {
	Name string
	DB   *sql.DB
	Kind dbx.Kind
	Keys []Key
	// CoreAny: at least one of these tables must exist, or the database is not the one it is named for
	// (a mistyped path makes an empty file).
	CoreAny []string
	// MailboxRoot, when set, is the maild storage root: users/<id> and attachments/outbound/<id> are removed.
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
		{Table: "email_signin_attempts", Column: "user_id", By: "id"},
		{Table: "device_authorizations", Column: "user_id", By: "id"},
		{Table: "account_set_member", Column: "user_id", By: "id"},
		{Table: "owner_root_key_wraps", Column: "owner_id", By: "id"},
		{Table: "owner_root_keys", Column: "owner_id", By: "id"},
		{Table: "pending_registrations", Column: "user_id", By: "id"},
		{Table: "pending_registrations", Column: "email", By: "email"},
		{Table: "registration_waitlist", Column: "email", By: "email"},
		{Table: "users", Column: "id", By: "id"},
	}
	// AuthSetColumn is account_set.active_user_id: repaired, not deleted (the set belongs to other accounts too).
	AuthSetColumn = Key{Table: "account_set", Column: "active_user_id", By: "id"}

	MaildKeys = []Key{
		// The inbound webhook log keeps each event's whole payload (recipient, addresses, message): the events of
		// the account's messages go, found through the provider message index, before that index is deleted.
		{Table: "email_webhook_events", Column: "provider_message_id", By: "id", Exists: "EXISTS (SELECT 1 FROM email_provider_message_index m WHERE m.provider = email_webhook_events.provider AND m.provider_message_id = email_webhook_events.provider_message_id AND m.owner_id IN ({list}))"},
		{Table: "email_sender_whitelist", Column: "owner_id", By: "id"},
		{Table: "email_provider_message_index", Column: "owner_id", By: "id"},
		{Table: "email_approval_token_index", Column: "owner_id", By: "id"},
		{Table: "email_owner_addresses", Column: "owner_id", By: "id"},
		{Table: "email_aliases", Column: "target_id", By: "id", Where: "target_type = 'user'"},
		{Table: "digest_sent", Column: "owner_id", By: "id"},
		{Table: "digest_runs", Column: "owner_id", By: "id"},
		{Table: "digest_subscriptions", Column: "owner_id", By: "id"},
		// The shared tables of the mail database before mailboxes were split per account
		// (store.go migrateLegacySharedMailbox copies out of them and leaves them): an account with rows
		// here is refused until they are migrated away or dropped, not half deleted.
		{Table: "email_messages", Column: "mailbox_owner_id", By: "id", Proof: true},
		{Table: "email_ingress_events", Column: "owner_id", By: "id", Proof: true},
		{Table: "email_drafts", Column: "owner_id", By: "id", Proof: true},
		{Table: "email_draft_approval_events", Column: "owner_id", By: "id", Proof: true},
		{Table: "email_draft_approval_tokens", Column: "owner_id", By: "id", Proof: true},
		{Table: "email_risk_alerts", Column: "owner_id", By: "id", Proof: true},
		{Table: "email_draft_edit_requests", Column: "owner_id", By: "id", Proof: true},
	}
	// PlatformKeys are the PostgreSQL tables (the control and graph databases) keyed by an owner; a table
	// is looked for by name in each database and absent ones are skipped. Rows in any of them mean the
	// account used a computer or the platform (Proof), so they refuse the account.
	PlatformKeys = func() []Key {
		var out []Key
		for _, c := range []string{"events", "choir_events", "run_memory_entries", "wire_publish_debounce_entries", "wire_publish_debounce_state",
			"co_super_slots", "media_progress", "media_recents", "user_preferences", "desktop_workspaces", "desktop_sessions", "desktop_app_instances",
			"desktop_window_placements", "texture_revisions", "texture_source_entities", "texture_source_refs", "texture_document_aliases",
			"texture_agent_mutations", "agent_evidence", "og_objects", "platform_texture_documents", "platform_texture_revisions",
			"proj_capture_resolution", "proj_capture_index", "proj_capture_terms", "vec_captures", "search_leads"} {
			out = append(out, Key{Table: c, Column: "owner_id", By: "id", Proof: true})
		}
		out = append(out,
			Key{Table: "owner_act_receipts", Column: "owner_user_id", By: "id", Proof: true},
			Key{Table: "publication_version_proposals", Column: "source_owner_id", By: "id", Proof: true},
			Key{Table: "publication_version_proposals", Column: "submitter_id", By: "id", Proof: true},
			Key{Table: "proposal_delivery_records", Column: "target_owner_id", By: "id", Proof: true},
			// target_id names the object delivered to; if a delivery ever targets a user (target_kind 'user') that user is refused.
			Key{Table: "proposal_delivery_records", Column: "target_id", By: "id", Where: "target_kind = 'user'", Proof: true},
			Key{Table: "computer_key_unwrap_requests", Column: "requested_by", By: "id", Proof: true},
			Key{Table: "computer_key_unwrap_approvals", Column: "approver", By: "id", Proof: true},
		)
		// The control database encodes the owner in the route slot id "computer:<owner>:<computer>".
		for _, t := range []string{"computer_version_route_slots", "computer_version_route_transition_receipts", "computer_version_route_authorization_evidence", "computer_version_route_authority_modes"} {
			out = append(out, Key{Table: t, Column: "route_slot_id", By: "id", LikePrefix: "computer:", Proof: true})
		}
		return out
	}()

	// ControlCore and GraphCore tell the two PostgreSQL databases apart: each must hold one of its own tables, so the
	// same database given twice, or the wrong one, is refused (every Proof table of the missing one would be skipped).
	ControlCore = []string{"owner_act_receipts", "computer_version_route_slots"}
	GraphCore   = []string{"og_objects", "publication_version_proposals"}

	// userColumnNames and userColumnSuffixes are the columns that name a user (or an owner, or a submitter);
	// a table with one that is not in a service's keys stops the run.
	userColumnNames    = map[string]bool{"requested_by": true, "approver": true, "target_id": true, "route_slot_id": true, "mailbox_owner_id": true, "submitter_id": true, "email": true, "user_email": true}
	userColumnSuffixes = []string{"owner_id", "submitter_id", "user_id", "account_id", "owner_user_id"}
	// notUserKeyed are table.column pairs reviewed and found not to name a user.
	notUserKeyed = map[string]bool{
		"users.email":                 true, // the account table itself (keyed by id)
		"recovery_tokens.email":       true, // keyed by user_id; an address-only row is a dummy for rate limiting
		"email_owner_addresses.email": true,
		"pending_registrations.email": true,
		"registration_waitlist.email": true,
	}
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// SafeID: an id fit to be a path element and a LIKE operand: no separator, not empty, not dotted, local.
func SafeID(id string) bool {
	return safeID.MatchString(id) && !strings.HasPrefix(id, ".") && id != "." && id != ".." && filepath.IsLocal(id) && id == filepath.Base(id)
}

// Candidate is an account whose address is not proved.
type Candidate struct {
	ID, Email, Fingerprint string
}

// Refusal is a candidate that is not purged, and why.
type Refusal struct{ IDHash, Reason string }

// Count is the rows a purge would delete (or repair) in one table.
type Count struct {
	Service, Table, Column string
	Rows                   int64
}

// Plan is what a run would do.
type Plan struct {
	Purge     []Candidate
	Refused   []Refusal
	Counts    []Count
	Mailboxes int
	Digest    string
	Services  []string
	// protected are the addresses a surviving account holds in another case: email-keyed rows are kept for them.
	protected map[string]bool
}

// Hash names an id in output without printing it.
func Hash(id string) string {
	s := sha256.Sum256([]byte("purge-unproved:" + id))
	return hex.EncodeToString(s[:6])
}

// Digest is the digest of what a run would do: the sorted accounts with their state fingerprints, the
// services, and the rows per table. A change to any of them is a different digest.
func Digest(parts []string) string {
	c := append([]string(nil), parts...)
	sort.Strings(c)
	s := sha256.Sum256([]byte(strings.Join(c, "\n")))
	return hex.EncodeToString(s[:])
}

// Owners answers the user ids that own a computer in vmctl's registry now, in any state.
type Owners func(ctx context.Context) (map[string]bool, error)

var (
	// ErrUnregistered: a table carries a user-keyed column the registry does not name.
	ErrUnregistered = errors.New("a table is keyed by a user and not in the purge registry")
	// ErrIncomplete: a service database is not the one it is named for.
	ErrIncomplete = errors.New("a service database lacks its core tables")
	// ErrChanged: an account changed between the plan and the lock.
	ErrChanged = errors.New("an account changed since the plan")
)

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

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// columnsOf lists a table's columns.
func columnsOf(ctx context.Context, db queryer, kind dbx.Kind, table string) ([]string, error) {
	var rows *sql.Rows
	var err error
	if kind == dbx.Postgres {
		rows, err = db.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1`, table)
	} else {
		rows, err = db.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, strings.ReplaceAll(table, "'", "")))
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

func isUserColumn(c string) bool {
	c = strings.ToLower(c)
	if userColumnNames[c] {
		return true
	}
	for _, s := range userColumnSuffixes {
		if strings.HasSuffix(c, s) {
			return true
		}
	}
	return false
}

// present: the keys of the service whose table exists in the database, after the checks that the database is the
// right one and that no other table is user-keyed.
func (s *Service) present(ctx context.Context) ([]Key, error) {
	tables, err := dbx.Tables(ctx, s.DB, s.Kind)
	if err != nil {
		return nil, fmt.Errorf("%s: list tables: %w", s.Name, err)
	}
	have := map[string]bool{}
	for _, t := range tables {
		have[strings.ToLower(t)] = true
	}
	if len(s.CoreAny) > 0 {
		ok := false
		for _, c := range s.CoreAny {
			ok = ok || have[c]
		}
		if !ok {
			return nil, fmt.Errorf("%w: %s has none of %s (a mistyped path?)", ErrIncomplete, s.Name, strings.Join(s.CoreAny, ", "))
		}
	}
	known := map[string]map[string]bool{}
	var out []Key
	for _, k := range s.Keys {
		t, c := strings.ToLower(k.Table), strings.ToLower(k.Column)
		if known[t] == nil {
			known[t] = map[string]bool{}
		}
		known[t][c] = true
		if have[t] {
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
			lt, lc := strings.ToLower(t), strings.ToLower(c)
			if isUserColumn(lc) && !known[lt][lc] && !notUserKeyed[lt+"."+lc] && !(s.Name == "auth" && lt == AuthSetColumn.Table && lc == AuthSetColumn.Column) {
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

// where builds the condition for n values starting at placeholder 1.
func (k Key) where(kind dbx.Kind, n int) string {
	col := k.Column
	if k.By == "email" {
		col = "lower(" + k.Column + ")"
	}
	w := fmt.Sprintf("%s IN (%s)", col, inList(kind, 1, n))
	if k.Exists != "" {
		w = strings.ReplaceAll(k.Exists, "{list}", inList(kind, 1, n))
	}
	if k.Where != "" {
		w = "(" + k.Where + ") AND " + w
	}
	return w
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// countKey counts the rows of one key for the values (ids, or addresses for an email key).
func (s *Service) countKey(ctx context.Context, q queryer, k Key, vals []string) (int64, error) {
	var total int64
	if k.LikePrefix != "" {
		for _, v := range vals {
			var n int64
			stmt := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s LIKE %s ESCAPE '\\'", k.Table, k.Column, ph(s.Kind, 1))
			if err := q.QueryRowContext(ctx, stmt, k.LikePrefix+escapeLike(v)+":%").Scan(&n); err != nil {
				return 0, fmt.Errorf("%s: count %s.%s: %w", s.Name, k.Table, k.Column, err)
			}
			total += n
		}
		return total, nil
	}
	for _, c := range chunks(vals, 400) {
		var n int64
		if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", k.Table, k.where(s.Kind, len(c))), args(c)...).Scan(&n); err != nil {
			return 0, fmt.Errorf("%s: count %s.%s: %w", s.Name, k.Table, k.Column, err)
		}
		total += n
	}
	return total, nil
}

// proofHits are the ids among vals that have a row under a Proof key.
func (s *Service) proofHits(ctx context.Context, k Key, ids []string) (map[string]bool, error) {
	hits := map[string]bool{}
	if k.LikePrefix != "" {
		for _, id := range ids {
			n, err := s.countKey(ctx, s.DB, k, []string{id})
			if err != nil {
				return nil, err
			}
			if n > 0 {
				hits[id] = true
			}
		}
		return hits, nil
	}
	for _, chunk := range chunks(ids, 400) {
		rs, err := s.DB.QueryContext(ctx, fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s", k.Column, k.Table, k.where(s.Kind, len(chunk))), args(chunk)...)
		if err != nil {
			return nil, fmt.Errorf("%s: %s.%s: %w", s.Name, k.Table, k.Column, err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				_ = rs.Close()
				return nil, err
			}
			hits[id] = true
		}
		if err := rs.Err(); err != nil {
			_ = rs.Close()
			return nil, fmt.Errorf("%s: reading %s.%s: %w", s.Name, k.Table, k.Column, err)
		}
		_ = rs.Close()
	}
	return hits, nil
}

// candidates reads the unproved accounts and each one's state fingerprint.
func candidates(ctx context.Context, q queryer) ([]Candidate, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, email, created_at FROM users WHERE email_verified_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list unproved accounts: %w", err)
	}
	type raw struct{ id, email, created string }
	var rs []raw
	for rows.Next() {
		var r raw
		var created any
		if err := rows.Scan(&r.id, &r.email, &created); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.created = fmt.Sprint(created)
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("list unproved accounts: %w", err)
	}
	_ = rows.Close()
	out := make([]Candidate, 0, len(rs))
	for _, r := range rs {
		fp, err := fingerprint(ctx, q, r.id, r.email, r.created)
		if err != nil {
			return nil, err
		}
		out = append(out, Candidate{ID: r.id, Email: r.email, Fingerprint: fp})
	}
	return out, nil
}

// fingerprint is the account's state as far as proving or changing it is concerned: a passkey added,
// a credential removed, an address changed or proved all change it.
func fingerprint(ctx context.Context, q queryer, id, email, created string) (string, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM credentials WHERE user_id = ? ORDER BY id`, id)
	if err != nil {
		return "", fmt.Errorf("fingerprint %s: %w", Hash(id), err)
	}
	defer rows.Close()
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|", id, strings.ToLower(email), created)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s,", c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("fingerprint %s: %w", Hash(id), err)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// BuildPlan reads and counts; it writes nothing.
func BuildPlan(ctx context.Context, authDB *sql.DB, owners Owners, services []Service) (*Plan, error) {
	plan, _, err := buildPlan(ctx, authDB, owners, services)
	return plan, err
}

func buildPlan(ctx context.Context, authDB *sql.DB, owners Owners, services []Service) (*Plan, map[string][]Key, error) {
	cands, err := candidates(ctx, authDB)
	if err != nil {
		return nil, nil, err
	}
	refused := map[string]string{}
	refuse := func(id, why string) {
		if _, ok := refused[id]; !ok {
			refused[id] = why
		}
	}
	rootRows, err := authDB.QueryContext(ctx, `SELECT owner_id FROM owner_root_keys`)
	if err != nil {
		return nil, nil, fmt.Errorf("owner keys: %w", err)
	}
	hasRoot := map[string]bool{}
	for rootRows.Next() {
		var id string
		if err := rootRows.Scan(&id); err != nil {
			_ = rootRows.Close()
			return nil, nil, err
		}
		hasRoot[id] = true
	}
	if err := rootRows.Err(); err != nil {
		_ = rootRows.Close()
		return nil, nil, fmt.Errorf("owner keys: %w", err)
	}
	_ = rootRows.Close()
	own, err := owners(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("computer owners (vmctl): %w", err)
	}
	for _, c := range cands {
		switch {
		case !SafeID(c.ID):
			refuse(c.ID, "its id is not safe to use as a path or a pattern")
		case c.ID == auth.BootstrapAdminAPIKeyUserID:
			refuse(c.ID, "the bootstrap admin")
		case hasRoot[c.ID]:
			refuse(c.ID, "holds an owner key")
		case own[c.ID]:
			refuse(c.ID, "owns a computer (vmctl registry)")
		}
	}

	keysBy := map[string][]Key{}
	for i := range services {
		keys, err := services[i].present(ctx)
		if err != nil {
			return nil, nil, err
		}
		keysBy[services[i].Name] = keys
	}
	ids := make([]string, 0, len(cands))
	for _, c := range cands {
		if SafeID(c.ID) {
			ids = append(ids, c.ID)
		}
	}
	// The rows of the stores that exist only for an account that used a computer refuse it too.
	for i := range services {
		for _, k := range keysBy[services[i].Name] {
			if !k.Proof {
				continue
			}
			hits, err := services[i].proofHits(ctx, k, ids)
			if err != nil {
				return nil, nil, err
			}
			for id := range hits {
				refuse(id, fmt.Sprintf("has rows in %s.%s (it used a computer or the platform)", services[i].Name, k.Table))
			}
		}
	}

	p := &Plan{protected: map[string]bool{}}
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
	// An address a surviving account holds in any case is not touched: its recovery links, pending sign-up and
	// waitlist entry are that account's.
	if err := protectedEmails(ctx, authDB, purgeEmails, purgeIDs, p.protected); err != nil {
		return nil, nil, err
	}
	var emailVals []string
	for _, e := range purgeEmails {
		if !p.protected[e] {
			emailVals = append(emailVals, e)
		}
	}
	parts := []string{"services:" + strings.Join(serviceNames(services), ",")}
	for _, c := range p.Purge {
		parts = append(parts, "account:"+c.ID+":"+c.Fingerprint)
	}
	for _, s := range services {
		p.Services = append(p.Services, s.Name)
		for _, k := range keysBy[s.Name] {
			vals := purgeIDs
			if k.By == "email" {
				vals = emailVals
			}
			n, err := s.countKey(ctx, s.DB, k, vals)
			if err != nil {
				return nil, nil, err
			}
			p.Counts = append(p.Counts, Count{Service: s.Name, Table: k.Table, Column: k.Column, Rows: n})
			parts = append(parts, fmt.Sprintf("count:%s.%s.%s=%d", s.Name, k.Table, k.Column, n))
		}
		if s.Name == "auth" {
			n, err := countSetsToRepair(ctx, s.DB, purgeIDs)
			if err != nil {
				return nil, nil, err
			}
			p.Counts = append(p.Counts, Count{Service: s.Name, Table: AuthSetColumn.Table, Column: AuthSetColumn.Column + " (repaired)", Rows: n})
		}
		if s.MailboxRoot != "" {
			for _, id := range purgeIDs {
				for _, dir := range mailDirs(s.MailboxRoot, id) {
					if st, err := os.Lstat(dir); err == nil && st.IsDir() {
						p.Mailboxes++
					}
				}
			}
		}
	}
	parts = append(parts, fmt.Sprintf("mail_dirs=%d", p.Mailboxes))
	p.Digest = Digest(parts)
	return p, keysBy, nil
}

func serviceNames(ss []Service) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	sort.Strings(out)
	return out
}

// protectedEmails marks the addresses a surviving user holds, in any case.
func protectedEmails(ctx context.Context, q queryer, emails, purgeIDs []string, into map[string]bool) error {
	gone := map[string]bool{}
	for _, id := range purgeIDs {
		gone[id] = true
	}
	for _, c := range chunks(emails, 400) {
		rows, err := q.QueryContext(ctx, fmt.Sprintf("SELECT id, lower(email) FROM users WHERE lower(email) IN (%s)", inList(dbx.SQLite, 1, len(c))), args(c)...)
		if err != nil {
			return fmt.Errorf("surviving addresses: %w", err)
		}
		for rows.Next() {
			var id, e string
			if err := rows.Scan(&id, &e); err != nil {
				_ = rows.Close()
				return err
			}
			if !gone[id] {
				into[e] = true
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("surviving addresses: %w", err)
		}
		_ = rows.Close()
	}
	return nil
}

// setsOf: the account sets that hold a member to purge.
func setsOf(ctx context.Context, q queryer, ids []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, c := range chunks(ids, 400) {
		rows, err := q.QueryContext(ctx, fmt.Sprintf("SELECT DISTINCT set_id FROM account_set_member WHERE user_id IN (%s)", inList(dbx.SQLite, 1, len(c))), args(c)...)
		if err != nil {
			return nil, fmt.Errorf("account sets: %w", err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("account sets: %w", err)
		}
		_ = rows.Close()
	}
	sort.Strings(out)
	return out, nil
}

func countSetsToRepair(ctx context.Context, q queryer, ids []string) (int64, error) {
	s, err := setsOf(ctx, q, ids)
	return int64(len(s)), err
}

// mailDirs are the directories maild keeps for an account: its mailbox, and its staged outbound attachments.
func mailDirs(root, id string) []string {
	root = filepath.Clean(root)
	return []string{filepath.Join(root, "users", id), filepath.Join(root, "attachments", "outbound", id)}
}

// checkMailRoot: the root, its users/ and its attachments/outbound/ are real directories (no symlink), so a
// removal below them stays below them.
func checkMailRoot(root string) error {
	root = filepath.Clean(root)
	for _, p := range []string{root, filepath.Join(root, "users")} {
		st, err := os.Lstat(p)
		if err != nil {
			return fmt.Errorf("mail root %s: %w", p, err)
		}
		if !st.Mode().IsDir() {
			return fmt.Errorf("mail root %s is not a real directory (a symlink or a file)", p)
		}
	}
	// attachments/outbound may not exist; if it does it must be real.
	for _, p := range []string{filepath.Join(root, "attachments"), filepath.Join(root, "attachments", "outbound")} {
		if st, err := os.Lstat(p); err == nil && !st.Mode().IsDir() {
			return fmt.Errorf("mail root %s is not a real directory (a symlink or a file)", p)
		}
	}
	return nil
}

func removeMailDirs(root, id string) (int, error) {
	if !SafeID(id) {
		return 0, fmt.Errorf("refusing to remove a directory for an unsafe id")
	}
	removed := 0
	for _, dir := range mailDirs(root, id) {
		parent := filepath.Dir(dir)
		if filepath.Clean(filepath.Join(parent, id)) != filepath.Clean(dir) || filepath.Base(dir) != id {
			return removed, fmt.Errorf("refusing: %s is not a direct child of its parent", Hash(id))
		}
		st, err := os.Lstat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		if !st.IsDir() {
			return removed, fmt.Errorf("refusing: the directory of %s is a symlink or a file", Hash(id))
		}
		if err := os.RemoveAll(dir); err != nil {
			return removed, fmt.Errorf("remove the directory of %s: %w", Hash(id), err)
		}
		removed++
	}
	return removed, nil
}

// ServiceResult is what happened to one service in a real run.
type ServiceResult struct {
	Name      string
	Committed bool
	Err       error
}

// Journal is written before each irreversible step, so a run that dies leaves what it was about to do. A
// failing Entry stops the run at that point (before the step it announces).
type Journal interface {
	Entry(stage string, res *Result) error
}

// Result is a real run's outcome.
type Result struct {
	Plan     *Plan
	Services []ServiceResult
	// MailDirs is how many directories were removed.
	MailDirs int
}

// Apply deletes what BuildPlan listed, if its digest is the one given. auth's write lock is taken first
// (authTx must be a transaction begun with the write lock: BEGIN IMMEDIATE) and held to the end: inside it
// every listed account is checked to be still unproved and unchanged, then the other services are deleted
// (one transaction each), then the mail directories, then the accounts, and auth commits last. Nothing is
// deleted on any error before the other services; after that, the result says which committed.
func Apply(ctx context.Context, authDB *sql.DB, owners Owners, services []Service, digest string, journal Journal) (res *Result, err error) {
	res = &Result{}
	note := func(stage string) error {
		if journal == nil {
			return nil
		}
		if jerr := journal.Entry(stage, res); jerr != nil {
			return fmt.Errorf("the journal could not be written (%s): %w", stage, jerr)
		}
		return nil
	}
	started := false
	defer func() {
		if started && err != nil && journal != nil {
			_ = journal.Entry("stopped: "+err.Error(), res)
		}
	}()
	plan, keysBy, perr := buildPlan(ctx, authDB, owners, services)
	err = perr
	res.Plan = plan
	if err != nil {
		return res, err
	}
	if afterPlanHook != nil {
		afterPlanHook()
	}
	if digest == "" || digest != plan.Digest {
		return res, fmt.Errorf("the digest given is not the digest of the list now (%s): run the dry run again and give its digest", plan.Digest)
	}
	var ids, emails []string
	for _, c := range plan.Purge {
		ids = append(ids, c.ID)
		if e := strings.ToLower(strings.TrimSpace(c.Email)); !plan.protected[e] {
			emails = append(emails, e)
		}
	}
	if len(ids) == 0 {
		if err := note("intent"); err != nil {
			return res, err
		}
		return res, note("done")
	}
	for _, s := range services {
		if s.MailboxRoot != "" {
			if err := checkMailRoot(s.MailboxRoot); err != nil {
				return res, err
			}
		}
	}
	var authSvc *Service
	var others []Service
	for i := range services {
		if services[i].Name == "auth" {
			authSvc = &services[i]
		} else {
			others = append(others, services[i])
		}
	}
	if authSvc == nil {
		return res, errors.New("the auth service is required")
	}

	// auth's write lock, before anything else is touched, held until it commits last.
	tx, err := authDB.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("auth: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "PRAGMA defer_foreign_keys = ON"); err != nil {
		return res, fmt.Errorf("auth: defer foreign keys: %w", err)
	}
	// Every listed account is still unproved and unchanged, now, under the lock.
	now, err := candidates(ctx, tx)
	if err != nil {
		return res, err
	}
	fp := map[string]string{}
	for _, c := range now {
		fp[c.ID] = c.Fingerprint
	}
	for _, c := range plan.Purge {
		if fp[c.ID] == "" || fp[c.ID] != c.Fingerprint {
			return res, fmt.Errorf("%w: %s was proved or changed (recovery or verification finished): nothing was deleted", ErrChanged, Hash(c.ID))
		}
	}
	started = true
	if err := note("intent"); err != nil {
		return res, err
	}
	for i := range others {
		if err := note("before-commit:" + others[i].Name); err != nil {
			return res, err
		}
		r := ServiceResult{Name: others[i].Name}
		err := others[i].deleteIn(ctx, keysBy[others[i].Name], ids, emails)
		r.Committed, r.Err = err == nil, err
		res.Services = append(res.Services, r)
		if err != nil {
			return res, err
		}
		if err := note("after-commit:" + others[i].Name); err != nil {
			return res, err
		}
	}
	if err := note("before-mail-dirs"); err != nil {
		return res, err
	}
	for _, s := range others {
		if s.MailboxRoot == "" {
			continue
		}
		for _, id := range ids {
			n, err := removeMailDirs(s.MailboxRoot, id)
			res.MailDirs += n
			if err != nil {
				return res, err
			}
		}
	}
	if err := note("after-mail-dirs"); err != nil {
		return res, err
	}
	if s := authSvc; true {
		sets, err := setsOf(ctx, tx, ids)
		if err != nil {
			return res, err
		}
		if err := execKeys(ctx, tx, s.Kind, keysBy["auth"], ids, emails); err != nil {
			return res, fmt.Errorf("auth: %w", err)
		}
		if err := repairSets(ctx, tx, sets, ids); err != nil {
			return res, fmt.Errorf("auth: %w", err)
		}
	}
	if err := note("before-commit:auth"); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		res.Services = append(res.Services, ServiceResult{Name: "auth", Err: err})
		return res, fmt.Errorf("auth: commit: %w", err)
	}
	res.Services = append(res.Services, ServiceResult{Name: "auth", Committed: true})
	if err := note("after-commit:auth"); err != nil {
		return res, err
	}
	// The directories maild may have recreated while the run was going (a late mail): one more look.
	for _, s := range others {
		if s.MailboxRoot == "" {
			continue
		}
		for _, id := range ids {
			for _, d := range mailDirs(s.MailboxRoot, id) {
				if _, err := os.Lstat(d); err == nil {
					return res, fmt.Errorf("a mail directory of %s exists again after the purge (maild was running?): remove it by hand", Hash(id))
				}
			}
		}
	}
	started = false
	return res, note("done")
}

// afterPlanHook lets a test change the world between the plan and the lock.
var afterPlanHook func()

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// deleteIn runs a service's keys in one transaction of its own.
func (s *Service) deleteIn(ctx context.Context, keys []Key, ids, emails []string) error {
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
	if err := execKeys(ctx, tx, s.Kind, keys, ids, emails); err != nil {
		return fmt.Errorf("%s: %w", s.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", s.Name, err)
	}
	return nil
}

// execKeys deletes the rows of every non-Proof key (a Proof key's rows refused the account, so none remain).
func execKeys(ctx context.Context, tx execer, kind dbx.Kind, keys []Key, ids, emails []string) error {
	for _, k := range keys {
		if k.Proof {
			continue
		}
		vals := ids
		if k.By == "email" {
			vals = emails
		}
		for _, c := range chunks(vals, 400) {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s", k.Table, k.where(kind, len(c))), args(c)...); err != nil {
				return fmt.Errorf("%s.%s: %w", k.Table, k.Column, err)
			}
		}
	}
	return nil
}

// repairSets: after the members are gone, a set with none is deleted, and a set whose active account was
// purged points at its most recent surviving member (the same repair the auth service makes when a member leaves).
func repairSets(ctx context.Context, tx interface {
	execer
	queryer
}, sets, purged []string) error {
	gone := map[string]bool{}
	for _, id := range purged {
		gone[id] = true
	}
	for _, set := range sets {
		var members int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_set_member WHERE set_id = ?", set).Scan(&members); err != nil {
			return fmt.Errorf("set %s: %w", set, err)
		}
		if members == 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM account_set WHERE id = ?", set); err != nil {
				return fmt.Errorf("delete empty set: %w", err)
			}
			continue
		}
		var active string
		if err := tx.QueryRowContext(ctx, "SELECT active_user_id FROM account_set WHERE id = ?", set).Scan(&active); err != nil {
			return fmt.Errorf("set %s: %w", set, err)
		}
		if !gone[active] {
			continue
		}
		// auth's repairSetTx (internal/auth/account_set_store.go): a member with a live session first, newest first;
		// here a proved member comes before one that is not, as ListAccountSet would not list the other.
		var next string
		if err := tx.QueryRowContext(ctx, `SELECT m.user_id FROM account_set_member m
			JOIN users u ON u.id = m.user_id
			LEFT JOIN refresh_sessions r ON r.id = m.refresh_session_id AND r.expires_at > ?
			WHERE m.set_id = ? ORDER BY (u.email_verified_at IS NULL), (r.id IS NULL), m.added_at DESC, m.user_id LIMIT 1`, time.Now().UTC(), set).Scan(&next); err != nil {
			return fmt.Errorf("set %s: next member: %w", set, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE account_set SET active_user_id = ? WHERE id = ?", next, set); err != nil {
			return fmt.Errorf("set %s: repair: %w", set, err)
		}
	}
	return nil
}

// EmailMAC is a keyed hash of an address (lower-cased, trimmed) for the deletion receipt: with the key,
// "was this address purged" can be answered later without keeping the address.
func EmailMAC(key []byte, email string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(m.Sum(nil))
}
