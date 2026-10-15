package store

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log"
	"math/rand/v2"
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
// Gate (review, 2026-10-15): off for the fleet. It runs only on computers
// listed in CHOIR_DOLT_COMPACT_COMPUTERS, never on the first tick after
// boot, and not before a random 0-30 minute delay. The record says
// in_progress (with the failure time already set) before a compaction
// starts, so a guest killed mid-run backs off for 6 h after it reboots.
//
// It runs on the open store's own write connection, held exclusively for the
// whole compaction, so no write interleaves with the checks, the squash or
// the GC. Two witnesses must match before and after:
//   - the light witness on the write connection (the offline runbook's
//     projection heads and event index count, plus every table's row
//     count), also checked between the squash and the GC;
//   - the full content witness (every row of every table, streamed into one
//     digest per table) on the read pool, a second session.
//
// A mismatch after the GC fails closed: the record says degraded with the
// reason, the 6 h backoff starts, and the log says so loudly. The projection
// is then rebuilt from the tape like any damaged projection.

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
	// doltCompactionFailureBackoff spaces retries after a failed, refused,
	// degraded or interrupted compaction: each attempt holds the write
	// connection for a full GC.
	doltCompactionFailureBackoff = 6 * time.Hour
	// doltCompactionMaxStartJitter spreads the first eligible compaction
	// after boot, so computers updated by one deploy do not all compact at
	// once.
	doltCompactionMaxStartJitter = 30 * time.Minute
	doltCompactionRecordFileName = ".choir-dolt-compaction.json"
	// doltCompactComputersEnv lists the ComputerIDs (comma or space
	// separated) that compact online. Empty or unset: no computer does.
	doltCompactComputersEnv = "CHOIR_DOLT_COMPACT_COMPUTERS"
)

// errDoltCompactionRefused marks a compaction that did not start because a
// precondition failed; the store is unchanged.
var errDoltCompactionRefused = errors.New("dolt compaction refused")

// errDoltCompactionDegraded marks a witness mismatch after the full GC: the
// store can no longer be shown equal to what it was, so it must be rebuilt
// from the tape.
var errDoltCompactionDegraded = errors.New("dolt compaction degraded")

// doltCompactionStartJitter is the delay before the first eligible
// compaction after boot, indirected for tests.
var doltCompactionStartJitter = defaultDoltCompactionStartJitter

func defaultDoltCompactionStartJitter() time.Duration {
	return rand.N(doltCompactionMaxStartJitter)
}

// doltCompactionAfterGCHook runs on the write connection after the full GC
// and before the post-GC witnesses. Tests only; nil in production.
var doltCompactionAfterGCHook func(ctx context.Context, conn *sql.Conn) error

// doltCompactionRecord is the last compaction decision, written on every
// tick and served on guest /health as dolt_compaction.
type doltCompactionRecord struct {
	At string `json:"at"`
	// Outcome: ran | noop | boot_wait | not_enabled | in_progress |
	// deferred_space | backoff | refused | error | degraded |
	// skipped_disabled.
	Outcome          string `json:"outcome"`
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
	// PeakRSSMiB is the runtime's resident high-water mark (VmHWM) after
	// the compaction: an upper bound on what the full GC needed.
	PeakRSSMiB      uint64 `json:"peak_rss_mib,omitempty"`
	HeadsDigest     string `json:"heads_digest,omitempty"`
	EventIndexCount int64  `json:"event_index_count,omitempty"`
	ContentRoot     string `json:"content_root,omitempty"`
	// WitnessMatch is true when both witnesses matched after the full GC.
	WitnessMatch  bool   `json:"witness_match,omitempty"`
	LastSuccessAt string `json:"last_success_at,omitempty"`
	LastFailureAt string `json:"last_failure_at,omitempty"`
	// Degraded names the witness mismatch of the last failed compaction. It
	// stays until a compaction succeeds.
	Degraded string `json:"degraded,omitempty"`
	Detail   string `json:"detail,omitempty"`
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

// writeDoltCompactionRecord replaces the record durably (fsync of the file
// and its directory), because the in_progress record must survive a VM kill
// mid-compaction.
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
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		_ = f.Close()
		return
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return
	}
	if err := f.Close(); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		return
	}
	if dir, err := os.Open(persistentDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
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

// doltCompactionEnabledFor reports whether the host listed this computer in
// CHOIR_DOLT_COMPACT_COMPUTERS. The default is off for every computer.
func doltCompactionEnabledFor(computerID string) bool {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return false
	}
	for _, id := range strings.FieldsFunc(os.Getenv(doltCompactComputersEnv), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if id == computerID {
			return true
		}
	}
	return false
}

// doltCompactionTick is one periodic-loop decision's timing context.
type doltCompactionTick struct {
	ComputerID string
	// First is the loop's first tick after boot: never a compaction.
	First bool
	// EligibleAt is boot plus the start jitter: no compaction before it.
	EligibleAt time.Time
	Now        time.Time
}

// MaybeCompactDoltHistory measures, plans and (when the gate and the plan
// say so) runs one online history compaction now, and records the decision.
// The periodic loop uses the boot-aware form.
func (s *Store) MaybeCompactDoltHistory(ctx context.Context, persistentDir, computerID string) error {
	return s.maybeCompactDoltHistory(ctx, persistentDir, doltCompactionTick{ComputerID: computerID, Now: time.Now()})
}

func (s *Store) maybeCompactDoltHistory(ctx context.Context, persistentDir string, tick doltCompactionTick) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("dolt compaction: nil store")
	}
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	if tick.Now.IsZero() {
		tick.Now = time.Now()
	}
	prior := loadDoltCompactionRecord(persistentDir)
	carry := func(rec doltCompactionRecord) doltCompactionRecord {
		rec.LastSuccessAt = prior.LastSuccessAt
		rec.LastFailureAt = prior.LastFailureAt
		rec.Degraded = prior.Degraded
		if rec.BaselineBytes == 0 {
			rec.BaselineBytes = prior.BaselineBytes
		}
		return rec
	}
	// A record still in_progress means the process died mid-compaction
	// (refresh, OOM, VM kill). Its failure time was written before the
	// compaction started, so the plan below backs off.
	interrupted := ""
	if prior.Outcome == "in_progress" {
		interrupted = fmt.Sprintf("previous compaction (started %s) was interrupted by a restart or crash; ", prior.At)
		log.Printf("store: dolt compaction: %sbacking off until %s", interrupted, doltCompactionRetryAt(prior.LastFailureAt))
	}
	if doltCompactionDisabled() {
		writeDoltCompactionRecord(persistentDir, carry(doltCompactionRecord{Outcome: "skipped_disabled", Detail: interrupted + "RUNTIME_DOLT_COMPACT_DISABLED or RUNTIME_DOLT_GC_DISABLED"}))
		return nil
	}
	if !doltCompactionEnabledFor(tick.ComputerID) {
		writeDoltCompactionRecord(persistentDir, carry(doltCompactionRecord{Outcome: "not_enabled", Detail: interrupted + "computer not listed in " + doltCompactComputersEnv}))
		return nil
	}
	if tick.First {
		writeDoltCompactionRecord(persistentDir, carry(doltCompactionRecord{Outcome: "boot_wait", Detail: interrupted + "first tick after boot; eligible at " + tick.EligibleAt.UTC().Format(time.RFC3339)}))
		return nil
	}
	if tick.Now.Before(tick.EligibleAt) {
		writeDoltCompactionRecord(persistentDir, carry(doltCompactionRecord{Outcome: "boot_wait", Detail: interrupted + "boot jitter; eligible at " + tick.EligibleAt.UTC().Format(time.RFC3339)}))
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
		Now:           tick.Now.UTC(),
	}
	if t, err := time.Parse(time.RFC3339Nano, prior.LastFailureAt); err == nil {
		in.LastFailureAt = t
	}
	plan := planDoltCompaction(in)
	rec := carry(doltCompactionRecord{
		StoreMiB:         in.StoreBytes / mibBytes,
		HistoryMiB:       plan.HistoryBytes / mibBytes,
		BudgetMiB:        plan.BudgetBytes / mibBytes,
		AvailMiB:         usage.AvailBytes / mibBytes,
		RequiredAvailMiB: plan.RequiredAvailBytes / mibBytes,
		Detail:           interrupted + plan.Reason,
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

	// Durable before anything changes: a kill from here on boots into the
	// backoff (FH14).
	started := time.Now().UTC()
	inProgress := rec
	inProgress.Outcome = "in_progress"
	inProgress.At = started.Format(time.RFC3339Nano)
	inProgress.LastFailureAt = inProgress.At
	writeDoltCompactionRecord(persistentDir, inProgress)

	log.Printf("store: dolt compaction (online) starting: %s", plan.Reason)
	result, runErr := s.compactDoltHistoryOnline(ctx, plan.Reason)
	rec.DurationMS = time.Since(started).Milliseconds()
	rec.CommitsBefore = result.CommitsBefore
	rec.CommitsAfter = result.CommitsAfter
	rec.WitnessMS = result.WitnessDuration.Milliseconds()
	rec.SquashMS = result.SquashDuration.Milliseconds()
	rec.GCMS = result.GCDuration.Milliseconds()
	rec.PeakRSSMiB = procStatusKiB("VmHWM") / 1024
	rec.HeadsDigest = result.Before.HeadsDigest
	rec.EventIndexCount = result.Before.EventIndexCount
	rec.ContentRoot = result.ContentBefore.Root
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if runErr != nil {
		rec.Outcome = "error"
		switch {
		case errors.Is(runErr, errDoltCompactionDegraded):
			rec.Outcome = "degraded"
			rec.Degraded = runErr.Error()
		case errors.Is(runErr, errDoltCompactionRefused):
			rec.Outcome = "refused"
		}
		rec.LastFailureAt = now
		rec.Detail = plan.Reason + ": " + runErr.Error()
		writeDoltCompactionRecord(persistentDir, rec)
		if rec.Outcome == "degraded" {
			log.Printf("store: DOLT COMPACTION DEGRADED: the store no longer matches its pre-compaction witness; rebuild the projection from the tape (docs/runbooks/offline-guest-gc.md). Compaction backs off until %s. %v",
				doltCompactionRetryAt(now), runErr)
		} else {
			log.Printf("store: dolt compaction (online) %s: %v", rec.Outcome, runErr)
		}
		return runErr
	}
	rec.Outcome = "ran"
	rec.WitnessMatch = true
	rec.LastSuccessAt = now
	rec.LastFailureAt = ""
	rec.Degraded = ""
	after := doltLiveStoreBytes(s.texturePath)
	rec.AfterStoreMiB = after / mibBytes
	rec.BaselineBytes = after
	if afterUsage, err := diskUsageForGC(persistentDir); err == nil {
		rec.AfterAvailMiB = afterUsage.AvailBytes / mibBytes
	}
	writeDoltCompactionRecord(persistentDir, rec)
	log.Printf("store: dolt compaction (online) complete in %s (gc %s, witness %s): commits %d -> %d, store %d MiB -> %d MiB, avail %d MiB -> %d MiB, heads %s, event index %d, content root %s, peak rss %d MiB",
		time.Since(started).Round(time.Second), result.GCDuration.Round(time.Millisecond), result.WitnessDuration.Round(time.Millisecond),
		result.CommitsBefore, result.CommitsAfter, rec.StoreMiB, rec.AfterStoreMiB, rec.AvailMiB, rec.AfterAvailMiB,
		shortDigest(rec.HeadsDigest), rec.EventIndexCount, shortDigest(rec.ContentRoot), rec.PeakRSSMiB)
	return nil
}

func doltCompactionRetryAt(lastFailureAt string) string {
	t, err := time.Parse(time.RFC3339Nano, lastFailureAt)
	if err != nil {
		return "unknown"
	}
	return t.Add(doltCompactionFailureBackoff).UTC().Format(time.RFC3339)
}

// procStatusKiB reads one kB field of /proc/self/status (Linux), or 0.
func procStatusKiB(field string) uint64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, field+":") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, field+":"))
		if len(fields) == 0 {
			return 0
		}
		v, _ := strconv.ParseUint(fields[0], 10, 64)
		return v
	}
	return 0
}

func shortDigest(digest string) string {
	if len(digest) > 16 {
		return digest[:16]
	}
	return digest
}

// doltCompactionWitness is the light witness a compaction must leave
// unchanged.
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

	names, err := listBaseTables(ctx, conn)
	if err != nil {
		return w, err
	}
	for _, name := range names {
		var n int64
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdent(name)).Scan(&n); err != nil {
			return w, fmt.Errorf("count %s: %w", name, err)
		}
		w.TableRows[name] = n
	}
	w.EventIndexCount = w.TableRows["computer_event_index"]
	return w, nil
}

type sqlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func listBaseTables(ctx context.Context, q sqlQuerier) ([]string, error) {
	tables, err := q.QueryContext(ctx, "SHOW FULL TABLES WHERE Table_type = 'BASE TABLE'")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	var names []string
	for tables.Next() {
		var name, kind string
		if err := tables.Scan(&name, &kind); err != nil {
			_ = tables.Close()
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		names = append(names, name)
	}
	if err := tables.Close(); err != nil {
		return nil, err
	}
	if err := tables.Err(); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// doltContentWitness is the full content witness: one digest per base table
// over every column of every row, in primary-key order, and a root over
// them. Rows are streamed into the digest, so memory stays flat however
// large the store is.
type doltContentWitness struct {
	Root   string
	Tables map[string]string
}

func (w doltContentWitness) diff(other doltContentWitness) error {
	if w.Root == other.Root {
		return nil
	}
	var changed []string
	for table, digest := range w.Tables {
		if other.Tables[table] != digest {
			changed = append(changed, table)
		}
	}
	for table := range other.Tables {
		if _, ok := w.Tables[table]; !ok {
			changed = append(changed, table)
		}
	}
	sort.Strings(changed)
	return fmt.Errorf("content root %s -> %s; tables changed: %s", shortDigest(w.Root), shortDigest(other.Root), strings.Join(changed, ", "))
}

func readDoltContentWitness(ctx context.Context, db *sql.DB) (doltContentWitness, error) {
	w := doltContentWitness{Tables: map[string]string{}}
	names, err := listBaseTables(ctx, db)
	if err != nil {
		return w, err
	}
	root := sha256.New()
	for _, name := range names {
		digest, err := tableContentDigest(ctx, db, name)
		if err != nil {
			return w, err
		}
		w.Tables[name] = digest
		fmt.Fprintf(root, "%s\x00%s\n", name, digest)
	}
	w.Root = "sha256:" + hex.EncodeToString(root.Sum(nil))
	return w, nil
}

func tableContentDigest(ctx context.Context, db *sql.DB, table string) (string, error) {
	keyRows, err := db.QueryContext(ctx, "SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY' ORDER BY SEQ_IN_INDEX", table)
	if err != nil {
		return "", fmt.Errorf("primary key of %s: %w", table, err)
	}
	var keys []string
	for keyRows.Next() {
		var col string
		if err := keyRows.Scan(&col); err != nil {
			_ = keyRows.Close()
			return "", fmt.Errorf("scan primary key of %s: %w", table, err)
		}
		keys = append(keys, quoteIdent(col))
	}
	if err := keyRows.Close(); err != nil {
		return "", err
	}
	query := "SELECT * FROM " + quoteIdent(table)
	if len(keys) > 0 {
		query += " ORDER BY " + strings.Join(keys, ", ")
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("columns of %s: %w", table, err)
	}
	h := sha256.New()
	writeJSONLine(h, cols)
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	n := 0
	for rows.Next() {
		if n++; n%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("scan %s: %w", table, err)
		}
		row := make([]any, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		writeJSONLine(h, row)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", table, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeJSONLine(h hash.Hash, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		raw = []byte(fmt.Sprintf("%v", v))
	}
	h.Write(raw)
	h.Write([]byte{'\n'})
}

// doltRootCommit returns the store's only parentless commit (FH18). More
// than one root refuses: a squash over one would leave the other history
// reachable through nothing, but its choice would be a guess.
func doltRootCommit(ctx context.Context, conn *sql.Conn) (string, error) {
	rows, err := conn.QueryContext(ctx, "SELECT commit_hash FROM dolt_commit_ancestors WHERE parent_hash IS NULL")
	if err != nil {
		return "", fmt.Errorf("find root commit: %w", err)
	}
	var roots []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			_ = rows.Close()
			return "", fmt.Errorf("scan root commit: %w", err)
		}
		roots = append(roots, hash)
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if len(roots) != 1 {
		return "", fmt.Errorf("%w: %d parentless commits; want exactly one root", errDoltCompactionRefused, len(roots))
	}
	return roots[0], nil
}

type doltCompactionResult struct {
	Squashed      bool
	CommitsBefore int
	CommitsAfter  int
	Before        doltCompactionWitness
	After         doltCompactionWitness
	ContentBefore doltContentWitness
	ContentAfter  doltContentWitness
	// Phase durations: the witness reads, the squash, the full GC.
	WitnessDuration time.Duration
	SquashDuration  time.Duration
	GCDuration      time.Duration
}

// compactDoltHistoryOnline squashes the store's single branch to one commit
// over its root and runs a full GC, on the store's own write connection held
// for the duration. It refuses (unchanged store) unless there is exactly one
// branch and one root. It fails unless the light witness is identical before
// the squash, after it and after the GC, and the full content witness (read
// on the read pool, a second session, while writes are still held off) is
// identical before and after. A light-witness change after the squash moves
// HEAD back. A mismatch after the GC cannot be undone in place: it returns
// errDoltCompactionDegraded, and the projection is rebuilt from the tape.
//
// The content witness is read before the write connection is released:
// once writes resume, the content legitimately changes and no comparison
// would mean anything.
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
	readDB := s.readDB
	if readDB == nil {
		return result, fmt.Errorf("%w: no read pool for the content witness", errDoltCompactionRefused)
	}

	var branches int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_branches").Scan(&branches); err != nil {
		return result, fmt.Errorf("dolt compaction: count branches: %w", err)
	}
	if branches != 1 {
		return result, fmt.Errorf("%w: %d branches keep history reachable; want exactly one", errDoltCompactionRefused, branches)
	}
	phase := time.Now()
	before, err := readDoltCompactionWitness(ctx, conn)
	if err != nil {
		return result, fmt.Errorf("%w: witness before: %v", errDoltCompactionRefused, err)
	}
	result.Before = before
	contentBefore, err := readDoltContentWitness(ctx, readDB)
	if err != nil {
		return result, fmt.Errorf("%w: content witness before: %v", errDoltCompactionRefused, err)
	}
	result.ContentBefore = contentBefore
	result.WitnessDuration += time.Since(phase)
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsBefore); err != nil {
		return result, fmt.Errorf("dolt compaction: count commits: %w", err)
	}
	if result.CommitsBefore > 2 {
		var oldHead string
		if err := conn.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&oldHead); err != nil {
			return result, fmt.Errorf("dolt compaction: read head: %w", err)
		}
		root, err := doltRootCommit(ctx, conn)
		if err != nil {
			return result, err
		}
		phase = time.Now()
		// HEAD moves to the root; the working set (every row) stays. A crash
		// before the commit below leaves an uncommitted working set that the
		// next checkpoint commits.
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
	if doltCompactionAfterGCHook != nil {
		if err := doltCompactionAfterGCHook(ctx, conn); err != nil {
			return result, fmt.Errorf("dolt compaction: after-gc hook: %w", err)
		}
	}
	phase = time.Now()
	after, err := readDoltCompactionWitness(ctx, conn)
	if err != nil {
		return result, fmt.Errorf("%w: light witness unreadable after the full gc: %v", errDoltCompactionDegraded, err)
	}
	result.After = after
	if err := before.diff(after); err != nil {
		return result, fmt.Errorf("%w: light witness changed by the full gc: %v", errDoltCompactionDegraded, err)
	}
	contentAfter, err := readDoltContentWitness(ctx, readDB)
	result.WitnessDuration += time.Since(phase)
	if err != nil {
		return result, fmt.Errorf("%w: content witness unreadable after the full gc: %v", errDoltCompactionDegraded, err)
	}
	result.ContentAfter = contentAfter
	if err := contentBefore.diff(contentAfter); err != nil {
		return result, fmt.Errorf("%w: content witness changed by the full gc: %v", errDoltCompactionDegraded, err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsAfter); err != nil {
		return result, fmt.Errorf("dolt compaction: count commits after: %w", err)
	}
	return result, nil
}
