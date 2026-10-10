package vmctl

import (
	"log"
	"net"
	"net/url"
	"strings"
	"time"
)

// Wedged guests (docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md).
// A guest whose process is alive but that refuses a TCP connection to its
// service port for wedgedAfterDefault running is wedged: it can do no work
// anyone will ever see. A busy guest still accepts the connection even when
// its /health is slow, so it is never mistaken for a wedged one. Reconcile
// stops a wedged guest by itself, with a destruction receipt and
// stopped_by "wedged"; the always-on policy or the next page load boots it
// again. The owner never has to (owner ruling 2026-10-10).

const (
	stoppedByWedged    = "wedged"
	wedgedAfterDefault = 5 * time.Minute
	wedgeProbeTimeout  = 3 * time.Second
)

// guestAcceptsConnection reports whether the guest's service port accepts a
// TCP connection. Only the connection is tested, not the service's answer.
func guestAcceptsConnection(computerURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(computerURL))
	if err != nil || parsed.Host == "" {
		return true // nothing to probe is never evidence of a wedge
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, wedgeProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// guestWedged records one network probe of a live guest and reports whether
// it has refused connections for the whole wedge window.
func (r *OwnershipRegistry) guestWedged(vmID, computerURL string, now time.Time) bool {
	probe := r.guestNetworkProbe
	if probe == nil {
		probe = guestAcceptsConnection
	}
	accepts := probe(computerURL)
	r.mu.Lock()
	defer r.mu.Unlock()
	if accepts {
		delete(r.networkDeadSince, vmID)
		return false
	}
	first, known := r.networkDeadSince[vmID]
	if !known {
		r.networkDeadSince[vmID] = now
		return false
	}
	return now.Sub(first) >= wedgedAfterDefault
}

// forgetWedgeProbes drops probe history for VMs no longer seen live.
func (r *OwnershipRegistry) forgetWedgeProbes(live map[string][]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for vmID := range r.networkDeadSince {
		if _, ok := live[vmID]; !ok {
			delete(r.networkDeadSince, vmID)
		}
	}
}

// stopWedgedUnmanaged powers off a wedged guest the manager does not track.
func (r *OwnershipRegistry) stopWedgedUnmanaged(pc vmProcessController, own *VMOwnership, res *VMProcessReconcileResult) {
	if err := pc.ReapUnmanagedVM(own.VMID); err != nil {
		log.Printf("vmctl: stop wedged VM %s failed: %v", own.VMID, err)
		return
	}
	res.Wedged++
	r.markWedgedStopped(own.VMID)
	log.Printf("vmctl: wedged VM %s (user %s) refused connections for %s; stopped, it boots again on resume", own.VMID, own.UserID, wedgedAfterDefault)
}

// stopWedgedManaged stops a wedged guest the manager tracks.
func (r *OwnershipRegistry) stopWedgedManaged(own *VMOwnership, res *VMProcessReconcileResult) {
	if err := r.StopVMForDesktop(own.UserID, own.DesktopID); err != nil {
		log.Printf("vmctl: stop wedged VM %s failed: %v", own.VMID, err)
		return
	}
	res.Wedged++
	r.markWedgedStopped(own.VMID)
	log.Printf("vmctl: wedged VM %s (user %s) refused connections for %s; stopped, it boots again on resume", own.VMID, own.UserID, wedgedAfterDefault)
}

func (r *OwnershipRegistry) markWedgedStopped(vmID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.networkDeadSince, vmID)
	delete(r.unmanagedSince, vmID)
	if cur, ok := r.vmByID[vmID]; ok {
		cur.State = VMStateStopped
		cur.StoppedBy = stoppedByWedged
		r.saveLocked()
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// liveUnmanagedBootRefusal recognizes vmmanager.ErrLiveUnmanaged across the
// manager adapter, which vmctl does not import.
func liveUnmanagedBootRefusal(err error) bool {
	return err != nil && strings.Contains(err.Error(), "live untracked Firecracker process")
}
