package autoputer

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The guest's half of capsule heavy work (station S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, held per job
// since docs/problems/build-capsule-holds-peak-while-agent-thinks-2026-11-04.md):
// it reports the live build capsules' baselines and their jobs' leases to
// vmctl over the guest's tap-bound vmctl route, and asks vmctl to admit a
// build spawn's baseline and each job's lease there.
//
// While any build capsule is live the whole set is re-reported every
// capsuleHostReportInterval, with each capsule that holds no job lease at
// its cgroup usage now (the baseline reader), so vmctl's in-flight TTL never
// expires a live entry and a capsule's charge follows what it uses; once
// the last one ends the empty set is reported once and the loop rests. A
// vmctl that does not serve the report (an older release) or cannot be
// reached leaves the guest as it was before S4: spawns go on, jobs start,
// and the reason is logged.

const (
	capsuleHostReportInterval = 5 * time.Second
	capsuleHostReportTimeout  = 10 * time.Second
)

type capsuleHostReporter interface {
	ReportCapsuleReservations(context.Context, vmctl.CapsuleReservationReport) (vmctl.CapsuleReservationResult, error)
}

// hostEntry is a capsule's baseline (job "") or a job's lease.
type hostEntry struct{ capsule, job string }

type capsuleHostMemory struct {
	client     capsuleHostReporter
	computerID string
	interval   time.Duration

	// reportMu serializes reports, so a heartbeat's older snapshot never
	// lands after an admit and undoes it in vmctl. A release takes only mu:
	// a heartbeat already in flight may carry a released entry once more,
	// and the next report drops it.
	reportMu sync.Mutex
	mu       sync.Mutex
	live     map[hostEntry]int64
	baseline func(capsuleID string) (int64, bool)
	dirty    bool // the set changed since the last report
	wake     chan struct{}
	// loggedErr: a failed report is logged once until one succeeds.
	loggedErr bool
}

func newCapsuleHostMemory(client capsuleHostReporter, computerID string) *capsuleHostMemory {
	return &capsuleHostMemory{
		client: client, computerID: computerID, interval: capsuleHostReportInterval,
		live: map[hostEntry]int64{}, dirty: true, wake: make(chan struct{}, 1),
	}
}

// configuredCapsuleHostMemory is the reporter for this computer, or nil when
// the guest has no vmctl route or no computer id.
func configuredCapsuleHostMemory() *capsuleHostMemory {
	baseURL := strings.TrimSpace(os.Getenv("RUNTIME_VMCTL_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	computerID := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID"))
	if computerID == "" || !(strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://")) {
		return nil
	}
	return newCapsuleHostMemory(vmctl.NewClient(baseURL), computerID)
}

// SetBaselineReader installs how a capsule's baseline is measured.
func (h *capsuleHostMemory) SetBaselineReader(read func(capsuleID string) (int64, bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.baseline = read
}

func (h *capsuleHostMemory) liveLocked() []vmctl.CapsuleReservation {
	out := make([]vmctl.CapsuleReservation, 0, len(h.live))
	for k, b := range h.live {
		out = append(out, vmctl.CapsuleReservation{CapsuleID: k.capsule, JobID: k.job, Bytes: uint64(b)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CapsuleID != out[j].CapsuleID {
			return out[i].CapsuleID < out[j].CapsuleID
		}
		return out[i].JobID < out[j].JobID
	})
	return out
}

// remeasure sets each capsule that holds no job lease to its usage now.
// The reader takes the executor's lock, so it runs outside h.mu. Callers
// hold reportMu.
func (h *capsuleHostMemory) remeasure() {
	h.mu.Lock()
	read := h.baseline
	var idle []string
	leased := map[string]bool{}
	for k := range h.live {
		if k.job != "" {
			leased[k.capsule] = true
		}
	}
	for k := range h.live {
		if k.job == "" && !leased[k.capsule] {
			idle = append(idle, k.capsule)
		}
	}
	h.mu.Unlock()
	if read == nil || len(idle) == 0 {
		return
	}
	measured := map[string]int64{}
	for _, id := range idle {
		if b, ok := read(id); ok && b > 0 {
			measured[id] = b
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, b := range measured {
		k := hostEntry{capsule: id}
		if _, live := h.live[k]; live {
			h.live[k] = b
		}
	}
}

func (h *capsuleHostMemory) signal() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// admit posts the live set with one entry to admit. Only vmctl's refusal is
// a *capsule.HostMemoryRefusal; admitted, or vmctl could not be asked, the
// entry is live as far as this guest knows, and the next report carries it.
// Callers hold reportMu.
func (h *capsuleHostMemory) admit(ctx context.Context, entry hostEntry, bytes int64) error {
	h.mu.Lock()
	report := vmctl.CapsuleReservationReport{ComputerID: h.computerID, Reservations: h.liveLocked(),
		Admit: &vmctl.CapsuleReservation{CapsuleID: entry.capsule, JobID: entry.job, Bytes: uint64(bytes)}}
	h.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, capsuleHostReportTimeout)
	defer cancel()
	result, err := h.client.ReportCapsuleReservations(cctx, report)
	if err == nil && result.Admitted != nil && !*result.Admitted {
		return &capsule.HostMemoryRefusal{Reason: result.Reason, Wait: memoryWaitView(result.Wait)}
	}
	h.mu.Lock()
	if _, capsuleLive := h.live[hostEntry{capsule: entry.capsule}]; entry.job == "" || capsuleLive {
		h.live[entry] = bytes
	}
	// else: the capsule was released during the ask; its job's lease
	// is not kept (review r1).
	h.mu.Unlock()
	h.signal()
	return err
}

// AdmitBuild asks vmctl to admit a build capsule's baseline with the live
// set beside it.
func (h *capsuleHostMemory) AdmitBuild(ctx context.Context, capsuleID string, baseline int64) error {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	return h.admit(ctx, hostEntry{capsule: capsuleID}, baseline)
}

// AdmitJob asks vmctl to admit a job's lease, with its capsule's baseline
// measured now carried in the same report.
func (h *capsuleHostMemory) AdmitJob(ctx context.Context, capsuleID, jobID string, baseline, lease int64) error {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	if baseline > 0 {
		h.mu.Lock()
		if _, live := h.live[hostEntry{capsule: capsuleID}]; live {
			h.live[hostEntry{capsule: capsuleID}] = baseline
		}
		h.mu.Unlock()
	}
	return h.admit(ctx, hostEntry{capsule: capsuleID, job: jobID}, lease)
}

// ReleaseJob drops a job's lease; the next report says so. It never
// blocks.
func (h *capsuleHostMemory) ReleaseJob(capsuleID, jobID string) {
	h.mu.Lock()
	if _, ok := h.live[hostEntry{capsuleID, jobID}]; ok {
		delete(h.live, hostEntry{capsuleID, jobID})
		h.dirty = true
	}
	h.mu.Unlock()
	h.signal()
}

// ReleaseBuild drops the capsule and its jobs' leases from the live set;
// the next report says so. It never blocks.
func (h *capsuleHostMemory) ReleaseBuild(capsuleID string) {
	h.mu.Lock()
	for k := range h.live {
		if k.capsule == capsuleID {
			delete(h.live, k)
			h.dirty = true
		}
	}
	h.mu.Unlock()
	h.signal()
}

// memoryWaitView is vmctl's refusal as the job view carries it.
func memoryWaitView(w *vmctl.CapsuleMemoryWait) *yaegikernel.MemoryWait {
	if w == nil {
		return nil
	}
	out := &yaegikernel.MemoryWait{NeedBytes: w.NeedBytes, RoomBytes: w.RoomBytes, AvailableBytes: w.AvailableBytes,
		FloorBytes: w.FloorBytes, PoolBytes: w.PoolBytes, ReservedBytes: w.ReservedBytes, PressureHigh: w.PressureHigh}
	for _, hd := range w.Holders {
		out.Holders = append(out.Holders, yaegikernel.MemoryHolder{Kind: hd.Kind, Own: hd.Own, Class: hd.Class,
			CapsuleID: hd.CapsuleID, JobID: hd.JobID, Bytes: hd.Bytes})
	}
	return out
}

// reportOnce posts the live set when there is one or it changed. It
// reports whether it posted.
func (h *capsuleHostMemory) reportOnce(ctx context.Context) bool {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	h.remeasure()
	h.mu.Lock()
	if len(h.live) == 0 && !h.dirty {
		h.mu.Unlock()
		return false
	}
	report := vmctl.CapsuleReservationReport{ComputerID: h.computerID, Reservations: h.liveLocked()}
	h.dirty = false
	h.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, capsuleHostReportTimeout)
	_, err := h.client.ReportCapsuleReservations(cctx, report)
	cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		// Not retried for its own sake: a live set is re-reported at the
		// next tick, and an unreported empty set expires in vmctl's TTL.
		if !h.loggedErr {
			h.loggedErr = true
			log.Printf("capsule host memory: report to vmctl for %s: %v; build jobs start as before while the host cannot be asked", h.computerID, err)
		}
		return true
	}
	h.loggedErr = false
	return true
}

// run reports until ctx ends: at once on a change, every interval while a
// build capsule is live. The first pass reports the empty set, clearing
// what a previous boot of this guest left in vmctl's ledger.
func (h *capsuleHostMemory) run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		h.reportOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-h.wake:
		}
	}
}

var _ capsule.HostMemory = (*capsuleHostMemory)(nil)
