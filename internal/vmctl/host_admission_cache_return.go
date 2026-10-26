package vmctl

import (
	"context"
	"time"

	"github.com/yusefmosiah/go-choir/internal/rssfall"
)

// Density D6 S4: step 0 of make-room
// (docs/design/density-d6-page-cache-return-2026-10-26.md section 4.4).
//
// A start refused for host memory pressure first asks idle running
// computers to return their cold page cache, before D1's victim plan
// hibernates anyone: returned cache is memory the host gets back without a
// computer stopping. Inside D1's makeRoomMu slot, one deadline
// (VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET, 12 s) covers the calls, the
// settle waits and the re-samples; the start's own context also ends it.
//
// Pending return is waited for and never counted (D1's M17): the only input
// to an admission is a fresh MemAvailable sample taken after the host has
// seen the computer's RSS fall. A guest's reclaimed bytes only size the
// settle wait. Whatever step 0 does not cover, D1's plan covers, unchanged,
// on its own fresh sample.

// CacheReturnedVM is one computer step 0 asked, with the fall of its
// Firecracker RSS as the host measured it (0 when the act did nothing the
// host could see).
type CacheReturnedVM struct {
	VMID       string `json:"vm_id"`
	RSSFallMiB uint64 `json:"rss_fall_mib"`
}

// makeRoomCacheReturn is step 0. It returns a ticket when a fresh sample
// admits the start after some computer's act, the latest decision it
// sampled (and whether it sampled at all), and the computers it asked. A
// start whose context ends stops it at once.
func (r *OwnershipRegistry) makeRoomCacheReturn(ctx context.Context, starter *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, acfg HostAdmissionConfig, sampler hostPressureSampler, gcfg GuestCacheReturnConfig) (*admissionTicket, HostAdmissionDecision, bool, []CacheReturnedVM) {
	var last HostAdmissionDecision
	if gcfg.Mode != GuestCacheReturnActive || len(gcfg.Only) == 0 || !gcfg.TriggerOn("pressure") {
		return nil, last, false, nil
	}
	budget := gcfg.PressureBudget
	if budget <= 0 {
		budget = DefaultGuestCacheReturnConfig().PressureBudget
	}
	deadline := time.Now().Add(budget)
	stepCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	st := &r.cacheReturn
	now := r.now()
	snap := r.cacheReturnSnapshot()
	var starterVM string
	if starter != nil {
		starterVM = starter.VMID
	}
	// The host's own measurements rank the candidates: the latest sampled
	// RSS less D4's settled RSS. No settled RSS (or no sample) is
	// unmeasured: it ranks after every measured computer.
	latest := r.latestVMResidentBytes()
	excessOf := func(own *VMOwnership) (excess uint64, measured bool) {
		rss, ok := latest[own.VMID]
		if !ok || own.MemorySettledRSSBytes == 0 {
			return 0, false
		}
		if rss > own.MemorySettledRSSBytes {
			excess = rss - own.MemorySettledRSSBytes
		}
		return excess, true
	}
	picks := cacheReturnCandidates(cacheReturnCandidateInput{
		Now: now, Config: gcfg, Policy: snap.policy, SweepHibernates: snap.sweepHibernates,
		Ownerships: snap.owns, IdleFloor: gcfg.PressureIdle, Limit: cacheReturnPressureMaxCandidates, Pressure: true,
		Claimed: func(own *VMOwnership) bool {
			return snap.claimed[own.VMID] || own.VMID == starterVM
		},
		Uncaptured:  r.cacheReturnUncaptured(now, snap.acfg),
		InFlight:    st.isInFlight,
		LastAct:     st.lastActAt,
		Unsupported: st.isUnsupported,
		Busy:        r.guestBusy(),
		Extra: func(own *VMOwnership) bool {
			excess, measured := excessOf(own)
			return !measured || excess >= cacheReturnPressureMinExcess
		},
		Order: func(a, b *VMOwnership) bool {
			ea, ma := excessOf(a)
			eb, mb := excessOf(b)
			if ma != mb {
				return ma
			}
			return ea > eb
		},
	})

	var acted []CacheReturnedVM
	attempted := false
	for _, own := range picks {
		remaining := time.Until(deadline)
		// An act with a quarter of the budget or less left would be cut off
		// before the host could see its fall.
		if stepCtx.Err() != nil || remaining < budget/4 {
			break
		}
		if !st.begin(own.VMID, now) {
			continue
		}
		_, fall := r.doGuestCacheReturn(stepCtx, own, "pressure", gcfg, min(rssfall.DefaultCap, max(time.Until(deadline), 0)))
		st.end(own.VMID)
		acted = append(acted, CacheReturnedVM{VMID: own.VMID, RSSFallMiB: fall >> 20})
		if ctx.Err() != nil {
			break // the start went away: admit nobody, hibernate nobody
		}
		d, ticket := r.attemptMeasuredAdmission(starter, class, cfg, acfg, sampler, false)
		attempted, last = true, d
		if ticket != nil {
			return ticket, d, true, acted
		}
	}
	return nil, last, attempted, acted
}
