package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yusefmosiah/go-choir/internal/guestmem"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// D6 S1: the guest's page cache return route
// (docs/design/density-d6-page-cache-return-2026-10-26.md section 4.1).
//
// vmctl asks an idle (or fenced, about to be snapshotted) guest to give its
// cold page cache back: a bounded memory.reclaim on the daemon's cgroup, the
// work of internal/guestmem. Free page reporting, already on in every guest,
// carries the freed pages to the host. The act is a density lever, not an
// isolation control: the route is host-peer only, takes no credential, and
// vmctl acts only on its own host-side measurements, never on this report.
//
// The act is not work in the CLAUDE.md "Restarts End Work" sense: it opens no
// obligation, writes nothing durable and is idempotent. The prepare route and
// its body are untouched.

// cacheReturnBody is the request. Decoded strictly.
type cacheReturnBody struct {
	Mode   string `json:"mode"`
	Scope  string `json:"scope"`
	DryRun bool   `json:"dry_run"`
	// CeilingMiB is ceiling mode's ceiling.
	CeilingMiB int64 `json:"ceiling_mib,omitempty"`
	// ThroughWork (ceiling mode only): the ceiling acts while a capsule or a
	// self-development operation is live, standing aside only for a
	// hibernate and a store snapshot. vmctl sets it for the computers its
	// VMCTL_GUEST_MEMORY_CAP_THROUGH_WORK_COMPUTERS selector names.
	ThroughWork bool `json:"through_work,omitempty"`
}

// cacheReturnRefusal is the answer when the act did not run.
type cacheReturnRefusal struct {
	Outcome string `json:"outcome"`
	// Reason: running, hibernating, store_unanswered, desk_pending,
	// selfdev_active, live_capsules (busy only).
	Reason     string   `json:"reason,omitempty"`
	Error      string   `json:"error,omitempty"`
	Unanswered []string `json:"unanswered,omitempty"`
}

// cacheReturnStoreBudget bounds the idle rule's store reads, as /health's.
const cacheReturnStoreBudget = time.Second

// HandleInternalMemoryCacheReturn serves POST /internal/runtime/memory/cache-return.
// 200 carries the act's report (any outcome, including partial and
// stopped_busy). 409 {"outcome": "busy" | "in_progress" | "not_fenced"} means
// the act did not run; an unanswered store read is busy, with the reads that
// did not answer listed. Host-peer only.
func (h *APIHandler) HandleInternalMemoryCacheReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "memory surface unavailable"})
		return
	}
	var body cacheReturnBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid cache return request"})
		return
	}
	if body.ThroughWork && body.Mode != guestmem.ModeCeiling {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "through_work is ceiling mode's"})
		return
	}
	if body.Scope == "" {
		body.Scope = guestmem.ScopeDaemon
	}
	switch body.Scope {
	case guestmem.ScopeDaemon, guestmem.ScopeRoot:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unknown scope"})
		return
	}
	req := guestmem.ReclaimRequest{
		Mode: body.Mode, Scope: body.Scope, DryRun: body.DryRun,
		Reporting: cacheReturnReporting,
	}
	switch body.Mode {
	case guestmem.ModeIdle:
		if refusal := h.rt.cacheReturnIdleRefusal(r.Context()); refusal != nil {
			writeAPIJSON(w, http.StatusConflict, refusal)
			return
		}
		req.Stop = h.rt.cacheReturnIdleStop
	case guestmem.ModeHibernate:
		// Only while the hibernate fence is closed: the fence already proved
		// the guest idle, and it keeps new work out for the act's duration.
		if !h.rt.hibernate.closed() {
			writeAPIJSON(w, http.StatusConflict, cacheReturnRefusal{Outcome: "not_fenced", Error: "hibernate mode needs the hibernate fence closed"})
			return
		}
		req.Stop = func() string {
			if !h.rt.hibernate.closed() {
				return "fence_lifted"
			}
			return ""
		}
	case guestmem.ModeBusyCold:
		// An always-busy computer while it works (S5; owner decision
		// 2026-11-01). Runs may be running; nothing unsafe may be in flight.
		if refusal := h.rt.cacheReturnBusyColdRefusal(r.Context()); refusal != nil {
			h.rt.cacheReturnLogf("agentcore: cache return mode=busy_cold scope=%s dry_run=%t outcome=%s reason=%s detail=%q",
				body.Scope, body.DryRun, refusal.Outcome, refusal.Reason, refusal.Error)
			writeAPIJSON(w, http.StatusConflict, refusal)
			return
		}
		req.Stop = func() string {
			if h.rt.hibernate.closed() {
				return "hibernate_started"
			}
			return ""
		}
		req.Refaults = &h.rt.cacheReturnRefaults
	case guestmem.ModeCeiling:
		// The elastic cap (S6), asked every minute. busy_cold's busy rule:
		// while a capsule or a self-development operation is live the
		// ceiling stands aside and the guest grows as it needs. Through
		// work it does not: dropping clean cache cuts no work and never
		// takes anonymous memory, so a build still grows to the guest's
		// full size, and an arm's round no longer holds its cache while its
		// own build waits for host memory
		// (docs/problems/working-guest-cache-held-while-builds-wait-2026-11-03.md).
		var refusal *cacheReturnRefusal
		if body.ThroughWork {
			refusal = h.rt.cacheReturnCeilingThroughWorkRefusal()
		} else {
			refusal = h.rt.cacheReturnBusyColdRefusal(r.Context())
		}
		if refusal != nil {
			writeAPIJSON(w, http.StatusConflict, refusal)
			return
		}
		req.CeilingBytes = body.CeilingMiB << 20
		req.Stop = func() string {
			if h.rt.hibernate.closed() {
				return "hibernate_started"
			}
			return ""
		}
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsupported mode"})
		return
	}
	report, err := h.rt.cacheReturn.Reclaim(r.Context(), req)
	if errors.Is(err, guestmem.ErrInProgress) {
		writeAPIJSON(w, http.StatusConflict, cacheReturnRefusal{Outcome: "in_progress", Error: err.Error()})
		return
	}
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if body.Mode == guestmem.ModeCeiling {
		// Asked every minute: only an act that wrote, a dry run (the observe
		// ring is read here, as busy_cold's) or one that could not run is a
		// line.
		if report.Steps > 0 || report.DryRun || report.Outcome == guestmem.OutcomeUnsupported || report.Outcome == guestmem.OutcomeSwapPresent {
			h.rt.cacheReturnLogf("agentcore: cache return mode=ceiling scope=%s dry_run=%t through_work=%t outcome=%s ceiling_mib=%d used_mib=%d target_mib=%d reclaimed_mib=%d steps=%d ms=%d detail=%q",
				report.Scope, report.DryRun, body.ThroughWork, report.Outcome, report.CeilingBytes>>20, report.UsedBytes>>20, report.TargetBytes>>20, report.ReclaimedBytes>>20, report.Steps, report.MS, report.Detail)
		}
	} else if !body.DryRun || body.Mode == guestmem.ModeBusyCold {
		// busy_cold logs its dry runs too: the paper's console log is where
		// its observe ring is read.
		h.rt.cacheReturnLogf("agentcore: cache return mode=%s scope=%s dry_run=%t outcome=%s method=%s target_mib=%d reclaimed_mib=%d steps=%d ms=%d%s detail=%q",
			report.Mode, report.Scope, report.DryRun, report.Outcome, report.Method, report.TargetBytes>>20, report.ReclaimedBytes>>20, report.Steps, report.MS,
			cacheReturnRefaultLog(report), report.Detail)
	}
	if body.Mode == guestmem.ModeBusyCold && !body.DryRun && report.Steps > 0 {
		go h.rt.cacheReturnPostReading()
	}
	writeAPIJSON(w, http.StatusOK, report)
}

// cacheReturnIdleRefusal is idle mode's busy rule, exactly /health's view:
// the fence closed, a run running, or any of the three store reads
// (desk pending, self-development active, live capsules) positive,
// unanswered or, for the capsule count, not reported. nil means idle.
func (rt *Runtime) cacheReturnIdleRefusal(ctx context.Context) *cacheReturnRefusal {
	if rt.hibernate.closed() {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "hibernating", Error: "a hibernate is in progress"}
	}
	if rt.RunningCount() > 0 {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "running", Error: "a run is running"}
	}
	read := rt.readHealthStore
	if rt.cacheReturnReads != nil {
		read = rt.cacheReturnReads
	}
	reads := read(ctx, cacheReturnStoreBudget)
	if len(reads.unanswered) > 0 {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "store_unanswered", Error: "a busy read did not answer", Unanswered: reads.unanswered}
	}
	switch {
	case reads.deskPending > 0:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "desk_pending", Error: "a desk mutation is pending"}
	case reads.selfdevActive > 0:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "selfdev_active", Error: "a self-development operation is active"}
	case reads.liveCapsules == nil:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "live_capsules", Error: "live capsules are not reported"}
	case *reads.liveCapsules > 0:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "live_capsules", Error: "a capsule is live"}
	}
	return nil
}

// cacheReturnBusyColdBudget bounds busy_cold's always-busy read.
const cacheReturnBusyColdBudget = time.Second

// cacheReturnBusyColdRefusal is busy_cold's busy rule: the CLAUDE.md
// always-busy exclusion, the same read a planned restart refuses on. A
// running run is not busy here (the mode exists for computers that always
// run); a live engineering capsule or a self-development operation in
// executing, accepted, materializing or rollback_pending is, and so is a
// closed hibernate fence. It fails closed: an error, an uncountable capsule
// list or a read that has not answered within a second is 409 busy
// store_unanswered. nil means the act may run.
func (rt *Runtime) cacheReturnBusyColdRefusal(ctx context.Context) *cacheReturnRefusal {
	if rt.hibernate.closed() {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "hibernating", Error: "a hibernate is in progress"}
	}
	read := rt.cacheReturnBusyColdWork
	if read == nil {
		read = func(ctx context.Context) (plannedRestartWork, error) {
			if !rt.liveCapsulesCountable() {
				return plannedRestartWork{}, errLiveCapsulesUncountable
			}
			return rt.plannedRestartBusy(ctx, true)
		}
	}
	rctx, cancel := context.WithTimeout(ctx, cacheReturnBusyColdBudget)
	defer cancel()
	type answer struct {
		work plannedRestartWork
		err  error
	}
	done := make(chan answer, 1)
	go func() {
		work, err := read(rctx)
		done <- answer{work, err}
	}()
	var got answer
	select {
	case got = <-done:
	case <-rctx.Done():
		return &cacheReturnRefusal{Outcome: "busy", Reason: "store_unanswered", Error: "the always-busy read did not answer", Unanswered: []string{"selfdev_active_operations", "live_capsules"}}
	}
	switch {
	case got.err != nil:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "store_unanswered", Error: got.err.Error(), Unanswered: []string{"selfdev_active_operations", "live_capsules"}}
	case got.work.SelfdevActive > 0:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "selfdev_active", Error: "a self-development operation is active"}
	case got.work.LiveCapsules > 0:
		return &cacheReturnRefusal{Outcome: "busy", Reason: "live_capsules", Error: "a capsule is live"}
	case got.work.SnapshotInFlight:
		// A snapshot streams the whole store through the cache; reclaiming
		// under it only makes it read again (review of S5, Devin #3).
		return &cacheReturnRefusal{Outcome: "busy", Reason: "snapshot_in_flight", Error: "a store snapshot is streaming"}
	}
	return nil
}

// cacheReturnCeilingThroughWorkRefusal is the ceiling's busy rule through
// work: a closed hibernate fence (the snapshot owns the memory) or a store
// snapshot streaming (reclaiming under it only makes it read again). Live
// capsules and self-development operations are not busy here, so no store
// read is needed. nil means the act may run.
func (rt *Runtime) cacheReturnCeilingThroughWorkRefusal() *cacheReturnRefusal {
	if rt.hibernate.closed() {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "hibernating", Error: "a hibernate is in progress"}
	}
	if rt.snapshots.inFlight.Load() {
		return &cacheReturnRefusal{Outcome: "busy", Reason: "snapshot_in_flight", Error: "a store snapshot is streaming"}
	}
	return nil
}

// cacheReturnPostDelay is when busy_cold's post-act reading is taken.
const cacheReturnPostDelay = 60 * time.Second

func (rt *Runtime) cacheReturnLogf(format string, args ...any) {
	if rt.cacheReturnLog != nil {
		rt.cacheReturnLog(fmt.Sprintf(format, args...))
		return
	}
	log.Printf(format, args...)
}

// cacheReturnPostReading takes one reading about 60 s after a busy_cold act
// that wrote and logs the refault rate since the act's end
// (refault_rate_post): the price of the act, read after the working set has
// had time to come back. One read of /proc/vmstat; nothing else reads it.
func (rt *Runtime) cacheReturnPostReading() {
	delay := rt.cacheReturnPostDelay
	if delay <= 0 {
		delay = cacheReturnPostDelay
	}
	time.Sleep(delay)
	root := rt.cacheReturn.ProcRoot
	if root == "" {
		root = "/proc"
	}
	raw, err := os.ReadFile(filepath.Join(root, "vmstat"))
	if err != nil {
		return
	}
	current := guestmem.VmstatField(string(raw), "workingset_refault_file")
	if current == nil {
		return
	}
	rate, ok := rt.cacheReturnRefaults.PostRate(*current)
	shown := "unknown"
	if ok {
		shown = strconv.FormatFloat(rate, 'f', 1, 64)
	}
	rt.cacheReturnLogf("agentcore: cache return post mode=busy_cold refaults_post=%d refault_rate_post=%s refault_limit=%.0f after_s=%.0f",
		*current, shown, guestmem.RefaultLimitPerSecond, delay.Seconds())
}

// cacheReturnRefaultLog is the refault part of the act's log line: the
// counter before and after, the guard's rates and limit (busy_cold).
func cacheReturnRefaultLog(report guestmem.ReclaimReport) string {
	num := func(p *int64) string {
		if p == nil {
			return "unknown"
		}
		return strconv.FormatInt(*p, 10)
	}
	rate := func(p *float64) string {
		if p == nil {
			return "unknown"
		}
		return strconv.FormatFloat(*p, 'f', 1, 64)
	}
	out := " refaults_before=" + num(report.Before.WorkingsetRefaultFile) + " refaults_after=" + num(report.After.WorkingsetRefaultFile)
	if report.Mode == guestmem.ModeBusyCold {
		out += " refault_rate_before=" + rate(report.RefaultRateBefore) + " refault_rate_after=" + rate(report.RefaultRateAfter) +
			" refault_limit=" + strconv.FormatFloat(report.RefaultLimitPerSec, 'f', 0, 64)
	}
	return out
}

// cacheReturnIdleStop is asked between steps: a run starting shows in
// RunningCount at once, and a hibernate starting closes the fence. The store
// reads are not repeated per step.
func (rt *Runtime) cacheReturnIdleStop() string {
	if rt.RunningCount() > 0 {
		return "run_started"
	}
	if rt.hibernate.closed() {
		return "hibernate_started"
	}
	return ""
}

// cacheReturnReporting reads free page reporting's state from the guest's
// real /sys and /proc, as the memory-return report does.
func cacheReturnReporting() guestmem.Reporting {
	mr := server.ReadMemoryReturn()
	out := guestmem.Reporting{}
	if mr.Balloon.Bound && mr.Balloon.Reporting != nil {
		negotiated := *mr.Balloon.Reporting && mr.PageReporting.Registered
		out.Negotiated = &negotiated
	}
	if mr.FreeBlocks != nil && mr.FreeBlocks.ReportableKiB != nil {
		out.ReportableKiB = mr.FreeBlocks.ReportableKiB
	}
	return out
}
