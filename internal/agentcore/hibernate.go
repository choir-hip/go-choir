package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// S3 snapshot hibernate, guest side
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md).
//
// Before the host pauses this guest and writes its memory to disk, it asks
// the guest to fence: close work admission, confirm nothing is running, renew
// the platform capability so the frozen copy lives as long as possible, and
// sync the filesystems. After the host loads the snapshot it tells the guest
// it resumed: step the wall clock to the host's, drop pooled connections that
// went stale while frozen, re-credential (install the fresh envelope the host
// delivered, or renew the frozen capability when none came), reopen
// admission.
//
// A snapshot resume is not a boot. Nothing restarts, no planned-restart
// marker is written or consumed, and the work admitted after the resume is
// new work on the same process. A resume that fails is turned by the host
// into a kill and a cold boot, which is a crash boot (no marker): its work is
// interrupted_by_restart like any other crash.

// hibernateFenceTTL bounds how long a fence holds without the host's resumed
// call: a host that fenced and then died or gave up must not leave the guest
// refusing work forever. It runs on the monotonic clock, which a snapshot
// pause does not advance (Firecracker 1.15.1).
const hibernateFenceTTL = 2 * time.Minute

// hibernateClockStepThreshold is the smallest host/guest wall clock gap the
// resume steps; smaller gaps are left to the guest's time sync.
const hibernateClockStepThreshold = time.Second

// hibernateRenewTimeout bounds one capability renewal during prepare/resume.
const hibernateRenewTimeout = 20 * time.Second

// hibernateCredential renews the platform capability on demand.
type hibernateCredential interface {
	RenewNow(ctx context.Context) (time.Time, error)
}

// hibernateResumeCredential installs a fresh bootstrap envelope the host
// delivered with the resumed call (selfdev.GuestCredentials). It exchanges
// and consumes the envelope; the consumption receipt is then appended as
// key_revoked by the fence's reconcile step.
type hibernateResumeCredential interface {
	InstallResumeEnvelope(ctx context.Context, encodedEnvelope string) (time.Time, error)
}

// hibernateResumeCredentialTimeout bounds the install (exchange, consume)
// and the key_revoked append on resume. The host waits 60 s for resumed.
const hibernateResumeCredentialTimeout = 45 * time.Second

// hibernateFence is the work admission gate the host closes before a
// snapshot. Work enters through enterWork (the actor handler's boundary);
// the fence closes only while nothing is inside.
type hibernateFence struct {
	mu       sync.Mutex
	inflight int
	fenced   bool
	until    time.Time
	lifted   chan struct{}

	// credential overrides the runtime's guest credential (tests).
	credential hibernateCredential
	// resumeCredential overrides the envelope installer (tests).
	resumeCredential hibernateResumeCredential
	// reconcile overrides the key_revoked append after an install (tests).
	reconcile func(context.Context) error
	// now overrides time.Now (tests).
	now func() time.Time
}

func (f *hibernateFence) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

// liftLocked reopens admission and wakes waiters. Callers hold f.mu.
func (f *hibernateFence) liftLocked() {
	if !f.fenced {
		return
	}
	f.fenced = false
	f.until = time.Time{}
	if f.lifted != nil {
		close(f.lifted)
		f.lifted = nil
	}
}

// close fences admission when nothing is inside. It reports how many work
// occurrences are inside when it refuses.
func (f *hibernateFence) close(ttl time.Duration) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inflight > 0 {
		return f.inflight, false
	}
	if !f.fenced {
		f.fenced = true
		f.lifted = make(chan struct{})
	}
	f.until = f.clock().Add(ttl)
	return 0, true
}

// closed reports whether the fence is closed and its TTL has not run out.
func (f *hibernateFence) closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fenced && f.clock().Before(f.until)
}

func (f *hibernateFence) lift() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.liftLocked()
}

// tryEnter admits one work occurrence only if the fence is open right now; it
// never waits. The planned-restart mark enters this way (D2 S5): a fenced
// guest refuses the mark at once, and a mark that is inside keeps the fence
// from closing until its marker write returns (close refuses while inflight >
// 0). An expired fence is lifted first, as enter does. No lock is held while
// the caller works.
func (f *hibernateFence) tryEnter() (func(), bool) {
	f.mu.Lock()
	if f.fenced {
		if f.clock().Before(f.until) {
			f.mu.Unlock()
			return func() {}, false
		}
		log.Printf("agentcore: hibernate fence expired without a resume; reopening work admission")
		f.liftLocked()
	}
	f.inflight++
	f.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			f.inflight--
			f.mu.Unlock()
		})
	}, true
}

// enter admits one work occurrence, waiting while the fence holds. The fence
// lifts on resume, on expiry, or never for a guest that is killed instead.
func (f *hibernateFence) enter(ctx context.Context) (func(), error) {
	f.mu.Lock()
	for f.fenced {
		if !f.clock().Before(f.until) {
			log.Printf("agentcore: hibernate fence expired without a resume; reopening work admission")
			f.liftLocked()
			break
		}
		lifted := f.lifted
		wait := f.until.Sub(f.clock())
		f.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-lifted:
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return func() {}, ctx.Err()
		}
		timer.Stop()
		f.mu.Lock()
	}
	f.inflight++
	f.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			f.inflight--
			f.mu.Unlock()
		})
	}, nil
}

// EnterWork admits one work occurrence through the hibernate fence. The
// actor handler calls it at its execution boundary and calls the returned
// release when the occurrence's handling returns. While the host is taking a
// snapshot it waits; it returns an error only when ctx ends first.
func (rt *Runtime) EnterWork(ctx context.Context) (func(), error) {
	if rt == nil {
		return func() {}, nil
	}
	return rt.hibernate.enter(ctx)
}

func (rt *Runtime) hibernateCredential() hibernateCredential {
	if rt.hibernate.credential != nil {
		return rt.hibernate.credential
	}
	if rt.selfdevControl != nil {
		return rt.selfdevControl
	}
	if rt.ownerRecoveryControl != nil {
		return rt.ownerRecoveryControl
	}
	return nil
}

// guestCredentials is the runtime's platform credential holder, if any.
func (rt *Runtime) guestCredentials() *selfdev.GuestCredentials {
	if rt.selfdevControl != nil {
		return rt.selfdevControl
	}
	return rt.ownerRecoveryControl
}

func (rt *Runtime) hibernateResumeCredential() hibernateResumeCredential {
	if rt.hibernate.resumeCredential != nil {
		return rt.hibernate.resumeCredential
	}
	if credentials := rt.guestCredentials(); credentials != nil && rt.eventAppender != nil {
		return credentials
	}
	return nil
}

// reconcileResumedCredential appends the installed envelope's consumption as
// key_revoked and switches to the post-revocation capability, as boot does.
func (rt *Runtime) reconcileResumedCredential(ctx context.Context) error {
	if rt.hibernate.reconcile != nil {
		return rt.hibernate.reconcile(ctx)
	}
	credentials := rt.guestCredentials()
	if credentials == nil || len(credentials.PendingLifecycleReceipts()) == 0 {
		return nil
	}
	if rt.eventAppender == nil {
		return fmt.Errorf("computer event appender unavailable")
	}
	return ReconcilePendingLifecycleReceipts(rt.eventAppender, credentials, credentials.ComputerID(), ctx)
}

// setSystemClock steps the wall clock (hibernate_clock_*.go); tests replace it.
var setSystemClock = stepSystemClock

// syncFilesystems flushes dirty pages before the snapshot; tests replace it.
var syncFilesystems = syncAllFilesystems

type hibernatePrepareResponse struct {
	Status              string    `json:"status,omitempty"`
	Outcome             string    `json:"outcome,omitempty"`
	Error               string    `json:"error,omitempty"`
	CapabilityExpiresAt time.Time `json:"capability_expires_at,omitempty"`
	GuestBuild          string    `json:"guest_build,omitempty"`
	// CredentialResume tells the host this guest installs a fresh envelope
	// delivered with resumed, so its snapshot is not bounded by the frozen
	// capability's renewal window.
	CredentialResume bool `json:"credential_resume,omitempty"`
	// Reason names a busy refusal that is not a run count: "planned_restart_pending"
	// (a fresh planned-restart marker is on disk; D2 S5).
	Reason           string `json:"reason,omitempty"`
	RunningRuns      int    `json:"running_runs,omitempty"`
	InflightWork     int    `json:"inflight_work,omitempty"`
	SelfdevActiveOps int    `json:"selfdev_active_operations,omitempty"`
}

type hibernateResumedRequest struct {
	HostUnixNano int64 `json:"host_unix_nano"`
	// CredentialEnvelope is a fresh bootstrap envelope for this realization
	// (base64url canonical JSON, as on the credential disk). Never logged.
	CredentialEnvelope string `json:"credential_envelope,omitempty"`
}

type hibernateResumedResponse struct {
	Status              string    `json:"status,omitempty"`
	Error               string    `json:"error,omitempty"`
	ClockStepMS         int64     `json:"clock_step_ms"`
	CapabilityExpiresAt time.Time `json:"capability_expires_at,omitempty"`
	// Credential is "installed" (delivered envelope) or "renewed".
	Credential string `json:"credential,omitempty"`
}

// HandleInternalHibernatePrepare serves POST /internal/runtime/hibernate/prepare.
// 200 {"status":"prepared"} means admission is closed, nothing is running and
// the filesystems are synced; the host may pause and snapshot. 409
// {"outcome":"busy"} means work is in flight; admission stays open and the
// host leaves the guest running or kills it on its own schedule. Host-peer
// only.
func (h *APIHandler) HandleInternalHibernatePrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "hibernate surface unavailable"})
		return
	}
	status, resp := h.rt.prepareHibernate(r.Context())
	writeAPIJSON(w, status, resp)
}

func (rt *Runtime) prepareHibernate(ctx context.Context) (int, hibernatePrepareResponse) {
	inflight, ok := rt.hibernate.close(hibernateFenceTTL)
	if !ok {
		// Work is inside, so the fence stayed open. The host's busy capture
		// (S5) trusts this reply's refusals, so it carries the same ones as
		// the closed-fence replies below: a pending marker, an open
		// self-development operation, or state that cannot be read.
		resp := hibernatePrepareResponse{
			Outcome: "busy", Error: "guest is busy; hibernate deferred",
			InflightWork: inflight,
		}
		if plannedRestartMarkerPending(rt.cfg.StorePath, rt.hibernate.clock()) {
			resp.Reason = "planned_restart_pending"
		} else if work, err := rt.plannedRestartBusy(ctx, false); err != nil {
			resp.Reason = "busy_state_unreadable"
		} else {
			resp.RunningRuns, resp.SelfdevActiveOps = work.Running, work.SelfdevActive
		}
		return http.StatusConflict, resp
	}
	// A planned-restart marker on disk would be captured in the snapshot, and
	// a resumed guest that crashed within the marker's window would boot as
	// planned and resume work (D2 S5, design 5.3). Refuse until it is gone.
	if plannedRestartMarkerPending(rt.cfg.StorePath, rt.hibernate.clock()) {
		rt.hibernate.lift()
		return http.StatusConflict, hibernatePrepareResponse{
			Outcome: "busy", Reason: "planned_restart_pending",
			Error: "a planned restart is pending; hibernate deferred",
		}
	}
	// Admission is closed; anything still running started outside the actor
	// boundary (or a self-development transaction is open). Either keeps the
	// guest awake.
	// A memory snapshot keeps capsules (they live in this process), so
	// hibernate does not count them.
	work, err := rt.plannedRestartBusy(ctx, false)
	if err != nil {
		rt.hibernate.lift()
		return http.StatusServiceUnavailable, hibernatePrepareResponse{Error: "hibernate: " + err.Error()}
	}
	running, selfdevActive := work.Running, work.SelfdevActive
	if running > 0 || selfdevActive > 0 {
		rt.hibernate.lift()
		return http.StatusConflict, hibernatePrepareResponse{
			Outcome: "busy", Error: "guest is busy; hibernate deferred",
			RunningRuns: running, SelfdevActiveOps: selfdevActive,
		}
	}
	resp := hibernatePrepareResponse{Status: "prepared", GuestBuild: buildinfo.Commit, CredentialResume: rt.hibernateResumeCredential() != nil}
	if credential := rt.hibernateCredential(); credential != nil {
		renewCtx, cancel := context.WithTimeout(ctx, hibernateRenewTimeout)
		expiresAt, err := credential.RenewNow(renewCtx)
		cancel()
		if err != nil {
			// A snapshot whose capability is already near expiry would wake
			// into a refused renewal; refuse the snapshot instead.
			rt.hibernate.lift()
			return http.StatusServiceUnavailable, hibernatePrepareResponse{Error: "hibernate: renew capability: " + err.Error()}
		}
		resp.CapabilityExpiresAt = expiresAt.UTC()
	}
	syncFilesystems()
	log.Printf("agentcore: hibernate fence closed; capability expires %s", resp.CapabilityExpiresAt.Format(time.RFC3339))
	return http.StatusOK, resp
}

// HandleInternalHibernateResumed serves POST /internal/runtime/hibernate/resumed,
// the host's word that this guest was loaded from its snapshot. Host-peer
// only. Any non-200 answer makes the host kill the guest and cold boot it.
func (h *APIHandler) HandleInternalHibernateResumed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "hibernate surface unavailable"})
		return
	}
	var request hibernateResumedRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.HostUnixNano <= 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid hibernate resumed request"})
		return
	}
	status, resp := h.rt.resumeFromHibernate(r.Context(), time.Unix(0, request.HostUnixNano), request.CredentialEnvelope)
	writeAPIJSON(w, status, resp)
}

func (rt *Runtime) resumeFromHibernate(ctx context.Context, hostNow time.Time, envelope string) (int, hibernateResumedResponse) {
	resp := hibernateResumedResponse{}
	step := hostNow.Sub(time.Now())
	if step >= hibernateClockStepThreshold || step <= -hibernateClockStepThreshold {
		if err := setSystemClock(hostNow); err != nil {
			// Admission stays closed: the host kills and cold boots a guest
			// whose clock it cannot trust.
			return http.StatusInternalServerError, hibernateResumedResponse{Error: "hibernate: step clock: " + err.Error()}
		}
		resp.ClockStepMS = step.Milliseconds()
	}
	// Pooled keep-alive connections to the host went stale while frozen.
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	if envelope != "" {
		// Re-credential (after the clock step: envelope and capability
		// expiry checks read the wall clock). The frozen capability may be
		// hours past expiry; the envelope is the proof, so it is not needed.
		// Any failure keeps admission closed; the host kills and cold boots.
		installer := rt.hibernateResumeCredential()
		if installer == nil {
			return http.StatusServiceUnavailable, hibernateResumedResponse{
				Error: "hibernate: this guest cannot install a delivered credential", ClockStepMS: resp.ClockStepMS,
			}
		}
		installCtx, cancel := context.WithTimeout(ctx, hibernateResumeCredentialTimeout)
		expiresAt, err := installer.InstallResumeEnvelope(installCtx, envelope)
		if err == nil {
			err = rt.reconcileResumedCredential(installCtx)
		}
		cancel()
		if err != nil {
			return http.StatusServiceUnavailable, hibernateResumedResponse{
				Error:       "hibernate: install delivered credential after resume: " + err.Error(),
				ClockStepMS: resp.ClockStepMS,
			}
		}
		resp.CapabilityExpiresAt = expiresAt.UTC()
		resp.Credential = "installed"
	} else if credential := rt.hibernateCredential(); credential != nil {
		renewCtx, cancel := context.WithTimeout(ctx, hibernateRenewTimeout)
		expiresAt, err := credential.RenewNow(renewCtx)
		cancel()
		if err != nil {
			return http.StatusServiceUnavailable, hibernateResumedResponse{
				Error:       "hibernate: renew capability after resume: " + err.Error(),
				ClockStepMS: resp.ClockStepMS,
			}
		}
		resp.CapabilityExpiresAt = expiresAt.UTC()
		resp.Credential = "renewed"
	}
	rt.hibernate.lift()
	resp.Status = "resumed"
	log.Printf("agentcore: resumed from hibernate snapshot; clock stepped %dms; credential %s; work admission open", resp.ClockStepMS, resp.Credential)
	return http.StatusOK, resp
}

// errHibernateClockUnsupported is returned where the platform cannot step
// the clock.
var errHibernateClockUnsupported = errors.New("stepping the system clock is unsupported on this platform")

func hibernateClockError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("settimeofday: %w", err)
}
