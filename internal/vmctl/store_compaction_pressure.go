package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Store compaction on host disk pressure
// (docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md,
// "Pressure trigger design"). The paper's data image never trims (Firecracker
// 1.16.1 has no virtio-blk discard), so its host allocation refills in hours,
// and the daily 04:00 sweep is too slow. This loop runs the same compaction
// (compactStore: a marker-backed planned restart, refused by the guest during
// a self-development apply or while a capsule is live) for a platform
// computer when host free space or the image's fill crosses a threshold, at
// most once per computer per gap. Off by default; observe only logs.

const (
	StoreCompactPressureOff     = "off"
	StoreCompactPressureObserve = "observe"
	StoreCompactPressureOn      = "on"

	pressureActionObserve     = "observe"
	pressureActionCompact     = "compact"
	pressureActionRateLimited = "rate-limited"
	pressureActionRefused     = "refused"

	defaultStoreCompactMinHostFree    = 32 << 30
	defaultStoreCompactPressureFree   = 64 << 30
	defaultStoreCompactPressureFill   = 0.6
	defaultStoreCompactPressureGap    = 6 * time.Hour
	minStoreCompactPressureGap        = time.Hour
	defaultStoreCompactPressureTicker = 5 * time.Minute
)

// StoreCompactPressureConfig is the pressure loop's host setting.
type StoreCompactPressureConfig struct {
	Mode string
	// Interval is how often the loop looks.
	Interval time.Duration
	// HostFreeBelow: a platform computer is due when host free space on the
	// VM state filesystem is below this.
	HostFreeBelow uint64
	// FillFraction: a platform computer is due when its data image holds at
	// least this fraction of its cap on the host.
	FillFraction float64
	// MinGap is the least time between two compaction attempts of one
	// computer, whoever made them.
	MinGap time.Duration
}

// StoreCompactPressureConfigFromEnv reads VMCTL_STORE_COMPACT_PRESSURE*.
// An unknown mode is off; invalid numbers keep their defaults; a gap under
// an hour is raised to an hour.
func StoreCompactPressureConfigFromEnv(getenv func(string) string) StoreCompactPressureConfig {
	cfg := StoreCompactPressureConfig{
		Mode:          StoreCompactPressureOff,
		Interval:      defaultStoreCompactPressureTicker,
		HostFreeBelow: defaultStoreCompactPressureFree,
		FillFraction:  defaultStoreCompactPressureFill,
		MinGap:        defaultStoreCompactPressureGap,
	}
	switch mode := strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE")); mode {
	case StoreCompactPressureObserve, StoreCompactPressureOn:
		cfg.Mode = mode
	case "", StoreCompactPressureOff:
	default:
		log.Printf("vmctl: VMCTL_STORE_COMPACT_PRESSURE=%q is not off, observe or on; the pressure trigger is off", mode)
	}
	if d, err := time.ParseDuration(strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE_INTERVAL"))); err == nil && d > 0 {
		cfg.Interval = d
	}
	if gib, err := strconv.ParseUint(strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE_HOST_FREE_GIB")), 10, 32); err == nil {
		cfg.HostFreeBelow = gib << 30
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE_FILL_FRACTION")), 64); err == nil && f > 0 && f <= 1 {
		cfg.FillFraction = f
	}
	if d, err := time.ParseDuration(strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE_MIN_GAP"))); err == nil {
		cfg.MinGap = d
	}
	if cfg.MinGap < minStoreCompactPressureGap {
		cfg.MinGap = minStoreCompactPressureGap
	}
	return cfg
}

// storeCompactMinHostFreeFromEnv reads VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB,
// the floor below which every compaction refuses; 0 turns it off.
func storeCompactMinHostFreeFromEnv(getenv func(string) string) uint64 {
	v := strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB"))
	if gib, err := strconv.ParseUint(v, 10, 32); err == nil {
		return gib << 30
	}
	if v != "" {
		log.Printf("vmctl: VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB=%q is not a number; the floor stays %s", v, fmtGiB(defaultStoreCompactMinHostFree))
	}
	return defaultStoreCompactMinHostFree
}

func fmtGiB(b uint64) string {
	return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30))
}

// storeCompactAttempts is the last compaction attempt of each computer,
// persisted so a restarted vmctl keeps the gap.
type storeCompactAttempts struct {
	mu      sync.Mutex
	path    string
	last    map[string]time.Time
	loadErr error
}

func loadStoreCompactAttempts(path string) *storeCompactAttempts {
	a := &storeCompactAttempts{path: path, last: map[string]time.Time{}}
	if path == "" {
		return a
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return a
	}
	if err == nil {
		err = json.Unmarshal(raw, &a.last)
	}
	if err != nil {
		a.loadErr = fmt.Errorf("store compaction attempts record %s unreadable: %w", path, err)
		a.last = map[string]time.Time{}
	}
	return a
}

// record writes the attempt before the compaction runs. A failed write is
// an error: the compaction then does not run.
func (a *storeCompactAttempts) record(vmID string, at time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.last[vmID] = at.UTC()
	if a.path == "" {
		return nil
	}
	raw, err := json.Marshal(a.last)
	if err != nil {
		return err
	}
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("store compaction attempt record: %w", err)
	}
	if err := os.Rename(tmp, a.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store compaction attempt record: %w", err)
	}
	return nil
}

func (a *storeCompactAttempts) lastAttempt(vmID string) (time.Time, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.last[vmID]
	return t, ok, a.loadErr
}

// hostFloorRefusal says why a compaction may not run now, or "" when it may:
// the compaction writes the squashed store into a reflink copy, and every
// block it writes is new host allocation until the swap.
func (s *storeCompactState) hostFloorRefusal() string {
	if s.minHostFree == 0 {
		return ""
	}
	free, err := s.hostFree()
	if err != nil {
		return fmt.Sprintf("host free space unreadable (%v); the compaction needs room for the compacted copy", err)
	}
	if free < s.minHostFree {
		return fmt.Sprintf("host free %s below the compaction floor %s; the compaction needs room for the compacted copy", fmtGiB(free), fmtGiB(s.minHostFree))
	}
	return ""
}

func (s *storeCompactState) hostFree() (uint64, error) {
	if s.freeBytes == nil {
		return 0, errors.New("no host free-space reader")
	}
	return s.freeBytes()
}

// SetStoreCompactPressure sets the pressure loop's config and loads the
// attempts record at attemptsPath ("" keeps attempts in memory). A load
// error is returned and also kept: every pressure compaction then refuses.
func (h *Handler) SetStoreCompactPressure(cfg StoreCompactPressureConfig, attemptsPath string) error {
	state := h.storeCompaction()
	attempts := loadStoreCompactAttempts(attemptsPath)
	state.attemptsMu.Lock()
	state.attempts = attempts
	state.attemptsMu.Unlock()
	h.storeCompactPressure = cfg
	return attempts.loadErr
}

// storeCompactPressureDecision is what one tick decided for one computer.
type storeCompactPressureDecision struct {
	VMID       string
	ComputerID string
	Action     string
	Reason     string
	Result     *storeCompactResponse
}

// pressureDue says why a computer's store should be compacted now, or "".
func pressureDue(cfg StoreCompactPressureConfig, free uint64, freeErr error, img *dataImageResponse) string {
	var reasons []string
	if img != nil && img.CapBytes > 0 && float64(img.FileBytes) >= cfg.FillFraction*float64(img.CapBytes) {
		reasons = append(reasons, fmt.Sprintf("data image %d%% of cap (%s of %s) >= %d%%",
			img.FileBytes*100/img.CapBytes, fmtGiB(img.FileBytes), fmtGiB(img.CapBytes), int(cfg.FillFraction*100+0.5)))
	}
	if freeErr == nil && cfg.HostFreeBelow > 0 && free < cfg.HostFreeBelow {
		reasons = append(reasons, fmt.Sprintf("host free %s below %s", fmtGiB(free), fmtGiB(cfg.HostFreeBelow)))
	}
	return strings.Join(reasons, "; ")
}

// storeCompactPressureTick looks once and acts on at most one computer.
func (h *Handler) storeCompactPressureTick(ctx context.Context, now time.Time) []storeCompactPressureDecision {
	cfg := h.storeCompactPressure
	if cfg.Mode != StoreCompactPressureObserve && cfg.Mode != StoreCompactPressureOn {
		return nil
	}
	ownerships, err := h.listOwnershipResponses(ctx)
	if err != nil {
		log.Printf("vmctl: store compaction pressure: ownership list: %v", err)
		return nil
	}
	h.fillDataImageStats(ownerships)
	_, _, picks := selectStoreCompaction(ownerships, 0)
	state := h.storeCompaction()
	free, freeErr := state.hostFree()
	freeNote := "unreadable"
	if freeErr == nil {
		freeNote = fmtGiB(free)
	}
	var decisions []storeCompactPressureDecision
	acted := false
	for _, own := range picks {
		reason := pressureDue(cfg, free, freeErr, own.DataImage)
		if reason == "" {
			continue
		}
		d := storeCompactPressureDecision{VMID: own.VMID, ComputerID: own.ComputerID, Reason: reason}
		last, seen, loadErr := state.attemptsRecord().lastAttempt(own.VMID)
		switch {
		case loadErr != nil:
			d.Action, d.Reason = pressureActionRefused, reason+"; "+loadErr.Error()
		case seen && now.Sub(last) < cfg.MinGap:
			d.Action = pressureActionRateLimited
			d.Reason = fmt.Sprintf("%s; last compaction attempt %s, next after %s", reason,
				last.UTC().Format(time.RFC3339), last.Add(cfg.MinGap).UTC().Format(time.RFC3339))
		default:
			if refusal := state.hostFloorRefusal(); refusal != "" {
				d.Action, d.Reason = pressureActionRefused, reason+"; "+refusal
			} else if cfg.Mode == StoreCompactPressureObserve {
				d.Action = pressureActionObserve
			} else if !acted {
				d.Action = pressureActionCompact
				result, _ := h.compactStore(ctx, own)
				d.Result = &result
				acted = true
			} else {
				// One compaction per tick; the next tick takes the next.
				continue
			}
		}
		switch d.Action {
		case pressureActionObserve:
			log.Printf("vmctl: store compaction pressure (observe): would compact %s (computer %s): reason=%q host_free=%s", d.VMID, d.ComputerID, d.Reason, freeNote)
		case pressureActionCompact:
			log.Printf("vmctl: store compaction pressure (on): compacted %s (computer %s): reason=%q host_free=%s outcome=%s detail=%q", d.VMID, d.ComputerID, d.Reason, freeNote, d.Result.Outcome, d.Result.Reason)
		default:
			log.Printf("vmctl: store compaction pressure (%s): %s %s (computer %s): reason=%q host_free=%s", cfg.Mode, d.Action, d.VMID, d.ComputerID, d.Reason, freeNote)
		}
		decisions = append(decisions, d)
	}
	return decisions
}

// StartStoreCompactPressure runs the loop until ctx ends; off does nothing.
func (h *Handler) StartStoreCompactPressure(ctx context.Context) {
	cfg := h.storeCompactPressure
	if cfg.Mode != StoreCompactPressureObserve && cfg.Mode != StoreCompactPressureOn {
		return
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultStoreCompactPressureTicker
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				h.storeCompactPressureTick(ctx, now)
			}
		}
	}()
}

// StoreCompactAttemptsPath is the attempts record beside the ownership state.
func StoreCompactAttemptsPath(stateDir string) string {
	if strings.TrimSpace(stateDir) == "" {
		return ""
	}
	return filepath.Join(stateDir, "store-compact-attempts.json")
}
