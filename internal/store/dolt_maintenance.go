package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	embedded "github.com/dolthub/driver/v2"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
)

const (
	doltGCEmergencyAvailBytes = 512 << 20 // 512 MiB free
	// defaultDoltGCJournalGiB bounds the noms journal before routine GC runs.
	// The journal is the collectible garbage; letting it grow unbounded is how
	// a 18.6 GiB journal accumulated under ~2 GiB of live data (2026-09-11)
	// and a 27 GB journal filled the paper's 32 GB disk (2026-10-15).
	defaultDoltGCJournalGiB = 1
	// doltGCMinJournalBytes floors the headroom-scaled threshold, so a nearly
	// full disk does not collect a near-empty journal on every tick.
	doltGCMinJournalBytes = 64 << 20
	// doltGCHeadroomDivisor scales the journal threshold to free space: the
	// journal never grows past 1/divisor of what is free. Default GC copies
	// the reachable part of the new generation (at most the journal) into a
	// new table file before it deletes the journal, so it needs free space up
	// to the journal's size. A quarter leaves 3x slack, and the threshold
	// tightens as the disk fills, so GC runs while it still fits; an
	// emergency GC at 0 MiB free cannot write its output (2026-10-15).
	doltGCHeadroomDivisor = 4
	// doltGCCompactionNeededPercent: when free space is still below this share
	// of the disk after a GC, the growth is Dolt history or live data, which a
	// default GC never reclaims (only squash + full GC, offline:
	// docs/runbooks/offline-guest-gc.md). The disposition says so.
	doltGCCompactionNeededPercent = 25
	// doltGCBootMaxJournalBytes caps the journal a boot GC takes on. Boot GC
	// runs before the HTTP listener, so a long one reads as refused
	// connections and the wedge detector stops the computer after 5 minutes
	// (10-14 23:22 -> 23:29:31). A 13 GiB journal took 148 s at boot
	// (10-14 10:22); 4 GiB stays well inside the window. A larger journal is
	// left to the online GC, whose first tick runs as soon as the computer
	// serves.
	doltGCBootMaxJournalBytes = 4 * gibBytes
	// doltJournalFileID is the fixed noms journal table-file name
	// (dolt store/chunks JournalFileID).
	doltJournalFileID = "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"
	gibBytes          = 1024 * 1024 * 1024
	mibBytes          = 1024 * 1024
)
const doltGCDispositionFileName = ".choir-dolt-gc-disposition.json"

// doltGCDisposition is the machine-readable record of the last GC decision,
// written on every boot and periodic decision (including no-ops) and served
// on guest /health as dolt_gc. A deferred or failing GC must be observable to
// operators and host sweepers, not just a log line: a deferral at scale is how
// a 9.8 GB journal grew for ~0.5 GB of live data (2026-09-03). Never fails the
// caller.
type doltGCDisposition struct {
	At              string `json:"at"`
	Mode            string `json:"mode,omitempty"` // boot | online
	Outcome         string `json:"outcome"`        // ran | noop | deferred_online | error | skipped_disabled
	UsedGiB         uint64 `json:"used_gib"`
	AvailMiB        uint64 `json:"avail_mib"`
	LiveGiB         uint64 `json:"live_gib"`
	JournalMiB      uint64 `json:"journal_mib"`
	ThresholdMiB    uint64 `json:"threshold_mib,omitempty"`
	AfterAvailMiB   uint64 `json:"after_avail_mib,omitempty"`
	AfterJournalMiB uint64 `json:"after_journal_mib,omitempty"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
	// CompactionNeeded: free space stayed low after the GC. The growth is
	// Dolt history or live data, which a default GC cannot reclaim.
	CompactionNeeded bool   `json:"compaction_needed,omitempty"`
	Detail           string `json:"detail,omitempty"`
}

func writeDoltGCDisposition(persistentDir string, disposition doltGCDisposition) {
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return
	}
	if strings.TrimSpace(disposition.At) == "" {
		disposition.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(disposition)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(persistentDir, doltGCDispositionFileName), append(raw, '\n'), 0o644)
}

// ReadDoltGCDisposition returns the last GC decision record under
// persistentDir, or nil when none was written. Served on guest /health.
func ReadDoltGCDisposition(persistentDir string) json.RawMessage {
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(persistentDir, doltGCDispositionFileName))
	if err != nil {
		return nil
	}
	raw = []byte(strings.TrimSpace(string(raw)))
	if !json.Valid(raw) {
		return nil
	}
	return json.RawMessage(raw)
}

type doltGCDiskUsage = persistentdisk.Usage

// doltGCInputs is what one GC decision reads.
type doltGCInputs struct {
	Usage        doltGCDiskUsage
	JournalBytes uint64
	// LiveBytes is the non-journal noms bytes (old generation plus new-gen
	// table files): history plus live data. Reported, never a gate.
	LiveBytes uint64
	// JournalLimitBytes is the configured routine journal threshold.
	JournalLimitBytes uint64
}

// doltGCPlan decides whether DOLT_GC() runs now.
type doltGCPlan struct {
	Run            bool
	Emergency      bool
	ThresholdBytes uint64
	Reason         string
}

// doltGCJournalGiB returns the configured journal size that triggers GC.
func doltGCJournalGiB() uint64 {
	raw := strings.TrimSpace(os.Getenv("RUNTIME_DOLT_GC_JOURNAL_GIB"))
	if raw == "" {
		return defaultDoltGCJournalGiB
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return defaultDoltGCJournalGiB
	}
	return v
}

// doltGCJournalThreshold is the journal size at which GC runs: the
// configured limit, lowered to a quarter of free space, floored at 64 MiB.
func doltGCJournalThreshold(limitBytes, availBytes uint64) uint64 {
	threshold := limitBytes
	if byHeadroom := availBytes / doltGCHeadroomDivisor; byHeadroom < threshold {
		threshold = byHeadroom
	}
	if threshold < doltGCMinJournalBytes {
		threshold = doltGCMinJournalBytes
	}
	return threshold
}

// planDoltGC is the guest GC policy. A default (generational) DOLT_GC walks
// only chunks not yet in the old generation (dolt store/types/value_store.go,
// OldGenGCFilter), so its cost follows the journal, not the store: no
// store-size gate applies. The 5 GiB live-store guard that stood here measured
// the old generation, which every GC grows and no default GC collects, so
// once history passed 5 GiB it switched GC off for good (2026-10-03, and again
// from 2026-10-14 13:43 until the paper's disk filled at 23:22).
func planDoltGC(in doltGCInputs) doltGCPlan {
	limit := in.JournalLimitBytes
	if limit == 0 {
		limit = defaultDoltGCJournalGiB * gibBytes
	}
	plan := doltGCPlan{ThresholdBytes: doltGCJournalThreshold(limit, in.Usage.AvailBytes)}
	if in.Usage.AvailBytes <= doltGCEmergencyAvailBytes {
		plan.Run = true
		plan.Emergency = true
		plan.Reason = fmt.Sprintf("emergency: avail %d MiB <= %d MiB", in.Usage.AvailBytes/mibBytes, doltGCEmergencyAvailBytes/mibBytes)
		return plan
	}
	if in.JournalBytes >= plan.ThresholdBytes {
		plan.Run = true
		plan.Reason = fmt.Sprintf("journal %d MiB >= threshold %d MiB (avail %d MiB)",
			in.JournalBytes/mibBytes, plan.ThresholdBytes/mibBytes, in.Usage.AvailBytes/mibBytes)
	}
	return plan
}

// doltJournalBytes returns the total size of noms journal files under the
// workspace. The journal holds every not-yet-collected chunk; it is the
// collectible garbage.
func doltJournalBytes(workspacePath string) uint64 {
	var total uint64
	matches, err := filepath.Glob(filepath.Join(workspacePath, "*", ".dolt", "noms", doltJournalFileID))
	if err != nil {
		return 0
	}
	for _, path := range matches {
		if info, statErr := os.Stat(path); statErr == nil {
			total += uint64(info.Size())
		}
	}
	return total
}

// doltLiveStoreBytes returns the Dolt store's non-journal bytes (old
// generation and new-gen table files). It counts only the workspace's noms
// directories: other files on the persistent disk (updater releases, app
// files) are not the store (2026-10-09).
func doltLiveStoreBytes(workspacePath string) uint64 {
	var total uint64
	roots, err := filepath.Glob(filepath.Join(workspacePath, "*", ".dolt", "noms"))
	if err != nil {
		return 0
	}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || !d.Type().IsRegular() || d.Name() == doltJournalFileID {
				return nil
			}
			if info, infoErr := d.Info(); infoErr == nil {
				total += uint64(info.Size())
			}
			return nil
		})
	}
	return total
}

func doltGCDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RUNTIME_DOLT_GC_DISABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func persistentDiskUsage(persistentDir string) (doltGCDiskUsage, error) {
	return persistentdisk.Statfs(persistentDir)
}

// diskUsageForGC is the statfs source for GC decisions, indirected so tests
// can stage small/large filesystems deterministically.
var diskUsageForGC = persistentDiskUsage

// RunDoltGCWorkspace runs embedded Dolt garbage collection (CALL DOLT_GC)
// on the given texture workspace path. It opens no Store handle; callers must
// have closed every handle on the workspace first. Unconditional: it does not
// consult the journal size or RUNTIME_DOLT_GC_DISABLED.
func RunDoltGCWorkspace(workspacePath string) error {
	return runDoltGCWorkspace(workspacePath)
}

func runDoltGCWorkspace(workspacePath string) error {
	rootDB, rootConnector, err := openDoltRootDB(workspacePath)
	if err != nil {
		return fmt.Errorf("dolt gc: open root db: %w", err)
	}
	databaseName, err := resolveTextureWorkspaceDatabaseName(rootDB, false)
	if closeErr := rootDB.Close(); closeErr != nil {
		_ = rootConnector.Close()
		return fmt.Errorf("dolt gc: close root db: %w", closeErr)
	}
	if closeErr := rootConnector.Close(); closeErr != nil {
		return fmt.Errorf("dolt gc: close root connector: %w", closeErr)
	}
	if err != nil {
		return fmt.Errorf("dolt gc: resolve database: %w", err)
	}
	if databaseName == "" {
		return nil
	}
	dbDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		workspacePath,
		databaseName,
	)
	cfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		return fmt.Errorf("dolt gc: parse dsn: %w", err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return fmt.Errorf("dolt gc: new connector: %w", err)
	}
	db := sql.OpenDB(connector)
	configureEmbeddedDoltDB(db)
	if _, err := db.Exec("CALL DOLT_GC()"); err != nil {
		_ = db.Close()
		_ = connector.Close()
		return fmt.Errorf("dolt gc: call dolt_gc: %w", err)
	}
	if err := db.Close(); err != nil {
		_ = connector.Close()
		return fmt.Errorf("dolt gc: close db: %w", err)
	}
	if err := connector.Close(); err != nil {
		return fmt.Errorf("dolt gc: close connector: %w", err)
	}
	return nil
}

// MaybeRunDoltGC is the boot-time GC: it runs before store.Open, with no
// handle on the workspace, and collects the journal when planDoltGC says so.
func MaybeRunDoltGC(persistentDir, storePath string) error {
	workspacePath := resolveTextureWorkspacePath(storePath)
	return maybeRunDoltGC(persistentDir, workspacePath, "boot", doltGCBootMaxJournalBytes, func() error {
		return runDoltGCWorkspace(workspacePath)
	})
}

// MaybeRunOnlineDoltGC is the periodic GC on a running computer. It runs
// CALL DOLT_GC() on the open store's own write connection, never on a second
// engine over a workspace this process already holds. Dolt's session-aware
// safepoint controller (dprocedures/dolt_gc.go; the default, installed by
// every SqlEngine) keeps the store's read session valid across a collection.
func (s *Store) MaybeRunOnlineDoltGC(ctx context.Context, persistentDir string) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("dolt gc: nil store")
	}
	return maybeRunDoltGC(persistentDir, s.texturePath, "online", 0, func() error {
		return s.runOnlineDoltGC(ctx)
	})
}

func (s *Store) runOnlineDoltGC(ctx context.Context) error {
	// Serialize with Dolt checkpoint commits; the single write connection
	// already serializes every other write.
	s.doltCommitMu.Lock()
	defer s.doltCommitMu.Unlock()
	// The pass is named while it runs (docs/problems/paper-remaining-store-
	// stall-holders-2026-10-23.md, section 1, fix shape 1): the stall header
	// and the observability store block read the flag, so a store_stalled
	// snapshot taken during or just after a pass says so. The deferred clear
	// runs on every exit, an error and a panic included.
	s.onlineGCSince.Store(time.Now().UnixNano())
	defer s.onlineGCSince.Store(0)
	if _, err := s.execOnlineDoltGC(ctx); err != nil {
		return fmt.Errorf("dolt gc: call dolt_gc (online): %w", err)
	}
	return nil
}

// execOnlineDoltGC issues the pass on the write connection. A test replaces it
// through onlineGCExecHook to hold a pass open, fail it, or panic in it.
func (s *Store) execOnlineDoltGC(ctx context.Context) (sql.Result, error) {
	if hook := s.onlineGCExecHook; hook != nil {
		return nil, hook(ctx)
	}
	return s.textureHandle().ExecContext(ctx, "CALL DOLT_GC()")
}

// OnlineGCStatus says whether an online Dolt GC pass is running in this
// process and since when. It takes no lock and never waits, so it answers
// during the pass it reports.
type OnlineGCStatus struct {
	Active bool   `json:"active"`
	Since  string `json:"since,omitempty"` // RFC 3339, UTC, set while Active
}

// OnlineGCStatus reads the flag runOnlineDoltGC sets.
func (s *Store) OnlineGCStatus() OnlineGCStatus {
	if s == nil {
		return OnlineGCStatus{}
	}
	ns := s.onlineGCSince.Load()
	if ns == 0 {
		return OnlineGCStatus{}
	}
	return OnlineGCStatus{Active: true, Since: time.Unix(0, ns).UTC().Format(time.RFC3339)}
}

// maybeRunDoltGC measures, plans and (when the plan says so) runs one GC.
// maxJournalBytes > 0 defers a larger journal to the online GC.
func maybeRunDoltGC(persistentDir, workspacePath, mode string, maxJournalBytes uint64, run func() error) error {
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	if doltGCDisabled() {
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Mode: mode, Outcome: "skipped_disabled", Detail: "RUNTIME_DOLT_GC_DISABLED"})
		return nil
	}
	if _, err := os.Stat(workspacePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("dolt gc: stat workspace: %w", err)
	}

	usage, err := diskUsageForGC(persistentDir)
	if err != nil {
		return err
	}
	in := doltGCInputs{
		Usage:             usage,
		JournalBytes:      doltJournalBytes(workspacePath),
		LiveBytes:         doltLiveStoreBytes(workspacePath),
		JournalLimitBytes: doltGCJournalGiB() * gibBytes,
	}
	plan := planDoltGC(in)
	disposition := doltGCDisposition{
		Mode:         mode,
		UsedGiB:      usage.UsedBytes / gibBytes,
		AvailMiB:     usage.AvailBytes / mibBytes,
		LiveGiB:      in.LiveBytes / gibBytes,
		JournalMiB:   in.JournalBytes / mibBytes,
		ThresholdMiB: plan.ThresholdBytes / mibBytes,
		Detail:       plan.Reason,
	}
	if !plan.Run {
		disposition.Outcome = "noop"
		writeDoltGCDisposition(persistentDir, disposition)
		return nil
	}
	if maxJournalBytes > 0 && in.JournalBytes > maxJournalBytes {
		disposition.Outcome = "deferred_online"
		disposition.Detail = fmt.Sprintf("%s; journal over the %d MiB %s cap, left to the online gc", plan.Reason, maxJournalBytes/mibBytes, mode)
		writeDoltGCDisposition(persistentDir, disposition)
		log.Printf("store: dolt gc (%s) deferred to online gc: %s", mode, disposition.Detail)
		return nil
	}

	log.Printf("store: dolt gc (%s) starting: %s; used=%d GiB live=%d GiB", mode, plan.Reason, usage.UsedBytes/gibBytes, in.LiveBytes/gibBytes)
	started := time.Now()
	runErr := run()
	disposition.DurationMS = time.Since(started).Milliseconds()
	if runErr != nil {
		disposition.Outcome = "error"
		disposition.Detail = plan.Reason + ": " + runErr.Error()
		writeDoltGCDisposition(persistentDir, disposition)
		return runErr
	}
	disposition.Outcome = "ran"
	disposition.AfterJournalMiB = doltJournalBytes(workspacePath) / mibBytes
	if after, err := diskUsageForGC(persistentDir); err == nil {
		disposition.AfterAvailMiB = after.AvailBytes / mibBytes
		if after.TotalBytes > 0 && after.AvailBytes*100 < after.TotalBytes*doltGCCompactionNeededPercent {
			disposition.CompactionNeeded = true
		}
	}
	writeDoltGCDisposition(persistentDir, disposition)
	log.Printf("store: dolt gc (%s) complete in %s: journal %d MiB -> %d MiB, avail %d MiB -> %d MiB",
		mode, time.Since(started).Round(time.Second), disposition.JournalMiB, disposition.AfterJournalMiB, disposition.AvailMiB, disposition.AfterAvailMiB)
	if disposition.CompactionNeeded {
		log.Printf("store: dolt gc (%s): avail %d MiB is under %d%% of the disk after gc; history or live data fills it, not the journal: compaction needed (docs/runbooks/offline-guest-gc.md)",
			mode, disposition.AfterAvailMiB, doltGCCompactionNeededPercent)
	}
	return nil
}

// StartPeriodicDoltGC runs the online GC once at start (collecting what a boot
// deferred) and then on a timer while the computer runs, so the journal is
// collected between boots. It returns immediately; the loop
// runs until ctx is cancelled, so ctx must live as long as the store. (Until
// 2026-10-15 the only caller cancelled ctx as soon as this returned, so the
// loop exited before its first tick and GC ran only at boot.)
func StartPeriodicDoltGC(ctx context.Context, s *Store, persistentDir string, interval time.Duration) {
	if doltGCDisabled() || s == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		if err := s.MaybeRunOnlineDoltGC(ctx, persistentDir); err != nil {
			log.Printf("store: periodic dolt gc: %v", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.MaybeRunOnlineDoltGC(ctx, persistentDir); err != nil {
					log.Printf("store: periodic dolt gc: %v", err)
				}
			}
		}
	}()
}
