package vmctl

import (
	"log"
	"sync"
	"time"
)

// A vmctl that the deploy is about to replace starts no refresh.
//
// The deploy moves the guest-release link inside its nixos switch
// (nix/node-b.nix, go-choir-guest-image activation) and restarts vmctl only
// later (.github/scripts/node-b-ci-deploy, "vmctl restart"; the unit has
// restartIfChanged=false). A vmctl that was running when the link moved is
// therefore a vmctl the deploy replaces next. Its one-minute ticker would
// walk rings and sweep refreshes toward the new release, and a refresh it
// began is closed by its replacement as interrupted-by-vmctl-restart, after
// the planned-restart marker was spent on it: the computer then takes a crash
// boot (docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md,
// 2026-11-18, the paper's 51 minutes).
//
// The vmctl decides by itself, from what it already reads each tick: the
// release it started on (PrimeRingWalk takes it before the first boot) against
// the release the link names now. A link that moved under a running vmctl
// holds TickRingWalk and SweepDeployRefreshPending; nothing else, so the
// deploy's own requests (plan, expedite, refresh) and every boot and wake are
// unchanged. The replacement vmctl starts on the moved link, takes it as its
// baseline and walks as before. No marker is written, none is re-issued, and
// nothing resumes: this only declines to start a refresh.
//
// The hold cannot outlast the deploy: it ends when the link returns (a
// rollback), when this vmctl is replaced, or at releaseHoldBound, the deploy
// job's own timeout (ci.yml deploy-staging, 60 minutes), when the moved link
// becomes the baseline and the line says so. A link moved by something other
// than a deploy that restarts vmctl is therefore held for at most that long.
const releaseHoldBound = 60 * time.Minute

type releaseHold struct {
	mu       sync.Mutex
	baseline ringReleasePair
	have     bool
	since    time.Time // zero unless holding
	now      func() time.Time
}

func (r *releaseHold) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// adoptCurrentRelease takes the release the link names now as the one this
// vmctl started on, and ends any hold.
func (h *Handler) adoptCurrentRelease() {
	if _, ok := h.registry.VMManagerHandle().(guestReleaseReader); !ok {
		return
	}
	pair := h.currentReleasePair()
	r := &h.replacement
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseline, r.have, r.since = pair, true, time.Time{}
}

// standDownForReplacement reports whether the walk and the sweep must not
// start a refresh now. It logs when a hold starts and when it ends, not each
// tick.
func (h *Handler) standDownForReplacement() bool {
	if _, ok := h.registry.VMManagerHandle().(guestReleaseReader); !ok {
		return false
	}
	pair := h.currentReleasePair()
	r := &h.replacement
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock()
	if !r.have {
		r.baseline, r.have = pair, true
		return false
	}
	if pair.sameIDs(r.baseline) {
		if !r.since.IsZero() {
			log.Printf("vmctl: refresh hold: ended after %s: the guest release link is back at %q, this vmctl's release",
				now.Sub(r.since).Round(time.Second), r.baseline.Single)
			r.since = time.Time{}
		}
		return false
	}
	if r.since.IsZero() {
		r.since = now
		log.Printf("vmctl: refresh hold: started: the guest release link moved from %q to %q while this vmctl runs; the deploy restarts vmctl next and no refresh starts before it does (at most %s)",
			r.baseline.Single, pair.Single, releaseHoldBound)
		return true
	}
	if now.Sub(r.since) >= releaseHoldBound {
		log.Printf("vmctl: refresh hold: ended after %s without a restart: taking %q as this vmctl's release; refreshes proceed",
			now.Sub(r.since).Round(time.Second), pair.Single)
		r.baseline, r.since = pair, time.Time{}
		return false
	}
	return true
}
