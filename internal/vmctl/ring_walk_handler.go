package vmctl

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// The ring walk's vmctl side (docs/design/ring-ordered-deploys-2026-11-03.md):
// release observation, the per-tick reading of every computer, the guest
// health guards, the pending-set actions, the manager's pin and the status
// endpoint. The walk's own state machine is ring_walk.go.

// ringReleaseReader is the manager's per-computer release view; a manager
// without it walks the single image only.
type ringReleaseReader interface {
	LayeredGuestReleaseInfo() (id, commit string)
	GuestReleaseInfoFor(computerID string) (id, commit string)
	GuestLayeredComputer(computerID string) bool
}

// ringR0EnrollGrace: the CI deploy refreshes R0 itself right after the link
// moves; the walk enrolls an R0 computer the deploy did not reach only after
// this long, so d@e is not refreshed twice for one deploy.
// Strictly below R0's 10-minute floor, so CI's refresh lands before the walk
// would enroll d@e itself.
const ringR0EnrollGrace = 7 * time.Minute

func (h *Handler) ringWalkRunner() *ringWalkRunner {
	h.deployRefreshMu.Lock()
	defer h.deployRefreshMu.Unlock()
	if h.ringWalk == nil {
		h.ringWalk = newRingWalkRunner()
	}
	return h.ringWalk
}

// SetRingWalkPath makes the walk durable at statePath and its history at
// historyPath, loading what an earlier vmctl left.
func (h *Handler) SetRingWalkPath(statePath, historyPath string) error {
	return h.ringWalkRunner().load(statePath, historyPath)
}

// RingWalkPinner is the manager's release pinner (vmmanager.ReleasePinner).
type RingWalkPinner struct{ r *ringWalkRunner }

func (p RingWalkPinner) Pin(computerID string) (string, string) { return p.r.pin(computerID) }
func (p RingWalkPinner) Pinned() []string                       { return p.r.pinnedReleases() }

// RingWalkPinner returns the pinner to install on the manager.
func (h *Handler) RingWalkPinner() RingWalkPinner { return RingWalkPinner{r: h.ringWalkRunner()} }

func (h *Handler) ringReader() ringReleaseReader {
	reader, _ := h.registry.VMManagerHandle().(ringReleaseReader)
	return reader
}

// currentReleasePair is the current single and layered release.
func (h *Handler) currentReleasePair() ringReleasePair {
	var pair ringReleasePair
	pair.Single, pair.SingleCommit = h.currentGuestRelease()
	if reader := h.ringReader(); reader != nil {
		pair.Layered, pair.LayeredCommit = reader.LayeredGuestReleaseInfo()
	}
	return pair
}

// syncRingWalk starts, supersedes or ends a walk when the current release
// moved. Cheap: a readlink and a release read.
func (h *Handler) syncRingWalk() {
	if _, ok := h.registry.VMManagerHandle().(guestReleaseReader); !ok {
		return
	}
	h.ringWalkRunner().observe(h.currentReleasePair(), time.Now().UTC())
}

// guestReleaseFor is the release a cold boot of computerID launches now and
// its commit; without the per-computer view, the current release.
func (h *Handler) guestReleaseFor(computerID string) (id, commit string) {
	if reader := h.ringReader(); reader != nil && computerID != "" {
		return reader.GuestReleaseInfoFor(computerID)
	}
	return h.currentGuestRelease()
}

func (h *Handler) layeredComputer(computerID string) bool {
	if reader := h.ringReader(); reader != nil {
		return reader.GuestLayeredComputer(computerID)
	}
	return false
}

// ringGate answers "" when a deploy refresh of the active computer own to
// target may run now, else why the ring walk holds it.
func (h *Handler) ringGate(own *VMOwnership, target string) string {
	id := stableComputerID(own.UserID, own.DesktopID, own.ComputerID)
	return h.ringWalkRunner().gate(id, h.registry.WarmnessClassForOwnership(own), target, h.layeredComputer(id))
}

// PrimeRingWalk reads the release and fills the ring cache before the first
// boot (cmd/vmctl calls it before reattach).
func (h *Handler) PrimeRingWalk() {
	h.syncRingWalk()
	obs := h.ringObservations(context.Background(), false)
	r := h.ringWalkRunner()
	r.mu.Lock()
	for _, o := range obs {
		r.rings[o.ComputerID] = o.Ring
	}
	r.mu.Unlock()
}

// ringObservations reads every interactive computer; with sample, the open
// ring's members running the target have their /health read.
func (h *Handler) ringObservations(ctx context.Context, sample bool) []ringMemberObs {
	sampleRing, to, walking := h.ringWalkRunner().sampleTargets()
	byVM := map[string]GuestReleaseVMStatus{}
	if reader, ok := h.registry.VMManagerHandle().(guestReleaseReader); ok && reader != nil {
		_, byVM = reader.GuestReleaseHealth()
	}
	var obs []ringMemberObs
	var probes []int
	owners := map[int]VMOwnership{}
	for _, own := range h.registry.ListOwnerships() {
		if own.Kind != "" && own.Kind != VMKindInteractive {
			continue
		}
		id := stableComputerID(own.UserID, own.DesktopID, own.ComputerID)
		o := ringMemberObs{
			ComputerID: id, UserID: own.UserID, DesktopID: normalizeDesktopID(own.DesktopID), VMID: own.VMID,
			Ring:      ringForComputer(id, h.registry.WarmnessClassForOwnership(own)),
			Active:    own.State == VMStateActive,
			Held:      own.IsHeld(),
			Divergent: own.DivergenceStatus == "divergent",
			Layered:   h.layeredComputer(id),
			Crashes:   append([]CrashRecord(nil), own.Crashes...),
		}
		if o.Active {
			o.Release = byVM[own.VMID].GuestRelease
		}
		o.Pending, o.FailedTarget = h.deployRefreshState().pendingAndFailed(own.UserID, own.DesktopID)
		o.Epoch = own.Epoch
		if sample && walking && sampleRing(o.Ring) && o.Active && o.Release != "" && o.Release == to.releaseFor(o.Layered) {
			probes = append(probes, len(obs))
			owners[len(obs)] = *own
		}
		obs = append(obs, o)
	}
	h.probeRingHealthAll(ctx, obs, probes, owners)
	return obs
}

// Health probes run in parallel, at most ringProbeParallel at once, under
// one per-tick deadline, so a few hung guests cannot stall the tick and the
// retry sweep after it. A probe cut off by the deadline answers "no answer",
// which counts one readiness strike (three in a row breach).
const (
	ringProbeParallel = 8
	ringProbeDeadline = 25 * time.Second
)

func (h *Handler) probeRingHealthAll(ctx context.Context, obs []ringMemberObs, probes []int, owners map[int]VMOwnership) {
	if len(probes) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, ringProbeDeadline)
	defer cancel()
	sem := make(chan struct{}, ringProbeParallel)
	results := make([]ringHealth, len(probes))
	var wg sync.WaitGroup
	for i, idx := range probes {
		wg.Add(1)
		go func(i int, own VMOwnership) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = ringHealth{Detail: "no probe slot before the tick deadline"}
				return
			}
			results[i] = h.probeRingHealth(ctx, own)
		}(i, owners[idx])
	}
	wg.Wait()
	for i, idx := range probes {
		health := results[i]
		obs[idx].Health = &health
	}
}

func (h *Handler) probeRingHealth(ctx context.Context, own VMOwnership) ringHealth {
	if h.ringHealthProbe != nil {
		return h.ringHealthProbe(ctx, own)
	}
	return httpRingHealth(ctx, own)
}

// httpRingHealth reads the guest's /health over the host-peer path (the same
// call CI and httpGuestServedCommit make; MarkInternal never sends the
// internal token to a guest port).
func httpRingHealth(ctx context.Context, own VMOwnership) ringHealth {
	base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
	if base == "" {
		return ringHealth{Detail: "computer has no URL"}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return ringHealth{Detail: err.Error()}
	}
	hostauth.MarkInternal(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ringHealth{Detail: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	return parseRingHealth(resp.StatusCode, body)
}

// TickRingWalk runs one walk tick: observe the release, read every computer,
// step the walk and apply its pending-set actions. The retry sweep that
// follows performs the refreshes.
func (h *Handler) TickRingWalk(ctx context.Context) {
	if _, ok := h.registry.VMManagerHandle().(guestReleaseReader); !ok {
		return
	}
	h.syncRingWalk()
	obs := h.ringObservations(ctx, true)
	now := time.Now().UTC()
	actions := h.ringWalkRunner().stepWith(now, obs)
	h.applyRingActions(now, actions)
}

func (h *Handler) applyRingActions(now time.Time, actions []ringAction) {
	t := h.deployRefreshState()
	for _, a := range actions {
		switch a.Kind {
		case ringActionEnroll:
			t.ensure(deployRefreshPendingEntry{
				UserID: a.UserID, DesktopID: a.DesktopID, VMID: a.VMID, ComputerID: a.ComputerID,
				Target: a.Target, FirstDeferredAt: now, LastReason: a.Reason,
			})
		case ringActionForget:
			t.forget(a.UserID, a.DesktopID)
		case ringActionWake:
			h.ringWake(a)
		}
	}
}

// ringWake starts a down R0 computer the way a request for it would (route
// check, then resolve of the existing computer, naming the VM the walk
// observed: an empty expected VM is refused as an ownership change). A refusal (held, crash loop,
// host memory) is logged; R0 then holds and says why.
func (h *Handler) ringWake(a ringAction) {
	if h.ringWaker != nil {
		h.ringWaker(a)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := h.requireComputerVersionRoute(ctx, a.UserID, a.DesktopID); err != nil {
		log.Printf("vmctl: ring walk: wake of %s refused by the route check: %v", a.ComputerID, err)
		return
	}
	if _, err := h.registry.resolveExistingDesktopContext(ctx, a.UserID, a.DesktopID, a.VMID); err != nil {
		log.Printf("vmctl: ring walk: wake of %s failed: %v", a.ComputerID, err)
		return
	}
	log.Printf("vmctl: ring walk: woke %s (%s)", a.ComputerID, a.Reason)
}

// ensure puts entry in the pending set aimed at its target: a new entry, or a
// known one moved to the target (attempts reset, as for a new deploy; a
// woke_behind entry keeps its kind and user-idle gate). An entry already
// aimed at the target is kept.
func (t *deployRefreshTracker) ensure(entry deployRefreshPendingEntry) {
	entry.DesktopID = normalizeDesktopID(entry.DesktopID)
	key := ownershipKey(entry.UserID, entry.DesktopID)
	t.mu.Lock()
	defer t.mu.Unlock()
	known, ok := t.pending[key]
	if ok && known.Target == entry.Target {
		return
	}
	if ok && known.Kind == deployRefreshKindWokeBehind {
		// A computer that woke behind keeps its kind, and so the user-idle
		// gate: the sweep aims it at the computer's own release.
		known.Target = entry.Target
		t.pending[key] = known
		t.saveLocked()
		return
	}
	t.pending[key] = entry
	t.saveLocked()
}

// HandleRingWalkExpedite serves POST /internal/vmctl/ring-walk/expedite
// {"reason", "by"}: the owner or operator act (and the CI trailer path) that
// opens every ring of the walking release at once. Refresh semantics, the
// guest's refusals and the guards are unchanged.
func (h *Handler) HandleRingWalkExpedite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
		By     string `json:"by"`
		// Release binds the act to a release (its commit or id): an
		// expedite never lands on another walk. Optional for an owner act.
		Release string `json:"release"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	req.Reason, req.By = strings.TrimSpace(req.Reason), strings.TrimSpace(req.By)
	if req.Reason == "" || req.By == "" || len(req.Reason) > 300 || len(req.By) > 120 {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "reason (at most 300 characters) and by (at most 120) are required"})
		return
	}
	if caller := strings.TrimSpace(r.Header.Get("X-Choir-Lifecycle-Caller")); caller != "" && len(caller) <= 80 {
		req.By += " (caller " + caller + ")"
	}
	req.Reason, req.By, req.Release = ringPrintable(req.Reason, 300), ringPrintable(req.By, 200), ringPrintable(strings.TrimSpace(req.Release), 80)
	// The deploy's new release starts its walk here if no tick has yet.
	h.syncRingWalk()
	summary, err := h.ringWalkRunner().expedite(time.Now().UTC(), req.Reason, req.By, req.Release)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, summary)
}

// HandleRingWalk serves GET /internal/vmctl/ring-walk: the walk's state and
// the last history lines (?tail=N, default 200).
func (h *Handler) HandleRingWalk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	tail := 200
	if v := strings.TrimSpace(r.URL.Query().Get("tail")); v != "" {
		var n int
		for _, c := range v {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
			if n > 100000 {
				break
			}
		}
		if n > 0 {
			tail = n
		}
	}
	writeVMCTLJSON(w, http.StatusOK, h.ringWalkRunner().status(tail))
}

// ringPrintable keeps printable characters (no control characters in the
// history or the logs) and caps the length.
func ringPrintable(v string, max int) string {
	out := make([]rune, 0, len(v))
	for _, c := range v {
		if c >= 0x20 && c != 0x7f {
			out = append(out, c)
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return string(out)
}
