package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	embedded "github.com/dolthub/driver"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
)

const (
	doltGCMilestoneMarkerName = ".choir-dolt-gc-milestone-gib"
	defaultDoltGCMilestoneGiB = 1
	doltGCWarningUsedGiB      = 7
	doltGCEmergencyAvailBytes = 512 << 20 // 512 MiB free
	gibBytes                  = 1024 * 1024 * 1024
)

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
	if usage.AvailBytes <= doltGCEmergencyAvailBytes {
		plan.Run = true
		plan.TargetMilestone = currentMilestone
		plan.Reason = fmt.Sprintf("low free space (%d MiB avail)", usage.AvailBytes/(1024*1024))
		return plan
	}
	if currentMilestone > previousMilestoneGiB {
		plan.Run = true
		plan.TargetMilestone = currentMilestone
		plan.Reason = fmt.Sprintf("used crossed %d GiB milestone (now ~%d GiB)", currentMilestone*milestoneGiB, usedGiB)
		return plan
	}
	return plan
}

func runDoltGCWorkspace(workspacePath string) error {
	dbDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&database=vtext&multistatements=true&clientfoundrows=true",
		workspacePath,
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
		return nil
	}
	persistentDir = strings.TrimSpace(persistentDir)
	if persistentDir == "" {
		return nil
	}
	workspacePath := resolveVTextWorkspacePath(storePath)
	if _, err := os.Stat(workspacePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("dolt gc: stat workspace: %w", err)
	}

	usage, err := persistentDiskUsage(persistentDir)
	if err != nil {
		return err
	}

	milestoneGiB := doltGCMilestoneGiB()
	markerPath := filepath.Join(persistentDir, doltGCMilestoneMarkerName)
	previous, err := readDoltGCMilestoneMarker(markerPath)
	if err != nil {
		return err
	}

	plan := planDoltGC(usage, previous, milestoneGiB)
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
		return err
	}

	after, err := persistentDiskUsage(persistentDir)
	if err != nil {
		return err
	}
	afterMilestone := after.UsedBytes / (milestoneGiB * gibBytes)
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
	return nil
}

// StartPeriodicDoltGC runs MaybeRunDoltGC on a timer to catch disk growth
// between sandbox restarts. It is safe to call multiple times — the milestone
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
