package vmmanager

// Balloon statistics: the host's read of a guest's own memory view (RES-10
// steps 1 and 2; docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md).
//
// Firecracker collects statistics only when the balloon booted with a
// non-zero stats_polling_interval_s; it cannot be turned on for a running
// guest without PATCH /balloon/statistics, which this code never sends.
// Statistics are staged per computer like the other FP8 launch features:
//
//	VM_BALLOON_STATS_ONLY                VM ids, computer ids, class:test or *
//	                                     (computer_select.go); empty: off
//	VM_BALLOON_STATS_POLLING_INTERVAL_S  interval for selected computers,
//	                                     1..65535 s, default 5
//
// It rides on the free page reporting balloon (VM_BALLOON_FREE_PAGE_REPORTING)
// and never adds a device. An unselected computer's config is byte-for-byte
// the pre-RES-10 one. The balloon is in the snapshot launch-config hash, so
// a snapshot taken under another selection is refused and the computer cold
// boots once. A restored guest keeps its saved device, so the reader asks
// the VMM what it has (GET /balloon) rather than trusting the selection.
//
// The reader is read only and bounded: it never waits on another operation
// of the VM, both calls share guestMemoryStatsTimeout, and every failure is
// a GuestMemoryStatsError reason, never a zero reading.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

const defaultBalloonStatsIntervalS = 5

// guestMemoryStatsTimeout bounds one reading (both API calls), so a stuck
// VMM costs its caller this long at most. A var for tests.
var guestMemoryStatsTimeout = 2 * time.Second

// balloonStatsIntervalS is cfg's stats_polling_interval_s: 0 unless the
// selector names the computer.
func (m *Manager) balloonStatsIntervalS(cfg VMConfig) int {
	if !selectsComputer(m.cfg.BalloonStatsOnly, cfg) {
		return 0
	}
	if n := m.cfg.BalloonStatsPollingIntervalS; n > 0 {
		return n
	}
	return defaultBalloonStatsIntervalS
}

// BalloonStatistics is Firecracker's GET /balloon/statistics, with its names
// and units: target/actual pages are 4 KiB pages, target/actual MiB are MiB,
// swap_in, swap_out, free_memory, total_memory, available_memory and
// disk_caches are bytes, faults and hugetlb counters are counts. The
// optional fields are pointers: absent means the guest has not reported
// that tag, which is not 0.
type BalloonStatistics struct {
	TargetPages        uint64  `json:"target_pages"`
	ActualPages        uint64  `json:"actual_pages"`
	TargetMiB          uint64  `json:"target_mib"`
	ActualMiB          uint64  `json:"actual_mib"`
	SwapIn             *uint64 `json:"swap_in,omitempty"`
	SwapOut            *uint64 `json:"swap_out,omitempty"`
	MajorFaults        *uint64 `json:"major_faults,omitempty"`
	MinorFaults        *uint64 `json:"minor_faults,omitempty"`
	FreeMemory         *uint64 `json:"free_memory,omitempty"`
	TotalMemory        *uint64 `json:"total_memory,omitempty"`
	AvailableMemory    *uint64 `json:"available_memory,omitempty"`
	DiskCaches         *uint64 `json:"disk_caches,omitempty"`
	HugetlbAllocations *uint64 `json:"hugetlb_allocations,omitempty"`
	HugetlbFailures    *uint64 `json:"hugetlb_failures,omitempty"`
}

// GuestMemoryStats is one reading of a running VM's balloon.
type GuestMemoryStats struct {
	VMID string `json:"vm_id"`
	// StatsPollingIntervalS is the device's own interval (GET /balloon).
	StatsPollingIntervalS int               `json:"stats_polling_interval_s"`
	Stats                 BalloonStatistics `json:"stats"`
	ObservedAt            time.Time         `json:"observed_at"`
}

// Reasons a reading is unavailable.
const (
	GuestMemoryStatsReasonNotRunning  = "not_running"
	GuestMemoryStatsReasonNoAPISocket = "no_api_socket"
	GuestMemoryStatsReasonNoBalloon   = "no_balloon_device"
	GuestMemoryStatsReasonDisabled    = "stats_disabled"
	GuestMemoryStatsReasonNotReported = "not_reported_yet"
	GuestMemoryStatsReasonBusy        = "busy"
	GuestMemoryStatsReasonTimeout     = "timeout"
	GuestMemoryStatsReasonAPIError    = "api_error"
)

// GuestMemoryStatsError is an unavailable reading and why.
type GuestMemoryStatsError struct {
	VMID   string
	Reason string
	Detail string
}

func (e *GuestMemoryStatsError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("balloon stats for VM %s unavailable: %s", e.VMID, e.Reason)
	}
	return fmt.Sprintf("balloon stats for VM %s unavailable: %s: %s", e.VMID, e.Reason, e.Detail)
}

// GuestMemoryStats reads a running VM's balloon statistics. It never blocks
// on another operation of the same VM and never mutates the balloon.
func (m *Manager) GuestMemoryStats(vmID string) (GuestMemoryStats, error) {
	fail := func(reason, detail string) (GuestMemoryStats, error) {
		return GuestMemoryStats{}, &GuestMemoryStatsError{VMID: vmID, Reason: reason, Detail: detail}
	}
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return fail(GuestMemoryStatsReasonBusy, "VM operation in progress")
	}
	defer unlock()
	sock, _, err := m.runningAPISocket(vmID)
	switch {
	case errors.Is(err, errVMNotRunning):
		return fail(GuestMemoryStatsReasonNotRunning, "")
	case err != nil:
		detail := err.Error()
		var nse *noAPISocketError
		if errors.As(err, &nse) {
			detail = fmt.Sprintf("%s: %v", nse.Path, nse.Err)
		}
		return fail(GuestMemoryStatsReasonNoAPISocket, detail)
	}
	ctx, cancel := context.WithTimeout(context.Background(), guestMemoryStatsTimeout)
	defer cancel()
	return guestMemoryStatsVia(ctx, newFCAPIClient(sock), vmID)
}

func guestMemoryStatsVia(ctx context.Context, api *fcAPIClient, vmID string) (GuestMemoryStats, error) {
	fail := func(reason, detail string) (GuestMemoryStats, error) {
		return GuestMemoryStats{}, &GuestMemoryStatsError{VMID: vmID, Reason: reason, Detail: detail}
	}
	classify := func(err error, refused string) (GuestMemoryStats, error) {
		var apiErr *fcAPIError
		switch {
		case errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500:
			return fail(refused, apiErr.FaultMessage)
		case ctx.Err() != nil:
			return fail(GuestMemoryStatsReasonTimeout, fmt.Sprintf("no answer within %s", guestMemoryStatsTimeout))
		default:
			return fail(GuestMemoryStatsReasonAPIError, err.Error())
		}
	}
	dev, err := api.Balloon(ctx)
	if err != nil {
		return classify(err, GuestMemoryStatsReasonNoBalloon)
	}
	if dev.StatsPollingIntervalS <= 0 {
		// A guest booted, or restored from a snapshot taken, without stats.
		return fail(GuestMemoryStatsReasonDisabled, "balloon booted with stats_polling_interval_s 0")
	}
	st, err := api.BalloonStatistics(ctx)
	if err != nil {
		return classify(err, GuestMemoryStatsReasonDisabled)
	}
	if st.AvailableMemory == nil || st.TotalMemory == nil {
		return fail(GuestMemoryStatsReasonNotReported, "the guest driver has not sent available_memory and total_memory")
	}
	return GuestMemoryStats{VMID: vmID, StatsPollingIntervalS: dev.StatsPollingIntervalS, Stats: st, ObservedAt: time.Now().UTC()}, nil
}

var (
	errVMNotRunning = errors.New("VM is not running")
	errNoAPISocket  = errors.New("VM has no API socket")
)

// noAPISocketError is errNoAPISocket with the path looked at and why.
type noAPISocketError struct {
	Path string
	Err  error
}

func (e *noAPISocketError) Error() string {
	return fmt.Sprintf("%v: %s: %v", errNoAPISocket, e.Path, e.Err)
}

func (e *noAPISocketError) Unwrap() error { return errNoAPISocket }

// runningAPISocket returns the API socket of a running VM and its base
// memory, or an error wrapping errVMNotRunning or errNoAPISocket.
func (m *Manager) runningAPISocket(vmID string) (string, int, error) {
	m.mu.Lock()
	inst := m.vms[vmID]
	if inst == nil || inst.State != StateRunning {
		m.mu.Unlock()
		return "", 0, fmt.Errorf("%w: VM %s", errVMNotRunning, vmID)
	}
	jailed := inst.confinement.jailed()
	base := inst.Config.MachineMemSizeMib
	m.mu.Unlock()
	sock := m.apiSocketPath(vmID)
	if jailed {
		sock = m.jailAPISocketHostPath(vmID)
	}
	if _, err := os.Stat(sock); err != nil {
		return "", 0, fmt.Errorf("VM %s: %w", vmID, &noAPISocketError{Path: sock, Err: err})
	}
	return sock, base, nil
}
