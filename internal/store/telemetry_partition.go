package store

// Telemetry in time-partitioned SQLite files (docs/design/one-tape-durability-2026-10-31.md
// section 6c). Retiring a window of telemetry by deleting rows leaves free
// pages and churns pages across the whole state file, so no two snapshots
// share chunks. A partition is a small SQLite file holding one class of
// telemetry for one UTC day. Retiring the day is a tape op that marks the
// partition retired in the main store (and copies the rows a run acceptance
// cites into choir_events); the file is then deleted. A closed partition is
// never written again, so its snapshot chunks are the same bytes in every
// snapshot.
//
// The rules, each a failure mode of section 6c:
//
//   - The tape decides. The writer stamps the op (ProjectionOp.Table
//     telemetryPartitionStamp); the projector routes on the stamp, and the
//     partition is a function of the op alone: the class of its event kind
//     and the UTC day of the object's updated_at, never the wall clock.
//   - A partition commits before the main store. The rows of a batch are
//     written to their partitions (synchronous=FULL) before the batch's main
//     transaction commits, so a head in the main store implies its rows in
//     the partitions. A crash between the two leaves rows above the head;
//     Open deletes them (rows above the head of the state file are the
//     effects of events the file has not applied: a restore, a compaction
//     rebase or a crash), and the replay writes them again.
//   - Retired is main-store state. The retire op records the partition in
//     telemetry_partitions_retired in the main transaction; readers, the
//     snapshot and replay all read that, and a row whose partition is
//     retired lands in choir_events. The file deletion (gcTelemetryPartitions)
//     is not state: it runs after, and never while a snapshot is capturing
//     or a compact copy is staged (the copy's head may be below the retire).
//   - A partial retire (an op whose slice or phase does not cover the
//     partition) deletes rows inside the partition, with the old op's exact
//     semantics; cited rows are never deleted from a partition.
//
// Classes: tool.result only (the first slice). A kind this binary does not
// partition, stamped by a later binary, lands in choir_events.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// telemetryPartitionStamp is the ProjectionOp.Table of a partitioned op.
	telemetryPartitionStamp = "telemetry_partition"
	// TelemetryPartitionDirName is the directory of the partition files
	// inside the store's sidecar workspace, so every path that moves the
	// workspace (rematerialize's flip, quarantine) moves them with it.
	TelemetryPartitionDirName = "telemetry"
	// telemetryPartitionDay is the span of one partition.
	telemetryPartitionDay = 24 * time.Hour
	// telemetryPartitionFloorEpoch is the store floor once a partition may
	// hold a row: an older binary neither reads nor retires partitions, so
	// its replay and its witness would disagree with this one.
	telemetryPartitionFloorEpoch  uint64 = 5
	telemetryPartitionFloorReason        = "telemetry partitions: epoch 5 is the floor (an older binary does not read or retire them)"
)

// Compile-time guard: the build's epoch must reach the floor it writes.
const _ = storeschema.Version - telemetryPartitionFloorEpoch

// telemetryPartitionClasses maps a partitioned event kind to its class.
var telemetryPartitionClasses = map[types.EventKind]string{
	types.EventToolResult: "tool_result",
}

// telemetryPartitionClass is the class of an event kind, or "" when this
// binary does not partition it.
func telemetryPartitionClass(kind types.EventKind) string {
	return telemetryPartitionClasses[kind]
}

// telemetryPartitionClassKinds is the event kinds a class holds.
func telemetryPartitionClassKinds(class string) []string {
	var out []string
	for kind, c := range telemetryPartitionClasses {
		if c == class {
			out = append(out, string(kind))
		}
	}
	sort.Strings(out)
	return out
}

// telemetryPartitionDayOf is the UTC day that holds t.
func telemetryPartitionDayOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// telemetryPartitionName names the partition of class for the day holding t.
func telemetryPartitionName(class string, t time.Time) string {
	return class + "-" + telemetryPartitionDayOf(t).Format("20060102")
}

// parseTelemetryPartitionName is the inverse of telemetryPartitionName.
func parseTelemetryPartitionName(name string) (class string, day time.Time, ok bool) {
	i := strings.LastIndexByte(name, '-')
	if i <= 0 || i == len(name)-1 {
		return "", time.Time{}, false
	}
	day, err := time.Parse("20060102", name[i+1:])
	if err != nil {
		return "", time.Time{}, false
	}
	class = name[:i]
	if len(telemetryPartitionClassKinds(class)) == 0 {
		return "", time.Time{}, false
	}
	return class, day.UTC(), true
}

// TelemetryPartitionDir is the partition directory of the store at storePath
// (the path Open takes). Restore installs a snapshot's partitions there.
func TelemetryPartitionDir(storePath string) string {
	return filepath.Join(deriveTextureWorkspacePath(storePath), TelemetryPartitionDirName)
}

// telemetryPartitionDDL is a partition's schema: choir_events' columns and
// indexes, so every choir_events reader runs unchanged on a partition.
const telemetryPartitionDDL = `
CREATE TABLE IF NOT EXISTS choir_events (
	tape_seq INTEGER NOT NULL,
	op_index INTEGER NOT NULL,
	canonical_id TEXT NOT NULL,
	object_kind TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	version_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	body          BLOB NOT NULL,
	metadata TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL DEFAULT 0,
	stream_seq INTEGER NOT NULL DEFAULT 0,
	trajectory_id TEXT NOT NULL DEFAULT '',
	event_kind TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (tape_seq, op_index)
);
CREATE INDEX IF NOT EXISTS idx_choir_events_run_seq ON choir_events(run_id, seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_owner_stream ON choir_events(owner_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_trajectory_stream ON choir_events(trajectory_id, stream_seq);
`

// telemetryPartition is one open partition file.
type telemetryPartition struct {
	name  string
	class string
	day   time.Time
	path  string

	// mu: readers and the snapshot hold it shared while they use db; the
	// file deletion holds it exclusively and sets closed.
	mu     sync.RWMutex
	closed bool
	db     *dbx.SQLiteDB
}

// telemetryPartitions is a store's set of partitions.
type telemetryPartitions struct {
	dir string

	// gcMu: a snapshot capture holds it shared from before its read
	// transaction until its last partition is read, so no partition the
	// capture's head calls live is deleted under it. The file deletion holds
	// it exclusively. Readers never take it.
	gcMu sync.RWMutex

	mu      sync.Mutex
	open    map[string]*telemetryPartition
	retired map[string]struct{}
}

func newTelemetryPartitions(dir string) *telemetryPartitions {
	return &telemetryPartitions{dir: dir, open: map[string]*telemetryPartition{}, retired: map[string]struct{}{}}
}

// openTelemetryPartitions opens the partitions of the store at Open: the
// retired set from the catalog, then every partition file that is not
// retired, with its rows above the state file's head deleted. It is a
// directory listing and one indexed delete per file: light.
func openTelemetryPartitions(ctx context.Context, mainDB *sql.DB, workspace string) (*telemetryPartitions, error) {
	set := newTelemetryPartitions(filepath.Join(workspace, TelemetryPartitionDirName))
	rows, err := mainDB.QueryContext(ctx, `SELECT name FROM telemetry_partitions_retired`)
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: read the retired set: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		set.retired[name] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(set.dir)
	if errors.Is(err, os.ErrNotExist) {
		return set, nil
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: list %s: %w", set.dir, err)
	}
	var head uint64
	if err := mainDB.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM computer_event_projection_heads`).Scan(&head); err != nil {
		return nil, fmt.Errorf("telemetry partitions: read the head: %w", err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db") {
			present[strings.TrimSuffix(e.Name(), ".db")] = true
		}
	}
	for _, e := range entries {
		fname := e.Name()
		switch {
		case strings.HasSuffix(fname, ".db"):
		case strings.HasSuffix(fname, ".db-wal"), strings.HasSuffix(fname, ".db-shm"):
			// A WAL or shared-memory file whose database is gone is what an
			// interrupted deletion leaves.
			base := strings.TrimSuffix(strings.TrimSuffix(fname, "-wal"), "-shm")
			if !present[strings.TrimSuffix(base, ".db")] {
				_ = os.Remove(filepath.Join(set.dir, fname))
			}
			continue
		default:
			continue
		}
		name := strings.TrimSuffix(fname, ".db")
		class, day, ok := parseTelemetryPartitionName(name)
		if !ok {
			continue
		}
		if _, gone := set.retired[name]; gone {
			continue // deleted by the next gcTelemetryPartitions
		}
		p, err := set.openFile(ctx, name, class, day, false)
		if err != nil {
			set.close()
			return nil, err
		}
		res, err := p.db.Write.ExecContext(ctx, `DELETE FROM choir_events WHERE tape_seq > ?`, head)
		if err != nil {
			set.close()
			return nil, fmt.Errorf("telemetry partitions: drop rows above head %d in %s: %w", head, name, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("store: telemetry partition %s: dropped %d rows above the head %d (written before a commit the state file does not hold)", name, n, head)
		}
	}
	return set, nil
}

// openFile opens (create: creates) the partition file name and registers it.
func (set *telemetryPartitions) openFile(ctx context.Context, name, class string, day time.Time, create bool) (*telemetryPartition, error) {
	path := filepath.Join(set.dir, name+".db")
	if create {
		// The floor before the first file: a replay or restore into a fresh
		// workspace creates partitions after Open's floor check.
		if err := writeTelemetryPartitionFloorAt(filepath.Dir(set.dir)); err != nil {
			return nil, fmt.Errorf("telemetry partitions: floor: %w", err)
		}
		if err := os.MkdirAll(set.dir, 0o755); err != nil {
			return nil, fmt.Errorf("telemetry partitions: create %s: %w", set.dir, err)
		}
	}
	db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 2})
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: open %s: %w", name, err)
	}
	if _, err := db.Write.ExecContext(ctx, telemetryPartitionDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("telemetry partitions: schema of %s: %w", name, err)
	}
	if create {
		// The new file's directory entry is durable before any main commit
		// names a row in it.
		if d, err := os.Open(set.dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	p := &telemetryPartition{name: name, class: class, day: day, path: path, db: db}
	set.mu.Lock()
	set.open[name] = p
	set.mu.Unlock()
	return p, nil
}

// get returns the open partition name, opening or creating it on a write.
func (set *telemetryPartitions) get(ctx context.Context, name string) (*telemetryPartition, error) {
	set.mu.Lock()
	p := set.open[name]
	set.mu.Unlock()
	if p != nil {
		return p, nil
	}
	class, day, ok := parseTelemetryPartitionName(name)
	if !ok {
		return nil, fmt.Errorf("telemetry partitions: unusable name %q", name)
	}
	return set.openFile(ctx, name, class, day, true)
}

// has reports whether the partition is open (holds a file).
func (set *telemetryPartitions) has(name string) bool {
	set.mu.Lock()
	defer set.mu.Unlock()
	return set.open[name] != nil
}

// live returns the open partitions that are not retired, in name order.
func (set *telemetryPartitions) live() []*telemetryPartition {
	if set == nil {
		return nil
	}
	set.mu.Lock()
	defer set.mu.Unlock()
	out := make([]*telemetryPartition, 0, len(set.open))
	for name, p := range set.open {
		if _, gone := set.retired[name]; !gone {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// markRetired records in memory what a committed retire op recorded.
func (set *telemetryPartitions) markRetired(names []string) {
	set.mu.Lock()
	defer set.mu.Unlock()
	for _, n := range names {
		set.retired[n] = struct{}{}
	}
}

func (set *telemetryPartitions) close() {
	if set == nil {
		return
	}
	set.mu.Lock()
	defer set.mu.Unlock()
	for name, p := range set.open {
		p.mu.Lock()
		if !p.closed {
			_, _ = p.db.Write.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
			_ = p.db.Close()
			p.closed = true
		}
		p.mu.Unlock()
		delete(set.open, name)
	}
}

// withRead runs fn on the partition's read pool unless the partition was
// deleted; skipped reports that.
func (p *telemetryPartition) withRead(fn func(db *sql.DB) error) (skipped bool, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return true, nil
	}
	return false, fn(p.db.Read)
}

// --- the projection side ---------------------------------------------------

// partitionFlushedHook, set only by tests, runs in finalize after the
// partitions committed and before the main transaction commits: an error
// there is a crash between the two.
var partitionFlushedHook func(sequence uint64) error

// partitionBatch collects one projection batch's partition writes; they are
// committed by flush, before the batch's main transaction.
type partitionBatch struct {
	set *telemetryPartitions
	// dry: validate, write nothing (the dry run).
	dry     bool
	actions []partitionAction
	retired []string
}

// partitionAction is one write to one partition, kept in op order.
type partitionAction struct {
	name   string
	insert *choirEventRow
	pos    uint64
	index  int
	where  string
	args   []any
}

// newPartitionBatch returns the collector for one projection batch.
func (s *Store) newPartitionBatch(dry bool) *partitionBatch {
	return &partitionBatch{set: s.partitions, dry: dry}
}

// flush commits the collected writes, one transaction per partition, in op
// order within each.
func (pb *partitionBatch) flush(ctx context.Context) error {
	if pb == nil || pb.dry || len(pb.actions) == 0 {
		return nil
	}
	if pb.set == nil {
		return fmt.Errorf("telemetry partitions: the store has none open")
	}
	var order []string
	byName := map[string][]partitionAction{}
	for _, a := range pb.actions {
		if _, seen := byName[a.name]; !seen {
			order = append(order, a.name)
		}
		byName[a.name] = append(byName[a.name], a)
	}
	for _, name := range order {
		if err := pb.flushOne(ctx, name, byName[name]); err != nil {
			return err
		}
	}
	pb.actions = nil
	return nil
}

func (pb *partitionBatch) flushOne(ctx context.Context, name string, actions []partitionAction) error {
	p, err := pb.set.get(ctx, name)
	if err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return fmt.Errorf("telemetry partitions: %s was deleted under a write", name)
	}
	tx, err := p.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("telemetry partitions: begin %s: %w", name, err)
	}
	defer tx.Rollback()
	for _, a := range actions {
		if a.insert != nil {
			if err := insertChoirEventRow(ctx, tx, a.pos, a.index, *a.insert, false); err != nil {
				return fmt.Errorf("telemetry partitions: write %s: %w", name, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM choir_events WHERE `+a.where, a.args...); err != nil {
			return fmt.Errorf("telemetry partitions: retire in %s: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("telemetry partitions: commit %s: %w", name, err)
	}
	return nil
}

// flushPartition commits the pending writes of one partition now (a retire
// op that reads it must see them).
func (pb *partitionBatch) flushPartition(ctx context.Context, name string) error {
	if pb == nil || pb.dry {
		return nil
	}
	var mine, rest []partitionAction
	for _, a := range pb.actions {
		if a.name == name {
			mine = append(mine, a)
		} else {
			rest = append(rest, a)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	if err := pb.flushOne(ctx, name, mine); err != nil {
		return err
	}
	pb.actions = rest
	return nil
}

func (pb *partitionBatch) pending(name string) bool {
	for _, a := range pb.actions {
		if a.name == name {
			return true
		}
	}
	return false
}

// committed applies to memory what the committed main transaction recorded.
func (pb *partitionBatch) committed() {
	if pb == nil || pb.set == nil || len(pb.retired) == 0 {
		return
	}
	pb.set.markRetired(pb.retired)
}

// partitionRetiredTx reports whether the main store records name retired.
func partitionRetiredTx(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM telemetry_partitions_retired WHERE name = ?`, name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// projectTelemetryPartitionEvent projects one op stamped for a partition. It
// refuses what choir_events refuses; a kind this binary does not partition,
// or a row whose partition is retired, lands in choir_events.
func projectTelemetryPartitionEvent(ctx context.Context, tx *sql.Tx, body json.RawMessage, pos uint64, opIndex int, pb *partitionBatch) error {
	row, err := decodeChoirEventRow(body, pos, opIndex)
	if err != nil {
		return err
	}
	class := telemetryPartitionClass(types.EventKind(row.eventKind))
	if class == "" {
		return insertChoirEventRow(ctx, tx, pos, opIndex, row, false)
	}
	name := telemetryPartitionName(class, row.updatedAt)
	retired, err := partitionRetiredTx(ctx, tx, name)
	if err != nil {
		return fmt.Errorf("telemetry partitions: read the retired set: %w", err)
	}
	if retired {
		return insertChoirEventRow(ctx, tx, pos, opIndex, row, false)
	}
	if pb == nil {
		return fmt.Errorf("telemetry partitions: a partition op needs the partition writer")
	}
	if pb.dry {
		return nil
	}
	pb.actions = append(pb.actions, partitionAction{name: name, insert: &row, pos: pos, index: opIndex})
	return nil
}

// --- retirement --------------------------------------------------------------

// citedEventIDsTx is the event ids a stored run acceptance cites.
func citedEventIDsTx(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT CAST(json_extract(e.value, '$.event_id') AS TEXT)
		FROM og_objects ra, json_each(CASE WHEN json_valid(CAST(ra.body AS TEXT)) THEN CAST(ra.body AS TEXT) ELSE '{}' END, '$.evidence_refs') e
		WHERE ra.object_kind = '`+string(ogKindRunAccept)+`' AND json_extract(e.value, '$.event_id') IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// partitionRetireWhere is a retire op's selection inside one partition: the
// choir_events predicate, with the cited set as a parameter (the partition
// has no og_objects to read it from).
func partitionRetireWhere(p computerevent.ObjectRetireProjection, from, before time.Time, cited []string) (string, []any, error) {
	marks, kindArgs := retireKindPlaceholders(p)
	where := `updated_at >= ? AND updated_at < ? AND event_kind IN (` + marks + `)` + retirePhasePredicate(p, "body", "metadata")
	args := append([]any{from, before}, kindArgs...)
	if p.KeepCited == computerevent.ObjectRetireKeepRunAcceptance {
		raw, err := json.Marshal(append([]string{}, cited...))
		if err != nil {
			return "", nil, err
		}
		where += ` AND COALESCE(CASE WHEN json_valid(metadata) THEN CAST(json_extract(metadata, '$.event_id') AS TEXT) END, '') NOT IN (SELECT value FROM json_each(?))`
		args = append(args, string(raw))
	}
	return where, args, nil
}

// retirePartitions applies a retire op to the partitions it reaches. A
// partition the op covers whole (its day inside the slice, every phase, all
// its kinds named) is retired: the cited rows move to choir_events in the
// main transaction, and the partition is recorded retired there. A partition
// it covers in part has the matching rows deleted, cited rows kept. It runs
// in finalize only (the dry run applies no retire op).
func retirePartitions(ctx context.Context, tx *sql.Tx, p computerevent.ObjectRetireProjection, from, before time.Time, pos uint64, pb *partitionBatch) (int64, error) {
	if pb == nil || pb.set == nil || pb.dry {
		return 0, nil
	}
	named := map[string]bool{}
	for _, k := range p.EventKinds {
		named[k] = true
	}
	var cited []string
	citedRead := false
	var removed int64
	classes := map[string]bool{}
	for _, c := range telemetryPartitionClasses {
		classes[c] = true
	}
	for class := range classes {
		kinds := telemetryPartitionClassKinds(class)
		reaches, all := false, true
		for _, k := range kinds {
			if named[k] {
				reaches = true
			} else {
				all = false
			}
		}
		if !reaches {
			continue
		}
		for day := telemetryPartitionDayOf(from); day.Before(before); day = day.Add(telemetryPartitionDay) {
			name := telemetryPartitionName(class, day)
			if !pb.set.has(name) && !pb.pending(name) {
				continue
			}
			retired, err := partitionRetiredTx(ctx, tx, name)
			if err != nil {
				return removed, err
			}
			if retired {
				continue
			}
			if p.KeepCited == computerevent.ObjectRetireKeepRunAcceptance && !citedRead {
				if cited, err = citedEventIDsTx(ctx, tx); err != nil {
					return removed, fmt.Errorf("telemetry partitions: read the cited events: %w", err)
				}
				citedRead = true
			}
			whole := !from.After(day) && !before.Before(day.Add(telemetryPartitionDay)) && all && p.Phase == computerevent.ObjectRetirePhaseAny
			if !whole {
				where, args, err := partitionRetireWhere(p, from, before, cited)
				if err != nil {
					return removed, err
				}
				pb.actions = append(pb.actions, partitionAction{name: name, where: where, args: args})
				continue
			}
			if err := pb.flushPartition(ctx, name); err != nil {
				return removed, err
			}
			moved, err := moveCitedRows(ctx, tx, pb.set, name, cited, p.KeepCited == computerevent.ObjectRetireKeepRunAcceptance)
			if err != nil {
				return removed, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO telemetry_partitions_retired (name, class, day, tape_seq, cited_moved) VALUES (?, ?, ?, ?, ?)`,
				name, class, day.Format("2006-01-02"), pos, moved); err != nil {
				return removed, fmt.Errorf("telemetry partitions: record %s retired: %w", name, err)
			}
			pb.retired = append(pb.retired, name)
			log.Printf("store: telemetry partition %s retired at %d (%d cited rows kept in %s)", name, pos, moved, choirEventsTable)
			removed++
		}
	}
	return removed, nil
}

// moveCitedRows copies the rows of partition name that a run acceptance
// cites into choir_events, in the main transaction.
func moveCitedRows(ctx context.Context, tx *sql.Tx, set *telemetryPartitions, name string, cited []string, keep bool) (int, error) {
	if !keep || len(cited) == 0 || !set.has(name) {
		return 0, nil
	}
	p, err := set.get(ctx, name)
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(cited)
	if err != nil {
		return 0, err
	}
	var rows []choirEventStoredRow
	skipped, err := p.withRead(func(db *sql.DB) error {
		var qerr error
		rows, qerr = readStoredRows(ctx, db, `WHERE COALESCE(CASE WHEN json_valid(metadata) THEN CAST(json_extract(metadata, '$.event_id') AS TEXT) END, '') IN (SELECT value FROM json_each(?))`, string(raw))
		return qerr
	})
	if err != nil {
		return 0, fmt.Errorf("telemetry partitions: read cited rows of %s: %w", name, err)
	}
	if skipped {
		return 0, fmt.Errorf("telemetry partitions: %s was deleted before its retirement", name)
	}
	for _, r := range rows {
		if err := insertChoirEventRow(ctx, tx, r.pos, r.index, r.row, true); err != nil {
			return 0, fmt.Errorf("telemetry partitions: keep a cited row of %s: %w", name, err)
		}
	}
	return len(rows), nil
}

// choirEventStoredRow is a row read back with its tape position.
type choirEventStoredRow struct {
	pos   uint64
	index int
	row   choirEventRow
}

func readStoredRows(ctx context.Context, db *sql.DB, where string, args ...any) ([]choirEventStoredRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata,
		created_at, updated_at, run_id, seq, stream_seq, trajectory_id, event_kind FROM choir_events `+where+` ORDER BY tape_seq, op_index`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []choirEventStoredRow
	for rows.Next() {
		var r choirEventStoredRow
		if err := rows.Scan(&r.pos, &r.index, &r.row.canonicalID, &r.row.objectKind, &r.row.ownerID, &r.row.computerID, &r.row.versionID,
			&r.row.contentHash, &r.row.body, &r.row.metadata, &r.row.createdAt, &r.row.updatedAt, &r.row.runID, &r.row.seq, &r.row.streamSeq,
			&r.row.trajectoryID, &r.row.eventKind); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- file deletion -------------------------------------------------------------

// GCTelemetryPartitions deletes the files of retired partitions. It is not
// state: the retire op recorded the retirement; this returns the bytes. It
// waits for a snapshot capture, and does nothing while a compact copy is
// staged (its head may be below a retirement, and its rebase replays the
// retire op, which needs the file). It returns the files deleted.
func (s *Store) GCTelemetryPartitions(ctx context.Context) (int, error) {
	if s == nil || s.partitions == nil {
		return 0, nil
	}
	if s.statePath != "" {
		for _, suffix := range []string{compactStagedSuffix, compactStagingSuffix} {
			if _, err := os.Stat(s.statePath + suffix); err == nil {
				return 0, nil
			}
		}
	}
	set := s.partitions
	set.gcMu.Lock()
	defer set.gcMu.Unlock()
	set.mu.Lock()
	retired := make(map[string]struct{}, len(set.retired))
	for n := range set.retired {
		retired[n] = struct{}{}
	}
	var victims []*telemetryPartition
	for name, p := range set.open {
		if _, gone := retired[name]; gone {
			victims = append(victims, p)
			delete(set.open, name)
		}
	}
	set.mu.Unlock()
	deleted := 0
	for _, p := range victims {
		p.mu.Lock()
		if !p.closed {
			_ = p.db.Close()
			p.closed = true
		}
		p.mu.Unlock()
	}
	// Every retired file, open or not (Open does not open retired ones).
	entries, err := os.ReadDir(set.dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".db")
		if name == e.Name() {
			continue
		}
		if _, gone := retired[name]; !gone {
			continue
		}
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		// The database first: a WAL left without it is removed at the next
		// Open or deletion.
		base := filepath.Join(set.dir, e.Name())
		if err := os.Remove(base); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}
		_ = os.Remove(base + "-wal")
		_ = os.Remove(base + "-shm")
		deleted++
	}
	if deleted > 0 {
		if d, err := os.Open(set.dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
		log.Printf("store: telemetry partitions: deleted %d retired files", deleted)
	}
	return deleted, nil
}

// --- the writer switch ---------------------------------------------------------

// SetTelemetryPartitionWrites applies store.telemetry_partitions to the
// writer. On writes the store floor first (an older binary would neither read
// nor retire the partitions); a store with no tape refuses.
func (s *Store) SetTelemetryPartitionWrites(on bool) error {
	if s == nil {
		return nil
	}
	if on {
		if s.projectionTape == nil {
			s.partitionWrites.Store(false)
			return fmt.Errorf("store: telemetry partition writer stays off: the projection tape is not bound")
		}
		if err := writeTelemetryPartitionFloorAt(s.texturePath); err != nil {
			s.partitionWrites.Store(false)
			return fmt.Errorf("store: telemetry partition writer stays off: %w", err)
		}
	}
	s.partitionWrites.Store(on)
	return nil
}

// TelemetryPartitionWritesOn reports whether the writer stamps now.
func (s *Store) TelemetryPartitionWritesOn() bool {
	return s != nil && s.partitionWrites.Load() && s.EventTableLeaseReady() == nil
}

// stampsTelemetryPartition reports whether an event of kind is stamped for a
// partition now: the switch on, the stream_seq lease ready on the tape (the
// same gate as choir_events: the exact stream seed reads every table), and
// the kind partitioned. The caller holds eventMu.
func (s *Store) stampsTelemetryPartition(kind types.EventKind) bool {
	return s.projectionTape != nil && s.partitionWrites.Load() &&
		s.lease.state == leaseReady && s.lease.onTape.Load() &&
		telemetryPartitionClass(kind) != ""
}

func writeTelemetryPartitionFloorAt(workspace string) error {
	if workspace == "" {
		return nil
	}
	return storeschema.WriteFloor(workspace, telemetryPartitionFloorEpoch, telemetryPartitionFloorReason)
}

// ensureTelemetryPartitionFloor floors a workspace that holds a partition
// file (a restore or replay into a fresh store).
func (s *Store) ensureTelemetryPartitionFloor() error {
	if s.partitions == nil {
		return nil
	}
	entries, err := os.ReadDir(s.partitions.dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db") {
			return writeTelemetryPartitionFloorAt(s.texturePath)
		}
	}
	return nil
}

// TelemetryPartitionStats is the partitions as /api/runtime/observability
// shows them.
type TelemetryPartitionStats struct {
	Writing bool     `json:"writing"`
	Live    []string `json:"live,omitempty"`
	Retired int      `json:"retired"`
}

// TelemetryPartitionStats reports the partitions.
func (s *Store) TelemetryPartitionStats() TelemetryPartitionStats {
	out := TelemetryPartitionStats{Writing: s.TelemetryPartitionWritesOn()}
	if s == nil || s.partitions == nil {
		return out
	}
	for _, p := range s.partitions.live() {
		out.Live = append(out.Live, p.name)
	}
	s.partitions.mu.Lock()
	out.Retired = len(s.partitions.retired)
	s.partitions.mu.Unlock()
	return out
}

// --- the snapshot side -----------------------------------------------------------

// TelemetrySnapshotFile is one partition a snapshot streams: its path in the
// snapshot (TelemetryPartitionDirName/<name>.db) and a read pool on it.
type TelemetrySnapshotFile struct {
	Path string
	DB   *sql.DB
}

// HoldTelemetryPartitions keeps every partition file in place until release:
// the snapshot calls it before its read transaction begins, so a partition
// its head calls live cannot be deleted before it is read.
func (s *Store) HoldTelemetryPartitions() (release func()) {
	if s == nil || s.partitions == nil {
		return func() {}
	}
	s.partitions.gcMu.RLock()
	return s.partitions.gcMu.RUnlock
}

// TelemetrySnapshotFiles lists, inside the snapshot's read transaction, the
// partitions its head calls live. Call it between HoldTelemetryPartitions
// and its release.
func (s *Store) TelemetrySnapshotFiles(ctx context.Context, tx *sql.Tx) ([]TelemetrySnapshotFile, error) {
	if s == nil || s.partitions == nil {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT name FROM telemetry_partitions_retired`)
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: read the retired set: %w", err)
	}
	retiredAtHead := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		retiredAtHead[name] = true
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	s.partitions.mu.Lock()
	defer s.partitions.mu.Unlock()
	var out []TelemetrySnapshotFile
	for name, p := range s.partitions.open {
		if retiredAtHead[name] {
			continue
		}
		out = append(out, TelemetrySnapshotFile{Path: TelemetryPartitionDirName + "/" + name + ".db", DB: p.db.Read})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
