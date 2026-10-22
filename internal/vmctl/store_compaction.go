package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Guest store compaction on a schedule, as a planned restart
// (docs/problems/guest-store-compaction-scheduled-2026-10-20.md; owner notes
// §26). The compaction rides the deploy-refresh path: the guest writes its
// planned-restart marker (or refuses while a self-development apply or a
// capsule is live), vmctl kills the VM, compacts a reflink copy of its data
// image offline, swaps the copy in by rename, and boots. Until the rename
// the original image is untouched, so a vmctl crash at any point leaves a
// bootable computer and a stale copy the next run removes.

const (
	storeCompactCopySuffix  = ".compacting"
	storeCompactTarget      = "store-compaction"
	defaultStoreCompactMin  = 32 << 30
	defaultCompactMemoryMax = "10G"
)

// commandRunner runs one host command and returns its combined output.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// storeCompactionReport is what one compaction did.
type storeCompactionReport struct {
	Squashed      bool   `json:"squashed"`
	CommitsBefore int    `json:"commits_before"`
	CommitsAfter  int    `json:"commits_after"`
	BytesBefore   int64  `json:"bytes_before"`
	BytesAfter    int64  `json:"bytes_after"`
	RootBefore    string `json:"root_before"`
	RootAfter     string `json:"root_after"`
	Duration      string `json:"duration,omitempty"`
	Error         string `json:"error,omitempty"`
}

// hostStoreCompactor compacts a stopped computer's data image on the host.
type hostStoreCompactor struct {
	run        commandRunner
	compactBin string
	memoryMax  string
	mountRoot  string
	// bin resolves a host tool by name (absolute paths on Node B).
	bin func(name string) string
}

func newHostStoreCompactor() *hostStoreCompactor {
	compactBin := strings.TrimSpace(os.Getenv("VMCTL_PROJECTION_COMPACT_BIN"))
	if compactBin == "" {
		compactBin = "/run/current-system/sw/bin/projection-compact"
	}
	memoryMax := strings.TrimSpace(os.Getenv("VMCTL_STORE_COMPACT_MEMORY_MAX"))
	if memoryMax == "" {
		memoryMax = defaultCompactMemoryMax
	}
	return &hostStoreCompactor{
		run:        execRunner{},
		compactBin: compactBin,
		memoryMax:  memoryMax,
		mountRoot:  "/var/lib/go-choir/store-compact",
		bin: func(name string) string {
			if p := "/run/current-system/sw/bin/" + name; fileExists(p) {
				return p
			}
			return name
		},
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (c *hostStoreCompactor) unitName(vmID string) string {
	return "vmctl-store-compact-" + strings.NewReplacer("/", "-", ".", "-").Replace(vmID)
}

// The mount, the compaction, the trim and the read-only fsck run in one
// transient unit: vmctl runs with MountFlags=private, so a mount made in its
// own namespace would be invisible to a unit PID 1 starts. The unit carries
// the memory cap for all of it, and its EXIT trap unmounts even when the tool
// fails. Arguments: mount dir, copy path, tool, workspace, computer, head.
const storeCompactScript = `set -eu
export PATH=/run/current-system/sw/bin:$PATH
mkdir -p "$1"
mount -o loop "$2" "$1"
trap 'umount "$1" 2>/dev/null || true' EXIT
"$3" -workspace "$4" -computer-id "$5" -canonical-head "$6"
fstrim "$1" >&2 || true
umount "$1"
trap - EXIT
e2fsck -n -f "$2" >&2
`

// The cleanup unit unmounts what an interrupted run left mounted.
const storeCompactCleanupScript = `export PATH=/run/current-system/sw/bin:$PATH
if mountpoint -q "$1"; then umount "$1"; fi
true
`

// cleanupStale removes what an interrupted earlier run left: its transient
// unit, its mount, and its copy. The original image is never touched.
func (c *hostStoreCompactor) cleanupStale(ctx context.Context, vmID, copyPath, mountDir string) {
	_, _ = c.run.Run(ctx, c.bin("systemctl"), "stop", c.unitName(vmID))
	_, _ = c.run.Run(ctx, c.bin("systemd-run"), "--quiet", "--wait", "--pipe", "--collect",
		"--unit="+c.unitName(vmID)+"-cleanup", c.bin("sh"), "-c", storeCompactCleanupScript, "sh", mountDir)
	_ = os.Remove(copyPath)
}

// Compact compacts dataImagePath through a reflink copy and swaps it in.
// Every error before the rename leaves dataImagePath as it was.
func (c *hostStoreCompactor) Compact(ctx context.Context, vmID, dataImagePath, computerID, canonicalHead string) (storeCompactionReport, error) {
	var report storeCompactionReport
	started := time.Now()
	copyPath := dataImagePath + storeCompactCopySuffix
	mountDir := filepath.Join(c.mountRoot, vmID)
	c.cleanupStale(ctx, vmID, copyPath, mountDir)

	if out, err := c.run.Run(ctx, c.bin("cp"), "--reflink=always", dataImagePath, copyPath); err != nil {
		_ = os.Remove(copyPath)
		return report, fmt.Errorf("store compaction: reflink copy: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if canonicalHead == "" {
		canonicalHead = "unreported"
	}
	out, err := c.run.Run(ctx, c.bin("systemd-run"), "--quiet", "--wait", "--pipe", "--collect",
		"--unit="+c.unitName(vmID),
		"-p", "MemoryMax="+c.memoryMax, "-p", "MemorySwapMax=0",
		c.bin("sh"), "-c", storeCompactScript, "sh",
		mountDir, copyPath, c.compactBin, filepath.Join(mountDir, "state.texture"), computerID, canonicalHead)
	if decodeErr := json.Unmarshal(lastJSONLine(out), &report); decodeErr != nil && err == nil {
		err = fmt.Errorf("unreadable report: %w", decodeErr)
	}
	if err != nil || report.Error != "" || report.RootBefore == "" || report.RootBefore != report.RootAfter {
		if err == nil {
			err = fmt.Errorf("refused (root %q to %q): %s", report.RootBefore, report.RootAfter, report.Error)
		}
		c.cleanupStale(context.Background(), vmID, copyPath, mountDir)
		return report, fmt.Errorf("store compaction: %v: %s", err, tail(out, 600))
	}
	if err := os.Rename(copyPath, dataImagePath); err != nil {
		// The original is still in place and bootable.
		_ = os.Remove(copyPath)
		return report, fmt.Errorf("store compaction: swap in the copy: %w", err)
	}
	report.Duration = time.Since(started).Round(time.Second).String()
	return report, nil
}

func tail(out []byte, n int) string {
	s := strings.TrimSpace(string(out))
	if len(s) > n {
		s = s[len(s)-n:]
	}
	return s
}

// lastJSONLine returns the last line of out that looks like a JSON object.
func lastJSONLine(out []byte) []byte {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); strings.HasPrefix(line, "{") {
			return []byte(line)
		}
	}
	return nil
}

type storeCompactorIface interface {
	Compact(ctx context.Context, vmID, dataImagePath, computerID, canonicalHead string) (storeCompactionReport, error)
}

type storeCompactRequest struct {
	UserID    string `json:"user_id"`
	DesktopID string `json:"desktop_id,omitempty"`
	// Force passes the per-computer gap (an operator's by-hand call); the
	// host free-space floor and the guest's refusals still apply.
	Force bool `json:"force,omitempty"`
}

type storeCompactResponse struct {
	deployRefreshResponse
	Compaction *storeCompactionReport `json:"compaction,omitempty"`
}

type storeCompactSweepRequest struct {
	MinFileBytes uint64 `json:"min_file_bytes,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
}

type storeCompactSweepEntry struct {
	VMID       string                `json:"vm_id"`
	ComputerID string                `json:"computer_id"`
	FileBytes  uint64                `json:"file_bytes"`
	Result     *storeCompactResponse `json:"result,omitempty"`
	Reason     string                `json:"reason,omitempty"`
}

type storeCompactSweepResponse struct {
	MinFileBytes uint64                   `json:"min_file_bytes"`
	DryRun       bool                     `json:"dry_run"`
	Selected     []storeCompactSweepEntry `json:"selected"`
	Skipped      []storeCompactSweepEntry `json:"skipped"`
}

type storeCompactState struct {
	mu        sync.Mutex
	compactor storeCompactorIface
	// minHostFree is the host free-space floor below which every
	// compaction refuses (0: no floor); freeBytes reads host free space.
	minHostFree uint64
	freeBytes   func() (uint64, error)
	// attempts is the last compaction attempt per computer (the pressure
	// trigger's gap; store_compaction_pressure.go).
	attemptsMu sync.Mutex
	attempts   *storeCompactAttempts
	// allocatedBytes reads a data image's host allocation (a test hook;
	// nil: allocatedBytesOf).
	allocatedBytes func(path string) uint64
}

func (h *Handler) storeCompaction() *storeCompactState {
	h.storeCompactOnce.Do(func() {
		if h.storeCompact == nil {
			h.storeCompact = &storeCompactState{
				compactor:   newHostStoreCompactor(),
				minHostFree: storeCompactMinHostFreeFromEnv(os.Getenv),
				// Free space on the filesystem holding the data images.
				freeBytes: func() (uint64, error) {
					free, _, err := statfsAvailable(h.registry.DataImageStateDir())
					return free, err
				},
			}
		}
	})
	return h.storeCompact
}

func (s *storeCompactState) attemptsRecord() *storeCompactAttempts {
	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()
	if s.attempts == nil {
		s.attempts = loadStoreCompactAttempts("")
	}
	return s.attempts
}

// compactStore compacts one computer's store as a planned restart. Before
// the guest is asked it refuses, in order: below the host free-space floor
// (507), inside the computer's gap unless force (429), and when the attempts
// record cannot be written (507), so no restart is spent on a compaction
// that cannot run or record itself.
func (h *Handler) compactStore(ctx context.Context, own ownershipResponse, force bool, now time.Time) (storeCompactResponse, int) {
	state := h.storeCompaction()
	if !state.mu.TryLock() {
		return storeCompactResponse{deployRefreshResponse: deployRefreshResponse{Outcome: deployRefreshDeferred, Reason: "another store compaction is running", VMID: own.VMID}}, http.StatusConflict
	}
	defer state.mu.Unlock()
	refuse := func(reason string, status int) (storeCompactResponse, int) {
		log.Printf("vmctl: store compaction %s (computer %s): refused: %s", own.VMID, own.ComputerID, reason)
		return storeCompactResponse{deployRefreshResponse: deployRefreshResponse{Outcome: deployRefreshSkipped, Reason: reason, VMID: own.VMID, ComputerID: own.ComputerID}}, status
	}
	if refusal := state.hostFloorRefusal(); refusal != "" {
		return refuse(refusal, http.StatusInsufficientStorage)
	}
	if !force {
		if refusal := h.storeCompactGapRefusal(own.VMID, now); refusal != "" {
			return refuse(refusal, http.StatusTooManyRequests)
		}
	}
	attempts := state.attemptsRecord()
	if err := attempts.checkWritable(); err != nil {
		return refuse("attempts record not writable: "+err.Error(), http.StatusInsufficientStorage)
	}
	head := ""
	if own.ReportedRelease != nil {
		head = own.ReportedRelease.CanonicalEventHead
	}
	var report *storeCompactionReport
	var compactErr error
	maintenance := func(dataImagePath string) error {
		// The guest took the planned restart and the VM is down: this is an
		// attempt, recorded before the compaction so a vmctl crash during it
		// still holds the gap. The VM is already down, so a failed write
		// (checked writable a moment ago) does not stop the compaction; the
		// attempt stays in memory.
		att := storeCompactAttempt{VMID: own.VMID, At: now, FileBytesBefore: state.allocated(dataImagePath)}
		if err := attempts.record(att); err != nil {
			log.Printf("vmctl: store compaction %s: attempt kept in memory only: %v", own.VMID, err)
		}
		r, err := state.compactor.Compact(context.Background(), own.VMID, dataImagePath, own.ComputerID, head)
		report, compactErr = &r, err
		if err == nil {
			att.FileBytesAfter = state.allocated(dataImagePath)
			if werr := attempts.record(att); werr != nil {
				log.Printf("vmctl: store compaction %s: result kept in memory only: %v", own.VMID, werr)
			}
		}
		return err
	}
	req := deployRefreshRequest{UserID: own.UserID, DesktopID: normalizeDesktopID(own.DesktopID), Target: storeCompactTarget}
	resp, status := h.deployRefreshWith(ctx, req, maintenance)
	out := storeCompactResponse{deployRefreshResponse: resp, Compaction: report}
	if compactErr != nil {
		if out.Compaction == nil {
			out.Compaction = &storeCompactionReport{}
		}
		out.Compaction.Error = compactErr.Error()
		if errors.Is(compactErr, ErrOfflineMaintenanceUnbootable) {
			status = http.StatusInternalServerError
		}
	}
	log.Printf("vmctl: store compaction %s (computer %s): outcome=%s reason=%q compaction=%+v", own.VMID, own.ComputerID, resp.Outcome, resp.Reason, out.Compaction)
	return out, status
}

// fillDataImageStats reads the host data image stats of each active
// interactive computer: the ownership list leaves them empty (only the
// single-computer lookup fills them), and the sweep selects by them.
func (h *Handler) fillDataImageStats(ownerships []ownershipResponse) {
	statsFor := h.dataImageStatsFor
	if statsFor == nil {
		statsFor = h.registry.DataImageStatsForVM
	}
	for i := range ownerships {
		own := &ownerships[i]
		if own.DataImage != nil || own.State != string(VMStateActive) || (own.Kind != "" && own.Kind != VMKindInteractive) {
			continue
		}
		if stats, ok := statsFor(own.VMID); ok {
			own.DataImage = dataImageResponseFromStats(stats)
		}
	}
}

// selectStoreCompaction picks the active, unheld interactive platform
// computers whose data image allocates at least minFileBytes on the host.
func selectStoreCompaction(ownerships []ownershipResponse, minFileBytes uint64) (selected, skipped []storeCompactSweepEntry, picks []ownershipResponse) {
	selected, skipped = []storeCompactSweepEntry{}, []storeCompactSweepEntry{}
	for _, own := range ownerships {
		entry := storeCompactSweepEntry{VMID: own.VMID, ComputerID: own.ComputerID}
		if own.DataImage != nil {
			entry.FileBytes = own.DataImage.FileBytes
		}
		switch {
		case own.State != string(VMStateActive):
			continue
		case own.Kind != "" && own.Kind != VMKindInteractive:
			continue
		case !deployRefreshAllowsBusy(WarmnessClass(own.WarmnessClass)):
			// History squashing is ruled for the platform's always-busy
			// computers (owner notes §26). An account's computer can hold
			// a small store in a large untrimmed image; it is compacted
			// only by hand, on the owner's ask.
			entry.Reason = "not a platform class"
		case own.Held:
			entry.Reason = "held"
		case own.DataImage == nil:
			entry.Reason = "no data image stats"
		case entry.FileBytes < minFileBytes:
			entry.Reason = "below threshold"
		}
		if entry.Reason != "" {
			skipped = append(skipped, entry)
			continue
		}
		selected = append(selected, entry)
		picks = append(picks, own)
	}
	return selected, skipped, picks
}

// HandleCompactStore serves POST /internal/vmctl/compact-store
// {"user_id","desktop_id"}: compact one computer's store now.
func (h *Handler) HandleCompactStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req storeCompactRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || strings.TrimSpace(req.UserID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	ownerships, err := h.listOwnershipResponses(r.Context())
	if err != nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: err.Error()})
		return
	}
	desktopID := normalizeDesktopID(req.DesktopID)
	for _, own := range ownerships {
		if own.UserID == strings.TrimSpace(req.UserID) && normalizeDesktopID(own.DesktopID) == desktopID {
			resp, status := h.compactStore(r.Context(), own, req.Force, time.Now())
			writeVMCTLJSON(w, status, resp)
			return
		}
	}
	writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no computer"})
}

// HandleCompactSweep serves POST /internal/vmctl/compact-sweep
// {"min_file_bytes","dry_run"}: compact, one at a time, every active, unheld
// interactive computer whose data image holds at least min_file_bytes on the
// host. A busy refusal is a deferral; the next day's sweep tries again. A
// computer inside its gap is skipped. A refusal at the host free-space floor
// answers 507, so the timer unit fails and says why.
func (h *Handler) HandleCompactSweep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req storeCompactSweepRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if req.MinFileBytes == 0 {
		req.MinFileBytes = defaultStoreCompactMin
	}
	ownerships, err := h.listOwnershipResponses(r.Context())
	if err != nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: err.Error()})
		return
	}
	h.fillDataImageStats(ownerships)
	selected, skipped, picks := selectStoreCompaction(ownerships, req.MinFileBytes)
	resp := storeCompactSweepResponse{MinFileBytes: req.MinFileBytes, DryRun: req.DryRun, Selected: selected, Skipped: skipped}
	status := http.StatusOK
	if !req.DryRun {
		for i, own := range picks {
			result, code := h.compactStore(r.Context(), own, false, time.Now())
			resp.Selected[i].Result = &result
			if code == http.StatusInsufficientStorage {
				// No room (or no writable attempts record): the timer unit
				// must fail and say why, not exit 0 having done nothing.
				status = code
				log.Printf("vmctl: store compaction sweep refused %s (computer %s): %s", own.VMID, own.ComputerID, result.Reason)
			}
		}
	}
	writeVMCTLJSON(w, status, resp)
}
