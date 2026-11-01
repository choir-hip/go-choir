package vmctl

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

// Guest memory caps (density D6 S6; owner, 2026-11-01: "More disk reads,
// less memory usage ... this should significantly increase density"):
//
//	VMCTL_GUEST_MEMORY_CAP_COMPUTERS  "id=MiB,..." (VM or computer id): the
//	                                  guest keeps MiB of its base memory and
//	                                  the balloon holds the rest
//	VMCTL_GUEST_MEMORY_CAP_INTERVAL   reconcile interval, default 1m; 0 stops
//
// Every pass reconciles every active computer: a named one to its cap, any
// other to no cap (amount 0), so dropping an entry deflates that guest at the
// next pass. The device's deflate_on_oom lets a guest that runs out of memory
// take balloon pages back. Rollback: drop the entry, or the variable.

// DefaultGuestMemoryCapInterval is the reconcile interval when unset.
const DefaultGuestMemoryCapInterval = time.Minute

// guestMemoryCapSetter is implemented by the Firecracker manager adapter.
type guestMemoryCapSetter interface {
	SetGuestMemoryCap(vmID string, capMiB int) (GuestMemoryCapResult, error)
}

// GuestMemoryCapResult mirrors vmmanager.BalloonTargetResult.
type GuestMemoryCapResult struct {
	BaseMiB   int
	BeforeMiB int
	AfterMiB  int
	Changed   bool
}

// ParseGuestMemoryCaps reads "id=MiB,..."; malformed or non-positive entries
// are dropped and named in the second result.
func ParseGuestMemoryCaps(raw string) (map[string]int, []string) {
	var out map[string]int
	var bad []string
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		id, v, ok := strings.Cut(tok, "=")
		id = strings.TrimSpace(id)
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if !ok || id == "" || err != nil || n <= 0 {
			bad = append(bad, tok)
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[id] = n
	}
	return out, bad
}

// guestMemoryCapFor is the computer's cap (by VM id, then computer id), or 0.
func guestMemoryCapFor(caps map[string]int, own *VMOwnership) int {
	if n, ok := caps[own.VMID]; ok {
		return n
	}
	if own.ComputerID != "" {
		return caps[own.ComputerID]
	}
	return 0
}

// StartGuestMemoryCapLoop reconciles balloon targets every interval.
func (r *OwnershipRegistry) StartGuestMemoryCapLoop(ctx context.Context, interval time.Duration, caps map[string]int) {
	if interval <= 0 {
		return
	}
	go func() {
		r.reconcileGuestMemoryCaps(caps)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.reconcileGuestMemoryCaps(caps)
			}
		}
	}()
}

// reconcileGuestMemoryCaps sets each active computer's balloon to its cap
// and logs each change and each failure on a named computer. It returns how
// many balloons it changed.
func (r *OwnershipRegistry) reconcileGuestMemoryCaps(caps map[string]int) int {
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	setter, ok := mgr.(guestMemoryCapSetter)
	if !ok || setter == nil {
		return 0
	}
	changed := 0
	seen := map[string]bool{}
	for _, own := range r.ListOwnerships() {
		if own == nil || own.VMID == "" || own.State != VMStateActive || seen[own.VMID] {
			continue
		}
		seen[own.VMID] = true
		capMiB := guestMemoryCapFor(caps, own)
		res, err := setter.SetGuestMemoryCap(own.VMID, capMiB)
		if err != nil {
			if capMiB > 0 {
				log.Printf("vmctl: guest memory cap computer=%s vm=%s cap_mib=%d skipped: %v", own.ComputerID, own.VMID, capMiB, err)
			}
			continue
		}
		if res.Changed {
			changed++
			log.Printf("vmctl: guest memory cap computer=%s vm=%s cap_mib=%d base_mib=%d balloon_mib %d -> %d",
				own.ComputerID, own.VMID, capMiB, res.BaseMiB, res.BeforeMiB, res.AfterMiB)
		}
	}
	return changed
}
