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
	doltGCMilestoneMarkerName = ".choir-dolt-gc-milestone-gib"
	defaultDoltGCMilestoneGiB = 1
	doltGCWarningUsedGiB      = 7
	doltGCEmergencyAvailBytes = 512 << 20 // 512 MiB free
	// defaultDoltGCJournalGiB bounds the noms journal before routine GC runs.
	// The journal is the collectible garbage; letting it grow unbounded is how
	// a 18.6 GiB journal accumulated under ~2 GiB of live data (2026-09-11).
	defaultDoltGCJournalGiB = 1
	// doltJournalFileID is the fixed noms journal table-file name
	// (dolt store/chunks JournalFileID).
	doltJournalFileID = "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"
	gibBytes          = 1024 * 1024 * 1024
)
const doltGCDispositionFileName = ".choir-dolt-gc-disposition.json"

// doltGCDisposition is the machine-readable record of the last GC decision,
// written beside the milestone marker on every MaybeRunDoltGC outcome
// (including skips). A skipped GC must be observable to operators and host
// sweepers, not just a log line: a deferral at scale is how a 9.8 GB journal
// grew for ~0.5 GB of live data (2026-09-03). Never fails the caller.
type doltGCDisposition struct {
	At           string `json:"at"`
	Outcome      string `json:"outcome"`
	UsedGiB      uint64 `json:"used_gib"`
	JournalGiB   uint64 `json:"journal_gib,omitempty"`
	ThresholdGiB uint64 `json:"threshold_gib,omitempty"`
	Detail       string `json:"detail,omitempty"`
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

type doltGCDiskUsage = persistentdisk.Usage

// doltGCPlan decides whether startup maintenance should run DOLT_GC().
type doltGCPlan struct {
	Run               bool
	Warning           bool
	TargetMilestone   uint64
	PreviousMilestone uint64
	Reason            string
}

func doltGCMilestoneGiB() uint64 {
	raw := strings.TrimSpace(os.Getenv("RUNTIME_DOLT_GC_MILESTONE_GIB"))
	if raw == "" {
		return defaultDoltGCMilestoneGiB
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return defaultDoltGCMilestoneGiB
	}
	return v
}

// doltGCJournalGiB returns the journal size that triggers routine GC.
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

// doltJournalBytes returns the total size of noms journal files under the
// workspace. The journal holds every not-yet-collected chunk; it is the
// collectible garbage, so it must not count toward the live-store size guard
// that protects bounded guests from OOM during GC.
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

func doltGCDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RUNTIME_DOLT_GC_DISABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func readDoltGCMilestoneMarker(markerPath string) (uint64, error) {
	data, err := os.ReadFile(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse dolt gc milestone marker: %w", err)
	}
	return v, nil
}

func writeDoltGCMilestoneMarker(markerPath string, milestoneGiB uint64) error {
	return os.WriteFile(markerPath, []byte(strconv.FormatUint(milestoneGiB, 10)+"\n"), 0o644)
}

func persistentDiskUsage(persistentDir string) (doltGCDiskUsage, error) {
	return persistentdisk.Statfs(persistentDir)
}

// diskUsageForGC is the statfs source for MaybeRunDoltGC, indirected so tests
// can stage small/large filesystems deterministically.
var diskUsageForGC = persistentDiskUsage

func planDoltGC(usage doltGCDiskUsage, previousMilestoneGiB, milestoneGiB uint64) doltGCPlan {
	if milestoneGiB == 0 {
		milestoneGiB = defaultDoltGCMilestoneGiB
	}
	currentMilestone := usage.UsedBytes / (milestoneGiB * gibBytes)
	usedGiB := usage.UsedBytes / gibBytes

	plan := doltGCPlan{
		PreviousMilestone: previousMilestoneGiB,
		TargetMilestone:   previousMilestoneGiB,
	}

	if persistentdisk.Warning(usage) {
		plan.Warning = true
	}
	// Emergency low-space GC is handled in MaybeRunDoltGC before this planner
	// runs; it must bypass the live-size guard, so it cannot live here.
	if currentMilestone > previousMilestoneGiB {
		plan.Run = true
		plan.TargetMilestone = currentMilestone
		plan.Reason = fmt.Sprintf("used crossed %d GiB milestone (now ~%d GiB)", currentMilestone*milestoneGiB, usedGiB)
		return plan
	}
	return plan
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

// MaybeRunDoltGC runs embedded Dolt garbage collection when persistent disk
// usage crosses configured GiB milestones. It is intended to run before
// store.Open so GC can drop unreachable chunk history without an active store.
func MaybeRunDoltGC(persistentDir, storePath string) error {
	if doltGCDisabled() {
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "skipped_disabled", Detail: "RUNTIME_DOLT_GC_DISABLED"})
		return nil
	}
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	workspacePath := resolveTextureWorkspacePath(storePath)
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

	journalBytes := doltJournalBytes(workspacePath)
	liveBytes := usage.UsedBytes
	if journalBytes < liveBytes {
		liveBytes -= journalBytes
	} else {
		liveBytes = 0
	}

	// Emergency first: below the low-space watermark GC must run regardless of
	// store size. ENOSPC is unrecoverable; an OOM during emergency GC is
	// diagnosable and retryable. The size guard below must never suppress this.
	if usage.AvailBytes <= doltGCEmergencyAvailBytes {
		log.Printf("store: dolt gc emergency: avail=%d MiB; running despite store size", usage.AvailBytes/(1024*1024))
		if err := runDoltGCWorkspace(workspacePath); err != nil {
			writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "error", UsedGiB: usage.UsedBytes / gibBytes, JournalGiB: journalBytes / gibBytes, Detail: err.Error()})
			return err
		}
		after, afterErr := diskUsageForGC(persistentDir)
		if afterErr == nil {
			afterLive := after.UsedBytes - min(after.UsedBytes, doltJournalBytes(workspacePath))
			milestoneGiB := doltGCMilestoneGiB()
			_ = writeDoltGCMilestoneMarker(filepath.Join(persistentDir, doltGCMilestoneMarkerName), afterLive/(milestoneGiB*gibBytes))
		}
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "ran", UsedGiB: usage.UsedBytes / gibBytes, JournalGiB: journalBytes / gibBytes, Detail: "emergency low-space gc"})
		return nil
	}

	// The embedded Dolt GC (DOLT_GC()) builds its working set in memory; its
	// demand scales with the live chunk set, not the journal. On a bounded
	// guest a multi-GiB live store gets the process OOM-killed before GC
	// finishes (evidence: recovery boot #2 crash loop, 2026-08-24). The guard
	// measures live bytes (used minus journal): the journal is the garbage GC
	// reclaims, so counting it would suppress exactly the runs that shrink it
	// — the 2026-09-11 feedback loop where an 18.6 GiB journal over ~2 GiB of
	// live data skipped every GC until the disk nearly filled.
	const safeGuestGCUsedGiB = 5
	if liveBytes > safeGuestGCUsedGiB*gibBytes {
		log.Printf("store: dolt gc skipped: live=%d GiB exceeds safe bounded-guest GC size (%d GiB); reclaim deferred",
			liveBytes/gibBytes, safeGuestGCUsedGiB)
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "skipped_size", UsedGiB: usage.UsedBytes / gibBytes, JournalGiB: journalBytes / gibBytes, ThresholdGiB: safeGuestGCUsedGiB, Detail: "host-side offline GC required; see docs/runbooks/offline-guest-gc.md"})
		return nil
	}

	milestoneGiB := doltGCMilestoneGiB()
	markerPath := filepath.Join(persistentDir, doltGCMilestoneMarkerName)
	previous, err := readDoltGCMilestoneMarker(markerPath)
	if err != nil {
		return err
	}

	// Milestones track live bytes so journal growth alone never advances the
	// marker and suppresses the next live-data trigger.
	liveUsage := usage
	liveUsage.UsedBytes = liveBytes
	plan := planDoltGC(liveUsage, previous, milestoneGiB)
	journalGiB := doltGCJournalGiB()
	if !plan.Run && journalBytes >= journalGiB*gibBytes {
		plan.Run = true
		plan.Reason = fmt.Sprintf("journal reached %d GiB (now ~%d GiB)", journalGiB, journalBytes/gibBytes)
	}
	if plan.Warning && !plan.Run {
		log.Printf(
			"store: persistent disk high-water notice: used=%d GiB total=%d GiB avail=%d MiB (default cap 8 GiB); next dolt gc at next %d GiB milestone or low-space emergency",
			usage.UsedBytes/gibBytes,
			usage.TotalBytes/gibBytes,
			usage.AvailBytes/(1024*1024),
			milestoneGiB,
		)
	}
	if !plan.Run {
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "noop", UsedGiB: usage.UsedBytes / gibBytes, JournalGiB: journalBytes / gibBytes, Detail: plan.Reason})
		return nil
	}

	if plan.Warning {
		log.Printf(
			"store: persistent disk high-water warning: used=%d GiB total=%d GiB avail=%d MiB (8 GiB default cap); running dolt gc (%s)",
			usage.UsedBytes/gibBytes,
			usage.TotalBytes/gibBytes,
			usage.AvailBytes/(1024*1024),
			plan.Reason,
		)
	} else {
		log.Printf(
			"store: persistent disk maintenance: used=%d GiB avail=%d MiB; running dolt gc (%s)",
			usage.UsedBytes/gibBytes,
			usage.AvailBytes/(1024*1024),
			plan.Reason,
		)
	}
	if err := runDoltGCWorkspace(workspacePath); err != nil {
		writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "error", UsedGiB: usage.UsedBytes / gibBytes, JournalGiB: journalBytes / gibBytes, Detail: err.Error()})
		return err
	}

	after, err := diskUsageForGC(persistentDir)
	if err != nil {
		return err
	}
	afterJournal := doltJournalBytes(workspacePath)
	afterLive := after.UsedBytes
	if afterJournal < afterLive {
		afterLive -= afterJournal
	} else {
		afterLive = 0
	}
	afterMilestone := afterLive / (milestoneGiB * gibBytes)
	if afterMilestone < plan.TargetMilestone {
		plan.TargetMilestone = afterMilestone
	}
	if err := writeDoltGCMilestoneMarker(markerPath, plan.TargetMilestone); err != nil {
		return err
	}

	log.Printf(
		"store: dolt gc complete: used %d GiB -> %d GiB (avail %d MiB); milestone=%d",
		usage.UsedBytes/gibBytes,
		after.UsedBytes/gibBytes,
		after.AvailBytes/(1024*1024),
		plan.TargetMilestone,
	)
	writeDoltGCDisposition(persistentDir, doltGCDisposition{Outcome: "ran", UsedGiB: after.UsedBytes / gibBytes, JournalGiB: afterJournal / gibBytes, Detail: plan.Reason})
	return nil
}

// StartPeriodicDoltGC runs MaybeRunDoltGC on a timer to catch disk growth
// between autoputer restarts. It is safe to call multiple times — the milestone
// marker prevents redundant GC runs at the same level. The function returns
// immediately and runs in a background goroutine until the context is cancelled.
func StartPeriodicDoltGC(ctx context.Context, persistentDir, storePath string, interval time.Duration) {
	if doltGCDisabled() {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := MaybeRunDoltGC(persistentDir, storePath); err != nil {
					log.Printf("store: periodic dolt gc: %v", err)
				}
			}
		}
	}()
}
