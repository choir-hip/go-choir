package vmctl

// S5 busy hibernate on a planned host stop
// (docs/problems/paper-never-checkpointed-2026-10-27.md, slice S5; the
// manager's half is internal/vmmanager/busy_hibernate.go).
//
// cmd/vmctl calls HibernateForHostStop at its exit when the exit is a planned
// host stop: a host shutdown, or VMCTL_STOP_MANAGED_ON_EXIT. A vmctl restart
// for a deploy is not one; it leaves every guest running for reattach, as
// before. A crash of vmctl runs no exit code, and a crash of the host runs
// none either, so neither ever reaches here.
//
// Each running computer, protected classes first, is hibernated by snapshot
// and recorded hibernated, so the next vmctl start keeps its snapshot and the
// next wake resumes it. The busy capture (a guest that answers busy at the
// fence) is offered only when the guest's /health says no self-development
// operation is active and no capsule is live; a guest that cannot be read,
// or does not report live_capsules, gets the ordinary fenced hibernate, which
// a busy guest refuses. A refused guest stays running, and the host stop
// takes it down as a crash, as before. A computer whose deploy refresh is in
// flight is skipped (its planned-restart marker window).
//
// Owner, 2026-10-28: on by default for every computer, protected classes
// included, with no off switch.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// StopReasonHostStop is VMOwnership.StoppedBy for a host-stop hibernate.
const StopReasonHostStop = "host_stop"

// hostStopHibernateManager is the optional manager call for a host-stop
// capture (vmmanager.Manager.HibernateVMForHostStop).
type hostStopHibernateManager interface {
	HibernateVMForHostStop(vmID string) error
}

// hostStopHealth is what a host stop reads from a guest's /health.
type hostStopHealth struct {
	// Answered: a 200 with a readable body.
	Answered      bool
	SelfdevActive int
	// LiveCapsules is nil when the guest does not report the field.
	LiveCapsules *int
}

// busyCaptureAllowed: the guest is known free of a self-development apply
// and of a live capsule.
func (h hostStopHealth) busyCaptureAllowed() bool {
	return h.Answered && h.SelfdevActive == 0 && h.LiveCapsules != nil && *h.LiveCapsules == 0
}

// HostStopReport counts what a host stop did.
type HostStopReport struct {
	// Hibernated: recorded hibernated (by snapshot, or by the manager's kill
	// path when the capture failed).
	Hibernated int `json:"hibernated"`
	// BusyOffered: computers offered the busy capture.
	BusyOffered int `json:"busy_offered"`
	// Skipped: left running (a busy guest refused, or a deploy refresh in
	// flight).
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	// NotStarted: the deadline passed before their turn.
	NotStarted int `json:"not_started"`
}

type hostStopTarget struct {
	own       VMOwnership
	protected bool
}

// HibernateForHostStop hibernates every running computer for a planned host
// stop, protected classes first, until ctx is done.
func (r *OwnershipRegistry) HibernateForHostStop(ctx context.Context) HostStopReport {
	r.mu.RLock()
	targets := make([]hostStopTarget, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		if own == nil || own.VMID == "" || (own.State != VMStateActive && own.State != VMStateDegraded) {
			continue
		}
		class := warmnessClassForOwnership(own, r.warmnessPolicy)
		targets = append(targets, hostStopTarget{
			own:       *own,
			protected: class == WarmnessClassCriticalProtected || class == WarmnessClassPublicPlatform,
		})
	}
	probe := r.hostStopHealthProbe
	_, canCapture := r.vmManager.(hostStopHibernateManager)
	r.mu.RUnlock()
	if probe == nil {
		probe = httpHostStopHealth()
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].protected != targets[j].protected {
			return targets[i].protected
		}
		return targets[i].own.VMID < targets[j].own.VMID
	})

	var report HostStopReport
	for i, target := range targets {
		if ctx.Err() != nil {
			report.NotStarted = len(targets) - i
			log.Printf("vmctl: host stop: deadline passed; %d computer(s) not hibernated", report.NotStarted)
			break
		}
		own := target.own
		offer := canCapture && probe(&own).busyCaptureAllowed()
		if offer {
			report.BusyOffered++
		}
		started := time.Now()
		err := r.hibernateVMForDesktop(own.UserID, own.DesktopID, StopReasonHostStop, offer)
		switch {
		case err == nil:
			report.Hibernated++
			log.Printf("vmctl: host stop: hibernated VM %s (busy_offered=%v protected=%v) in %s", own.VMID, offer, target.protected, time.Since(started).Round(time.Millisecond))
		case errors.Is(err, errHibernateGuestBusy):
			report.Skipped++
			log.Printf("vmctl: host stop: VM %s left running (busy_offered=%v): %v; the host stop ends it as a crash", own.VMID, offer, err)
		default:
			report.Failed++
			log.Printf("vmctl: host stop: hibernate of VM %s failed: %v", own.VMID, err)
		}
	}
	log.Printf("vmctl: host stop: hibernated=%d busy_offered=%d skipped=%d failed=%d not_started=%d",
		report.Hibernated, report.BusyOffered, report.Skipped, report.Failed, report.NotStarted)
	return report
}

// setHostStopHealthProbeForTest pins the host-stop health reading.
func (r *OwnershipRegistry) setHostStopHealthProbeForTest(probe func(*VMOwnership) hostStopHealth) {
	r.hostStopHealthProbe = probe
}

func httpHostStopHealth() func(*VMOwnership) hostStopHealth {
	client := &http.Client{Timeout: 4 * time.Second}
	return func(own *VMOwnership) hostStopHealth {
		base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
		if base == "" {
			return hostStopHealth{}
		}
		req, err := http.NewRequest(http.MethodGet, base+"/health", nil)
		if err != nil {
			return hostStopHealth{}
		}
		hostauth.MarkInternal(req)
		resp, err := client.Do(req)
		if err != nil {
			return hostStopHealth{}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return hostStopHealth{}
		}
		var health struct {
			SelfdevActiveOps int  `json:"selfdev_active_operations"`
			LiveCapsules     *int `json:"live_capsules"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health); err != nil {
			return hostStopHealth{}
		}
		return hostStopHealth{Answered: true, SelfdevActive: health.SelfdevActiveOps, LiveCapsules: health.LiveCapsules}
	}
}
