package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Online history compaction (docs/problems/guest-dolt-history-bound-2026-10-15.md).
//
// The journal GC (dolt_maintenance.go) bounds garbage that no commit
// reaches. It cannot bound history: every Dolt checkpoint (one per 100
// finalized events) pins the prolly-tree nodes it changed, and a default GC
// promotes them to the old generation, which only a full GC collects, and
// only once nothing reaches them. On the paper that is about 6 GB a day.
//
// Nothing reads that history. The tape is the authority; the store is its
// projection. Texture history follows the revision parent chain, the content
// witness is the content root (not HEAD), and HASHOF('HEAD') is audit-only.
// So the store keeps one commit over its root: a squash (soft reset to the
// root, commit the working set) followed by DOLT_GC('--full'). It changes no
// row. It never touches computer_event_index, the projection heads, or any
// projected row; it moves only the Dolt commit graph.
//
// It runs on the open store's own write connection, held exclusively for the
// whole compaction, so no write interleaves with the checks, the squash or
// the GC. The checks are the offline runbook's (projection heads, event index
// count) plus every table's row count, before the squash, after it, and after
// the GC.

const (
	// defaultDoltHistoryBudgetMiB is the history (store growth since the last
	// compaction) that triggers a compaction. Full GC rewrites the live set,
	// so a budget near the live size (about 2 GiB on the paper) amortizes the
	// rewrite against the history it frees.
	defaultDoltHistoryBudgetMiB = 2048
	// doltCompactionBudgetFloorBytes floors the headroom-scaled budget, so a
	// nearly full disk does not compact on every tick.
	doltCompactionBudgetFloorBytes = 512 * mibBytes
	// doltCompactionHeadroomDivisor: the budget never exceeds 1/divisor of
	// free space, so history is reclaimed while the output still fits.
	doltCompactionHeadroomDivisor = 4
	// doltCompactionAvailMarginBytes is kept free beyond the full GC's
	// largest possible output (everything the store holds now).
	doltCompactionAvailMarginBytes = 512 * mibBytes
	// doltCompactionFailureBackoff spaces retries after a failed or refused
	// compaction: each attempt holds the write connection for a full GC.
	doltCompactionFailureBackoff = 6 * time.Hour
	doltCompactionRecordFileName = ".choir-dolt-compaction.json"
	// doltRootCommitQuery finds the store's first commit (the offline tool's
	// query, internal/projectionbase/compact.go).
	doltRootCommitQuery = "SELECT commit_hash FROM dolt_log ORDER BY date ASC, commit_hash ASC LIMIT 1"
)

// errDoltCompactionRefused marks a compaction that did not start because a
// precondition failed; the store is unchanged.
var errDoltCompactionRefused = errors.New("dolt compaction refused")

// doltCompactionRecord is the last compaction decision, written on every
// tick and served on guest /health as dolt_compaction.
type doltCompactionRecord struct {
	At               string `json:"at"`
	Outcome          string `json:"outcome"` // ran | noop | deferred_space | backoff | refused | error | skipped_disabled
	StoreMiB         uint64 `json:"store_mib"`
	BaselineBytes    uint64 `json:"baseline_bytes"`
	HistoryMiB       uint64 `json:"history_mib"`
	BudgetMiB        uint64 `json:"budget_mib,omitempty"`
	AvailMiB         uint64 `json:"avail_mib"`
	RequiredAvailMiB uint64 `json:"required_avail_mib,omitempty"`
	CommitsBefore    int    `json:"commits_before,omitempty"`
	CommitsAfter     int    `json:"commits_after,omitempty"`
	AfterStoreMiB    uint64 `json:"after_store_mib,omitempty"`
	AfterAvailMiB    uint64 `json:"after_avail_mib,omitempty"`
	DurationMS       int64  `json:"duration_ms,omitempty"`
	WitnessMS        int64  `json:"witness_ms,omitempty"`
	SquashMS         int64  `json:"squash_ms,omitempty"`
	GCMS             int64  `json:"gc_ms,omitempty"`
	HeadsDigest      string `json:"heads_digest,omitempty"`
	EventIndexCount  int64  `json:"event_index_count,omitempty"`
	LastSuccessAt    string `json:"last_success_at,omitempty"`
	LastFailureAt    string `json:"last_failure_at,omitempty"`
	Detail           string `json:"detail,omitempty"`
}

// ReadDoltCompactionRecord returns the last compaction record under
// persistentDir, or nil. Served on guest /health.
func ReadDoltCompactionRecord(persistentDir string) json.RawMessage {
	raw, ok := readDoltCompactionRecordBytes(persistentDir)
	if !ok {
		return nil
	}
	return json.RawMessage(raw)
}

func readDoltCompactionRecordBytes(persistentDir string) ([]byte, bool) {
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(persistentDir, doltCompactionRecordFileName))
	if err != nil {
		return nil, false
	}
	raw = []byte(strings.TrimSpace(string(raw)))
	if !json.Valid(raw) {
		return nil, false
	}
	return raw, true
}

func loadDoltCompactionRecord(persistentDir string) doltCompactionRecord {
	var rec doltCompactionRecord
	if raw, ok := readDoltCompactionRecordBytes(persistentDir); ok {
		_ = json.Unmarshal(raw, &rec)
	}
	return rec
}

func writeDoltCompactionRecord(persistentDir string, rec doltCompactionRecord) {
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return
	}
	if strings.TrimSpace(rec.At) == "" {
		rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return
	}
	path := filepath.Join(persistentDir, doltCompactionRecordFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// doltCompactionInputs is what one compaction decision reads.
type doltCompactionInputs struct {
	Usage doltGCDiskUsage
	// StoreBytes is the non-journal noms bytes: live data plus history.
	StoreBytes   uint64
	JournalBytes uint64
	// BaselineBytes is StoreBytes right after the last compaction (0 when
	// the store was never compacted online).
	BaselineBytes uint64
	BudgetBytes   uint64
	LastFailureAt time.Time
	Now           time.Time
}

type doltCompactionPlan struct {
	Run                bool
	Outcome            string // when not run: noop | deferred_space | backoff
	BaselineBytes      uint64
	HistoryBytes       uint64
	BudgetBytes        uint64
	RequiredAvailBytes uint64
	Reason             string
}

// planDoltCompaction is the history compaction policy. History is estimated
// as the store's growth since the last compaction (live growth counts too, so
// it errs early). There is no store-size gate: a full GC walks only what is
// reachable, so its cost follows the live set, not the store (a size gate is
// how the journal GC switched itself off for good, 2026-10-14).
func planDoltCompaction(in doltCompactionInputs) doltCompactionPlan {
	plan := doltCompactionPlan{BaselineBytes: in.BaselineBytes}
	if plan.BaselineBytes > in.StoreBytes {
		// Rebased, restored or compacted offline: the store is the baseline.
		plan.BaselineBytes = in.StoreBytes
	}
	plan.HistoryBytes = in.StoreBytes - plan.BaselineBytes
	budget := in.BudgetBytes
	if budget == 0 {
		budget = defaultDoltHistoryBudgetMiB * mibBytes
	}
	byHeadroom := in.Usage.AvailBytes / doltCompactionHeadroomDivisor
	if byHeadroom < doltCompactionBudgetFloorBytes {
		byHeadroom = doltCompactionBudgetFloorBytes
	}
	if byHeadroom < budget {
		budget = byHeadroom
	}
	plan.BudgetBytes = budget
	// Full GC writes everything reachable before it deletes anything; that
	// cannot exceed what the store holds now.
	plan.RequiredAvailBytes = in.StoreBytes + in.JournalBytes + doltCompactionAvailMarginBytes

	if plan.HistoryBytes < plan.BudgetBytes {
		plan.Outcome = "noop"
		plan.Reason = fmt.Sprintf("history %d MiB < budget %d MiB", plan.HistoryBytes/mibBytes, plan.BudgetBytes/mibBytes)
		return plan
	}
	if !in.LastFailureAt.IsZero() && in.Now.Sub(in.LastFailureAt) < doltCompactionFailureBackoff {
		plan.Outcome = "backoff"
		plan.Reason = fmt.Sprintf("history %d MiB >= budget %d MiB; last failure at %s, retry after %s",
			plan.HistoryBytes/mibBytes, plan.BudgetBytes/mibBytes, in.LastFailureAt.UTC().Format(time.RFC3339), doltCompactionFailureBackoff)
		return plan
	}
	if in.Usage.AvailBytes < plan.RequiredAvailBytes {
		plan.Outcome = "deferred_space"
		plan.Reason = fmt.Sprintf("history %d MiB >= budget %d MiB, but avail %d MiB < %d MiB a full gc may write; offline compaction (docs/runbooks/offline-guest-gc.md)",
			plan.HistoryBytes/mibBytes, plan.BudgetBytes/mibBytes, in.Usage.AvailBytes/mibBytes, plan.RequiredAvailBytes/mibBytes)
		return plan
	}
	plan.Run = true
	plan.Reason = fmt.Sprintf("history %d MiB >= budget %d MiB (store %d MiB, baseline %d MiB, avail %d MiB)",
		plan.HistoryBytes/mibBytes, plan.BudgetBytes/mibBytes, in.StoreBytes/mibBytes, plan.BaselineBytes/mibBytes, in.Usage.AvailBytes/mibBytes)
	return plan
}

func doltHistoryBudgetBytes() uint64 {
	raw := strings.TrimSpace(os.Getenv("RUNTIME_DOLT_HISTORY_BUDGET_MIB"))
	if raw == "" {
		return defaultDoltHistoryBudgetMiB * mibBytes
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return defaultDoltHistoryBudgetMiB * mibBytes
	}
	return v * mibBytes
}

// historyBudgetForCompaction is the budget source, indirected so tests can
// stage a store-sized budget (the env is in MiB).
var historyBudgetForCompaction = doltHistoryBudgetBytes

func doltCompactionDisabled() bool {
	if doltGCDisabled() {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RUNTIME_DOLT_COMPACT_DISABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// MaybeCompactDoltHistory measures, plans and (when the plan says so) runs
// one online history compaction, and records the decision. The periodic loop
// calls it after the journal GC.
func (s *Store) MaybeCompactDoltHistory(ctx context.Context, persistentDir string) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("dolt compaction: nil store")
	}
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	prior := loadDoltCompactionRecord(persistentDir)
	carry := func(rec doltCompactionRecord) doltCompactionRecord {
		rec.LastSuccessAt = prior.LastSuccessAt
		rec.LastFailureAt = prior.LastFailureAt
		if rec.BaselineBytes == 0 {
			rec.BaselineBytes = prior.BaselineBytes
		}
		return rec
	}
	if doltCompactionDisabled() {
		writeDoltCompactionRecord(persistentDir, carry(doltCompactionRecord{Outcome: "skipped_disabled", Detail: "RUNTIME_DOLT_COMPACT_DISABLED or RUNTIME_DOLT_GC_DISABLED"}))
		return nil
	}
	usage, err := diskUsageForGC(persistentDir)
	if err != nil {
		return err
	}
	in := doltCompactionInputs{
		Usage:         usage,
		StoreBytes:    doltLiveStoreBytes(s.texturePath),
		JournalBytes:  doltJournalBytes(s.texturePath),
		BaselineBytes: prior.BaselineBytes,
		BudgetBytes:   historyBudgetForCompaction(),
		Now:           time.Now().UTC(),
	}
	if t, err := time.Parse(time.RFC3339Nano, prior.LastFailureAt); err == nil {
		in.LastFailureAt = t
	}
	plan := planDoltCompaction(in)
	rec := carry(doltCompactionRecord{
		StoreMiB:         in.StoreBytes / mibBytes,
		BaselineBytes:    plan.BaselineBytes,
		HistoryMiB:       plan.HistoryBytes / mibBytes,
		BudgetMiB:        plan.BudgetBytes / mibBytes,
		AvailMiB:         usage.AvailBytes / mibBytes,
		RequiredAvailMiB: plan.RequiredAvailBytes / mibBytes,
		Detail:           plan.Reason,
	})
	rec.BaselineBytes = plan.BaselineBytes
	if !plan.Run {
		rec.Outcome = plan.Outcome
		writeDoltCompactionRecord(persistentDir, rec)
		if plan.Outcome == "deferred_space" {
			log.Printf("store: dolt compaction deferred: %s", plan.Reason)
		}
		return nil
	}

	log.Printf("store: dolt compaction (online) starting: %s", plan.Reason)
	started := time.Now()
	result, runErr := s.compactDoltHistoryOnline(ctx, plan.Reason)
	rec.DurationMS = time.Since(started).Milliseconds()
	rec.CommitsBefore = result.CommitsBefore
	rec.WitnessMS = result.WitnessDuration.Milliseconds()
	rec.SquashMS = result.SquashDuration.Milliseconds()
	rec.GCMS = result.GCDuration.Milliseconds()
	rec.CommitsAfter = result.CommitsAfter
	rec.HeadsDigest = result.Before.HeadsDigest
	rec.EventIndexCount = result.Before.EventIndexCount
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if runErr != nil {
		rec.Outcome = "error"
		if errors.Is(runErr, errDoltCompactionRefused) {
			rec.Outcome = "refused"
		}
		rec.LastFailureAt = now
		rec.Detail = plan.Reason + ": " + runErr.Error()
		writeDoltCompactionRecord(persistentDir, rec)
		log.Printf("store: dolt compaction (online) %s: %v", rec.Outcome, runErr)
		return runErr
	}
	rec.Outcome = "ran"
	rec.LastSuccessAt = now
	rec.LastFailureAt = ""
	after := doltLiveStoreBytes(s.texturePath)
	rec.AfterStoreMiB = after / mibBytes
	rec.BaselineBytes = after
	if afterUsage, err := diskUsageForGC(persistentDir); err == nil {
		rec.AfterAvailMiB = afterUsage.AvailBytes / mibBytes
	}
	writeDoltCompactionRecord(persistentDir, rec)
	log.Printf("store: dolt compaction (online) complete in %s: commits %d -> %d, store %d MiB -> %d MiB, avail %d MiB -> %d MiB, heads %s, event index %d",
		time.Since(started).Round(time.Second), result.CommitsBefore, result.CommitsAfter, rec.StoreMiB, rec.AfterStoreMiB, rec.AvailMiB, rec.AfterAvailMiB,
		shortDigest(rec.HeadsDigest), rec.EventIndexCount)
	return nil
}

func shortDigest(digest string) string {
	if len(digest) > 16 {
		return digest[:16]
	}
	return digest
}

// doltCompactionWitness is what a compaction must leave unchanged.
type doltCompactionWitness struct {
	// HeadsDigest digests every computer_event_projection_heads row's
	// sequence and heads (the runbook's head check, for all computers).
	HeadsDigest     string
	EventIndexCount int64
	TableRows       map[string]int64
}

func (w doltCompactionWitness) diff(other doltCompactionWitness) error {
	if w.HeadsDigest != other.HeadsDigest {
		return fmt.Errorf("projection heads digest %s -> %s", shortDigest(w.HeadsDigest), shortDigest(other.HeadsDigest))
	}
	if w.EventIndexCount != other.EventIndexCount {
		return fmt.Errorf("computer_event_index count %d -> %d", w.EventIndexCount, other.EventIndexCount)
	}
	if len(w.TableRows) != len(other.TableRows) {
		return fmt.Errorf("table count %d -> %d", len(w.TableRows), len(other.TableRows))
	}
	for table, rows := range w.TableRows {
		if got, ok := other.TableRows[table]; !ok || got != rows {
			return fmt.Errorf("table %s rows %d -> %d", table, rows, got)
		}
	}
	return nil
}

func readDoltCompactionWitness(ctx context.Context, conn *sql.Conn) (doltCompactionWitness, error) {
	w := doltCompactionWitness{TableRows: map[string]int64{}}
	rows, err := conn.QueryContext(ctx, `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head FROM computer_event_projection_heads ORDER BY computer_id`)
	if err != nil {
		return w, fmt.Errorf("read projection heads: %w", err)
	}
	h := sha256.New()
	for rows.Next() {
		var computerID, canonical, desired, effective string
		var sequence uint64
		if err := rows.Scan(&computerID, &sequence, &canonical, &desired, &effective); err != nil {
			_ = rows.Close()
			return w, fmt.Errorf("scan projection heads: %w", err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00%s\x00%s\x00%s\n", computerID, sequence, canonical, desired, effective)
	}
	if err := rows.Close(); err != nil {
		return w, err
	}
	if err := rows.Err(); err != nil {
		return w, err
	}
	w.HeadsDigest = hex.EncodeToString(h.Sum(nil))

	tables, err := conn.QueryContext(ctx, "SHOW FULL TABLES WHERE Table_type = 'BASE TABLE'")
	if err != nil {
		return w, fmt.Errorf("list tables: %w", err)
	}
	var names []string
	for tables.Next() {
		var name, kind string
		if err := tables.Scan(&name, &kind); err != nil {
			_ = tables.Close()
			return w, fmt.Errorf("scan table name: %w", err)
		}
		names = append(names, name)
	}
	if err := tables.Close(); err != nil {
		return w, err
	}
	sort.Strings(names)
	for _, name := range names {
		var n int64
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+strings.ReplaceAll(name, "`", "``")+"`").Scan(&n); err != nil {
			return w, fmt.Errorf("count %s: %w", name, err)
		}
		w.TableRows[name] = n
	}
	w.EventIndexCount = w.TableRows["computer_event_index"]
	return w, nil
}

type doltCompactionResult struct {
	Squashed      bool
	CommitsBefore int
	CommitsAfter  int
	Before        doltCompactionWitness
	After         doltCompactionWitness
	// Phase durations: the three witness reads, the squash, the full GC.
	WitnessDuration time.Duration
	SquashDuration  time.Duration
	GCDuration      time.Duration
}

// compactDoltHistoryOnline squashes the store's single branch to one commit
// over its root and runs a full GC, on the store's own write connection held
// for the duration. It refuses (unchanged store) unless there is exactly one
// branch, and fails unless the witness is identical before the squash, after
// it, and after the GC. A witness change after the squash moves HEAD back;
// one after the GC cannot be undone in place, and the projection is then
// rebuilt from the tape like any damaged projection.
func (s *Store) compactDoltHistoryOnline(ctx context.Context, reason string) (doltCompactionResult, error) {
	var result doltCompactionResult
	s.doltCommitMu.Lock()
	defer s.doltCommitMu.Unlock()
	// The pool holds one connection; holding it excludes every other writer
	// until the compaction ends.
	conn, err := s.textureHandle().Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("dolt compaction: acquire write connection: %w", err)
	}
	defer conn.Close()

	var branches int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_branches").Scan(&branches); err != nil {
		return result, fmt.Errorf("dolt compaction: count branches: %w", err)
	}
	if branches != 1 {
		return result, fmt.Errorf("%w: %d branches keep history reachable; want exactly one", errDoltCompactionRefused, branches)
	}
	phase := time.Now()
	before, err := readDoltCompactionWitness(ctx, conn)
	result.WitnessDuration += time.Since(phase)
	if err != nil {
		return result, fmt.Errorf("%w: witness before: %v", errDoltCompactionRefused, err)
	}
	result.Before = before
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsBefore); err != nil {
		return result, fmt.Errorf("dolt compaction: count commits: %w", err)
	}
	if result.CommitsBefore > 2 {
		var oldHead, root string
		if err := conn.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&oldHead); err != nil {
			return result, fmt.Errorf("dolt compaction: read head: %w", err)
		}
		if err := conn.QueryRowContext(ctx, doltRootCommitQuery).Scan(&root); err != nil {
			return result, fmt.Errorf("dolt compaction: find root commit: %w", err)
		}
		// HEAD moves to the root; the working set (every row) stays. A crash
		// before the commit below leaves an uncommitted working set that the
		// next checkpoint commits.
		phase = time.Now()
		if _, err := conn.ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", root); err != nil {
			return result, fmt.Errorf("dolt compaction: soft reset to root: %w", err)
		}
		message := fmt.Sprintf("history compaction: %d commits squashed at projection heads %s (%s)", result.CommitsBefore, shortDigest(before.HeadsDigest), reason)
		if _, err := conn.ExecContext(ctx, "CALL DOLT_COMMIT('-A', '--allow-empty', '-m', ?)", message); err != nil {
			_, _ = conn.ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", oldHead)
			return result, fmt.Errorf("dolt compaction: squash commit: %w", err)
		}
		s.doltHistoryDirty = false
		result.SquashDuration = time.Since(phase)
		phase = time.Now()
		mid, err := readDoltCompactionWitness(ctx, conn)
		result.WitnessDuration += time.Since(phase)
		if err == nil {
			err = before.diff(mid)
		}
		if err != nil {
			_, _ = conn.ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", oldHead)
			return result, fmt.Errorf("dolt compaction: witness changed by the squash (head restored to %s): %w", shortDigest(oldHead), err)
		}
		result.Squashed = true
	}
	phase = time.Now()
	if _, err := conn.ExecContext(ctx, "CALL DOLT_GC('--full')"); err != nil {
		return result, fmt.Errorf("dolt compaction: full gc: %w", err)
	}
	result.GCDuration = time.Since(phase)
	phase = time.Now()
	after, err := readDoltCompactionWitness(ctx, conn)
	result.WitnessDuration += time.Since(phase)
	if err != nil {
		return result, fmt.Errorf("dolt compaction: witness after full gc: %w", err)
	}
	result.After = after
	if err := before.diff(after); err != nil {
		return result, fmt.Errorf("dolt compaction: witness changed by the full gc; rebuild the projection from the tape: %w", err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsAfter); err != nil {
		return result, fmt.Errorf("dolt compaction: count commits after: %w", err)
	}
	return result, nil
}
