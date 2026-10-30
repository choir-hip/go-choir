package vmctl

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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Deploy refresh retry (CLAUDE.md "Restarts End Work (Crash) Or Resume It
// (Planned Update)": deploys leave busy computers "to their next idle
// moment"; docs/problems/always-busy-computer-never-updates-2026-10-15.md).
// Until this, a deferred computer waited for the next deploy. Every deferral
// is recorded here, and a vmctl sweep asks again until the computer takes the
// update. Bounds:
//   - a refreshed computer is forgotten: one refresh per deferral, so a
//     layered computer (which keeps serving its release commit) is never
//     refreshed again for the same target;
//   - a failed refresh is forgotten too (logged): retrying a refresh that
//     fails would be a kill loop; the next deploy records it again;
//   - held, divergent or vanished computers are forgotten, as the deploy
//     skips them; a computer stopped by idle, logout or an owner act boots the
//     current image when it next starts. A protected always-on computer
//     stopped by a failure or a vmctl restart is a rescue: it stays pending
//     while a live guest keeps it from booting, and the sweep boots it once
//     the wedge detector has ended that guest (deployRescue). The boot is one
//     per deploy target (GrantDeployRetry), so the sweep cannot loop it.

// DeployRefreshRetryInterval is how often the sweep asks deferred computers
// again. A guest refusal is cheap (a run count and two indexed lists), and a
// self-development apply that deferred a platform computer usually settles
// within minutes.
const DeployRefreshRetryInterval = time.Minute

// deployRefreshInFlightReason defers a second deploy refresh of a computer
// whose refresh is running; it is not recorded (the running one settles it).
const deployRefreshInFlightReason = "a deploy refresh of this computer is already in flight"

// deployRefreshPendingEntry is one deferred computer.
type deployRefreshPendingEntry struct {
	UserID          string    `json:"user_id"`
	DesktopID       string    `json:"desktop_id"`
	VMID            string    `json:"vm_id,omitempty"`
	ComputerID      string    `json:"computer_id,omitempty"`
	Target          string    `json:"target"`
	FirstDeferredAt time.Time `json:"first_deferred_at"`
	LastAttemptAt   time.Time `json:"last_attempt_at"`
	Attempts        int       `json:"attempts"`
	LastReason      string    `json:"last_reason,omitempty"`
	BusyAllowed     bool      `json:"busy_allowed,omitempty"`
	// Kind is "woke_behind" for a computer resumed from a snapshot onto a guest
	// release (D2 S4: resume, then update); empty for a deploy's deferral.
	// Release is the release it resumed onto. A deploy's refresh of the same
	// computer clears both (settle).
	Kind    string `json:"kind,omitempty"`
	Release string `json:"release,omitempty"`
}

// A computer still deferred after deployRefreshRetryBackoffAfter attempts
// (about half an hour at the one-minute sweep) is refusing for a reason that
// does not clear by itself, such as a held rollback awaiting an owner act. It
// is asked with a gap that doubles from two minutes up to
// deployRefreshRetryMaxGap, and never dropped: the act that clears the
// refusal still leads to an update.
const (
	deployRefreshRetryBackoffAfter = 30
	deployRefreshRetryMaxGap       = time.Hour
)

// deployRefreshRetryDue reports whether the sweep asks entry again at now.
func deployRefreshRetryDue(e deployRefreshPendingEntry, now time.Time) bool {
	if e.Attempts < deployRefreshRetryBackoffAfter || e.LastAttemptAt.IsZero() {
		return true
	}
	gap := deployRefreshRetryMaxGap
	if shift := e.Attempts - deployRefreshRetryBackoffAfter; shift < 6 {
		gap = min(2*time.Minute<<shift, deployRefreshRetryMaxGap)
	}
	return now.Sub(e.LastAttemptAt) >= gap
}

// deployRefreshKindWokeBehind marks the "resume, then update" entry. Only this
// kind waits for the user-idle gate.
const deployRefreshKindWokeBehind = "woke_behind"

// DefaultBehindRefreshIdle is how long a user must have been away before the
// sweep updates a computer that woke behind (VMCTL_BEHIND_REFRESH_IDLE).
const DefaultBehindRefreshIdle = 2 * time.Minute

// SetBehindRefresh configures the D2 "resume, then update" path: the user-idle
// gate (idle) and the clock (now; nil is time.Now), and registers the resume
// enroller on the registry. Called once from cmd/vmctl.
func (h *Handler) SetBehindRefresh(idle time.Duration, now func() time.Time) {
	if idle <= 0 {
		idle = DefaultBehindRefreshIdle
	}
	if now == nil {
		now = time.Now
	}
	h.behindMu.Lock()
	h.behindRefreshIdle, h.behindNow = idle, now
	h.behindMu.Unlock()
	h.registry.SetResumeEnroller(h.enrollResumed)
}

func (h *Handler) behindGate() (time.Duration, func() time.Time) {
	h.behindMu.Lock()
	defer h.behindMu.Unlock()
	idle, now := h.behindRefreshIdle, h.behindNow
	if idle <= 0 {
		idle = DefaultBehindRefreshIdle
	}
	if now == nil {
		now = time.Now
	}
	return idle, now
}

// enrollResumed adds a computer that resumed from a snapshot onto a guest
// release to the pending set. It does not depend on the validation-time
// "behind": a switch can repoint the current release between validation and
// the load, so the sweep resolves the current release itself. The target is
// only a placeholder (the pending loader drops an empty one). A computer that
// already has a deploy entry keeps it.
func (h *Handler) enrollResumed(own VMOwnership, release string) {
	if release == "" {
		return
	}
	_, commit := h.currentGuestRelease()
	target := commit
	if target == "" {
		target = release
	}
	_, now := h.behindGate()
	h.deployRefreshState().enroll(deployRefreshPendingEntry{
		UserID: own.UserID, DesktopID: normalizeDesktopID(own.DesktopID), VMID: own.VMID, ComputerID: own.ComputerID,
		Target: target, FirstDeferredAt: now().UTC(), LastAttemptAt: now().UTC(),
		LastReason: "resumed on release " + release, Kind: deployRefreshKindWokeBehind, Release: release,
	})
}

// currentGuestRelease asks the manager for the current release and its commit.
func (h *Handler) currentGuestRelease() (id, commit string) {
	reader, ok := h.registry.VMManagerHandle().(guestReleaseReader)
	if !ok || reader == nil {
		return "", ""
	}
	return reader.CurrentGuestReleaseInfo()
}

// instanceGuestRelease is the release the computer's live VM was launched
// from, "" when it is not running or launched legacy.
func (h *Handler) instanceGuestRelease(vmID string) string {
	reader, ok := h.registry.VMManagerHandle().(guestReleaseReader)
	if !ok || reader == nil {
		return ""
	}
	_, byVM := reader.GuestReleaseHealth()
	return byVM[vmID].GuestRelease
}

// enroll adds a woke_behind entry unless the computer is already pending.
func (t *deployRefreshTracker) enroll(entry deployRefreshPendingEntry) {
	key := ownershipKey(entry.UserID, entry.DesktopID)
	t.mu.Lock()
	defer t.mu.Unlock()
	if known, ok := t.pending[key]; ok {
		if known.Kind == deployRefreshKindWokeBehind {
			known.Release = entry.Release
			known.VMID, known.ComputerID = entry.VMID, entry.ComputerID
			t.pending[key] = known
			t.saveLocked()
		}
		return
	}
	t.pending[key] = entry
	t.saveLocked()
}

// setTarget moves a known entry to a newer target.
func (t *deployRefreshTracker) setTarget(userID, desktopID, target string) {
	key := ownershipKey(userID, normalizeDesktopID(desktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry, ok := t.pending[key]; ok && entry.Target != target {
		entry.Target = target
		t.pending[key] = entry
		t.saveLocked()
	}
}

type deployRefreshTracker struct {
	mu       sync.Mutex
	path     string
	pending  map[string]deployRefreshPendingEntry
	inFlight map[string]struct{}
}

func newDeployRefreshTracker() *deployRefreshTracker {
	return &deployRefreshTracker{pending: map[string]deployRefreshPendingEntry{}, inFlight: map[string]struct{}{}}
}

func (h *Handler) deployRefreshState() *deployRefreshTracker {
	h.deployRefreshMu.Lock()
	defer h.deployRefreshMu.Unlock()
	if h.deployRefreshTracker == nil {
		h.deployRefreshTracker = newDeployRefreshTracker()
	}
	return h.deployRefreshTracker
}

// SetDeployRefreshPendingPath makes the pending set durable at path and loads
// what an earlier vmctl left there. A missing file is an empty set.
func (h *Handler) SetDeployRefreshPendingPath(path string) error {
	t := h.deployRefreshState()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.path = strings.TrimSpace(path)
	if t.path == "" {
		return nil
	}
	raw, err := os.ReadFile(t.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deploy refresh pending: read %s: %w", t.path, err)
	}
	var entries []deployRefreshPendingEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("deploy refresh pending: decode %s: %w", t.path, err)
	}
	for _, entry := range entries {
		if entry.UserID == "" || entry.Target == "" {
			continue
		}
		entry.DesktopID = normalizeDesktopID(entry.DesktopID)
		t.pending[ownershipKey(entry.UserID, entry.DesktopID)] = entry
	}
	return nil
}

// deployRefreshInFlightFor reports whether a deploy refresh of the computer is
// running now.
func (h *Handler) deployRefreshInFlightFor(userID, desktopID string) bool {
	t := h.deployRefreshState()
	key := ownershipKey(userID, normalizeDesktopID(desktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	_, busy := t.inFlight[key]
	return busy
}

// begin takes the computer's in-flight slot; ok is false while another deploy
// refresh of it runs.
func (t *deployRefreshTracker) begin(userID, desktopID string) (release func(), ok bool) {
	key := ownershipKey(userID, normalizeDesktopID(desktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, busy := t.inFlight[key]; busy {
		return func() {}, false
	}
	t.inFlight[key] = struct{}{}
	return func() {
		t.mu.Lock()
		delete(t.inFlight, key)
		t.mu.Unlock()
	}, true
}

// settle records a deploy refresh's outcome: deferred keeps (or adds) the
// computer, anything else forgets it.
func (t *deployRefreshTracker) settle(req deployRefreshRequest, resp deployRefreshResponse, now time.Time, keepKind bool) {
	key := ownershipKey(req.UserID, normalizeDesktopID(req.DesktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, known := t.pending[key]
	if resp.Outcome != deployRefreshDeferred {
		if known {
			delete(t.pending, key)
			t.saveLocked()
		}
		return
	}
	if !known {
		entry = deployRefreshPendingEntry{UserID: req.UserID, DesktopID: normalizeDesktopID(req.DesktopID), FirstDeferredAt: now}
	}
	// A deploy's refresh of a computer that woke behind replaces the entry:
	// its kind (and so the user-idle gate) goes; only the retry sweep keeps it.
	if !keepKind {
		entry.Kind, entry.Release = "", ""
	}
	entry.Target = req.Target
	entry.VMID, entry.ComputerID = resp.VMID, resp.ComputerID
	entry.LastAttemptAt, entry.LastReason, entry.BusyAllowed = now, resp.Reason, resp.BusyAllowed
	entry.Attempts++
	if entry.Attempts == deployRefreshRetryBackoffAfter {
		log.Printf("vmctl: deploy refresh of %s to %s still deferred after %d attempts since %s: %q; asking with a growing gap up to %s",
			entry.VMID, entry.Target, entry.Attempts, entry.FirstDeferredAt.Format(time.RFC3339), entry.LastReason, deployRefreshRetryMaxGap)
	}
	t.pending[key] = entry
	t.saveLocked()
}

func (t *deployRefreshTracker) forget(userID, desktopID string) {
	key := ownershipKey(userID, normalizeDesktopID(desktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, known := t.pending[key]; known {
		delete(t.pending, key)
		t.saveLocked()
	}
}

func (t *deployRefreshTracker) list() []deployRefreshPendingEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]deployRefreshPendingEntry, 0, len(t.pending))
	for _, entry := range t.pending {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].FirstDeferredAt.Equal(out[j].FirstDeferredAt) {
			return out[i].FirstDeferredAt.Before(out[j].FirstDeferredAt)
		}
		return ownershipKey(out[i].UserID, out[i].DesktopID) < ownershipKey(out[j].UserID, out[j].DesktopID)
	})
	return out
}

// saveLocked writes the pending set atomically. A write failure is logged:
// the in-memory set still drives the sweep, and the next deploy re-records
// every deferral.
func (t *deployRefreshTracker) saveLocked() {
	if t.path == "" {
		return
	}
	entries := make([]deployRefreshPendingEntry, 0, len(t.pending))
	for _, entry := range t.pending {
		entries = append(entries, entry)
	}
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err == nil {
		err = writeFileDurably(t.path, raw)
	}
	if err != nil {
		log.Printf("vmctl: deploy refresh pending: save %s: %v", t.path, err)
	}
}

func writeFileDurably(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (h *Handler) pendingDeployRefreshes() []deployRefreshPendingEntry {
	return h.deployRefreshState().list()
}

// trackedDeployRefresh runs one deploy refresh and records its outcome in the
// pending set. An "already in flight" deferral is not recorded: the refresh
// in flight settles the computer.
func (h *Handler) trackedDeployRefresh(ctx context.Context, req deployRefreshRequest, caller string) (deployRefreshResponse, int) {
	resp, status := h.deployRefresh(ctx, req)
	log.Printf("vmctl: deploy refresh caller=%s user=%s desktop=%s vm=%s target=%s outcome=%s planned_restart=%v busy_allowed=%v reason=%q",
		caller, req.UserID, req.DesktopID, resp.VMID, req.Target, resp.Outcome, resp.PlannedRestart, resp.BusyAllowed, resp.Reason)
	if resp.Outcome == deployRefreshDeferred && resp.Reason == deployRefreshInFlightReason {
		return resp, status
	}
	if resp.Outcome == deployRefreshFailed {
		log.Printf("vmctl: deploy refresh of %s failed; not retried until the next deploy: %s", resp.VMID, resp.Reason)
	}
	h.deployRefreshState().settle(req, resp, time.Now().UTC(), caller == "retry-sweep")
	return resp, status
}

// SweepDeployRefreshPending asks every deferred computer again, one at a
// time.
func (h *Handler) SweepDeployRefreshPending(ctx context.Context) {
	// A planned host stop is hibernating computers: no refresh may mark a
	// guest (a marker would be captured) or boot a rescue (S5).
	if h.registry.hostStopping.Load() {
		return
	}
	for _, entry := range h.pendingDeployRefreshes() {
		if ctx.Err() != nil {
			return
		}
		own := h.registry.GetOwnershipForDesktop(entry.UserID, entry.DesktopID)
		if own == nil {
			log.Printf("vmctl: deploy refresh retry: user=%s desktop=%s has no computer; forgotten", entry.UserID, entry.DesktopID)
			h.deployRefreshState().forget(entry.UserID, entry.DesktopID)
			continue
		}
		if !deployRefreshRetryDue(entry, time.Now().UTC()) {
			continue
		}
		eligible := !own.IsHeld() && own.State == VMStateActive && own.DivergenceStatus != "divergent"
		// A protected computer stopped by a failure or a vmctl restart is a
		// rescue: it stays pending until the sweep boots it on the target.
		rescue := h.registry.deployRescueEligible(own)
		if eligible || rescue {
			if err := h.requireComputerVersionRoute(ctx, entry.UserID, entry.DesktopID); err != nil {
				log.Printf("vmctl: deploy refresh retry: user=%s desktop=%s route refused, kept: %v", entry.UserID, entry.DesktopID, err)
				continue
			}
		}
		// A computer that woke behind: resolve the current release now (a
		// switch may have moved it since validation), forget one that is not
		// behind, and aim the refresh at the current release's commit.
		if entry.Kind == deployRefreshKindWokeBehind && eligible {
			currentID, commit := h.currentGuestRelease()
			if currentID != "" {
				if running := h.instanceGuestRelease(own.VMID); running != "" && running == currentID {
					log.Printf("vmctl: deploy refresh retry: %s runs the current release %s; forgotten", own.VMID, currentID)
					h.deployRefreshState().forget(entry.UserID, entry.DesktopID)
					continue
				}
				if commit != "" && commit != entry.Target {
					entry.Target = commit
					h.deployRefreshState().setTarget(entry.UserID, entry.DesktopID, commit)
				}
			}
		}
		if eligible {
			if served := h.servedCommit(ctx, *own); served != "" && served == entry.Target {
				log.Printf("vmctl: deploy refresh retry: %s already serves %s; forgotten", own.VMID, entry.Target)
				h.deployRefreshState().forget(entry.UserID, entry.DesktopID)
				continue
			}
		}
		// The user-idle gate holds only the "resume, then update" entry: a
		// person who has just woken their computer would otherwise get a cold
		// boot within a minute. A deploy's refresh is never held by it.
		if entry.Kind == deployRefreshKindWokeBehind && eligible {
			idle, now := h.behindGate()
			if !own.LastActiveAt.IsZero() && now().Sub(own.LastActiveAt) < idle {
				continue
			}
		}
		// Ineligible computers go through the same path, which answers
		// "skipped" without contacting the guest and forgets them.
		h.trackedDeployRefresh(ctx, deployRefreshRequest{UserID: entry.UserID, DesktopID: entry.DesktopID, Target: entry.Target}, "retry-sweep")
	}
}

// StartDeployRefreshRetry runs the sweep every interval until ctx ends.
func (h *Handler) StartDeployRefreshRetry(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DeployRefreshRetryInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.SweepDeployRefreshPending(ctx)
			}
		}
	}()
}

func (h *Handler) servedCommit(ctx context.Context, own VMOwnership) string {
	if h.guestServedCommit != nil {
		return h.guestServedCommit(ctx, own)
	}
	return httpGuestServedCommit(ctx, own)
}

// httpGuestServedCommit reads build.commit from the guest's /health (what CI
// compares after a refresh). Any failure answers "", which never matches a
// target.
func httpGuestServedCommit(ctx context.Context, own VMOwnership) string {
	base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
	if base == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return ""
	}
	hostauth.MarkInternal(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var health struct {
		Build struct {
			Commit string `json:"commit"`
		} `json:"build"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&health); err != nil {
		return ""
	}
	return strings.TrimSpace(health.Build.Commit)
}
