package vmmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yusefmosiah/go-choir/internal/rssfall"
)

// Density D6 S3: the cache return before a snapshot hibernate
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.3, 5.1).
//
// A snapshot writes the guest's memory, cold page cache included. After the
// fence (prepare) and before the Full or Diff
// decision and the pause, the host asks the fenced guest to return its cold
// cache, then waits for the Firecracker process's RSS to fall. It is its own
// call, never part of the prepare body: a stuck guest write costs at most
// its 5 second bound, and any failure skips the step and the snapshot is
// taken as without it. It writes nothing to the VM's configuration, the
// launch config or the prepare body, so no snapshot is voided either way.

const (
	guestCacheReturnPath = "/internal/runtime/memory/cache-return"

	// snapshotCacheReturnCallTimeout bounds the call to the guest.
	snapshotCacheReturnCallTimeout = 5 * time.Second
	// snapshotCacheReturnFenceBudget: the guest's fence lifts two minutes
	// after it closes (hibernateFenceTTL) and the host's worst case before
	// D6 was already 110 s (prepare 90 s, unplug 10 s, pause 10 s). The act
	// is skipped once more than this has passed since prepare was sent, so
	// the worst case with it, 60 + 5 + 5 + 10 s, stays 40 s inside the TTL.
	snapshotCacheReturnFenceBudget = 60 * time.Second
)

// HibernateOptions are per-hibernate options. The zero value is today's
// hibernate.
type HibernateOptions struct {
	// CacheReturn, when set, adds the step above to a snapshot hibernate.
	CacheReturn *CacheReturnOption
}

// CacheReturnOption selects the act's scope and whether it is a dry run
// (observe).
type CacheReturnOption struct {
	Scope  string
	DryRun bool
}

// CacheReturnTimeline is the step as the hibernate timeline records it.
// ReclaimedMiB is the guest's claim; the RSS fields are the host's readings.
type CacheReturnTimeline struct {
	Skipped      string `json:"skipped,omitempty"` // fence_budget
	Error        string `json:"error,omitempty"`   // status_NNN, timeout, bad_report, error
	DryRun       bool   `json:"dry_run,omitempty"`
	Outcome      string `json:"outcome,omitempty"`
	Method       string `json:"method,omitempty"`
	ReclaimedMiB uint64 `json:"reclaimed_mib"`
	RSS0MiB      uint64 `json:"rss0_mib"`
	RSSEndMiB    uint64 `json:"rss_end_mib"`
	SettleMS     int64  `json:"settle_ms"`
	SettleExit   string `json:"settle_exit,omitempty"`
}

func (c *CacheReturnTimeline) detail() string {
	switch {
	case c == nil:
		return ""
	case c.Skipped != "":
		return "skipped=" + c.Skipped
	case c.Error != "":
		return "error=" + c.Error
	}
	return fmt.Sprintf("outcome=%s reclaimed_mib=%d rss0_mib=%d rss_end_mib=%d settle_ms=%d settle_exit=%s",
		c.Outcome, c.ReclaimedMiB, c.RSS0MiB, c.RSSEndMiB, c.SettleMS, c.SettleExit)
}

// logSuffix is appended to the hibernate log line.
func (c *CacheReturnTimeline) logSuffix() string {
	switch {
	case c == nil:
		return ""
	case c.Skipped != "":
		return " cache_return=skipped:" + c.Skipped
	case c.Error != "":
		return " cache_return=error:" + c.Error
	}
	fell := uint64(0)
	if c.RSS0MiB > c.RSSEndMiB {
		fell = c.RSS0MiB - c.RSSEndMiB
	}
	return fmt.Sprintf(" cache_return_mib=%d rss_fell_mib=%d settle_ms=%d", c.ReclaimedMiB, fell, c.SettleMS)
}

// guestCacheReturnReport is the part of the guest's report the host records.
// It is the guest's claim: it sizes the settle wait and nothing else.
type guestCacheReturnReport struct {
	Outcome        string `json:"outcome"`
	Method         string `json:"method"`
	ReclaimedBytes uint64 `json:"reclaimed_bytes"`
}

func (m *Manager) cacheReturnNow() time.Time {
	if m.cacheReturnClock != nil {
		return m.cacheReturnClock()
	}
	return time.Now()
}

func (m *Manager) cacheReturnCallBound() time.Duration {
	if m.cacheReturnTimeout > 0 {
		return m.cacheReturnTimeout
	}
	return snapshotCacheReturnCallTimeout
}

func (m *Manager) cacheReturnSleepFn() func(time.Duration) {
	if m.cacheReturnSleep != nil {
		return m.cacheReturnSleep
	}
	return time.Sleep
}

// cacheReturnRSS reads the VM's Firecracker process for the length of one
// hibernate: the pids are resolved once, each poll reads their status.
func (m *Manager) cacheReturnRSSFor(vmID string) func() (uint64, bool) {
	if m.cacheReturnRSSReader != nil {
		return m.cacheReturnRSSReader(vmID)
	}
	pids := m.LiveFirecrackerVMs()[vmID]
	return func() (uint64, bool) {
		out := vmResidentBytesFrom(map[string][]int{vmID: pids}, func(pid int) (string, error) {
			data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
			return string(data), err
		})
		v, ok := out[vmID]
		return v, ok
	}
}

// snapshotCacheReturn runs the step and records it on snap. It never fails
// the hibernate: whatever happens, the caller carries on to the decision.
func (m *Manager) snapshotCacheReturn(vmID, hostURL string, opt *CacheReturnOption, fenceBegan time.Time, snap *SnapshotTimeline) {
	cr := &CacheReturnTimeline{DryRun: opt.DryRun}
	snap.CacheReturn = cr
	if waited := m.cacheReturnNow().Sub(fenceBegan); waited > snapshotCacheReturnFenceBudget {
		cr.Skipped = "fence_budget"
		log.Printf("vmmanager: snapshot hibernate of VM %s: cache return skipped, %s since the fence was sent (budget %s)", vmID, waited.Round(time.Millisecond), snapshotCacheReturnFenceBudget)
		return
	}
	read := m.cacheReturnRSSFor(vmID)
	rss0, have := read()
	if have {
		cr.RSS0MiB = rss0 >> 20
	}
	scope := opt.Scope
	if scope == "" {
		scope = "daemon"
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.cacheReturnCallBound())
	defer cancel()
	var report guestCacheReturnReport
	status, err := postGuestJSON(ctx, hostURL+guestCacheReturnPath, map[string]any{"mode": "hibernate", "scope": scope, "dry_run": opt.DryRun}, &report)
	switch {
	case err != nil && (errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil):
		cr.Error = "timeout"
	case err != nil:
		cr.Error = "error"
	case status != http.StatusOK:
		cr.Error = "status_" + strconv.Itoa(status)
	case report.Outcome == "":
		cr.Error = "bad_report"
	}
	if cr.Error != "" {
		log.Printf("vmmanager: snapshot hibernate of VM %s: cache return %s; snapshotting without it", vmID, cr.Error)
		return
	}
	cr.Outcome, cr.Method, cr.ReclaimedMiB = report.Outcome, report.Method, report.ReclaimedBytes>>20
	cr.RSSEndMiB = cr.RSS0MiB
	// A dry run frees nothing, whatever the guest claims: it is not waited
	// for (host guard; a buggy or hostile guest would add the cap to every
	// observed hibernate).
	if opt.DryRun || !have {
		return
	}
	// The wait's cap is its own: the call's time is not charged to it, so
	// the step adds at most call bound + cap (10 s) to the hibernate.
	res := rssfall.Wait(read, m.cacheReturnSleepFn(), rss0, report.ReclaimedBytes, rssfall.DefaultCap)
	cr.RSSEndMiB, cr.SettleMS, cr.SettleExit = res.RSS>>20, res.MS, res.Exit
}
