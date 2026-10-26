package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
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
	default:
		// busy_cold (always-busy computers) is S5's, behind an owner decision;
		// it is not reachable before it exists.
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
	if !body.DryRun {
		log.Printf("agentcore: cache return mode=%s scope=%s outcome=%s method=%s target_mib=%d reclaimed_mib=%d steps=%d ms=%d detail=%q",
			report.Mode, report.Scope, report.Outcome, report.Method, report.TargetBytes>>20, report.ReclaimedBytes>>20, report.Steps, report.MS, report.Detail)
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
