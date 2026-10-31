package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Real deletion of a destroyed computer. The destroyed sweep (tape_sweep.go)
// used to remove only the loose tape files. It now also removes, for a
// computer it has proven destroyed:
//
//   - tape-seg/<computer>/ (repacked segments);
//   - sha256/file-cas-chunks/<computer>/ and sha256/file-cas-roots/<computer>/;
//   - the computer's rows in every per-computer table (tapePurgeTables):
//     append receipts, file and snapshot roots, lifecycle, projection and
//     checkpoint rows, and its key escrow entries (the crypto-shred);
//
// and keeps the head row and the tombstone: they are the reuse guard (a
// reused ComputerID cannot start a second genesis, readers fail closed) and
// hold no content.
//
// Order is the crash-safety argument. Files go first, then the directories,
// then every row (escrow included) in ONE transaction, then the tombstone
// turns "deleted". A crash leaves files gone and rows intact, which the same
// manifest finishes; it can never leave rows gone with files kept, and the
// escrow row (the key) goes only with the last step. The rows transaction
// removes exactly the manifest's counts or rolls back.
//
// docs: owner direction 2026-10-31, "real deletion for our test VMs, and for
// users who hit the delete option".

const (
	tapeNSSeg           = "tape-seg"
	tapeNSFileCASChunks = "file-cas-chunks"
	tapeNSFileCASRoots  = "file-cas-roots"
)

// tapeSweepProtectedComputers is the hard refusal list: the paper and the
// owner's computer. No request field, proof, manifest or flag overrides it.
var tapeSweepProtectedComputers = map[string]string{
	"computer-1355b83bced1613eb048ee28d66688a8": "the paper computer",
	"computer-03335285269bdba4f94377e56879f9e6": "the owner computer",
}

// tapeSweepProtected returns an error for a computer on the hard refusal list.
func tapeSweepProtected(computerID string) error {
	if what, ok := tapeSweepProtectedComputers[strings.TrimSpace(computerID)]; ok {
		return fmt.Errorf("tape sweep: computer %s is protected (%s); it is never deleted", computerID, what)
	}
	return nil
}

// tapePurgeTable is one per-computer table the purge empties. where holds a
// single placeholder for the computer id. secret tables hold key material and
// never go into an archive.
type tapePurgeTable struct {
	name   string
	where  string
	secret bool
}

// tapePurgeTables lists every per-computer table except the kept ones. The
// order is the delete order (approvals read requests, so they go first).
var tapePurgeTables = []tapePurgeTable{
	{name: "computer_event_append_receipts", where: "computer_id=?"},
	{name: "control_key_history", where: "computer_id=?"},
	{name: "computer_self_development_modes", where: "computer_id=?"},
	{name: "computer_lifecycle_operations", where: "computer_id=?"},
	{name: "computer_lifecycle_receipts", where: "computer_id=?"},
	{name: "computer_checkpoints", where: "computer_id=?"},
	{name: "computer_route_projection_certificates", where: "computer_id=?"},
	{name: "computer_file_roots", where: "computer_id=?"},
	{name: "computer_snapshot_roots", where: "computer_id=?"},
	{name: "computer_replay_watermarks", where: "computer_id=?"},
	{name: "computer_projection_jobs", where: "computer_id=?"},
	{name: "computer_projection_base_refusals", where: "computer_id=?"},
	{name: "computer_projection_bases", where: "computer_id=?"},
	{name: "computer_projection_epoch_roots", where: "computer_id=?"},
	{name: "computer_projection_base_pins", where: "computer_id=?"},
	{name: "computer_checkpoint_verdicts", where: "computer_id=?"},
	{name: "owner_act_receipts", where: "computer_id=?"},
	{name: "self_development_mode_receipts", where: "computer_id=?"},
	{name: "platform_update_offers", where: "computer_id=?"},
	{name: "consensus_decisions", where: "computer_id=?"},
	{name: "consensus_guest_core_keys", where: "computer_id=?"},
	{name: "computer_key_unwrap_approvals", where: "request_id IN (SELECT request_id FROM computer_key_unwrap_requests WHERE computer_id=?)", secret: true},
	{name: "computer_key_unwrap_requests", where: "computer_id=?", secret: true},
	{name: "computer_key_escrows", where: "computer_id=?", secret: true},
}

// tapeKeptComputerTables are per-computer tables the purge leaves: the head
// (reuse guard), the tombstone (the deletion record) and route/version tables
// the destroyed proof requires to be empty for the computer already.
var tapeKeptComputerTables = []string{
	"computer_event_heads",
	"computer_tape_tombstones",
	"computer_version_route_slots",
}

// TapeSweepClassCounts is the files, bytes and rows of one class: a tape
// namespace, tape-seg, file-cas-chunks, file-cas-roots, or "rows:<table>".
type TapeSweepClassCounts struct {
	Files int   `json:"files,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`
	Rows  int64 `json:"rows,omitempty"`
}

func addClass(m map[string]TapeSweepClassCounts, class string, files int, bytes, rows int64) map[string]TapeSweepClassCounts {
	if m == nil {
		m = map[string]TapeSweepClassCounts{}
	}
	c := m[class]
	c.Files += files
	c.Bytes += bytes
	c.Rows += rows
	m[class] = c
	return m
}

// tapeSweepRowsLine is the manifest's row count for one table of a computer.
type tapeSweepRowsLine struct {
	Kind     string `json:"kind"` // rows
	Computer string `json:"computer"`
	Table    string `json:"table"`
	Count    int64  `json:"count"`
}

// tapeSweepDirLine names a directory apply removes once it is empty.
type tapeSweepDirLine struct {
	Kind     string `json:"kind"` // dir
	Computer string `json:"computer"`
	Path     string `json:"path"`
}

func isPurgeNamespace(ns string) bool {
	return ns == tapeNSSeg || ns == tapeNSFileCASChunks || ns == tapeNSFileCASRoots
}

func purgeTreeBase(ns, computerID string) string {
	switch ns {
	case tapeNSSeg:
		return tapestore.SegDirName + "/" + computerID
	case tapeNSFileCASChunks:
		return "sha256/file-cas-chunks/" + computerID
	case tapeNSFileCASRoots:
		return "sha256/file-cas-roots/" + computerID
	}
	return ""
}

func purgeIDOK(computerID string) bool {
	return tapestore.ValidComputerID(computerID) && safeFileCASComponent(computerID)
}

// validPurgeItem checks a manifest item of a per-computer tree: destroyed
// sweep only, a clean path strictly beneath the computer's own tree.
func validPurgeItem(it tapeSweepItem) bool {
	if it.Sweep != TapeSweepDestroyed || !purgeIDOK(it.Computer) {
		return false
	}
	base := purgeTreeBase(it.Namespace, it.Computer)
	return base != "" && strings.HasPrefix(it.Path, base+"/") && path.Clean(it.Path) == it.Path
}

func validPurgeDir(computerID, dir string) bool {
	if !purgeIDOK(computerID) || path.Clean(dir) != dir {
		return false
	}
	for _, ns := range []string{tapeNSSeg, tapeNSFileCASChunks, tapeNSFileCASRoots} {
		base := purgeTreeBase(ns, computerID)
		if dir == base || strings.HasPrefix(dir, base+"/") {
			return true
		}
	}
	return false
}

func tapePurgeTableKnown(name string) bool {
	for _, t := range tapePurgeTables {
		if t.name == name {
			return true
		}
	}
	return false
}

// tapePurgeRowCounts counts the computer's rows in every purge table (only
// tables with rows are returned).
func (s *Service) tapePurgeRowCounts(ctx context.Context, computerID string) (map[string]int64, error) {
	out := map[string]int64{}
	for _, t := range tapePurgeTables {
		var n int64
		if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+t.name+` WHERE `+t.where, computerID).Scan(&n); err != nil {
			return nil, fmt.Errorf("tape sweep: count %s rows of %s: %w", t.name, computerID, err)
		}
		if n > 0 {
			out[t.name] = n
		}
	}
	return out, nil
}

// tapePurgeReferencedElsewhere reports another computer's file root that names
// this computer's file-CAS paths. File-CAS namespaces are per computer by
// construction (ClaimFileChunk never finds another computer's chunk), so this
// is a belt: a computer something else points into is not deleted.
func (s *Service) tapePurgeReferencedElsewhere(ctx context.Context, computerID string) (string, error) {
	var other string
	err := s.store.db.QueryRowContext(ctx, `SELECT computer_id FROM computer_file_roots WHERE computer_id<>? AND POSITION(? IN manifest_ref)>0 LIMIT 1`, computerID, "/"+computerID+"/").Scan(&other)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("tape sweep: cross-reference check of %s: %w", computerID, err)
	}
	return fmt.Sprintf("computer %s has a file root naming this computer's file-CAS paths", other), nil
}

// tapePurgeListing lists everything of the computer's own beyond the loose
// tape files: files and directories of its three trees, and its row counts.
// Any entry the sweep does not own cleanly (a symlink, a special file, a file
// newer than cutoff) refuses the computer.
func (s *Service) tapePurgeListing(ctx context.Context, computerID string, cutoff time.Time) (items []tapeSweepItem, dirs []string, rows map[string]int64, err error) {
	if err := tapeSweepProtected(computerID); err != nil {
		return nil, nil, nil, err
	}
	if !purgeIDOK(computerID) {
		return nil, nil, nil, fmt.Errorf("tape sweep: computer id %q cannot name a tape tree", computerID)
	}
	for _, ns := range []string{tapeNSSeg, tapeNSFileCASChunks, tapeNSFileCASRoots} {
		base := purgeTreeBase(ns, computerID)
		root := filepath.Join(s.artifactsRoot, filepath.FromSlash(base))
		info, lerr := os.Lstat(root)
		if errors.Is(lerr, fs.ErrNotExist) {
			continue
		}
		if lerr != nil {
			return nil, nil, nil, fmt.Errorf("tape sweep: %s: %w", base, lerr)
		}
		if !info.IsDir() {
			return nil, nil, nil, fmt.Errorf("tape sweep: %s is not a plain directory", base)
		}
		werr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			rel, rerr := filepath.Rel(s.artifactsRoot, p)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				dirs = append(dirs, rel)
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("tape sweep: %s is not a regular file", rel)
			}
			fi, ferr := d.Info()
			if ferr != nil {
				return ferr
			}
			if !fi.ModTime().Before(cutoff) {
				return fmt.Errorf("tape sweep: %s was written within the grace window", rel)
			}
			digest := path.Base(rel)
			if ns == tapeNSFileCASRoots {
				digest = strings.TrimSuffix(digest, ".json")
			}
			if ns == tapeNSSeg || !validHexDigest(digest) {
				digest = ""
			}
			items = append(items, tapeSweepItem{Kind: "item", Sweep: TapeSweepDestroyed, Computer: computerID, Namespace: ns, Digest: digest, Path: rel, Size: fi.Size(), MTime: fi.ModTime().UTC().Format(time.RFC3339Nano), Reason: "destroyed computer " + ns})
			return nil
		})
		if werr != nil {
			return nil, nil, nil, werr
		}
	}
	if reason, rerr := s.tapePurgeReferencedElsewhere(ctx, computerID); rerr != nil {
		return nil, nil, nil, rerr
	} else if reason != "" {
		return nil, nil, nil, errors.New(reason)
	}
	rows, err = s.tapePurgeRowCounts(ctx, computerID)
	if err != nil {
		return nil, nil, nil, err
	}
	return items, dirs, rows, nil
}

// tapeChainPurged reports a computer whose receipts are gone after a purge: it
// has a tombstone and no append-receipt rows, so its chain is not walkable and
// is not a gap.
func (s *Service) tapeChainPurged(ctx context.Context, computerID string) (bool, error) {
	_, hasTomb, err := s.store.TapeTombstone(ctx, computerID)
	if err != nil || !hasTomb {
		return false, err
	}
	var n int64
	if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, computerID).Scan(&n); err != nil {
		return false, fmt.Errorf("tape sweep: count receipts of %s: %w", computerID, err)
	}
	return n == 0, nil
}

// tapeArchiveExtra is a virtual archive entry (the rows dump).
type tapeArchiveExtra struct {
	Name string
	Body []byte
}

// tapePurgeRowsDump dumps the computer's rows as JSON lines, one entry per
// table, for the archive. Key material tables are never dumped.
func (s *Service) tapePurgeRowsDump(ctx context.Context, computerID string) ([]tapeArchiveExtra, error) {
	var out []tapeArchiveExtra
	for _, t := range tapePurgeTables {
		if t.secret {
			continue
		}
		rows, err := s.store.db.QueryContext(ctx, `SELECT row_to_json(t)::text FROM `+t.name+` t WHERE `+t.where, computerID)
		if err != nil {
			return nil, fmt.Errorf("tape sweep: dump %s: %w", t.name, err)
		}
		var body []byte
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				return nil, err
			}
			body = append(append(body, line...), '\n')
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			out = append(out, tapeArchiveExtra{Name: "rows/" + t.name + ".jsonl", Body: body})
		}
	}
	return out, nil
}

// removePurgeDirs removes the manifest's directories, deepest first. A
// directory that is not empty (a file appeared after the dry-run) is an
// error: nothing outside the manifest is deleted.
func (s *Service) removePurgeDirs(run *tapeSweepApplyRun, computerID string, dirs []string) (ok bool) {
	sorted := append([]string(nil), dirs...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	ok = true
	for _, d := range sorted {
		if !validPurgeDir(computerID, d) {
			ok = false
			continue
		}
		err := os.Remove(filepath.Join(s.artifactsRoot, filepath.FromSlash(d)))
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			if err == nil {
				run.log.record(tapeSweepLogRecord{Op: "deleted_dir", Sweep: TapeSweepDestroyed, Computer: computerID, Path: d})
			}
			continue
		}
		ok = false
		run.counts.Errors++
		run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepDestroyed, Computer: computerID, Path: d, Error: "remove directory: " + err.Error()})
	}
	_ = run.log.flush()
	return ok
}

// purgeComputerRows deletes the computer's rows in every purge table (key
// escrow included) in one transaction and compares each table's deleted count
// with the manifest's. A mismatch rolls the whole transaction back. If nothing
// at all was deleted the rows are already gone (a resumed apply after the
// commit) and the step is a no-op. The escrow shred is appended to the key
// escrow transparency log after the commit.
func (s *Service) purgeComputerRows(ctx context.Context, run *tapeSweepApplyRun, computerID string, expected map[string]int64) (map[string]int64, error) {
	if err := tapeSweepProtected(computerID); err != nil {
		return nil, err
	}
	if hook := s.tapeSweep.beforeRowsDelete; hook != nil {
		if err := hook(computerID); err != nil {
			return nil, err
		}
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("tape sweep: begin rows transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	type escrowKey struct{ Protector, KeyDigest string }
	var shredded []escrowKey
	keyRows, err := tx.QueryContext(ctx, `SELECT protector, key_digest FROM computer_key_escrows WHERE computer_id=?`, computerID)
	if err != nil {
		return nil, fmt.Errorf("tape sweep: read escrow: %w", err)
	}
	for keyRows.Next() {
		var k escrowKey
		if err := keyRows.Scan(&k.Protector, &k.KeyDigest); err != nil {
			keyRows.Close()
			return nil, err
		}
		shredded = append(shredded, k)
	}
	err = keyRows.Err()
	keyRows.Close()
	if err != nil {
		return nil, err
	}
	affected := map[string]int64{}
	var total int64
	for _, t := range tapePurgeTables {
		res, err := tx.ExecContext(ctx, `DELETE FROM `+t.name+` WHERE `+t.where, computerID)
		if err != nil {
			return nil, fmt.Errorf("tape sweep: delete %s rows of %s: %w", t.name, computerID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n > 0 {
			affected[t.name] = n
			total += n
		}
	}
	if total > 0 {
		for _, t := range tapePurgeTables {
			if affected[t.name] != expected[t.name] {
				return nil, fmt.Errorf("tape sweep: %s of %s: apply would remove %d rows, the manifest lists %d; rows kept", t.name, computerID, affected[t.name], expected[t.name])
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("tape sweep: commit rows transaction: %w", err)
	}
	if len(shredded) > 0 {
		keys := make([]map[string]string, 0, len(shredded))
		for _, k := range shredded {
			keys = append(keys, map[string]string{"protector": k.Protector, "key_digest": k.KeyDigest})
		}
		payload, _ := json.Marshal(map[string]any{"type": "computer_escrow_shredded", "computer_id": computerID, "run_id": run.p.runID, "keys": keys, "shredded_at": time.Now().UTC().Format(time.RFC3339Nano)})
		if _, _, err := s.store.AppendKeyEscrowTransparency(context.WithoutCancel(ctx), payload); err != nil {
			log.Printf("tape sweep: computer=%s escrow shredded; transparency append failed: %v", computerID, err)
			run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepDestroyed, Computer: computerID, Error: "transparency append: " + err.Error()})
		}
	}
	return affected, nil
}
