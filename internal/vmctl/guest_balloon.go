package vmctl

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Guest memory ceilings (density D6 S6; owner, 2026-11-01: "More disk reads,
// less memory usage", then "reduce real memory usage while maximizing
// flexibility for bursts/spikes"):
//
//	VMCTL_GUEST_MEMORY_CAP_COMPUTERS  "id=MiB,..." (VM id, computer id, or
//	                                  class:<warmness class>): the guest's
//	                                  elastic ceiling; an id wins over a class
//	VMCTL_GUEST_MEMORY_CAP_INTERVAL   reconcile interval, default 1m; 0 stops
//	VMCTL_GUEST_MEMORY_CAP_THROUGH_WORK_COMPUTERS
//	                                  "id,class:<class>,*": the ceiling acts
//	                                  while a capsule or a self-development
//	                                  operation is live (the act's
//	                                  through_work); others stand aside
//
// Through work (2026-11-03,
// docs/problems/working-guest-cache-held-while-builds-wait-2026-11-03.md):
// the ceiling had borrowed the restart rule, so a self-development arm kept
// its page cache for its whole round, while its own build waited for host
// memory. Dropping clean cache cuts no work and never takes anonymous
// memory. A guest that predates the field answers 400 once per boot and gets
// the plain ceiling.
//
// Every pass asks each named active computer for a ceiling act (guestmem
// ModeCeiling): while its used memory is over the ceiling, the guest drops
// clean page cache, never anonymous memory, so a burst grows to the guest's
// full size at once and gives the memory back through free page reporting
// when it ends. The first version held the cap with the balloon, a fixed
// ceiling a burst could only escape through deflate_on_oom; every pass now
// sets every active computer's balloon to 0, deflating what it left. The
// acts follow VMCTL_GUEST_CACHE_RETURN (off stops them, observe makes them
// dry runs). Rollback: drop the entry, or the variable.

// DefaultGuestMemoryCapInterval is the reconcile interval when unset;
// MinGuestMemoryCapInterval is the floor a set interval is raised to.
const (
	DefaultGuestMemoryCapInterval = time.Minute
	MinGuestMemoryCapInterval     = 10 * time.Second
)

// GuestMemoryCapFloorMiB mirrors vmmanager.GuestMemoryCapFloorMiB: a smaller
// cap is a bad entry, named once at startup.
const GuestMemoryCapFloorMiB = 1024

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

// ParseGuestMemoryCaps reads "id=MiB,..."; malformed entries and caps under
// the floor are dropped and named in the second result.
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
		if !ok || id == "" || id == guestMemoryCapClassPrefix || err != nil || n < GuestMemoryCapFloorMiB {
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

// ParseGuestMemoryCapThroughWork reads "id,class:<class>,*"; "class:" with
// no name is dropped and named in the second result.
func ParseGuestMemoryCapThroughWork(raw string) (map[string]bool, []string) {
	var out map[string]bool
	var bad []string
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if tok == guestMemoryCapClassPrefix || strings.ContainsAny(tok, "= ") {
			bad = append(bad, tok)
			continue
		}
		if out == nil {
			out = map[string]bool{}
		}
		out[tok] = true
	}
	return out, bad
}

// SetGuestMemoryCapThroughWork sets the through-work selector.
func (r *OwnershipRegistry) SetGuestMemoryCapThroughWork(sel map[string]bool) {
	r.guestMemoryCapMu.Lock()
	r.guestMemoryCapThroughWork = sel
	r.guestMemoryCapMu.Unlock()
}

// guestMemoryCapThroughWorkFor reports whether the selector names the
// computer (by VM id, computer id, its class, or "*").
func guestMemoryCapThroughWorkFor(sel map[string]bool, own *VMOwnership, class WarmnessClass) bool {
	if len(sel) == 0 {
		return false
	}
	return sel["*"] || sel[own.VMID] || (own.ComputerID != "" && sel[own.ComputerID]) ||
		(class != "" && sel[guestMemoryCapClassPrefix+string(class)])
}

// guestMemoryCapClassPrefix names a warmness class in an entry
// ("class:primary=3072"), so the ceiling widens ring by ring.
const guestMemoryCapClassPrefix = "class:"

// guestMemoryCapFor is the computer's cap (by VM id, then computer id, then
// its class), or 0.
func guestMemoryCapFor(caps map[string]int, own *VMOwnership, class WarmnessClass) int {
	if n, ok := caps[own.VMID]; ok {
		return n
	}
	if own.ComputerID != "" {
		if n, ok := caps[own.ComputerID]; ok {
			return n
		}
	}
	if class != "" {
		return caps[guestMemoryCapClassPrefix+string(class)]
	}
	return 0
}

// StartGuestMemoryCapLoop reconciles balloon targets every interval.
func (r *OwnershipRegistry) StartGuestMemoryCapLoop(ctx context.Context, interval time.Duration, caps map[string]int) {
	if interval <= 0 {
		return
	}
	if interval < MinGuestMemoryCapInterval {
		interval = MinGuestMemoryCapInterval
	}
	go func() {
		r.reconcileGuestMemoryCaps(ctx, caps)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.reconcileGuestMemoryCaps(ctx, caps)
			}
		}
	}()
}

// reconcileGuestMemoryCaps deflates each active computer's balloon and asks
// each named one for a ceiling act. It logs each balloon change and each act
// that wrote, and a failure only when it differs from that VM's last one, so
// a persistent failure is one line (review, Devin #3). It returns how many
// ceiling acts wrote.
func (r *OwnershipRegistry) reconcileGuestMemoryCaps(ctx context.Context, caps map[string]int) int {
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	setter, _ := mgr.(guestMemoryCapSetter)
	cfg := r.guestCacheReturnConfig()
	r.guestMemoryCapMu.Lock()
	defer r.guestMemoryCapMu.Unlock()
	if r.guestMemoryCapLastErr == nil {
		r.guestMemoryCapLastErr = map[string]string{}
	}
	changed := 0
	seen := map[string]bool{}
	for _, own := range r.ListOwnerships() {
		if own == nil || own.VMID == "" || own.State != VMStateActive || seen[own.VMID] {
			continue
		}
		seen[own.VMID] = true
		if setter != nil {
			res, err := setter.SetGuestMemoryCap(own.VMID, 0)
			if err != nil {
				// No ceiling act over a balloon that may still be inflated:
				// the guest counts balloon pages as used, and the act would
				// squeeze cache it can never bring under the ceiling
				// (review, Devin #2).
				r.noteGuestMemoryCapErr(own, "balloon", err.Error())
				continue
			} else if res.Changed {
				log.Printf("vmctl: guest memory cap computer=%s vm=%s balloon_mib %d -> %d (deflated)",
					own.ComputerID, own.VMID, res.BeforeMiB, res.AfterMiB)
			}
		}
		class := r.WarmnessClassForOwnership(own)
		ceilingMiB := guestMemoryCapFor(caps, own, class)
		if ceilingMiB == 0 || cfg.Mode == GuestCacheReturnOff || strings.TrimSpace(own.ComputerURL) == "" || ctx.Err() != nil {
			continue
		}
		key := cacheReturnUnsupportedKey(own.VMID, guestCeilingMode)
		if r.cacheReturn.isUnsupported(key, own.Epoch) {
			continue
		}
		fields := map[string]any{
			"mode": guestCeilingMode, "scope": "root", "dry_run": cfg.Mode == GuestCacheReturnObserve, "ceiling_mib": ceilingMiB,
		}
		throughKey := cacheReturnUnsupportedKey(own.VMID, guestCeilingThroughWorkMode)
		throughWork := guestMemoryCapThroughWorkFor(r.guestMemoryCapThroughWork, own, class) && !r.cacheReturn.isUnsupported(throughKey, own.Epoch)
		if throughWork {
			fields["through_work"] = true
		}
		status, report, err := r.cacheReturn.callBody(ctx, own, fields)
		if throughWork && err == nil && (status == http.StatusBadRequest || status == http.StatusNotFound) {
			// Maybe a guest that predates through_work (strict decode): the
			// plain ceiling now. Only when the plain ask is not refused the
			// same way is the field the cause; then no through_work until it
			// boots. A plain 400 or 404 (a bad ceiling, no route) is the
			// plain ceiling's to mark below (review r1, Devin F1, DeepSeek F1).
			first := status
			delete(fields, "through_work")
			throughWork = false
			status, report, err = r.cacheReturn.callBody(ctx, own, fields)
			if err == nil && status != http.StatusBadRequest && status != http.StatusNotFound {
				r.cacheReturn.markUnsupported(throughKey, own.Epoch)
				log.Printf("vmctl: guest memory ceiling computer=%s vm=%s through_work unsupported (http %d) until it boots", own.ComputerID, own.VMID, first)
			}
		}
		switch {
		case err != nil:
			r.noteGuestMemoryCapErr(own, "ceiling", err.Error())
		case status == http.StatusNotFound || status == http.StatusBadRequest:
			// A guest without ceiling mode: not again until it boots.
			r.cacheReturn.markUnsupported(key, own.Epoch)
			log.Printf("vmctl: guest memory ceiling computer=%s vm=%s unsupported (http %d) until it boots", own.ComputerID, own.VMID, status)
		case status == http.StatusConflict:
			// Busy (a live capsule, a self-development operation, a
			// hibernate) or another act in flight: the ceiling stands aside.
			delete(r.guestMemoryCapLastErr, own.VMID)
		case status != http.StatusOK:
			r.noteGuestMemoryCapErr(own, "ceiling", "http_"+strconv.Itoa(status))
		case report.Outcome == "unsupported" || report.Outcome == "swap_present":
			// The guest cannot act (no memory.reclaim, a swap device): one
			// line, not a silent retry every minute (review, Devin #1).
			r.noteGuestMemoryCapErr(own, "ceiling", report.Outcome+": "+report.Detail)
		default:
			delete(r.guestMemoryCapLastErr, own.VMID)
			if report.Steps > 0 || report.Outcome == "dry_run" {
				changed++
				log.Printf("vmctl: guest memory ceiling computer=%s vm=%s ceiling_mib=%d through_work=%t used_mib=%d outcome=%s target_mib=%d reclaimed_mib=%d ms=%d",
					own.ComputerID, own.VMID, ceilingMiB, throughWork, report.UsedBytes>>20, report.Outcome, report.TargetBytes>>20, report.ReclaimedBytes>>20, report.MS)
			}
		}
	}
	return changed
}

// guestCeilingMode is guestmem.ModeCeiling (vmctl does not import guestmem);
// guestCeilingThroughWorkMode keys a guest without through_work.
const (
	guestCeilingMode            = "ceiling"
	guestCeilingThroughWorkMode = "ceiling_through_work"
)

// noteGuestMemoryCapErr logs a failure when it differs from the VM's last.
// The caller holds guestMemoryCapMu.
func (r *OwnershipRegistry) noteGuestMemoryCapErr(own *VMOwnership, what, msg string) {
	msg = what + ": " + msg
	if r.guestMemoryCapLastErr[own.VMID] != msg {
		log.Printf("vmctl: guest memory cap computer=%s vm=%s skipped %s", own.ComputerID, own.VMID, msg)
	}
	r.guestMemoryCapLastErr[own.VMID] = msg
}
