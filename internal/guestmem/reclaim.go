package guestmem

// D6 S1: the guest's cache return act
// (docs/design/density-d6-page-cache-return-2026-10-26.md section 4.1).
//
// A guest frees clean page cache only under its own memory pressure, so an
// 8 GiB guest holding 2 GiB of anonymous memory keeps its cold cache for
// ever, and free page reporting (which hands freed blocks to the host) has
// nothing to report. The act writes to memory.reclaim of one cgroup, in
// bounded steps, cold pages first, never swapping, and stops when the guest
// turns busy. It leaves no state behind: not a target, not a mode.
//
// The act decides nothing for the host. Its report is the guest's claim; the
// host acts only on its own measurements.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Modes. idle: a quiet computer, cold pages less a keep. hibernate: the
// guest is fenced and about to be snapshotted, so all cold pages go.
// busy_cold: reserved for always-busy computers (S5); sized here, not
// reachable through the route.
const (
	ModeIdle      = "idle"
	ModeHibernate = "hibernate"
	ModeBusyCold  = "busy_cold"
)

// Scopes: the cgroup whose memory.reclaim is written.
const (
	ScopeDaemon = "daemon"
	ScopeRoot   = "root"
)

// Outcomes.
const (
	OutcomeReturned        = "returned"
	OutcomePartial         = "partial"
	OutcomeStoppedBusy     = "stopped_busy"
	OutcomeNothingToReturn = "nothing_to_return"
	OutcomeUnsupported     = "unsupported"
	OutcomeSwapPresent     = "swap_present"
	// OutcomeDryRun: sized and reported, nothing written.
	OutcomeDryRun = "dry_run"
)

// Methods.
const (
	MethodMemoryReclaim = "memory_reclaim"
	MethodDropCaches    = "drop_caches"
	// MethodNone: nothing was or could be written.
	MethodNone = "none"
)

const (
	reclaimStepBytes  = 64 << 20
	reclaimFloorBytes = 128 << 20
	idleKeepBytes     = 256 << 20
	busyColdMinKeep   = 512 << 20
	busyColdCapBytes  = 1 << 30

	idleBudget      = 2 * time.Second
	hibernateBudget = 3 * time.Second
	busyColdBudget  = 1 * time.Second
)

// ErrInProgress: another act is already running in this guest.
var ErrInProgress = errors.New("a cache return act is already in progress")

// ReclaimRequest is one act.
type ReclaimRequest struct {
	Mode   string
	Scope  string
	DryRun bool
	// Stop is asked before every step; a non-empty answer ends the act as
	// stopped_busy with that reason. Per request, because the stop rule
	// depends on the mode.
	Stop func() string
	// Reporting reads free page reporting's state, once before and once
	// after the writes.
	Reporting func() Reporting
}

// Snapshot is what the act reads before and after. A field the kernel did
// not report is nil, never zero.
type Snapshot struct {
	// The scope cgroup's memory.stat, in bytes.
	File          *int64 `json:"file,omitempty"`
	InactiveFile  *int64 `json:"inactive_file,omitempty"`
	ActiveFile    *int64 `json:"active_file,omitempty"`
	Shmem         *int64 `json:"shmem,omitempty"`
	FileDirty     *int64 `json:"file_dirty,omitempty"`
	FileWriteback *int64 `json:"file_writeback,omitempty"`
	// /proc/meminfo, in bytes.
	MemFree      *int64 `json:"mem_free,omitempty"`
	MemAvailable *int64 `json:"mem_available,omitempty"`
	AnonPages    *int64 `json:"anon_pages,omitempty"`
	MeminfoShmem *int64 `json:"meminfo_shmem,omitempty"`
	// /proc/vmstat counters.
	PgstealProactive      *int64 `json:"pgsteal_proactive,omitempty"`
	WorkingsetRefaultFile *int64 `json:"workingset_refault_file,omitempty"`
	Pgmajfault            *int64 `json:"pgmajfault,omitempty"`
	Pgpgout               *int64 `json:"pgpgout,omitempty"`
	// ReportableKiB: free memory in blocks free page reporting can hand back.
	ReportableKiB *int64 `json:"reportable_kib,omitempty"`
}

// ReclaimReport is the act's report.
type ReclaimReport struct {
	Outcome string `json:"outcome"`
	Mode    string `json:"mode"`
	Scope   string `json:"scope"`
	DryRun  bool   `json:"dry_run,omitempty"`
	// Method is how the cache was (or would be) returned: memory_reclaim,
	// drop_caches (the hibernate-mode fallback) or none. Path is the file
	// written, or the memory.reclaim path found absent.
	Method string `json:"method"`
	Path   string `json:"path,omitempty"`
	Cgroup string `json:"cgroup,omitempty"`
	// TargetBytes: what the sizing asked for. ReclaimedBytes: the scope's
	// memory.stat "file" before minus after (what the kernel actually freed).
	TargetBytes    int64 `json:"target_bytes"`
	ReclaimedBytes int64 `json:"reclaimed_bytes"`
	// Steps counts completed writes.
	Steps int   `json:"steps"`
	MS    int64 `json:"ms"`
	// SwappinessKey: the writes carried "swappiness=0" (false after an
	// EINVAL: that kernel rejects the key).
	SwappinessKey       bool  `json:"swappiness_key"`
	ReportingNegotiated *bool `json:"reporting_negotiated,omitempty"`
	// Detail names why an act did not run or stopped: the missing path, the
	// stop reason, the write error.
	Detail string   `json:"detail,omitempty"`
	Before Snapshot `json:"before"`
	After  Snapshot `json:"after"`
}

// Reporting is what the balloon driver says about free page reporting.
type Reporting struct {
	Negotiated    *bool
	ReportableKiB *int64
}

// Reclaimer runs acts. The zero value is ready; the fields override the real
// guest for tests and wiring. One act at a time.
type Reclaimer struct {
	// ProcRoot is /proc; CgroupRoot is /sys/fs/cgroup.
	ProcRoot   string
	CgroupRoot string
	// Write writes one value to a control file. The default is a raw
	// non-blocking write that returns EAGAIN instead of parking.
	Write func(path, value string) error
	// Quiet lowers the CPU priority of the thread it runs on. The writes run
	// on a goroutine of their own, locked to its OS thread, which ends with
	// the act: the runtime then ends the thread, so no priority is ever
	// restored and no niced thread goes back to the scheduler. The default
	// is nice 19.
	Quiet func()
	// Budget overrides the mode's time budget (idle 2 s, hibernate 3 s,
	// busy_cold 1 s). It is checked between steps.
	Budget time.Duration

	mu sync.Mutex
}

// MemoryStat is the cgroup memory.stat fields the sizing reads, in bytes.
type MemoryStat struct {
	File, InactiveFile, ActiveFile, Shmem, FileDirty, FileWriteback int64
}

// ParseMemoryStat reads the fields it needs from a cgroup v2 memory.stat; an
// absent field is zero.
func ParseMemoryStat(data string) MemoryStat {
	var st MemoryStat
	for _, line := range strings.Split(data, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "file":
			st.File = n
		case "inactive_file":
			st.InactiveFile = n
		case "active_file":
			st.ActiveFile = n
		case "shmem":
			st.Shmem = n
		case "file_dirty":
			st.FileDirty = n
		case "file_writeback":
			st.FileWriteback = n
		}
	}
	return st
}

// ReclaimTarget is the bytes to ask for. The cold set is the inactive file
// pages, clamped to the file pages that are not shared memory (shmem lives
// on the anonymous LRU, but the clamp is right either way), less the dirty
// and writeback pages (reclaiming those is write I/O). A result under the
// floor is the caller's nothing_to_return.
func ReclaimTarget(mode string, st MemoryStat) int64 {
	cold := st.InactiveFile
	if nonShared := st.File - st.Shmem; nonShared < cold {
		cold = nonShared
	}
	cold -= st.FileDirty + st.FileWriteback
	if cold < 0 {
		cold = 0
	}
	var target int64
	switch mode {
	case ModeIdle:
		target = cold - idleKeepBytes
	case ModeHibernate:
		target = cold
	case ModeBusyCold:
		keep := st.ActiveFile
		if keep < busyColdMinKeep {
			keep = busyColdMinKeep
		}
		target = cold - keep
		if target > busyColdCapBytes {
			target = busyColdCapBytes
		}
	}
	if target < 0 {
		return 0
	}
	return target
}

func (r *Reclaimer) procRoot() string {
	if r.ProcRoot != "" {
		return r.ProcRoot
	}
	return "/proc"
}

func (r *Reclaimer) cgroupRoot() string {
	if r.CgroupRoot != "" {
		return r.CgroupRoot
	}
	return "/sys/fs/cgroup"
}

func (r *Reclaimer) write(path, value string) error {
	if r.Write != nil {
		return r.Write(path, value)
	}
	return writeControlFile(path, value)
}

func (r *Reclaimer) budget(mode string) time.Duration {
	if r.Budget > 0 {
		return r.Budget
	}
	switch mode {
	case ModeHibernate:
		return hibernateBudget
	case ModeBusyCold:
		return busyColdBudget
	}
	return idleBudget
}

// Reclaim runs one act. It returns ErrInProgress, with an empty report, when
// another act is running, and an error for a request it does not understand.
// Every other answer is a report: the act's outcome is data, not an error.
func (r *Reclaimer) Reclaim(ctx context.Context, req ReclaimRequest) (ReclaimReport, error) {
	switch req.Mode {
	case ModeIdle, ModeHibernate, ModeBusyCold:
	default:
		return ReclaimReport{}, fmt.Errorf("unknown cache return mode %q", req.Mode)
	}
	switch req.Scope {
	case ScopeDaemon, ScopeRoot:
	default:
		return ReclaimReport{}, fmt.Errorf("unknown cache return scope %q", req.Scope)
	}
	if !r.mu.TryLock() {
		return ReclaimReport{}, ErrInProgress
	}
	defer r.mu.Unlock()

	started := time.Now()
	rep := ReclaimReport{Mode: req.Mode, Scope: req.Scope, DryRun: req.DryRun, Method: MethodNone}
	finish := func(outcome, detail string) (ReclaimReport, error) {
		rep.Outcome, rep.Detail = outcome, detail
		rep.MS = time.Since(started).Milliseconds()
		return rep, nil
	}

	// Never reclaim on a guest that has swap: with swap the kernel may push
	// anonymous pages out, and guests do not swap (owner, 2026-10-09). An
	// unreadable /proc/swaps is not "no swap".
	swaps, err := os.ReadFile(filepath.Join(r.procRoot(), "swaps"))
	if err != nil {
		return finish(OutcomeSwapPresent, "cannot read /proc/swaps: "+err.Error())
	}
	if strings.TrimSpace(string(swaps)) == "" {
		// The kernel always writes a header line: empty is not "no swap".
		return finish(OutcomeSwapPresent, "/proc/swaps is empty")
	}
	if hasSwapDevice(string(swaps)) {
		return finish(OutcomeSwapPresent, "/proc/swaps lists a device")
	}

	cgroup, why := r.scopeDir(req.Scope)
	if why != "" {
		return finish(OutcomeUnsupported, why)
	}
	rep.Cgroup = cgroup
	reclaimPath := filepath.Join(cgroup, "memory.reclaim")
	method := MethodMemoryReclaim
	rep.Path = reclaimPath
	if _, err := os.Stat(reclaimPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) || req.Mode != ModeHibernate {
			return finish(OutcomeUnsupported, "memory.reclaim unavailable: "+reclaimPath)
		}
		// Only a fenced guest about to be snapshotted may drop its caches
		// whole; the hot set then costs only refaults after the wake.
		method = MethodDropCaches
		rep.Path = filepath.Join(r.procRoot(), "sys", "vm", "drop_caches")
	}
	rep.Method = method

	rep.Before = r.snapshot(cgroup)
	// Free page reporting's state is a read: every report that got this far,
	// a dry run included, carries it.
	if req.Reporting != nil {
		reporting := req.Reporting()
		rep.ReportingNegotiated = reporting.Negotiated
		rep.Before.ReportableKiB = reporting.ReportableKiB
	}
	if method == MethodMemoryReclaim {
		raw, err := os.ReadFile(filepath.Join(cgroup, "memory.stat"))
		if err != nil {
			return finish(OutcomeUnsupported, "memory.stat unreadable: "+filepath.Join(cgroup, "memory.stat"))
		}
		rep.TargetBytes = ReclaimTarget(req.Mode, ParseMemoryStat(string(raw)))
		if rep.TargetBytes < reclaimFloorBytes {
			rep.TargetBytes = 0
			return finish(OutcomeNothingToReturn, "")
		}
	}
	if req.DryRun {
		return finish(OutcomeDryRun, "")
	}

	quiet := r.Quiet
	if quiet == nil {
		quiet = quietThread
	}
	type result struct{ outcome, detail string }
	done := make(chan result, 1)
	go func() {
		// Never unlocked: the thread ends with this goroutine.
		runtime.LockOSThread()
		quiet()
		outcome, detail := r.act(ctx, req, method, reclaimPath, rep.TargetBytes, &rep)
		done <- result{outcome, detail}
	}()
	got := <-done
	outcome, detail := got.outcome, got.detail

	rep.After = r.snapshot(cgroup)
	if req.Reporting != nil {
		rep.After.ReportableKiB = req.Reporting().ReportableKiB
	}
	if rep.Before.File != nil && rep.After.File != nil && *rep.Before.File > *rep.After.File {
		rep.ReclaimedBytes = *rep.Before.File - *rep.After.File
	}
	return finish(outcome, detail)
}

// act is the writing phase.
func (r *Reclaimer) act(ctx context.Context, req ReclaimRequest, method, reclaimPath string, target int64, rep *ReclaimReport) (string, string) {
	if method == MethodDropCaches {
		path := filepath.Join(r.procRoot(), "sys", "vm", "drop_caches")
		if err := r.write(path, "1"); err != nil {
			return OutcomePartial, "drop_caches: " + err.Error()
		}
		rep.Steps = 1
		return OutcomeReturned, "fallback: " + reclaimPath + " is absent; drop_caches used"
	}
	deadline := time.Now().Add(r.budget(req.Mode))
	keyed := true
	rep.SwappinessKey = true
	remaining := target
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return OutcomePartial, "context ended: " + err.Error()
		}
		if !time.Now().Before(deadline) {
			return OutcomePartial, "budget spent"
		}
		if req.Stop != nil {
			if reason := req.Stop(); reason != "" {
				return OutcomeStoppedBusy, reason
			}
		}
		step := int64(reclaimStepBytes)
		if remaining < step {
			step = remaining
		}
		err := r.write(reclaimPath, stepValue(step, keyed))
		if keyed && errors.Is(err, syscall.EINVAL) {
			// A kernel that rejects the swappiness key: this step and every
			// later one go bare.
			keyed = false
			rep.SwappinessKey = false
			err = r.write(reclaimPath, stepValue(step, false))
		}
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) {
				return OutcomePartial, "EAGAIN: the kernel reclaimed less than asked"
			}
			return OutcomePartial, "write: " + err.Error()
		}
		rep.Steps++
		remaining -= step
	}
	return OutcomeReturned, ""
}

func stepValue(bytes int64, keyed bool) string {
	if keyed {
		return strconv.FormatInt(bytes, 10) + " swappiness=0"
	}
	return strconv.FormatInt(bytes, 10)
}

// hasSwapDevice reports whether /proc/swaps lists anything past its header.
func hasSwapDevice(data string) bool {
	lines := 0
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			lines++
		}
	}
	return lines > 1
}

// scopeDir resolves the scope's cgroup directory. A daemon in the root
// cgroup, a missing "0::" line (cgroup v1) and a path that leaves the cgroup
// root are unsupported: the act never widens its scope on its own.
func (r *Reclaimer) scopeDir(scope string) (dir, unsupported string) {
	root := r.cgroupRoot()
	if scope == ScopeRoot {
		return root, ""
	}
	raw, err := os.ReadFile(filepath.Join(r.procRoot(), "self", "cgroup"))
	if err != nil {
		return "", "cannot read /proc/self/cgroup: " + err.Error()
	}
	rel := ""
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), "0::"); ok {
			rel, found = path, true
			break
		}
	}
	if !found {
		return "", "/proc/self/cgroup has no cgroup v2 entry"
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return "", "daemon cgroup path leaves the cgroup root"
		}
	}
	rel = filepath.Clean("/" + strings.TrimPrefix(rel, "/"))
	if rel == "/" {
		return "", "the daemon runs in the root cgroup; ask for scope root explicitly"
	}
	return filepath.Join(root, rel), ""
}

// snapshot reads what it can; every field is best effort.
func (r *Reclaimer) snapshot(cgroup string) Snapshot {
	var s Snapshot
	if raw, err := os.ReadFile(filepath.Join(cgroup, "memory.stat")); err == nil {
		fields := statFields(string(raw))
		s.File, s.InactiveFile, s.ActiveFile = fields["file"], fields["inactive_file"], fields["active_file"]
		s.Shmem, s.FileDirty, s.FileWriteback = fields["shmem"], fields["file_dirty"], fields["file_writeback"]
	}
	if raw, err := os.ReadFile(filepath.Join(r.procRoot(), "meminfo")); err == nil {
		mem := Meminfo(string(raw))
		pick := func(name string) *int64 {
			if v, ok := mem[name]; ok {
				n := int64(v)
				return &n
			}
			return nil
		}
		s.MemFree, s.MemAvailable, s.AnonPages, s.MeminfoShmem = pick("MemFree"), pick("MemAvailable"), pick("AnonPages"), pick("Shmem")
	}
	if raw, err := os.ReadFile(filepath.Join(r.procRoot(), "vmstat")); err == nil {
		fields := statFields(string(raw))
		s.PgstealProactive, s.WorkingsetRefaultFile = fields["pgsteal_proactive"], fields["workingset_refault_file"]
		s.Pgmajfault, s.Pgpgout = fields["pgmajfault"], fields["pgpgout"]
	}
	return s
}

// statFields parses "name value" lines to a map of present fields.
func statFields(data string) map[string]*int64 {
	out := map[string]*int64{}
	for _, line := range strings.Split(data, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			v := n
			out[name] = &v
		}
	}
	return out
}
