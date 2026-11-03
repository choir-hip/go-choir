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
)

// The guest's half of capsule heavy work (station S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md): it reports the
// live build-class capsules' host reservations to vmctl over the guest's
// tap-bound vmctl route, admits a build spawn there, and keeps the host
// facts each reply carries for the capsule executor's job starts.
//
// While any build capsule is live the whole set is re-reported every
// capsuleHostReportInterval, so vmctl's in-flight TTL never expires a live
// reservation and the facts stay fresh; once the last one ends the empty set
// is reported once and the loop rests. A vmctl that does not serve the
// report (an older release) or cannot be reached leaves the guest as it was
// before S4: spawns go on, jobs start, and the reason is logged.

const (
	capsuleHostReportInterval = 5 * time.Second
	capsuleHostReportTimeout  = 10 * time.Second
	// capsuleHostFactsFresh: facts older than this are not acted on.
	capsuleHostFactsFresh = 30 * time.Second
)

type capsuleHostReporter interface {
	ReportCapsuleReservations(context.Context, vmctl.CapsuleReservationReport) (vmctl.CapsuleReservationResult, error)
}

type capsuleHostMemory struct {
	client     capsuleHostReporter
	computerID string
	now        func() time.Time
	interval   time.Duration

	// reportMu serializes reports, so a heartbeat's older snapshot never
	// lands after an admit or a release and undoes it in vmctl.
	reportMu sync.Mutex
	mu       sync.Mutex
	live     map[string]int64
	facts    capsule.HostMemoryFacts
	factsAt  time.Time
	dirty    bool // the set changed since the last report
	wake     chan struct{}
	// loggedErr: a failed report is logged once until one succeeds.
	loggedErr bool
}

func newCapsuleHostMemory(client capsuleHostReporter, computerID string) *capsuleHostMemory {
	return &capsuleHostMemory{
		client: client, computerID: computerID, now: time.Now, interval: capsuleHostReportInterval,
		live: map[string]int64{}, dirty: true, wake: make(chan struct{}, 1),
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

func (h *capsuleHostMemory) liveLocked() []vmctl.CapsuleReservation {
	out := make([]vmctl.CapsuleReservation, 0, len(h.live))
	for id, b := range h.live {
		out = append(out, vmctl.CapsuleReservation{CapsuleID: id, Bytes: uint64(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CapsuleID < out[j].CapsuleID })
	return out
}

func (h *capsuleHostMemory) keepFactsLocked(f vmctl.HostMemoryFacts) {
	headroom := int64(f.HeadroomBytes)
	if headroom < 0 { // more than an int64 can hold
		headroom = int64(^uint64(0) >> 1)
	}
	h.facts = capsule.HostMemoryFacts{
		HeadroomBytes: headroom, MemorySomeAvg10: f.MemorySomeAvg10, PressureHigh: f.PressureHigh,
		Protected: f.Priority == vmctl.CapsulePriorityProtected,
	}
	h.factsAt = h.now()
}

func (h *capsuleHostMemory) signal() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// AdmitBuild asks vmctl to admit the build spawn with the live set beside
// it. Only vmctl's refusal is an error the spawn sees.
func (h *capsuleHostMemory) AdmitBuild(ctx context.Context, capsuleID string, bytes int64) error {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	h.mu.Lock()
	report := vmctl.CapsuleReservationReport{ComputerID: h.computerID, Reservations: h.liveLocked(),
		Admit: &vmctl.CapsuleReservation{CapsuleID: capsuleID, Bytes: uint64(bytes)}}
	h.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, capsuleHostReportTimeout)
	defer cancel()
	result, err := h.client.ReportCapsuleReservations(cctx, report)
	h.mu.Lock()
	defer h.mu.Unlock()
	if err == nil {
		h.keepFactsLocked(result.Facts)
		if result.Admitted != nil && !*result.Admitted {
			return &capsule.HostMemoryRefusal{Reason: result.Reason}
		}
	}
	// Admitted, or vmctl could not be asked: the capsule is live as far as
	// this guest knows, and the next report carries it.
	h.live[capsuleID] = bytes
	h.signal()
	return err
}

// ReleaseBuild drops the capsule from the live set; the next report says
// so. It never blocks.
func (h *capsuleHostMemory) ReleaseBuild(capsuleID string) {
	h.mu.Lock()
	if _, ok := h.live[capsuleID]; ok {
		delete(h.live, capsuleID)
		h.dirty = true
	}
	h.mu.Unlock()
	h.signal()
}

// Facts is the latest host facts and whether they are fresh.
func (h *capsuleHostMemory) Facts() (capsule.HostMemoryFacts, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fresh := !h.factsAt.IsZero() && h.now().Sub(h.factsAt) <= capsuleHostFactsFresh
	return h.facts, fresh
}

// reportOnce posts the live set when there is one or it changed. It
// reports whether it posted.
func (h *capsuleHostMemory) reportOnce(ctx context.Context) bool {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	h.mu.Lock()
	if len(h.live) == 0 && !h.dirty {
		h.mu.Unlock()
		return false
	}
	report := vmctl.CapsuleReservationReport{ComputerID: h.computerID, Reservations: h.liveLocked()}
	h.dirty = false
	h.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, capsuleHostReportTimeout)
	result, err := h.client.ReportCapsuleReservations(cctx, report)
	cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		// Not retried for its own sake: a live set is re-reported at the
		// next tick, and an unreported empty set expires in vmctl's TTL.
		if !h.loggedErr {
			h.loggedErr = true
			log.Printf("capsule host memory: report to vmctl for %s: %v; build jobs start as before while the host gives no facts", h.computerID, err)
		}
		return true
	}
	h.loggedErr = false
	h.keepFactsLocked(result.Facts)
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
