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
	"syscall"
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

	// The floor (round 2, finding 3): the compaction's full GC writes a new
	// copy of the compacted store into the reflink copy (14.45 GB on
	// 2026-10-22 15:35, from 33.0 GB), and nothing it frees in the copy is
	// freed on the host until the swap. 24 GiB is that write plus about
	// 10 GiB for growth of the live set, ext4 and btrfs metadata, and the
	// rest of the host writing for the run's six minutes.
	defaultStoreCompactMinHostFree    = 24 << 30
	defaultStoreCompactPressureFree   = 64 << 30
	defaultStoreCompactPressureFill   = 0.6
	defaultStoreCompactPressureGap    = 6 * time.Hour
	defaultStoreCompactPressureImage  = 32 << 30
	minStoreCompactPressureGap        = time.Hour
	minStoreCompactPressureTicker     = 30 * time.Second
	defaultStoreCompactPressureTicker = 5 * time.Minute
	// A compaction that gave back less than this backs the next one off to
	// storeCompactBackoffFactor times the gap: compacting again soon would
	// restart the computer for nothing (round 2, finding 4).
	storeCompactSmallReclaim  = 8 << 30
	storeCompactBackoffFactor = 4
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
	// computer, whoever made them (a by-hand call may force past it).
	MinGap time.Duration
	// MinImageBytes: a computer whose image holds less is never due, as the
	// 04:00 timer's 32 GiB minimum: there is little to give back.
	MinImageBytes uint64
}

// StoreCompactPressureConfigFromEnv reads VMCTL_STORE_COMPACT_PRESSURE*.
// An unknown mode is off; invalid numbers keep their defaults; a gap under
// an hour is raised to an hour, an interval under 30 s to 30 s.
func StoreCompactPressureConfigFromEnv(getenv func(string) string) StoreCompactPressureConfig {
	cfg := StoreCompactPressureConfig{
		Mode:          StoreCompactPressureOff,
		Interval:      defaultStoreCompactPressureTicker,
		HostFreeBelow: defaultStoreCompactPressureFree,
		FillFraction:  defaultStoreCompactPressureFill,
		MinGap:        defaultStoreCompactPressureGap,
		MinImageBytes: defaultStoreCompactPressureImage,
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
	if gib, err := strconv.ParseUint(strings.TrimSpace(getenv("VMCTL_STORE_COMPACT_PRESSURE_MIN_IMAGE_GIB")), 10, 32); err == nil {
		cfg.MinImageBytes = gib << 30
	}
	if cfg.MinGap < minStoreCompactPressureGap {
		cfg.MinGap = minStoreCompactPressureGap
	}
	if cfg.Interval < minStoreCompactPressureTicker {
		cfg.Interval = minStoreCompactPressureTicker
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

// storeCompactAttempt is one computer's last compaction attempt: when, and
// the image's host allocation before and after (0: not known).
type storeCompactAttempt struct {
	VMID            string    `json:"-"`
	At              time.Time `json:"at"`
	FileBytesBefore uint64    `json:"file_bytes_before,omitempty"`
	FileBytesAfter  uint64    `json:"file_bytes_after,omitempty"`
}

// storeCompactAttempts is the last compaction attempt of each computer,
// persisted so a restarted vmctl keeps the gap.
type storeCompactAttempts struct {
	mu   sync.Mutex
	path string
	last map[string]storeCompactAttempt
	// loadErr: the record on disk could not be read. The pressure trigger
	// refuses while it is set; the next successful write keeps the bad
	// file aside as <path>.corrupt and clears it.
	loadErr error
	// writeFile writes the record (writeFileAtomic; a test hook).
	writeFile func(path string, raw []byte) error
}

func loadStoreCompactAttempts(path string) *storeCompactAttempts {
	a := &storeCompactAttempts{path: path, last: map[string]storeCompactAttempt{}}
	if path == "" {
		return a
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return a
	}
	var last map[string]storeCompactAttempt
	if err == nil {
		err = json.Unmarshal(raw, &last)
	}
	if err == nil && last == nil {
		// JSON null unmarshals into a nil map with no error.
		err = errors.New("not a JSON object")
	}
	if err != nil {
		a.loadErr = fmt.Errorf("store compaction attempts record %s unreadable: %w", path, err)
		return a
	}
	a.last = last
	return a
}

func writeFileAtomic(path string, raw []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// persistLocked writes the record; a.mu is held.
func (a *storeCompactAttempts) persistLocked() error {
	if a.last == nil {
		a.last = map[string]storeCompactAttempt{}
	}
	if a.path == "" {
		return nil
	}
	if a.loadErr != nil {
		// Keep the unreadable record as evidence before replacing it.
		_ = os.Rename(a.path, a.path+".corrupt")
	}
	raw, err := json.Marshal(a.last)
	if err != nil {
		return err
	}
	write := a.writeFile
	if write == nil {
		write = writeFileAtomic
	}
	if err := write(a.path, raw); err != nil {
		return fmt.Errorf("store compaction attempts record %s: %w", a.path, err)
	}
	a.loadErr = nil
	return nil
}

// record keeps the attempt in memory, whatever the write does, and writes it.
func (a *storeCompactAttempts) record(att storeCompactAttempt) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.last == nil {
		a.last = map[string]storeCompactAttempt{}
	}
	a.last[att.VMID] = att
	return a.persistLocked()
}

// checkWritable rewrites the record as it is, so a compaction learns before
// it asks the guest for a restart that it can record itself.
func (a *storeCompactAttempts) checkWritable() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.persistLocked()
}

func (a *storeCompactAttempts) lastAttempt(vmID string) (storeCompactAttempt, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.last[vmID]
	return t, ok, a.loadErr
}

// allocatedBytesOf is the host allocation of path (0 when unreadable).
func allocatedBytesOf(path string) uint64 {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0
	}
	return uint64(st.Blocks) * 512
}

func (s *storeCompactState) allocated(path string) uint64 {
	if s.allocatedBytes != nil {
		return s.allocatedBytes(path)
	}
	return allocatedBytesOf(path)
}

// gapRefusal says why vmID may not be compacted again yet, or "". The gap
// is the pressure config's MinGap, times storeCompactBackoffFactor when the
// last compaction gave back little.
func (h *Handler) storeCompactGapRefusal(vmID string, now time.Time) string {
	gap := h.storeCompactPressure.MinGap
	if gap <= 0 {
		return ""
	}
	last, seen, _ := h.storeCompaction().attemptsRecord().lastAttempt(vmID)
	if !seen {
		return ""
	}
	note := ""
	if last.FileBytesBefore > 0 && last.FileBytesAfter > 0 {
		var reclaimed uint64
		if last.FileBytesBefore > last.FileBytesAfter {
			reclaimed = last.FileBytesBefore - last.FileBytesAfter
		}
		if reclaimed < storeCompactSmallReclaim {
			gap *= storeCompactBackoffFactor
			note = fmt.Sprintf("; the last compaction reclaimed %s (under %s), so the gap is %s", fmtGiB(reclaimed), fmtGiB(storeCompactSmallReclaim), gap)
		}
	}
	if now.Sub(last.At) >= gap {
		return ""
	}
	return fmt.Sprintf("last compaction attempt %s, next after %s%s",
		last.At.UTC().Format(time.RFC3339), last.At.Add(gap).UTC().Format(time.RFC3339), note)
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
	if img == nil || img.FileBytes < cfg.MinImageBytes {
		return ""
	}
	if img.CapBytes > 0 && float64(img.FileBytes) >= cfg.FillFraction*float64(img.CapBytes) {
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
		_, _, loadErr := state.attemptsRecord().lastAttempt(own.VMID)
		gapRefusal := h.storeCompactGapRefusal(own.VMID, now)
		switch {
		case loadErr != nil:
			d.Action, d.Reason = pressureActionRefused, reason+"; "+loadErr.Error()
		case gapRefusal != "":
			d.Action, d.Reason = pressureActionRateLimited, reason+"; "+gapRefusal
		default:
			if refusal := state.hostFloorRefusal(); refusal != "" {
				d.Action, d.Reason = pressureActionRefused, reason+"; "+refusal
			} else if cfg.Mode == StoreCompactPressureObserve {
				d.Action = pressureActionObserve
			} else if !acted {
				d.Action = pressureActionCompact
				result, _ := h.compactStore(ctx, own, false, now)
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
			compactionErr := ""
			if d.Result.Compaction != nil {
				compactionErr = d.Result.Compaction.Error
			}
			log.Printf("vmctl: store compaction pressure (on): compacted %s (computer %s): reason=%q host_free=%s outcome=%s detail=%q compaction_error=%q", d.VMID, d.ComputerID, d.Reason, freeNote, d.Result.Outcome, d.Result.Reason, compactionErr)
		default:
			log.Printf("vmctl: store compaction pressure (%s): %s %s (computer %s): reason=%q host_free=%s", cfg.Mode, d.Action, d.VMID, d.ComputerID, d.Reason, freeNote)
		}
		decisions = append(decisions, d)
	}
	return decisions
}

// runStoreCompactPressureTick is one tick that cannot take vmctl down: a
// panic is logged and the loop goes on.
func (h *Handler) runStoreCompactPressureTick(ctx context.Context, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("vmctl: store compaction pressure: tick panicked: %v", r)
		}
	}()
	h.storeCompactPressureTick(ctx, now)
}

// StartStoreCompactPressure runs the loop until ctx ends; off does nothing.
func (h *Handler) StartStoreCompactPressure(ctx context.Context) {
	cfg := h.storeCompactPressure
	if cfg.Mode != StoreCompactPressureObserve && cfg.Mode != StoreCompactPressureOn {
		return
	}
	interval := cfg.Interval
	if interval < minStoreCompactPressureTicker {
		interval = minStoreCompactPressureTicker
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				h.runStoreCompactPressureTick(ctx, now)
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
