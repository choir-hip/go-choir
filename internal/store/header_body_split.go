package store

// The header/body split migration (docs/design/store-header-body-split-2026-11-01.md
// section 6): at boot, before the store opens, a pre-split state file (og_objects
// and choir_events each carry their body column) is copied into a new file in
// the split layout, checked, and swapped in by renames. The old file is kept
// beside it as state.db.presplit, the rollback, until the split-finalize route
// removes it (or free space falls under the floor).
//
// The shape mirrors compaction (compact.go): a staging file that is untrusted
// until its sidecar is written durably, a swap of two renames that the next
// boot finishes or undoes, and a refusal before any mutation when the disk is
// short. Every step is a BootPhase line, and the long ones log progress every
// headerBodySplitLogEvery.
//
// Crash matrix (each case resumes or redoes):
//   - before the staging sidecar exists: the next boot deletes the staging
//     files, counts an attempt and redoes (journal OFF: a partial staging file
//     is never trusted);
//   - sidecar intact, state.db old layout, source unchanged: the next boot
//     swaps; source changed (an old binary ran in between): discard, redo;
//   - state.db missing, .presplit present: finish the swap when the staging
//     file and its sidecar are intact, else undo it (rename .presplit back);
//   - state.db new layout: done; a swap's last steps (floor, layout marker,
//     summary) are finished when its sidecar is still there;
//   - between the swap and the floor: an old binary refuses through
//     store_meta.schema_version 2.
//
// The migration runs on crash and planned boots alike: it is boot
// infrastructure, and this binary cannot serve the old layout.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

const (
	// The files beside state.db: the split copy being written, its sidecar
	// (written last, durably: the copy is staged only once it exists), the
	// original kept as the rollback after the swap, the summary of the swap,
	// and the attempt counter.
	splitStagingSuffix    = ".split-staging"
	splitSidecarSuffix    = ".split-staging.json"
	presplitSuffix        = ".presplit"
	presplitSummarySuffix = ".presplit.json"
	splitAttemptsSuffix   = ".split-attempts"

	// HeaderBodySplitRefusalKind is the typed boot refusal kind the guest's
	// early health gate serves for a refused split.
	HeaderBodySplitRefusalKind = "store_header_body_split"

	// headerBodySplitFloor is the store floor written after the swap: the
	// epoch of the split layout.
	headerBodySplitFloor uint64 = 6
)

// The floor must not be above this build's epoch.
const _ = storeschema.Version - headerBodySplitFloor

var (
	// headerBodySplitMaxAttempts is how many full migrations a store may
	// start; the next boot refuses with a typed refusal.
	headerBodySplitMaxAttempts = 3
	// headerBodySplitBatch is the rowid range one copy batch covers.
	headerBodySplitBatch = 256
	// headerBodySplitSample is the rows per split table the verify step
	// compares byte for byte.
	headerBodySplitSample = 1000
	// headerBodySplitSpaceFactor and headerBodySplitSortTempBytes size the
	// preflight: the new file (about the live size), the sorter's temp files
	// for the index builds, and compactFreeFloorBytes of headroom.
	headerBodySplitSpaceFactor         = 1.05
	headerBodySplitSortTempBytes int64 = 1 << 30
	// headerBodySplitLogEvery paces the progress lines of the long steps.
	headerBodySplitLogEvery = 10 * time.Second
	// headerBodySplitCopyBudget bounds each copy step, and
	// headerBodySplitIndexBudget each index build.
	headerBodySplitCopyBudget  = 60 * time.Minute
	headerBodySplitIndexBudget = 20 * time.Minute
	// headerBodySplitCacheKiB is the staging connection's page cache.
	headerBodySplitCacheKiB = 65536

	// testHookHeaderBodySplitStep runs after each named step; an error from
	// it ends the migration there, as a crash would (tests only).
	testHookHeaderBodySplitStep func(step string) error
)

// HeaderBodySplitRefusal is a migration that refused before any mutation of
// the store: too little disk, or the attempts exhausted. The early health
// gate serves it as the typed boot refusal HeaderBodySplitRefusalKind.
type HeaderBodySplitRefusal struct {
	Reason   string
	Live     int64
	Free     int64
	Need     int64
	Attempts int
}

func (e *HeaderBodySplitRefusal) Error() string {
	return fmt.Sprintf("store: header-body-split refused: %s (live=%d free=%d need=%d attempts=%d)", e.Reason, e.Live, e.Free, e.Need, e.Attempts)
}

// HeaderBodySplitSummary is the staging sidecar (state.db.split-staging.json)
// and, after the swap, the summary beside the rollback copy
// (state.db.presplit.json).
type HeaderBodySplitSummary struct {
	// SourceHead names the old file's projection heads when it was staged;
	// SourceSize and SourceModUnixNano its size and mtime after the
	// checkpoint. The swap requires all three unchanged.
	SourceHead         string           `json:"source_head"`
	SourceHeadSequence uint64           `json:"source_head_sequence"`
	SourceSize         int64            `json:"source_size"`
	SourceModUnixNano  int64            `json:"source_mod_unix_nano"`
	StagingSize        int64            `json:"staging_size"`
	Rows               map[string]int64 `json:"rows"`
	StepMillis         map[string]int64 `json:"step_ms"`
	StagedAt           time.Time        `json:"staged_at"`
	DurationMillis     int64            `json:"duration_ms"`
	SwappedAt          *time.Time       `json:"swapped_at,omitempty"`
	// PresplitRemoved is why the rollback copy is gone: "finalized" (the
	// split-finalize route) or "auto_dropped_low_space".
	PresplitRemoved   string     `json:"presplit_removed,omitempty"`
	PresplitRemovedAt *time.Time `json:"presplit_removed_at,omitempty"`
}

// HeaderBodySplitStats is store.split in /api/runtime/observability.
type HeaderBodySplitStats struct {
	Summary             *HeaderBodySplitSummary `json:"summary,omitempty"`
	PresplitPresent     bool                    `json:"presplit_present"`
	PresplitBytes       int64                   `json:"presplit_bytes,omitempty"`
	PresplitAutoDropped bool                    `json:"presplit_auto_dropped,omitempty"`
}

// splitAutoDropped is the observability flag of an automatic drop of the
// rollback copy in this process.
var splitAutoDropped struct {
	sync.Mutex
	paths map[string]bool
}

func splitStepHook(step string) error {
	if hook := testHookHeaderBodySplitStep; hook != nil {
		return hook(step)
	}
	return nil
}

// MigrateHeaderBodySplit migrates the store at dbPath to the header/body split
// layout when it is still in the old one, and finishes or undoes a swap a
// crash interrupted. progress, when set, receives the rows copied so far (a
// monotone count). autoputer runs it under the early health gate, before the
// store opens; Open runs it again, which is then a cheap check.
func MigrateHeaderBodySplit(dbPath string, progress func(uint64)) error {
	return migrateHeaderBodySplitIfNeeded(dbPath, progress)
}

// HeaderBodySplitNeeded reports whether MigrateHeaderBodySplit has work at
// dbPath: an old-layout state file, or a swap to finish or undo.
//
// A telemetry partition file still in the old layout is work too: the state
// file may be split already while a partition's rewrite did not finish (it
// runs after the swap and a boot cut short leaves it), and the retry boot
// must bind the gate again.
func HeaderBodySplitNeeded(dbPath string) (bool, error) {
	needed, err := stateSplitNeeded(dbPath)
	if err != nil || needed {
		return needed, err
	}
	parts, err := oldLayoutPartitions(dbPath)
	return len(parts) > 0, err
}

func stateSplitNeeded(dbPath string) (bool, error) {
	statePath := StateDBPath(dbPath)
	if _, err := os.Stat(statePath); errors.Is(err, os.ErrNotExist) {
		_, preErr := os.Stat(statePath + presplitSuffix)
		return preErr == nil, nil
	} else if err != nil {
		return false, err
	}
	return statePresplitLayout(statePath)
}

func migrateHeaderBodySplitIfNeeded(dbPath string, progress func(uint64)) error {
	statePath := StateDBPath(dbPath)
	// The state phase reports rows copied; the partition phase continues the
	// same monotone counter from where it ended, so the gate's progress never
	// stalls between the two.
	var base atomic.Uint64
	stateProgress := progress
	if progress != nil {
		stateProgress = func(n uint64) {
			for {
				cur := base.Load()
				if n <= cur || base.CompareAndSwap(cur, n) {
					break
				}
			}
			progress(n)
		}
	}
	// A pass that discards a staged copy whose source moved redoes once; a
	// second move in one process is not looped on.
	done := false
	for pass := 0; pass < 2 && !done; pass++ {
		redo, err := headerBodySplitPass(dbPath, statePath, stateProgress)
		if err != nil {
			return err
		}
		done = !redo
	}
	if !done {
		return fmt.Errorf("store: header-body-split: the source moved twice during one boot")
	}
	return migrateTelemetryPartitionFiles(dbPath, func(n uint64) {
		if progress != nil {
			progress(base.Load() + n)
		}
	})
}

// headerBodySplitPass runs one pass of the crash matrix. redo asks for another.
func headerBodySplitPass(dbPath, statePath string, progress func(uint64)) (redo bool, err error) {
	pre := statePath + presplitSuffix
	if _, statErr := os.Stat(statePath); errors.Is(statErr, os.ErrNotExist) {
		if _, preErr := os.Stat(pre); preErr != nil {
			return false, nil // a fresh store
		}
		done := objectgraph.BootPhase("store: open", "header-body-split/finish-interrupted-swap")
		if side, ok := readSplitSidecar(statePath); ok {
			err := finishSplitSwap(dbPath, statePath, side)
			done(err)
			if err == nil {
				log.Printf("store: header-body-split: finished an interrupted swap")
			}
			return false, err
		}
		err := undoSwap(statePath, pre)
		done(err)
		if err != nil {
			return false, err
		}
		log.Printf("store: header-body-split: undid an interrupted swap (no intact staged copy); migrating again")
	} else if statErr != nil {
		return false, statErr
	}
	old, err := statePresplitLayout(statePath)
	if err != nil {
		return false, fmt.Errorf("store: header-body-split: detect layout: %w", err)
	}
	if !old {
		// New layout. A swap whose last steps a crash cut short still has
		// its sidecar: finish them. A staging file without one is debris.
		if side, ok := readSplitSidecarFile(statePath); ok {
			if _, preErr := os.Stat(pre); preErr == nil {
				if err := postSplitSwap(dbPath, statePath, side); err != nil {
					return false, err
				}
			}
		}
		discardSplitStaging(statePath)
		dropPresplitIfLow(statePath)
		return false, nil
	}
	if side, ok := readSplitSidecar(statePath); ok {
		return swapHeaderBodySplit(dbPath, statePath, side)
	}
	discardSplitStaging(statePath)
	return runHeaderBodySplit(dbPath, statePath, progress)
}

// statePresplitLayout reports whether the SQLite file at path is in the
// pre-split layout: og_objects or choir_events has a body column. It opens
// the file read-write with no pragmas, so nothing in it changes (a WAL a
// crash left is recovered as any open would).
func statePresplitLayout(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return false, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(30000)")
	if err != nil {
		return false, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()
	for _, table := range []string{"og_objects", choirEventsTable} {
		has, err := dbx.ColumnExists(ctx, db, dbx.SQLite, table, "body")
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// splitPreflight discards the old-layout compaction files, checks the disk
// and the attempt counter, and counts this attempt. A refusal leaves the
// store as it was.
func splitPreflight(statePath string, partitionPeak int64) (live int64, err error) {
	removeSQLiteFiles(statePath + compactStagingSuffix)
	removeSQLiteFiles(statePath + compactStagedSuffix)
	_ = os.Remove(statePath + compactSidecarSuffix)

	var pageCount, freelist, pageSize int64
	err = func() error {
		db, err := sql.Open("sqlite", "file:"+statePath+"?_pragma=busy_timeout(30000)")
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(1)
		defer db.Close()
		for q, dst := range map[string]*int64{"PRAGMA page_count": &pageCount, "PRAGMA freelist_count": &freelist, "PRAGMA page_size": &pageSize} {
			if err := db.QueryRow(q).Scan(dst); err != nil {
				return fmt.Errorf("%s: %w", q, err)
			}
		}
		return nil
	}()
	if err != nil {
		return 0, err
	}
	live = (pageCount - freelist) * pageSize
	free, err := compactFreeBytes(filepath.Dir(statePath))
	if err != nil {
		return live, &HeaderBodySplitRefusal{Reason: "free space unreadable: " + err.Error(), Live: live}
	}
	need := int64(float64(live)*headerBodySplitSpaceFactor) + headerBodySplitSortTempBytes + compactFreeFloorBytes
	// The partitions are rewritten after the swap, while the rollback copy
	// still holds its space: the largest one's peak is counted now.
	need += partitionPeak
	attempts := readSplitAttempts(statePath)
	log.Printf("store: header-body-split: preflight live=%d free=%d need=%d attempts=%d/%d", live, free, need, attempts, headerBodySplitMaxAttempts)
	if free < need {
		return live, &HeaderBodySplitRefusal{Reason: "too little free disk for the split copy", Live: live, Free: free, Need: need, Attempts: attempts}
	}
	if attempts >= headerBodySplitMaxAttempts {
		return live, &HeaderBodySplitRefusal{Reason: fmt.Sprintf("%d migration attempts did not finish", attempts), Live: live, Free: free, Need: need, Attempts: attempts}
	}
	if err := writeFileDurable(statePath+splitAttemptsSuffix, []byte(strconv.Itoa(attempts+1))); err != nil {
		return live, err
	}
	return live, nil
}

func readSplitAttempts(statePath string) int {
	raw, err := os.ReadFile(statePath + splitAttemptsSuffix)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || n < 0 {
		// A torn counter counts as exhausted: never loop on an unreadable one.
		return headerBodySplitMaxAttempts
	}
	return n
}

// splitStep runs one named step as a BootPhase, records its duration, then
// runs the crash hook.
func splitStep(side *HeaderBodySplitSummary, name string, fn func() error) error {
	done := objectgraph.BootPhase("store: open", "header-body-split/"+name)
	started := time.Now()
	err := fn()
	done(err)
	if side != nil && side.StepMillis != nil {
		side.StepMillis[name] = time.Since(started).Milliseconds()
	}
	if err != nil {
		return err
	}
	return splitStepHook(name)
}

// runHeaderBodySplit is the full migration of an old-layout state file.
func runHeaderBodySplit(dbPath, statePath string, progress func(uint64)) (bool, error) {
	started := time.Now()
	side := &HeaderBodySplitSummary{Rows: map[string]int64{}, StepMillis: map[string]int64{}}
	if err := splitStep(side, "preflight", func() error {
		parts, _ := oldLayoutPartitions(dbPath)
		_, err := splitPreflight(statePath, partitionRewritePeak(largestPartition(parts)))
		return err
	}); err != nil {
		return false, err
	}
	if err := splitStep(side, "checkpoint-old", func() error {
		return checkpointSplitSource(statePath, side)
	}); err != nil {
		return false, err
	}
	staging := statePath + splitStagingSuffix
	m := &splitCopier{statePath: statePath, staging: staging, side: side, progress: progress}
	if err := splitStep(side, "stage", m.open); err != nil {
		m.close()
		return false, err
	}
	err := m.run()
	m.close()
	if err != nil {
		return false, err
	}
	if err := fsyncFile(staging); err != nil {
		return false, err
	}
	info, err := os.Stat(staging)
	if err != nil {
		return false, err
	}
	if err := checkStagedHeader(staging); err != nil {
		return false, fmt.Errorf("store: header-body-split: staged file: %w", err)
	}
	side.StagingSize = info.Size()
	side.StagedAt = time.Now().UTC()
	side.DurationMillis = time.Since(started).Milliseconds()
	raw, err := json.Marshal(side)
	if err != nil {
		return false, err
	}
	if err := writeFileDurable(statePath+splitSidecarSuffix, raw); err != nil {
		return false, err
	}
	if err := splitStepHook("staged"); err != nil {
		return false, err
	}
	return swapHeaderBodySplit(dbPath, statePath, *side)
}

// checkpointSplitSource folds the old file's WAL into it, then records its
// heads, size and mtime: the swap requires them unchanged.
func checkpointSplitSource(statePath string, side *HeaderBodySplitSummary) error {
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		_ = db.Close()
		return fmt.Errorf("wal_checkpoint: %w", err)
	}
	head, seq, err := readSplitHeads(db)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	side.SourceHead, side.SourceHeadSequence = head, seq
	size, mod, err := splitSourceStat(statePath)
	if err != nil {
		return err
	}
	side.SourceSize, side.SourceModUnixNano = size, mod
	return nil
}

func splitSourceStat(statePath string) (int64, int64, error) {
	info, err := os.Stat(statePath)
	if err != nil {
		return 0, 0, err
	}
	return info.Size(), info.ModTime().UnixNano(), nil
}

// readSplitHeads names every projection head the file holds, in computer
// order, and the highest sequence ("" when there is none).
func readSplitHeads(db *sql.DB) (string, uint64, error) {
	has, err := dbx.TableExists(context.Background(), db, dbx.SQLite, "computer_event_projection_heads")
	if err != nil || !has {
		return "", 0, err
	}
	rows, err := db.Query(`SELECT computer_id, sequence, canonical_event_head FROM computer_event_projection_heads ORDER BY computer_id`)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var parts []string
	var top uint64
	for rows.Next() {
		var id, head string
		var seq uint64
		if err := rows.Scan(&id, &seq, &head); err != nil {
			return "", 0, err
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%s", id, seq, head))
		if seq > top {
			top = seq
		}
	}
	return strings.Join(parts, ";"), top, rows.Err()
}

// splitSourceUnchanged reports whether the old file is the one the staged copy
// was made from: the same heads, size and mtime.
func splitSourceUnchanged(statePath string, side HeaderBodySplitSummary) (bool, error) {
	db, err := sql.Open("sqlite", "file:"+statePath+"?_pragma=busy_timeout(30000)")
	if err != nil {
		return false, err
	}
	db.SetMaxOpenConns(1)
	head, _, err := readSplitHeads(db)
	_ = db.Close()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(statePath + "-wal"); err == nil {
		if info, _ := os.Stat(statePath + "-wal"); info != nil && info.Size() > 0 {
			return false, nil
		}
	}
	size, mod, err := splitSourceStat(statePath)
	if err != nil {
		return false, err
	}
	return head == side.SourceHead && size == side.SourceSize && mod == side.SourceModUnixNano, nil
}

// swapHeaderBodySplit swaps the staged copy in: state.db to state.db.presplit
// (its WAL with it), the staging file to state.db, then the floor, the
// layout marker and the summary.
func swapHeaderBodySplit(dbPath, statePath string, side HeaderBodySplitSummary) (bool, error) {
	var redo bool
	err := splitStep(nil, "swap", func() error {
		same, err := splitSourceUnchanged(statePath, side)
		if err != nil {
			return err
		}
		if !same {
			log.Printf("store: header-body-split: the old file changed since the copy was staged (another binary ran); discarding the staged copy and migrating again")
			discardSplitStaging(statePath)
			redo = true
			return nil
		}
		pre := statePath + presplitSuffix
		if _, err := os.Stat(pre); err == nil {
			// state.db is old layout, so it is the authority; a stray
			// rollback copy beside it is from a migration rolled back by hand.
			log.Printf("store: header-body-split: removing a stale %s beside an old-layout state file", filepath.Base(pre))
			removeSQLiteFiles(pre)
		}
		if err := os.Rename(statePath, pre); err != nil {
			return err
		}
		if err := moveWAL(statePath, pre); err != nil {
			return errors.Join(err, undoSwap(statePath, pre))
		}
		syncDir(filepath.Dir(statePath))
		if err := splitStepHook("swap/renamed-old"); err != nil {
			return err
		}
		return finishSplitSwap(dbPath, statePath, side)
	})
	return redo, err
}

// finishSplitSwap renames the staged copy to state.db and finishes the swap.
func finishSplitSwap(dbPath, statePath string, side HeaderBodySplitSummary) error {
	if err := moveWAL(statePath, statePath+presplitSuffix); err != nil {
		return err
	}
	if err := os.Rename(statePath+splitStagingSuffix, statePath); err != nil {
		return err
	}
	syncDir(filepath.Dir(statePath))
	if err := splitStepHook("swap/renamed-new"); err != nil {
		return err
	}
	return postSplitSwap(dbPath, statePath, side)
}

// postSplitSwap writes the floor, the layout marker and the summary, then
// drops the sidecar and the attempt counter. Every step is idempotent: a
// crash in it is finished by the next boot.
func postSplitSwap(dbPath, statePath string, side HeaderBodySplitSummary) error {
	sidecarDir := deriveTextureWorkspacePath(dbPath)
	if err := os.MkdirAll(sidecarDir, 0o755); err != nil {
		return err
	}
	if err := storeschema.WriteFloor(sidecarDir, headerBodySplitFloor, "store header/body split (og_bodies, choir_event_bodies)"); err != nil {
		return err
	}
	if err := splitStepHook("swap/floored"); err != nil {
		return err
	}
	// The new file is compact: compaction is not due for a while.
	writeLayoutMarker(statePath, time.Now(), side.SourceHeadSequence)
	now := time.Now().UTC()
	side.SwappedAt = &now
	raw, err := json.Marshal(side)
	if err != nil {
		return err
	}
	if err := writeFileDurable(statePath+presplitSummarySuffix, raw); err != nil {
		return err
	}
	_ = os.Remove(statePath + splitSidecarSuffix)
	_ = os.Remove(statePath + splitAttemptsSuffix)
	presplit := fileSizeOrZero(statePath + presplitSuffix)
	log.Printf("store: header-body-split: done rows og=%d ev=%d bytes=%d duration=%s presplit=%d",
		side.Rows["og_objects"], side.Rows[choirEventsTable], fileSizeOrZero(statePath),
		(time.Duration(side.DurationMillis) * time.Millisecond).Round(time.Millisecond), presplit)
	return nil
}

// discardSplitStaging removes the staging file and its sidecar.
func discardSplitStaging(statePath string) {
	removeSQLiteFiles(statePath + splitStagingSuffix)
	_ = os.Remove(statePath + splitSidecarSuffix)
}

// readSplitSidecarFile reads the staging sidecar alone.
func readSplitSidecarFile(statePath string) (HeaderBodySplitSummary, bool) {
	raw, err := os.ReadFile(statePath + splitSidecarSuffix)
	if err != nil {
		return HeaderBodySplitSummary{}, false
	}
	var side HeaderBodySplitSummary
	if json.Unmarshal(raw, &side) != nil || side.StagingSize == 0 || side.StagedAt.IsZero() {
		return HeaderBodySplitSummary{}, false
	}
	return side, true
}

// readSplitSidecar reads the sidecar and checks the staging file it names:
// present, of the recorded size, with a SQLite header that matches it.
func readSplitSidecar(statePath string) (HeaderBodySplitSummary, bool) {
	side, ok := readSplitSidecarFile(statePath)
	if !ok {
		return side, false
	}
	staging := statePath + splitStagingSuffix
	info, err := os.Stat(staging)
	if err != nil || info.Size() != side.StagingSize || checkStagedHeader(staging) != nil {
		return HeaderBodySplitSummary{}, false
	}
	return side, true
}

// presplitHeld reports whether the split's rollback copy is beside the state
// file.
func presplitHeld(statePath string) bool {
	_, err := os.Stat(statePath + presplitSuffix)
	return err == nil
}

// dropPresplitIfLow removes the rollback copy when free space is under
// compactFreeFloorBytes, loudly: the rollback is then the pre-migration
// snapshot. It reports whether it dropped one.
func dropPresplitIfLow(statePath string) bool {
	if !presplitHeld(statePath) {
		return false
	}
	free, err := compactFreeBytes(filepath.Dir(statePath))
	if err != nil || free >= compactFreeFloorBytes {
		return false
	}
	bytes := fileSizeOrZero(statePath + presplitSuffix)
	removeSQLiteFiles(statePath + presplitSuffix)
	markPresplitRemoved(statePath, "auto_dropped_low_space")
	splitAutoDropped.Lock()
	if splitAutoDropped.paths == nil {
		splitAutoDropped.paths = map[string]bool{}
	}
	splitAutoDropped.paths[statePath] = true
	splitAutoDropped.Unlock()
	log.Printf("store: header-body-split: WARNING: DROPPED the rollback copy %s (%d bytes): %d bytes free, under the %d-byte floor; the rollback is now the pre-migration snapshot",
		filepath.Base(statePath+presplitSuffix), bytes, free, compactFreeFloorBytes)
	return true
}

func markPresplitRemoved(statePath, why string) {
	path := statePath + presplitSummarySuffix
	var summary HeaderBodySplitSummary
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &summary)
	}
	now := time.Now().UTC()
	summary.PresplitRemoved, summary.PresplitRemovedAt = why, &now
	if raw, err := json.Marshal(summary); err == nil {
		_ = writeFileDurable(path, raw)
	}
}

// FinalizeHeaderBodySplit removes the split's rollback copy (state.db.presplit
// and its WAL) once the differential is green: the split-finalize route. It
// refuses while the state file is not in the split layout. It returns the
// bytes it freed (0 when there was nothing to remove).
func FinalizeHeaderBodySplit(dbPath string) (int64, error) {
	statePath := StateDBPath(dbPath)
	if !presplitHeld(statePath) {
		return 0, nil
	}
	old, err := statePresplitLayout(statePath)
	if err != nil {
		return 0, err
	}
	if old {
		return 0, fmt.Errorf("store: header-body-split: the state file is not split; refusing to remove the rollback copy")
	}
	if _, err := os.Stat(statePath); err != nil {
		return 0, fmt.Errorf("store: header-body-split: no state file; refusing to remove the rollback copy: %w", err)
	}
	var freed int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		freed += fileSizeOrZero(statePath + presplitSuffix + suffix)
	}
	removeSQLiteFiles(statePath + presplitSuffix)
	syncDir(filepath.Dir(statePath))
	markPresplitRemoved(statePath, "finalized")
	log.Printf("store: header-body-split: finalized: removed the rollback copy (%d bytes)", freed)
	return freed, nil
}

// HeaderBodySplitStats is the split's state for observability: nil when the
// store never migrated.
func (s *Store) HeaderBodySplitStats() *HeaderBodySplitStats {
	if s == nil || s.statePath == "" {
		return nil
	}
	return headerBodySplitStatsAt(s.statePath)
}

func headerBodySplitStatsAt(statePath string) *HeaderBodySplitStats {
	out := &HeaderBodySplitStats{PresplitPresent: presplitHeld(statePath)}
	if out.PresplitPresent {
		out.PresplitBytes = fileSizeOrZero(statePath + presplitSuffix)
	}
	if raw, err := os.ReadFile(statePath + presplitSummarySuffix); err == nil {
		var summary HeaderBodySplitSummary
		if json.Unmarshal(raw, &summary) == nil {
			out.Summary = &summary
		}
	}
	splitAutoDropped.Lock()
	out.PresplitAutoDropped = splitAutoDropped.paths[statePath]
	splitAutoDropped.Unlock()
	if out.Summary == nil && !out.PresplitPresent {
		return nil
	}
	return out
}

// splitCopier writes the split copy: one connection to the staging file with
// the old file attached read-only.
type splitCopier struct {
	statePath, staging string
	side               *HeaderBodySplitSummary
	progress           func(uint64)

	db     *sql.DB
	conn   *sql.Conn
	copied uint64

	// The old file's other objects, by kind, in sqlite_master order.
	tables, indexes, triggers, views []splitSchemaEntry
	hasOld                           map[string]bool
}

type splitSchemaEntry struct{ name, sql string }

func (m *splitCopier) report(n int64) {
	m.copied += uint64(n)
	if m.progress != nil {
		m.progress(m.copied)
	}
}

func (m *splitCopier) exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := m.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// open creates the staging file with the copy's pragmas and attaches the old
// file read-only. The sorter's temp files go beside the store: the guest /tmp
// may be RAM.
func (m *splitCopier) open() error {
	removeSQLiteFiles(m.staging)
	dir := filepath.Dir(m.statePath)
	_ = os.Setenv("SQLITE_TMPDIR", dir)
	db, err := sql.Open("sqlite", "file:"+m.staging+"?_time_format=sqlite")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	m.db = db
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	m.conn = conn
	for _, pragma := range []string{
		`PRAGMA page_size=4096`,
		`PRAGMA auto_vacuum=INCREMENTAL`,
		`PRAGMA journal_mode=OFF`,
		`PRAGMA synchronous=OFF`,
		fmt.Sprintf(`PRAGMA cache_size=-%d`, headerBodySplitCacheKiB),
		`PRAGMA temp_store=FILE`,
		`PRAGMA temp_store_directory=` + sqlQuote(dir),
		`PRAGMA mmap_size=0`,
	} {
		if _, err := conn.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS old`, "file:"+m.statePath+"?mode=ro"); err != nil {
		return fmt.Errorf("attach the old file: %w", err)
	}
	return nil
}

func (m *splitCopier) close() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	if m.db != nil {
		_ = m.db.Close()
		m.db = nil
	}
}

// splitTables are the two tables the split rewrites.
func isSplitTable(name string) bool {
	switch strings.ToLower(name) {
	case "og_objects", choirEventsTable, "og_bodies", "choir_event_bodies":
		return true
	}
	return false
}

func (m *splitCopier) run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"copy-schema", m.copySchema},
		{"copy-tables", m.copyTables},
		{"copy-og_objects", m.copyObjects},
		{"copy-choir_events", m.copyEvents},
		{"build-indexes", m.buildIndexes},
		{"verify", m.verify},
	}
	for _, step := range steps {
		if err := splitStep(m.side, step.name, step.fn); err != nil {
			return err
		}
	}
	return nil
}

// copySchema reads the old file's schema and creates every table that is not
// one of the split tables, verbatim; their indexes, triggers and views are
// created after the rows (buildIndexes).
func (m *splitCopier) copySchema() error {
	ctx := context.Background()
	rows, err := m.conn.QueryContext(ctx, `SELECT type, name, tbl_name, sql FROM old.sqlite_master WHERE sql IS NOT NULL ORDER BY rowid`)
	if err != nil {
		return err
	}
	m.hasOld = map[string]bool{}
	for rows.Next() {
		var typ, name, tbl, text string
		if err := rows.Scan(&typ, &name, &tbl, &text); err != nil {
			_ = rows.Close()
			return err
		}
		if typ == "table" {
			m.hasOld[strings.ToLower(name)] = true
		}
		if strings.HasPrefix(strings.ToLower(name), "sqlite_") || isSplitTable(tbl) || isSplitTable(name) {
			continue
		}
		entry := splitSchemaEntry{name: name, sql: text}
		switch typ {
		case "table":
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(text)), "CREATE VIRTUAL TABLE") {
				_ = rows.Close()
				return fmt.Errorf("virtual table %s: the split copy does not carry virtual tables", name)
			}
			m.tables = append(m.tables, entry)
		case "index":
			m.indexes = append(m.indexes, entry)
		case "trigger":
			m.triggers = append(m.triggers, entry)
		case "view":
			m.views = append(m.views, entry)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !m.hasOld["og_objects"] {
		return fmt.Errorf("the old file has no og_objects")
	}
	for _, table := range m.tables {
		if _, err := m.conn.ExecContext(ctx, table.sql); err != nil {
			return fmt.Errorf("create %s: %w", table.name, err)
		}
	}
	// The split tables from the new DDL, without their secondary indexes.
	if _, err := m.conn.ExecContext(ctx, objectgraph.ObjectHeaderTableDDL()+objectgraph.ObjectBodyTableDDL); err != nil {
		return fmt.Errorf("create og_objects/og_bodies: %w", err)
	}
	eventTables, _ := splitChoirEventsDDL()
	if _, err := m.conn.ExecContext(ctx, eventTables); err != nil {
		return fmt.Errorf("create choir_events/choir_event_bodies: %w", err)
	}
	return nil
}

// splitChoirEventsDDL splits choirEventsDDL into the tables and trigger, and
// the index statements (built after the rows).
func splitChoirEventsDDL() (tables string, indexes []string) {
	var b strings.Builder
	for _, line := range strings.Split(choirEventsDDL, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE INDEX") {
			indexes = append(indexes, strings.TrimSuffix(strings.TrimSpace(line), ";"))
			continue
		}
		b.WriteString(line + "\n")
	}
	return b.String(), indexes
}

// insertableColumns are a table's stored columns (generated ones excluded),
// quoted, in declaration order.
func (m *splitCopier) insertableColumns(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := m.conn.QueryContext(ctx, `SELECT name, hidden FROM pragma_table_xinfo(?, ?) ORDER BY cid`, table, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		var hidden int
		if err := rows.Scan(&name, &hidden); err != nil {
			return nil, err
		}
		if hidden == 0 {
			out = append(out, `"`+strings.ReplaceAll(name, `"`, `""`)+`"`)
		}
	}
	return out, rows.Err()
}

func quoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// copyTables copies every other table's rows, then sqlite_sequence.
func (m *splitCopier) copyTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), headerBodySplitCopyBudget)
	defer cancel()
	var total int64
	for _, table := range m.tables {
		cols, err := m.insertableColumns(ctx, "old", table.name)
		if err != nil {
			return err
		}
		list := strings.Join(cols, ", ")
		done := objectgraph.BootPhase("store: open", "header-body-split/copy-tables/"+table.name)
		n, err := m.exec(ctx, `INSERT INTO main.`+quoteIdent(table.name)+` (`+list+`) SELECT `+list+` FROM old.`+quoteIdent(table.name))
		done(err)
		if err != nil {
			return fmt.Errorf("copy %s: %w", table.name, err)
		}
		m.report(n)
		total += n
	}
	m.side.Rows["other_tables"] = total
	if m.hasOld["sqlite_sequence"] {
		var hasMain int
		if err := m.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE name = 'sqlite_sequence'`).Scan(&hasMain); err != nil {
			return err
		}
		if hasMain > 0 {
			if _, err := m.exec(ctx, `DELETE FROM main.sqlite_sequence`); err != nil {
				return err
			}
			if _, err := m.exec(ctx, `INSERT INTO main.sqlite_sequence (name, seq) SELECT name, seq FROM old.sqlite_sequence`); err != nil {
				return fmt.Errorf("copy sqlite_sequence: %w", err)
			}
		}
	}
	return nil
}

// splitObjectHeaderColumns are og_objects' stored header columns in the new
// layout, after obj_id and before the two body-derived ones.
var splitObjectHeaderColumns = []string{"canonical_id", "object_kind", "owner_id", "computer_id", "version_id", "content_hash", "metadata", "created_at", "updated_at", "tombstone", "superseded_by"}

// splitEventHeaderColumns are choir_events' stored header columns, after
// ev_id and before event_phase.
var splitEventHeaderColumns = []string{"tape_seq", "op_index", "canonical_id", "object_kind", "owner_id", "computer_id", "version_id", "content_hash", "metadata", "created_at", "updated_at", "tombstone", "superseded_by", "run_id", "seq", "stream_seq", "trajectory_id", "event_kind"}

func prefixed(alias string, cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + "." + c
	}
	return strings.Join(out, ", ")
}

// splitObjectDerived are the body-derived header values over the old row o.
func splitObjectDerived() (claim, phase string) {
	return objectgraph.BodyDerivedSQL("o.object_kind", "o.body", "o.metadata")
}

func splitEventPhase() string { return objectgraph.ChoirEventPhaseSQLFor("o.body", "o.metadata") }

func (m *splitCopier) copyObjects() error {
	claim, phase := splitObjectDerived()
	header := `INSERT INTO main.og_objects (obj_id, ` + strings.Join(splitObjectHeaderColumns, ", ") + `, og_rec_claim, event_phase)
		SELECT o.rowid, ` + prefixed("o", splitObjectHeaderColumns) + `, ` + claim + `, ` + phase + `
		FROM old.og_objects o WHERE o.rowid > ? AND o.rowid <= ?`
	body := `INSERT INTO main.og_bodies (obj_id, body) SELECT o.rowid, o.body FROM old.og_objects o WHERE o.rowid > ? AND o.rowid <= ?`
	return m.copySplitTable("og_objects", header, body)
}

func (m *splitCopier) copyEvents() error {
	if !m.hasOld[choirEventsTable] {
		m.side.Rows[choirEventsTable] = 0
		return nil
	}
	header := `INSERT INTO main.choir_events (ev_id, ` + strings.Join(splitEventHeaderColumns, ", ") + `, event_phase)
		SELECT o.rowid, ` + prefixed("o", splitEventHeaderColumns) + `, ` + splitEventPhase() + `
		FROM old.choir_events o WHERE o.rowid > ? AND o.rowid <= ?`
	body := `INSERT INTO main.choir_event_bodies (ev_id, body) SELECT o.rowid, o.body FROM old.choir_events o WHERE o.rowid > ? AND o.rowid <= ?`
	return m.copySplitTable(choirEventsTable, header, body)
}

// copySplitTable copies one split table in rowid ranges of
// headerBodySplitBatch rows, the header then the body of each range in one
// transaction. The new key is the old rowid: deterministic, and the physical
// order is kept.
func (m *splitCopier) copySplitTable(table, headerSQL, bodySQL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), headerBodySplitCopyBudget)
	defer cancel()
	var maxRowid, total int64
	if err := m.conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(rowid), 0) FROM old.`+table).Scan(&maxRowid); err != nil {
		return err
	}
	if err := m.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM old.`+table).Scan(&total); err != nil {
		return err
	}
	nextSQL := fmt.Sprintf(`SELECT rowid FROM old.%s WHERE rowid > ? ORDER BY rowid LIMIT 1 OFFSET %d`, table, headerBodySplitBatch-1)
	started, lastLog := time.Now(), time.Now()
	var cursor, headers, bodies int64
	for cursor < maxRowid {
		hi := maxRowid
		switch err := m.conn.QueryRowContext(ctx, nextSQL, cursor).Scan(&hi); {
		case errors.Is(err, sql.ErrNoRows):
			hi = maxRowid
		case err != nil:
			return err
		}
		tx, err := m.conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		h, err := tx.ExecContext(ctx, headerSQL, cursor, hi)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("copy %s headers (%d, %d]: %w", table, cursor, hi, err)
		}
		b, err := tx.ExecContext(ctx, bodySQL, cursor, hi)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("copy %s bodies (%d, %d]: %w", table, cursor, hi, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		hn, _ := h.RowsAffected()
		bn, _ := b.RowsAffected()
		headers += hn
		bodies += bn
		m.report(hn)
		cursor = hi
		if time.Since(lastLog) >= headerBodySplitLogEvery {
			lastLog = time.Now()
			elapsed := time.Since(started)
			log.Printf("store: header-body-split: %s rows=%d/%d cursor=%d/%d elapsed=%s rate=%.0f rows/s",
				table, headers, total, cursor, maxRowid, elapsed.Round(time.Second), float64(headers)/elapsed.Seconds())
		}
	}
	if headers != total || bodies != total {
		return fmt.Errorf("copy %s: %d rows in the old table, %d headers and %d bodies copied", table, total, headers, bodies)
	}
	m.side.Rows[table] = total
	log.Printf("store: header-body-split: %s copied rows=%d in %s", table, total, time.Since(started).Round(time.Millisecond))
	return nil
}

// buildIndexes creates every secondary index (the other tables' verbatim,
// then the split tables' from the new DDL), one statement at a time, then the
// triggers and views.
func (m *splitCopier) buildIndexes() error {
	type index struct{ name, sql string }
	var all []index
	for _, ix := range m.indexes {
		all = append(all, index{ix.name, ix.sql})
	}
	_, eventIndexes := splitChoirEventsDDL()
	for _, text := range append(objectgraph.ObjectIndexDDL(), eventIndexes...) {
		all = append(all, index{splitIndexName(text), text})
	}
	for _, ix := range all {
		ctx, cancel := context.WithTimeout(context.Background(), headerBodySplitIndexBudget)
		done := objectgraph.BootPhase("store: open", "header-body-split/build-indexes/"+ix.name)
		_, err := m.conn.ExecContext(ctx, ix.sql)
		cancel()
		done(err)
		if err != nil {
			return fmt.Errorf("build %s: %w", ix.name, err)
		}
	}
	ctx := context.Background()
	for _, entry := range append(append([]splitSchemaEntry{}, m.triggers...), m.views...) {
		if _, err := m.conn.ExecContext(ctx, entry.sql); err != nil {
			return fmt.Errorf("create %s: %w", entry.name, err)
		}
	}
	return nil
}

// splitIndexName is the index name of a CREATE INDEX [IF NOT EXISTS] statement.
func splitIndexName(text string) string {
	fields := strings.Fields(text)
	for i, f := range fields {
		if strings.EqualFold(f, "ON") && i > 0 {
			return fields[i-1]
		}
	}
	return text
}

// splitSame is a byte-exact comparison of two values: the same storage class
// and the same value (NULL equals NULL).
func splitSame(a, b string) string {
	return `(` + a + ` IS ` + b + ` AND typeof(` + a + `) = typeof(` + b + `))`
}

// verify checks the copy: row tallies of every table, a random sample of
// each split table compared byte for byte with the joined new row, then sets
// the schema version, switches the file to WAL and closes it.
func (m *splitCopier) verify() error {
	ctx, cancel := context.WithTimeout(context.Background(), headerBodySplitCopyBudget)
	defer cancel()
	count := func(query string) (int64, error) {
		var n int64
		err := m.conn.QueryRowContext(ctx, query).Scan(&n)
		return n, err
	}
	for _, table := range m.tables {
		oldN, err := count(`SELECT COUNT(*) FROM old.` + quoteIdent(table.name))
		if err != nil {
			return err
		}
		newN, err := count(`SELECT COUNT(*) FROM main.` + quoteIdent(table.name))
		if err != nil {
			return err
		}
		if oldN != newN {
			return fmt.Errorf("verify %s: %d rows in the old file, %d in the copy", table.name, oldN, newN)
		}
	}
	type splitTable struct {
		name, header, body, key string
		cols                    []string
		derived                 [][2]string
	}
	claim, phase := splitObjectDerived()
	split := []splitTable{{
		name: "og_objects", header: "og_objects", body: "og_bodies", key: "obj_id",
		cols: splitObjectHeaderColumns, derived: [][2]string{{claim, "h.og_rec_claim"}, {phase, "h.event_phase"}},
	}}
	if m.hasOld[choirEventsTable] {
		split = append(split, splitTable{
			name: choirEventsTable, header: choirEventsTable, body: "choir_event_bodies", key: "ev_id",
			cols: splitEventHeaderColumns, derived: [][2]string{{splitEventPhase(), "h.event_phase"}},
		})
	}
	for _, t := range split {
		oldN, err := count(`SELECT COUNT(*) FROM old.` + t.name)
		if err != nil {
			return err
		}
		headers, err := count(`SELECT COUNT(*) FROM main.` + t.header)
		if err != nil {
			return err
		}
		bodies, err := count(`SELECT COUNT(*) FROM main.` + t.body)
		if err != nil {
			return err
		}
		if oldN != headers || oldN != bodies || oldN != m.side.Rows[t.name] {
			return fmt.Errorf("verify %s: old=%d headers=%d bodies=%d copied=%d", t.name, oldN, headers, bodies, m.side.Rows[t.name])
		}
		sample, err := m.sampleRowids(ctx, t.name, oldN)
		if err != nil {
			return err
		}
		conds := make([]string, 0, len(t.cols)+len(t.derived)+1)
		for _, c := range t.cols {
			conds = append(conds, splitSame("o."+c, "h."+c))
		}
		conds = append(conds, splitSame("o.body", "b.body"))
		for _, d := range t.derived {
			conds = append(conds, splitSame(d[0], d[1]))
		}
		check := `SELECT COUNT(*) FROM old.` + t.name + ` o
			JOIN main.` + t.header + ` h ON h.` + t.key + ` = o.rowid
			JOIN main.` + t.body + ` b ON b.` + t.key + ` = o.rowid
			WHERE o.rowid = ? AND ` + strings.Join(conds, " AND ")
		for _, rowid := range sample {
			var n int
			if err := m.conn.QueryRowContext(ctx, check, rowid).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("verify %s: the row at rowid %d differs between the old file and the copy", t.name, rowid)
			}
		}
		log.Printf("store: header-body-split: verify %s rows=%d sample=%d ok", t.name, oldN, len(sample))
	}
	if _, err := m.exec(ctx, storeMetaDDL); err != nil {
		return err
	}
	if _, err := m.exec(ctx, `INSERT INTO main.store_meta (key, value) VALUES ('schema_version', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, strconv.Itoa(storeSchemaVersion)); err != nil {
		return fmt.Errorf("store_meta schema_version: %w", err)
	}
	if _, err := m.exec(ctx, `DETACH DATABASE old`); err != nil {
		return err
	}
	var mode string
	if err := m.conn.QueryRowContext(ctx, `PRAGMA journal_mode=WAL`).Scan(&mode); err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("journal_mode=WAL answered %q", mode)
	}
	m.close()
	return nil
}

// sampleRowids picks up to headerBodySplitSample distinct rowids of the old
// table at random, by seeks (never a scan): every one when the table is
// smaller.
func (m *splitCopier) sampleRowids(ctx context.Context, table string, n int64) ([]int64, error) {
	if n <= int64(headerBodySplitSample) {
		rows, err := m.conn.QueryContext(ctx, `SELECT rowid FROM old.`+table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		return out, rows.Err()
	}
	var lo, hi int64
	if err := m.conn.QueryRowContext(ctx, `SELECT MIN(rowid), MAX(rowid) FROM old.`+table).Scan(&lo, &hi); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	seek := `SELECT rowid FROM old.` + table + ` WHERE rowid >= ? ORDER BY rowid LIMIT 1`
	for tries := 0; len(seen) < headerBodySplitSample && tries < 4*headerBodySplitSample; tries++ {
		var id int64
		if err := m.conn.QueryRowContext(ctx, seek, lo+rand.Int64N(hi-lo+1)).Scan(&id); err != nil {
			return nil, err
		}
		seen[id] = true
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
