package objectgraph

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// The og_key_index side table: a rollback-safe replacement for scanning JSON
// in og_objects for four hot equality reads (a run by agent_id, a
// commitment record by addressee, a source ref by doc_id, an actor wake
// outbox row by projected). The design, its
// failure modes and its rollback are in
// docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md.
//
// In short. The table is created empty at open (nothing scales with
// og_objects). Every writer of og_objects derives the object's keys into the
// table in the same transaction (Maintain, Drop). A family of keys is read
// from the table only while the process knows it is complete and current: it
// is unvalidated at every open, never persisted as valid (an older binary
// would leave that lying), and becomes validated only when an online pass
// (ValidateDerivedKeys, started after the listener is up) has compared the
// table with og_objects page by page. A read checks what it fetched against
// what the table promised and, on any difference, answers from the JSON path
// and withdraws the family. Anything this code cannot answer exactly keeps
// the JSON path.

// DerivedKeysTable is the name of the side table. The replay and witness
// extractors skip it: a derived index is not state.
const DerivedKeysTable = "og_key_index"

// The derived key families.
const (
	KeyRunAgentID          = "run.agent_id"
	KeyCommitmentAddressee = "commitment_record.addressee"
	KeySourceRefDocID      = "source_ref.doc_id"
	// KeyActorWakeOutboxProjected answers the open-wakes list (projected =
	// "false") without reading every outbox row ever written
	// (docs/problems/actor-wake-outbox-list-full-scan-2026-10-23.md).
	KeyActorWakeOutboxProjected = "actor_wake_outbox.projected"
)

// ActorWakeOutboxKind is the object kind of the durable actor wake outbox.
const ActorWakeOutboxKind = ObjectKind("choir.actor_wake_outbox")

const (
	derivedKeyValueMaxChars  = 255
	derivedKeyDefaultPageLen = 200
	// derivedKeyFetchBatch bounds each primary-key IN list; see
	// snapshotFetchBatch.
	derivedKeyProgressEvery = 5 * time.Second
)

const derivedKeysDDL = `CREATE TABLE IF NOT EXISTS ` + DerivedKeysTable + ` (
	canonical_id TEXT     NOT NULL,
	key_name     TEXT     NOT NULL,
	key_value    TEXT     NULL,
	owner_id     TEXT     NOT NULL,
	updated_at   DATETIME NOT NULL,
	content_hash TEXT     NOT NULL,
	derive_ok    INTEGER  NOT NULL DEFAULT 1,
	PRIMARY KEY (canonical_id, key_name)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_og_key_index_lookup ON ` + DerivedKeysTable + `(key_name, key_value, canonical_id);
CREATE INDEX IF NOT EXISTS idx_og_key_index_recent ON ` + DerivedKeysTable + `(key_name, key_value, owner_id, updated_at, canonical_id);
`

// derivedFamily is one family of keys: the kind it covers and the SQL
// expression over an og_objects row that the JSON read evaluated.
type derivedFamily struct {
	Name string
	Kind ObjectKind
	// expr yields the untruncated value; the table holds substr(expr, 1, 255).
	expr string
}

// The outbox family is first: a pass validates the families in this order,
// and the outbox is read by a loop every minute, while the others wait on a
// larger kind.
var derivedFamilies = []derivedFamily{
	{Name: KeyActorWakeOutboxProjected, Kind: ActorWakeOutboxKind, expr: jsonTextSQL(`metadata`, `'$.projected'`)},
	{Name: KeyRunAgentID, Kind: "choir.run", expr: jsonTextSQL(`metadata`, `'$.agent_id'`)},
	{Name: KeyCommitmentAddressee, Kind: commitmentRecordKind, expr: jsonTextSQL(bodyTextSQL, `'$.addressee'`)},
	{Name: KeySourceRefDocID, Kind: "choir.source_ref", expr: jsonTextSQL(`metadata`, `'$.doc_id'`)},
}

func derivedFamilyByName(name string) *derivedFamily {
	for i := range derivedFamilies {
		if derivedFamilies[i].Name == name {
			return &derivedFamilies[i]
		}
	}
	return nil
}

// keyed reports whether objects of the kind have derived keys.
func (d *DerivedKeys) keyed(kind ObjectKind) bool {
	return d != nil && d.tableReady.Load() && derivedFamilyForKind(kind) != nil
}

// Keyed reports whether objects of the kind have derived keys a delete of
// the object must drop. A nil DerivedKeys (replay) keys nothing.
func (d *DerivedKeys) Keyed(kind ObjectKind) bool { return d.keyed(kind) }

func derivedFamilyForKind(kind ObjectKind) *derivedFamily {
	for i := range derivedFamilies {
		if derivedFamilies[i].Kind == kind {
			return &derivedFamilies[i]
		}
	}
	return nil
}

// derivedFamilyForRead maps a metadata lookup to its family.
func derivedFamilyForMetadataRead(kind, jsonPath string) *derivedFamily {
	switch {
	case kind == "choir.run" && jsonPath == "$.agent_id":
		return derivedFamilyByName(KeyRunAgentID)
	case kind == "choir.source_ref" && jsonPath == "$.doc_id":
		return derivedFamilyByName(KeySourceRefDocID)
	case kind == string(ActorWakeOutboxKind) && jsonPath == "$.projected":
		return derivedFamilyByName(KeyActorWakeOutboxProjected)
	}
	return nil
}

// DerivedKeyExecer is the part of *sql.Tx that maintenance needs.
type DerivedKeyExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DerivedKeys is the process-wide state of the side table, shared by the
// write and read stores of one database. Its maps are built once and never
// changed; the values are atomics.
//
// Withdrawal and publication are ordered by pubMu: Invalidate, a write that
// marks a family bad, and a pass's final check-and-set take it, so a
// withdrawal either lands before the pass compares its counters (and the pass
// does not publish) or after it publishes (and withdraws it again). A writer
// withdraws inside its transaction, before its commit, so a read that checks
// readability after its snapshot is taken sees every withdrawal of a row the
// snapshot holds (readDerived).
type DerivedKeys struct {
	tableReady    atomic.Bool
	validated     map[string]*atomic.Bool
	bad           map[string]*atomic.Bool
	badWrites     map[string]*atomic.Uint64
	invalidations map[string]*atomic.Uint64
	mismatches    map[string]*atomic.Uint64
	// writes counts the family's maintained writes and drops; passWrites is
	// its value when the last published pass started. A bad family written
	// since may have had its malformed row repaired (RepairPending).
	writes     map[string]*atomic.Uint64
	passWrites map[string]*atomic.Uint64
	pubMu      sync.Mutex
	passMu     sync.Mutex
	wake       chan struct{}
	// sideReads counts reads answered from the side table; passPages and
	// maxPageHoldNanos count the validator's pages and its longest one.
	sideReads        atomic.Uint64
	passPages        atomic.Uint64
	maxPageHoldNanos atomic.Int64
	// Test seams, nil in production: beforeRead runs as a side-table read
	// starts, beforePublish as a pass is about to check and publish. The interleaving tests put a writer at each.
	beforeRead, beforePublish func(name string)
}

func newDerivedKeys() *DerivedKeys {
	d := &DerivedKeys{
		validated:     map[string]*atomic.Bool{},
		bad:           map[string]*atomic.Bool{},
		badWrites:     map[string]*atomic.Uint64{},
		invalidations: map[string]*atomic.Uint64{},
		mismatches:    map[string]*atomic.Uint64{},
		writes:        map[string]*atomic.Uint64{},
		passWrites:    map[string]*atomic.Uint64{},
		wake:          make(chan struct{}, 1),
	}
	for _, fam := range derivedFamilies {
		d.validated[fam.Name] = new(atomic.Bool)
		d.bad[fam.Name] = new(atomic.Bool)
		d.badWrites[fam.Name] = new(atomic.Uint64)
		d.invalidations[fam.Name] = new(atomic.Uint64)
		d.mismatches[fam.Name] = new(atomic.Uint64)
		d.writes[fam.Name] = new(atomic.Uint64)
		d.passWrites[fam.Name] = new(atomic.Uint64)
	}
	return d
}

// DerivedKeys returns the store's side-table state.
func (s *SQLStore) DerivedKeys() *DerivedKeys {
	if s == nil {
		return nil
	}
	return s.derived
}

// ShareDerivedKeys points this store at another store's side-table state:
// the read store must see what the write store validated.
func (s *SQLStore) ShareDerivedKeys(other *SQLStore) {
	if other != nil && other.derived != nil {
		s.derived = other.derived
	}
}

// Validated reports whether the family has been compared with og_objects in
// this process and is still current.
func (d *DerivedKeys) Validated(keyName string) bool {
	v := d.validated[keyName]
	return v != nil && v.Load()
}

// readable reports whether a read of the family may use the table.
func (d *DerivedKeys) readable(keyName string) bool {
	if d == nil || !d.tableReady.Load() {
		return false
	}
	v, b := d.validated[keyName], d.bad[keyName]
	return v != nil && v.Load() && !b.Load()
}

func (d *DerivedKeys) setValidated(name string, v bool) { d.validated[name].Store(v) }

// RepairPending reports whether a validated family is on the JSON path for a
// malformed row and has been written since its last pass started: the row may
// have been repaired, and only another pass can tell.
func (d *DerivedKeys) RepairPending(name string) bool {
	if d == nil || d.validated[name] == nil {
		return false
	}
	return d.validated[name].Load() && d.bad[name].Load() && d.writes[name].Load() != d.passWrites[name].Load()
}

// noteWrite counts a maintained write or drop of the family and, if the
// family is bad, asks for a pass.
func (d *DerivedKeys) noteWrite(name string) {
	d.writes[name].Add(1)
	if d.bad[name].Load() {
		d.signal()
	}
}

func (d *DerivedKeys) signal() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Mismatches counts the read tripwire's catches for a family.
func (d *DerivedKeys) Mismatches(name string) uint64 {
	if c := d.mismatches[name]; c != nil {
		return c.Load()
	}
	return 0
}

// Wake returns a channel that receives when a family was withdrawn and wants
// another validation pass.
func (d *DerivedKeys) Wake() <-chan struct{} { return d.wake }

// Invalidate withdraws one family: its reads return to the JSON path until
// the next pass validates it.
func (d *DerivedKeys) Invalidate(name, reason string) {
	v := d.validated[name]
	if v == nil {
		return
	}
	d.pubMu.Lock()
	d.invalidations[name].Add(1)
	was := v.Swap(false)
	d.pubMu.Unlock()
	if was {
		log.Printf("objectgraph sql: derived keys %s withdrawn: %s", name, reason)
	}
	d.signal()
}

// InvalidateAll withdraws every family.
func (d *DerivedKeys) InvalidateAll(reason string) {
	if d == nil {
		return
	}
	for _, fam := range derivedFamilies {
		d.Invalidate(fam.Name, reason)
	}
}

// deriveSQL derives one object's row for a family from the stored og_objects
// row, with the same expression the JSON read evaluates,
// it. ok selects the success form; the failure form stores no key and
// derive_ok = false, for a row whose expression errors (a body that is not
// JSON).
func deriveSQL(fam *derivedFamily, where string, ok bool) string {
	value, flag := `substr(`+fam.expr+`, 1, 255)`, `1`
	if !ok {
		value, flag = `NULL`, `0`
	}
	return `INSERT INTO ` + DerivedKeysTable + ` (canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok)
		SELECT canonical_id, ?, ` + value + `, owner_id, updated_at, content_hash, ` + flag + `
		FROM og_objects WHERE object_kind = ? AND ` + where + `
		ON CONFLICT (canonical_id, key_name) DO UPDATE SET
			key_value = excluded.key_value,
			owner_id = excluded.owner_id,
			updated_at = excluded.updated_at,
			content_hash = excluded.content_hash,
			derive_ok = excluded.derive_ok`
}

// Maintain brings the derived keys of one object up to date with its
// og_objects row. Call it after the upsert, inside the same transaction, so
// the two commit or roll back together. It does nothing for a kind with no
// family, and it never fails the write: an expression error stores a
// derive_ok = false row and takes the family to the JSON path (the old read
// fails on such a row too); any other error withdraws the family. Both
// outcomes only cost speed.
func (d *DerivedKeys) Maintain(ctx context.Context, x DerivedKeyExecer, canonicalID string, kind ObjectKind) {
	if d == nil || !d.tableReady.Load() {
		return
	}
	fam := derivedFamilyForKind(kind)
	if fam == nil {
		return
	}
	d.deriveOne(ctx, x, fam, canonicalID)
}

// deriveOne derives one row, tolerating an expression error.
func (d *DerivedKeys) deriveOne(ctx context.Context, x DerivedKeyExecer, fam *derivedFamily, canonicalID string) {
	if _, err := x.ExecContext(ctx, deriveSQL(fam, `canonical_id = ?`, true), fam.Name, string(fam.Kind), canonicalID); err == nil {
		d.noteWrite(fam.Name)
		return
	}
	if _, err := x.ExecContext(ctx, deriveSQL(fam, `canonical_id = ?`, false), fam.Name, string(fam.Kind), canonicalID); err != nil {
		d.Invalidate(fam.Name, fmt.Sprintf("maintain %s: %v", canonicalID, err))
		return
	}
	d.pubMu.Lock()
	d.badWrites[fam.Name].Add(1)
	marked := d.bad[fam.Name].CompareAndSwap(false, true)
	d.pubMu.Unlock()
	if marked {
		log.Printf("objectgraph sql: derived keys %s: %s has a body or metadata the expression cannot read; the family keeps the JSON path", fam.Name, canonicalID)
	}
}

// Drop removes the derived keys of one object. Call it with a delete or a
// rename of the og_objects row, in its transaction. A failure leaves an
// orphan row, which the reads ignore (they fetch the object by key) and the
// next pass removes.
func (d *DerivedKeys) Drop(ctx context.Context, x DerivedKeyExecer, canonicalID string) {
	if d == nil || !d.tableReady.Load() {
		return
	}
	if _, err := x.ExecContext(ctx, `DELETE FROM `+DerivedKeysTable+` WHERE canonical_id = ?`, canonicalID); err != nil {
		d.InvalidateAll(fmt.Sprintf("drop %s: %v", canonicalID, err))
		return
	}
	// The dropped row may have been a family's malformed one.
	for _, fam := range derivedFamilies {
		d.noteWrite(fam.Name)
	}
}

// ensureDerivedKeysTable creates the side table, empty, if it is absent. It
// is cheap on any store: nothing scales with og_objects. A failure leaves the
// table unready, which keeps every read on the JSON path and every writer's
// maintenance off; it does not stop the boot.
func (s *SQLStore) ensureDerivedKeysTable(ctx context.Context) {
	done := BootPhase("store: open", "objectgraph-schema/derived-keys-table")
	_, err := s.db.ExecContext(ctx, derivedKeysDDL)
	done(err)
	if err != nil {
		s.derived.tableReady.Store(false)
		log.Printf("objectgraph sql: %s unavailable, reads keep the JSON path: %v", DerivedKeysTable, err)
		return
	}
	s.derived.tableReady.Store(true)
}

// --- reads ----------------------------------------------------------------

// derivedMetadataRead reports the family a metadata page read can use, or
// nil when it must keep the JSON path: another kind or field, a family not
// validated, or a value of 255 characters or more (below that length
// substr(x, 1, 255) = v exactly when x = v).
func (s *SQLStore) derivedMetadataRead(kind, jsonPath, value string) *derivedFamily {
	fam := derivedFamilyForMetadataRead(kind, jsonPath)
	if fam == nil || !s.derived.readable(fam.Name) || utf8.RuneCountInString(value) >= derivedKeyValueMaxChars {
		return nil
	}
	return fam
}

// derivedAddresseeRead reports the family a body lookup can use: the
// addressee of a commitment record, one plain equality, a value that fits.
func (s *SQLStore) derivedAddresseeRead(kind string, matches []JSONFieldMatch) *derivedFamily {
	if kind != string(commitmentRecordKind) || len(matches) != 1 {
		return nil
	}
	m := matches[0]
	if m.JSONPath != "$.addressee" || m.MissingMatchesEmpty || utf8.RuneCountInString(m.Value) >= derivedKeyValueMaxChars {
		return nil
	}
	fam := derivedFamilyByName(KeyCommitmentAddressee)
	if !s.derived.readable(fam.Name) {
		return nil
	}
	return fam
}

// derivedPageIDsSQL is the keyset page of one key's ids, in canonical_id
// order, through (key_name, key_value, canonical_id). The plan-shape test
// EXPLAINs exactly this text.
func derivedPageIDsSQL(key, value, after string, limit int) (string, []any) {
	return `SELECT canonical_id, owner_id, updated_at, content_hash FROM ` + DerivedKeysTable + `
		WHERE key_name = ? AND key_value = ? AND canonical_id > ?
		ORDER BY key_name ASC, key_value ASC, canonical_id ASC LIMIT ?`,
		[]any{key, value, after, NormalizedLimit(limit)}
}

// derivedIDsByKeySQL is one owner's newest ids for a key, a reverse walk of
// (key_name, key_value, owner_id, updated_at, canonical_id) that stops after
// limit rows, as commitmentClaimSQL does. Equal updated_at come by
// canonical_id descending.
func derivedIDsByKeySQL(key, value, owner, _ string, limit int) (string, []any) {
	return `SELECT canonical_id, owner_id, updated_at, content_hash FROM ` + DerivedKeysTable + `
		WHERE key_name = ? AND key_value = ? AND owner_id = ?
		ORDER BY key_name DESC, key_value DESC, owner_id DESC, updated_at DESC, canonical_id DESC LIMIT ?`,
		[]any{key, value, owner, NormalizedLimit(limit)}
}

type derivedIDRow struct {
	id, owner, hash string
	updatedAt       time.Time
}

// readDerived runs an ids query and fetches the objects by primary key in
// one read-only transaction, so both see one commit. Every fetched row is
// checked against the row the table promised (kind, owner, updated_at,
// content_hash); a difference, or a promised row that is gone, is a stale
// table: the family is withdrawn and ok is false, and the caller answers
// from the JSON path. The family's readability is checked again once the
// transaction's snapshot is taken: a writer withdraws before it commits,
// so any row in the snapshot that withdrew the family is seen here, and the
// caller's earlier check is only a shortcut.
func (s *SQLStore) readDerived(ctx context.Context, fam *derivedFamily, query string, args []any) (objs []Object, ok bool, err error) {
	s.derived.sideReads.Add(1)
	if hook := s.derived.beforeRead; hook != nil {
		hook(fam.Name)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	if err != nil {
		return nil, false, fmt.Errorf("objectgraph sql: begin key index tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("objectgraph sql: read key index: %w", err)
	}
	var promised []derivedIDRow
	for rows.Next() {
		var r derivedIDRow
		if err := rows.Scan(&r.id, &r.owner, &r.updatedAt, &r.hash); err != nil {
			rows.Close()
			return nil, false, fmt.Errorf("objectgraph sql: scan key index: %w", err)
		}
		promised = append(promised, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, fmt.Errorf("objectgraph sql: iterate key index: %w", err)
	}
	rows.Close()
	if !s.derived.readable(fam.Name) {
		return nil, false, nil
	}
	if len(promised) == 0 {
		return nil, true, nil
	}
	ids := make([]string, len(promised))
	for i, r := range promised {
		ids[i] = r.id
	}
	fetched, err := snapshotFetchByID(ctx, tx, ids)
	if err != nil {
		return nil, false, err
	}
	byID := make(map[string]Object, len(fetched))
	for _, o := range fetched {
		byID[o.CanonicalID] = o
	}
	out := make([]Object, 0, len(promised))
	for _, r := range promised {
		o, present := byID[r.id]
		if !present || o.ObjectKind != fam.Kind || o.OwnerID != r.owner || o.ContentHash != r.hash || !o.UpdatedAt.Equal(r.updatedAt.UTC()) {
			s.derived.mismatches[fam.Name].Add(1)
			s.derived.Invalidate(fam.Name, fmt.Sprintf("read found %s different from its key row", r.id))
			return nil, false, nil
		}
		out = append(out, o)
	}
	return out, true, nil
}

// listByDerivedKeyPage is ListObjectsByMetadataPage on the side table.
func (s *SQLStore) listByDerivedKeyPage(ctx context.Context, fam *derivedFamily, value, after string, limit int) ([]Object, bool, error) {
	query, args := derivedPageIDsSQL(fam.Name, value, after, limit)
	return s.readDerived(ctx, fam, query, args)
}

// listByDerivedKeyNewest is ListObjectsByOwnerAndBody's addressee read on the
// side table: the owner's newest limit rows of the key.
func (s *SQLStore) listByDerivedKeyNewest(ctx context.Context, fam *derivedFamily, value, owner string, limit int) ([]Object, bool, error) {
	query, args := derivedIDsByKeySQL(fam.Name, value, owner, "", limit)
	return s.readDerived(ctx, fam, query, args)
}

// --- validation -------------------------------------------------------------

// DerivedKeyValidation tunes ValidateDerivedKeys.
type DerivedKeyValidation struct {
	// PageSize is the number of og_objects rows compared per transaction;
	// the write connection is held for one page at a time. Zero means 200.
	PageSize int
	// DutyCycle is the share of time the pass may hold the write connection, in
	// (0, 1]: after a page that held it h, the pass sleeps h*(1-d)/d. Zero
	// means no pause (tests).
	DutyCycle float64
	// Logf receives progress lines. Nil means log.Printf.
	Logf func(format string, args ...any)
	// ReadDB, when set, is a read pool on the same database. Pages are
	// compared there, in one read snapshot each, and the write connection is
	// taken only to repair the ids that differ: a busy writer then delays a
	// repair, not the whole pass. Nil compares on the write connection.
	ReadDB *sql.DB
}

// derivedKeyRange bounds the canonical ids of a kind: every id built by
// BuildCanonicalID is "obj:<kind>:<owner>:<suffix>", so the kind's rows are
// the primary-key range (lo, hi). A row of the kind whose id is outside it
// (none is built by the code; a test or an old import may) is handled
// separately, so the pass stays exact.
func derivedKeyRange(kind ObjectKind) (lo, hi string) {
	return "obj:" + string(kind) + ":", "obj:" + string(kind) + ";"
}

// ogHeadPageSQL is the validator's page of og_objects headers inside the
// id range (after, before), in canonical_id order, with no body and no
// metadata: a primary-key range walk that stops after limit rows. The kind is
// not in the predicate on purpose: with it the planner takes a secondary
// index on object_kind and sorts the whole kind for every page (the
// plan-shape test EXPLAINs this text). The caller drops a row of another
// kind.
func ogHeadPageSQL(after, before string, limit int) (string, []any) {
	return `SELECT canonical_id, owner_id, updated_at, content_hash, object_kind FROM og_objects
		WHERE canonical_id > ? AND canonical_id < ?
		ORDER BY canonical_id ASC LIMIT ?`, []any{after, before, limit}
}

type derivedSideRow struct {
	id, owner, hash string
	updatedAt       time.Time
	ok              bool
	kind            string
}

// ValidateDerivedKeys compares every family's rows with og_objects page by
// page, re-derives what differs (a missing row, a changed content_hash,
// owner or updated_at), drops orphans, and then marks the family validated.
// It holds the write connection for one page at a time and logs its progress. It is
// safe to call again, and safe against concurrent writers: a page is one
// transaction on the store's one connection, so no writer's transaction is
// split by it, and a writer that maintains the table behind the cursor or
// ahead of it leaves a row the pass finds right or repairs.
func (s *SQLStore) ValidateDerivedKeys(ctx context.Context, opt DerivedKeyValidation) error {
	if s == nil || s.db == nil || s.derived == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	d := s.derived
	if !d.tableReady.Load() {
		return fmt.Errorf("objectgraph sql: %s is not available", DerivedKeysTable)
	}
	d.passMu.Lock()
	defer d.passMu.Unlock()
	for i := range derivedFamilies {
		if err := s.validateFamily(ctx, &derivedFamilies[i], opt); err != nil {
			return err
		}
	}
	return nil
}

// passState carries one family's pass.
type passState struct {
	d                          *DerivedKeys
	fam                        *derivedFamily
	opt                        DerivedKeyValidation
	pageSize                   int
	logf                       func(format string, args ...any)
	total                      int64
	checked, rederived, orphan int64
	sawBad                     bool
	started, lastLog           time.Time
}

// page runs one unit of work, then paces and reports.
func (p *passState) page(ctx context.Context, work func() (n, fixed, dropped int, bad bool, err error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	start := time.Now()
	n, fixed, dropped, bad, err := work()
	if err != nil {
		return fmt.Errorf("objectgraph sql: validate %s: %w", p.fam.Name, err)
	}
	hold := time.Since(start)
	p.d.passPages.Add(1)
	for old := p.d.maxPageHoldNanos.Load(); int64(hold) > old && !p.d.maxPageHoldNanos.CompareAndSwap(old, int64(hold)); old = p.d.maxPageHoldNanos.Load() {
	}
	p.checked += int64(n)
	p.rederived += int64(fixed)
	p.orphan += int64(dropped)
	p.sawBad = p.sawBad || bad
	if now := time.Now(); now.Sub(p.lastLog) >= derivedKeyProgressEvery {
		p.lastLog = now
		rate := float64(p.checked) / now.Sub(p.started).Seconds()
		p.logf("derived keys %s: %d of %d rows checked (%d left), %d re-derived, %d orphans dropped, %.0f rows/s", p.fam.Name, p.checked, p.total, max(p.total-p.checked, 0), p.rederived, p.orphan, rate)
	}
	if p.opt.DutyCycle > 0 && p.opt.DutyCycle < 1 {
		pause := time.Duration(float64(hold) * (1 - p.opt.DutyCycle) / p.opt.DutyCycle)
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *SQLStore) validateFamily(ctx context.Context, fam *derivedFamily, opt DerivedKeyValidation) error {
	d := s.derived
	p := &passState{d: d, fam: fam, opt: opt, pageSize: opt.PageSize, logf: opt.Logf}
	if p.logf == nil {
		p.logf = func(format string, args ...any) { log.Printf("objectgraph sql: "+format, args...) }
	}
	if p.pageSize <= 0 {
		p.pageSize = derivedKeyDefaultPageLen
	}
	startInvalidations := d.invalidations[fam.Name].Load()
	startBadWrites := d.badWrites[fam.Name].Load()
	startWrites := d.writes[fam.Name].Load()

	readDB := s.db
	if opt.ReadDB != nil {
		readDB = opt.ReadDB
	}
	err := readDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, string(fam.Kind)).Scan(&p.total)
	if err != nil {
		return fmt.Errorf("objectgraph sql: count %s rows: %w", fam.Kind, err)
	}
	p.logf("derived keys %s: validation pass starts over %d rows (page %d)", fam.Name, p.total, p.pageSize)
	p.started, p.lastLog = time.Now(), time.Now()

	// The kind's primary-key range, a page at a time.
	lo, hi := derivedKeyRange(fam.Kind)
	cursor := lo
	for done := false; !done; {
		var last string
		if err := p.page(ctx, func() (n, fixed, dropped int, bad bool, err error) {
			if opt.ReadDB != nil {
				n, last, done, fixed, dropped, bad, err = s.validatePageOnReadPool(ctx, opt.ReadDB, fam, cursor, hi, p.pageSize)
			} else {
				n, last, done, fixed, dropped, bad, err = s.validatePage(ctx, fam, cursor, hi, p.pageSize)
			}
			return
		}); err != nil {
			return err
		}
		cursor = last
	}

	// Rows and side rows whose ids are outside the range.
	var strays []string
	if err := p.page(ctx, func() (n, fixed, dropped int, bad bool, err error) {
		strays, err = derivedStrayIDs(ctx, readDB, fam, lo, hi)
		return 0, 0, 0, false, err
	}); err != nil {
		return err
	}
	if len(strays) > 0 {
		p.logf("derived keys %s: %d rows or key rows have ids outside %q; checking them by id", fam.Name, len(strays), lo)
	}
	for start := 0; start < len(strays); start += p.pageSize {
		chunk := strays[start:min(start+p.pageSize, len(strays))]
		if err := p.page(ctx, func() (n, fixed, dropped int, bad bool, err error) {
			if opt.ReadDB != nil {
				return s.validateIDsOnReadPool(ctx, opt.ReadDB, fam, chunk)
			}
			return s.validateIDs(ctx, fam, chunk)
		}); err != nil {
			return err
		}
	}

	elapsed := time.Since(p.started).Round(time.Millisecond)
	if hook := d.beforePublish; hook != nil {
		hook(fam.Name)
	}
	// Check and publish under pubMu: a withdrawal or a bad write is either
	// counted here or comes after and undoes the publication.
	d.pubMu.Lock()
	withdrawn := d.invalidations[fam.Name].Load() != startInvalidations
	bad := p.sawBad || d.badWrites[fam.Name].Load() != startBadWrites
	if !withdrawn {
		d.bad[fam.Name].Store(bad)
		d.passWrites[fam.Name].Store(startWrites)
		d.validated[fam.Name].Store(true)
	}
	d.pubMu.Unlock()
	if withdrawn {
		p.logf("derived keys %s: pass over %d rows ended in %s but the family was withdrawn during it; it will be run again", fam.Name, p.checked, elapsed)
		return nil
	}
	p.logf("derived keys %s: validated %d rows in %s (%d re-derived, %d orphans dropped, malformed rows %t)", fam.Name, p.checked, elapsed, p.rederived, p.orphan, bad)
	return nil
}

func scanHeads(rows *sql.Rows) ([]derivedSideRow, error) {
	defer rows.Close()
	var out []derivedSideRow
	for rows.Next() {
		var h derivedSideRow
		if err := rows.Scan(&h.id, &h.owner, &h.updatedAt, &h.hash, &h.kind); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func scanSideRows(rows *sql.Rows) (map[string]derivedSideRow, []string, error) {
	defer rows.Close()
	out := map[string]derivedSideRow{}
	var order []string
	for rows.Next() {
		var r derivedSideRow
		if err := rows.Scan(&r.id, &r.owner, &r.updatedAt, &r.hash, &r.ok); err != nil {
			return nil, nil, err
		}
		out[r.id] = r
		order = append(order, r.id)
	}
	return out, order, rows.Err()
}

const sideRowColumns = `canonical_id, owner_id, updated_at, content_hash, derive_ok`

// validatePage compares one page of the kind's primary-key range (cursor,
// before). last is the page's last canonical_id; done is true when it was the
// range's last page (it then also dropped every key row past the cursor
// inside the range)..
func (s *SQLStore) validatePage(ctx context.Context, fam *derivedFamily, cursor, before string, pageSize int) (n int, last string, done bool, fixed, dropped int, bad bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	raw, heads, side, sideOrder, err := readValidationPage(ctx, tx, fam, cursor, before, pageSize)
	if err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	fixed, dropped, bad, err = s.reconcile(ctx, tx, fam, heads, side, sideOrder)
	if err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	last = cursor
	if len(raw) > 0 {
		last = raw[len(raw)-1].id
	}
	return len(heads), last, len(raw) < pageSize, fixed, dropped, bad, nil
}

// validatePageOnReadPool compares one page in a read snapshot on readDB and
// sends the ids that differ to validateIDs, which re-reads both sides on the
// write connection and repairs them. A writer that maintains the table never
// leaves a difference inside one snapshot, so a page with nothing to repair
// never takes the write connection.
func (s *SQLStore) validatePageOnReadPool(ctx context.Context, readDB *sql.DB, fam *derivedFamily, cursor, before string, pageSize int) (n int, last string, done bool, fixed, dropped int, bad bool, err error) {
	tx, err := readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	raw, heads, side, sideOrder, err := readValidationPage(ctx, tx, fam, cursor, before, pageSize)
	_ = tx.Rollback()
	if err != nil {
		return 0, cursor, false, 0, 0, false, err
	}
	redo, orphans, bad := diffSide(heads, side, sideOrder)
	ids := append(redo, orphans...)
	sort.Strings(ids)
	for start := 0; start < len(ids); start += pageSize {
		_, f, d, b, err := s.validateIDs(ctx, fam, ids[start:min(start+pageSize, len(ids))])
		if err != nil {
			return 0, cursor, false, 0, 0, false, err
		}
		fixed, dropped, bad = fixed+f, dropped+d, bad || b
	}
	last = cursor
	if len(raw) > 0 {
		last = raw[len(raw)-1].id
	}
	return len(heads), last, len(raw) < pageSize, fixed, dropped, bad, nil
}

// readValidationPage reads a page of og_objects headers in (cursor, before)
// and the family's key rows over the same ids, inside tx: raw is every header
// read, heads only the family's kind.
func readValidationPage(ctx context.Context, tx *sql.Tx, fam *derivedFamily, cursor, before string, pageSize int) (raw, heads []derivedSideRow, side map[string]derivedSideRow, sideOrder []string, err error) {
	headQuery, headArgs := ogHeadPageSQL(cursor, before, pageSize)
	rows, err := tx.QueryContext(ctx, headQuery, headArgs...)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	raw, err = scanHeads(rows)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	heads = make([]derivedSideRow, 0, len(raw))
	for _, h := range raw {
		if h.kind == string(fam.Kind) {
			heads = append(heads, h)
		}
	}
	// The key rows over the same id range; past the last head, to the end.
	sideQuery := `SELECT ` + sideRowColumns + ` FROM ` + DerivedKeysTable + `
		WHERE key_name = ? AND canonical_id > ? AND canonical_id < ?`
	sideArgs := []any{fam.Name, cursor, before}
	if len(raw) == pageSize {
		sideQuery += ` AND canonical_id <= ?`
		sideArgs = append(sideArgs, raw[len(raw)-1].id)
	}
	sideQuery += ` ORDER BY canonical_id ASC`
	srows, err := tx.QueryContext(ctx, sideQuery, sideArgs...)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	side, sideOrder, err = scanSideRows(srows)
	return raw, heads, side, sideOrder, err
}

// reconcile makes the key rows of a set of ids match og_objects: heads are the
// og_objects rows, side the key rows (in sideOrder) over the same ids. The
// caller holds the transaction.
func (s *SQLStore) reconcile(ctx context.Context, tx *sql.Tx, fam *derivedFamily, heads []derivedSideRow, side map[string]derivedSideRow, sideOrder []string) (fixed, dropped int, bad bool, err error) {
	redo, orphans, bad := diffSide(heads, side, sideOrder)
	for _, id := range orphans {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+DerivedKeysTable+` WHERE canonical_id = ? AND key_name = ?`, id, fam.Name); err != nil {
			return 0, 0, false, err
		}
		dropped++
	}
	if len(redo) > 0 {
		derivedBad, err := s.rederive(ctx, tx, fam, redo)
		if err != nil {
			return 0, 0, false, err
		}
		bad = bad || derivedBad
		fixed = len(redo)
	}
	return fixed, dropped, bad, nil
}

// diffSide compares heads (og_objects rows) with the key rows over the same
// ids: redo are heads with a missing or stale key row, orphans key rows with
// no head, and bad reports a key row marked malformed.
func diffSide(heads []derivedSideRow, side map[string]derivedSideRow, sideOrder []string) (redo, orphans []string, bad bool) {
	present := make(map[string]bool, len(heads))
	for _, h := range heads {
		present[h.id] = true
		r, have := side[h.id]
		if have && !r.ok {
			bad = true
		}
		if !have || r.owner != h.owner || r.hash != h.hash || !r.updatedAt.Equal(h.updatedAt) {
			redo = append(redo, h.id)
		}
	}
	for _, id := range sideOrder {
		if !present[id] {
			orphans = append(orphans, id)
		}
	}
	return redo, orphans, bad
}

// derivedStrayIDs lists the ids outside the kind's primary-key range: rows of
// the kind, and key rows of the family..
func derivedStrayIDs(ctx context.Context, db *sql.DB, fam *derivedFamily, lo, hi string) ([]string, error) {
	seen := map[string]bool{}
	for _, q := range []struct {
		query string
		args  []any
	}{
		{`SELECT canonical_id FROM og_objects WHERE object_kind = ? AND (canonical_id <= ? OR canonical_id >= ?)`, []any{string(fam.Kind), lo, hi}},
		{`SELECT canonical_id FROM ` + DerivedKeysTable + ` WHERE key_name = ? AND (canonical_id <= ? OR canonical_id >= ?)`, []any{fam.Name, lo, hi}},
	} {
		rows, err := db.QueryContext(ctx, q.query, q.args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			seen[id] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// validateIDs reconciles a chunk of ids by key, in one transaction that
// re-reads both sides..
func (s *SQLStore) validateIDs(ctx context.Context, fam *derivedFamily, ids []string) (n, fixed, dropped int, bad bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, false, err
	}
	defer func() { _ = tx.Rollback() }()
	heads, side, sideOrder, err := readValidationIDs(ctx, tx, fam, ids)
	if err != nil {
		return 0, 0, 0, false, err
	}
	fixed, dropped, bad, err = s.reconcile(ctx, tx, fam, heads, side, sideOrder)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, 0, false, err
	}
	return len(heads), fixed, dropped, bad, nil
}

// validateIDsOnReadPool compares a chunk of ids in a read snapshot on readDB
// and repairs only the ids that differ, through validateIDs.
func (s *SQLStore) validateIDsOnReadPool(ctx context.Context, readDB *sql.DB, fam *derivedFamily, ids []string) (n, fixed, dropped int, bad bool, err error) {
	tx, err := readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, 0, 0, false, err
	}
	heads, side, sideOrder, err := readValidationIDs(ctx, tx, fam, ids)
	_ = tx.Rollback()
	if err != nil {
		return 0, 0, 0, false, err
	}
	redo, orphans, bad := diffSide(heads, side, sideOrder)
	if diff := append(redo, orphans...); len(diff) > 0 {
		sort.Strings(diff)
		_, f, d, b, err := s.validateIDs(ctx, fam, diff)
		if err != nil {
			return 0, 0, 0, false, err
		}
		fixed, dropped, bad = f, d, bad || b
	}
	return len(heads), fixed, dropped, bad, nil
}

// readValidationIDs reads the og_objects headers of the family's kind and the
// family's key rows for ids, inside tx.
func readValidationIDs(ctx context.Context, tx *sql.Tx, fam *derivedFamily, ids []string) (heads []derivedSideRow, side map[string]derivedSideRow, sideOrder []string, err error) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, `SELECT canonical_id, owner_id, updated_at, content_hash, object_kind FROM og_objects
		WHERE object_kind = '`+string(fam.Kind)+`' AND canonical_id IN (`+marks+`) ORDER BY canonical_id ASC`, args...)
	if err != nil {
		return nil, nil, nil, err
	}
	heads, err = scanHeads(rows)
	if err != nil {
		return nil, nil, nil, err
	}
	srows, err := tx.QueryContext(ctx, `SELECT `+sideRowColumns+` FROM `+DerivedKeysTable+`
		WHERE key_name = ? AND canonical_id IN (`+marks+`) ORDER BY canonical_id ASC`, append([]any{fam.Name}, args...)...)
	if err != nil {
		return nil, nil, nil, err
	}
	side, sideOrder, err = scanSideRows(srows)
	return heads, side, sideOrder, err
}

// rederive derives the given rows in one statement per batch, falling back to
// one statement per row when the batch holds a row whose expression errors.
// It reports whether any row could not be derived.
func (s *SQLStore) rederive(ctx context.Context, tx *sql.Tx, fam *derivedFamily, ids []string) (bad bool, err error) {
	for start := 0; start < len(ids); start += snapshotFetchBatch {
		batch := ids[start:min(start+snapshotFetchBatch, len(ids))]
		marks := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, 0, len(batch)+2)
		args = append(args, fam.Name, string(fam.Kind))
		for _, id := range batch {
			args = append(args, id)
		}
		if _, err := tx.ExecContext(ctx, deriveSQL(fam, `canonical_id IN (`+marks+`)`, true), args...); err == nil {
			continue
		}
		for _, id := range batch {
			if _, err := tx.ExecContext(ctx, deriveSQL(fam, `canonical_id = ?`, true), fam.Name, string(fam.Kind), id); err == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, deriveSQL(fam, `canonical_id = ?`, false), fam.Name, string(fam.Kind), id); err != nil {
				return false, fmt.Errorf("derive %s: %w", id, err)
			}
			bad = true
		}
	}
	return bad, nil
}
