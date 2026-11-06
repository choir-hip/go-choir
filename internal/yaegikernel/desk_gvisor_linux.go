//go:build linux

package yaegikernel

import (
	"bytes"
	"context"
	"crypto/rand"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/childthread"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The Linux half of desk_gvisor.go.

const (
	// gvisorSandboxOverheadBytes is a sandbox's own share of the desks
	// memory: the sentry, gofer and launcher (its cgroup's memory.current
	// with one idle worker is 66 MiB on arm64, about 40 of it the sandbox's;
	// 29 MiB on x86 rootless) plus its memory /tmp, which is capped at
	// gvisorSandboxTmpBytes.
	gvisorSandboxOverheadBytes = 64<<20 + gvisorSandboxTmpBytes
	// gvisorSandboxTmpBytes caps a sandbox's memory /tmp (every cell tmp dir
	// of its class together).
	gvisorSandboxTmpBytes = 32 << 20
	// gvisorWorkerOverheadBytes is what an idle worker holds in its
	// sandbox (measured 26 to 29 MiB), with a margin.
	gvisorWorkerOverheadBytes = 32 << 20
	// gvisorAddressStepBytes is the Go heap's growth step (a 64 MiB arena):
	// at a given address limit what a worker holds before it dies varies by
	// about one step, so its claim covers a share plus a step, a quarter and
	// a margin (gvisorWorkerClaim; measured past its share by at most 88 MiB
	// at 96 MiB and 115 MiB at 512 MiB, design doc calibration table).
	gvisorAddressStepBytes = 64 << 20
	// gvisorAddressJitterBytes is the margin on that step.
	gvisorAddressJitterBytes = 16 << 20
	// gvisorWorkerMinBytes is the smallest heap share a worker is admitted
	// with; below it the spawn is refused (the desks memory is committed).
	// A research cell parsing a 3 MiB document rises 13 to 16 MiB.
	gvisorWorkerMinBytes = 96 << 20
	// gvisorShareWorkers and gvisorShareSandboxes size a worker's share of
	// the desks cgroup: the claims of this many workers in this many
	// sandboxes fit (gvisorShareFor). Below the minimum share fewer fit: a
	// 2 GiB guest (desks memory.max 1 GiB) admits three workers at the
	// minimum, never smaller shares.
	gvisorShareWorkers   = 6
	gvisorShareSandboxes = 3
	// gvisorSandboxPidsMax bounds host tasks in a sandbox's cgroup: the
	// sentry's threads and systrap's stub threads for every worker thread.
	gvisorSandboxPidsMax = 4096
	// gvisorStartTimeout bounds a sandbox's start (measured 30 to 73 ms).
	gvisorStartTimeout = 15 * time.Second
	// gvisorRetryAfter is how long a missing runsc, or a class whose sandbox
	// would not start, waits before the next attempt.
	gvisorRetryAfter = 2 * time.Minute
	// gvisorSandboxPrefix starts every container id this spawner creates.
	gvisorSandboxPrefix = "choir-desk-"
)

// gvisorRunscCandidates lists where runsc may be, first match wins. A test
// replaces it.
var gvisorRunscCandidates = func() []string {
	// CHOIR_RUNSC, when set, is the only candidate: a wrong path makes
	// gVisor unavailable rather than quietly picking another runsc.
	if p := strings.TrimSpace(os.Getenv(RunscPathEnv)); p != "" {
		return []string{p}
	}
	out := []string{"/run/current-system/sw/bin/runsc"}
	if p, err := exec.LookPath("runsc"); err == nil {
		out = append(out, p)
	}
	return out
}

// gvisorStateBase is where the spawner keeps runsc's state root, bundles
// and process specs. A test replaces it.
var gvisorStateBase = func() string {
	if os.Geteuid() == 0 {
		return "/run/choir-desk-gvisor"
	}
	return filepath.Join(os.TempDir(), "choir-desk-gvisor-"+strconv.Itoa(os.Geteuid()))
}

// gvisorPool holds every sandbox. mu guards only its fields and each
// sandbox's counters: checking runsc, starting, stopping, killing and
// inspecting run runsc (and read /proc) outside it.
//
// runsc, version and base are written once, under mu, by the first
// successful available, which then sweeps and sets ready; they are never
// written again. Every sandbox, cell and runsc command comes after a read of
// ready under mu, so they read them without the lock.
type gvisorPool struct {
	mu sync.Mutex
	// checkMu serializes the runsc check and the sweep (both outside mu).
	checkMu sync.Mutex
	checked bool
	ready   bool
	runsc   string
	version string
	// reason and retryAt: why runsc is unusable, until when (cached, so a
	// missing runsc does not cost every spawn a lookup and a --version).
	reason  string
	retryAt time.Time
	base    string
	swept   bool
	// sandboxes by class key and binary.
	sandboxes map[string]*gvisorSandbox
	// failed: a class whose sandbox would not start, until its retry time.
	// Other classes are not affected.
	failed map[string]gvisorFailure
	// committed is the desks memory every live sandbox and worker holds a
	// claim on (overheads plus each worker's share); admission keeps it
	// within the desks cgroup's memory.max.
	committed uint64
	// queue holds spawns waiting for committed memory, granted in order:
	// class a (the custodians) first, then everyone else, each by arrival.
	queue []*gvisorWaiter
	seq   uint64
	// poke is closed and replaced when a waiter may make progress other
	// than by its own grant: the queue changed (a new head) or a desk session
	// went idle (NotifyDeskSessionIdle), so the head waiter asks again for
	// an idle session to end.
	poke chan struct{}
}

// gvisorWaiter is one spawn waiting for admission.
type gvisorWaiter struct {
	seq      uint64
	classA   bool
	cls      deskSandboxClass
	mapKey   string
	bin      string
	worker   string
	claim    uint64
	desksMax uint64
	// process: a first-party desk's worker under process confinement (gVisor
	// unusable for its class); it claims memory but joins no sandbox.
	process bool
	// granted receives the grant: the sandbox the worker joins and whether
	// this worker starts it.
	granted chan gvisorGrant
}

type gvisorGrant struct {
	sb      *gvisorSandbox
	creator bool
	// err refuses the waiter instead: its class's sandbox failed to start
	// while it waited, and the class is in its retry window.
	err error
}

// gvisorAdmissionDefaultWait bounds the admission wait of a spawn whose
// caller set no deadline.
const gvisorAdmissionDefaultWait = 2 * time.Minute

type gvisorFailure struct {
	reason string
	until  time.Time
}

var deskGvisor = newGvisorPool()

func newGvisorPool() *gvisorPool {
	return &gvisorPool{sandboxes: map[string]*gvisorSandbox{}, failed: map[string]gvisorFailure{}}
}

type gvisorSandbox struct {
	pool      *gvisorPool
	mapKey    string
	key       string
	class     DeskTrustClass
	id        string
	bin       string
	workerArg string
	dir       string
	cgroupDir string
	// ready is closed when the start finished; startErr says how.
	ready    chan struct{}
	startErr error
	cmd      *exec.Cmd
	log      *tailBuffer
	done     chan struct{}
	started  time.Time
	// Under pool.mu: live workers, every worker served, the memory the live
	// workers claim, and uid releases held until the sandbox stops (a worker
	// whose kill was not confirmed may still run as that uid).
	workers   int
	served    int
	liveBytes uint64
	heldUIDs  []func()
	// heldBytes is the claims of reaped workers whose end was not confirmed
	// (they may still hold memory): counted in liveBytes and the pool's
	// committed until the sandbox stops.
	heldBytes uint64
	stopping  bool
	cg        *workerCgroup
}

// gvisorCell is one worker's place in a sandbox.
type gvisorCell struct {
	sb         *gvisorSandbox
	sandboxDir string
	specFile   string
	pidFile    string
	uid        uint32
	// shareBytes is the heap the worker is admitted with (its address-space
	// limit is this plus gvisorAddressOverheadBytes); claimBytes adds its
	// overhead and is what it holds in the ledger.
	shareBytes uint64
	claimBytes uint64
	// queued is how long the spawn waited for admission.
	queued time.Duration
	once   sync.Once
	// ended: the worker's end inside the sandbox is confirmed.
	ended atomic.Bool
}

func (p *gvisorPool) rootless() bool { return os.Geteuid() != 0 }

func (p *gvisorPool) stateRoot() string { return filepath.Join(p.base, "state") }

// globalFlags are runsc's flags for every command on this pool's sandboxes.
func (p *gvisorPool) globalFlags() []string {
	f := []string{
		"--root=" + p.stateRoot(),
		"--platform=systrap",
		"--network=none",
		"--ignore-cgroups",
		"--overlay2=none",
		"--gvisor-marker-file",
	}
	if p.rootless() {
		f = append(f, "--rootless")
	}
	return f
}

// runscCmd runs one short runsc command and returns its combined output.
func (p *gvisorPool) runscCmd(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.runsc, append(p.globalFlags(), args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	return cmd.CombinedOutput()
}

// available finds and checks runsc once, and returns "" when gVisor can be
// used, else why not. A negative result is cached for gvisorRetryAfter. The
// lookup, runsc --version and the sweep run outside mu (under checkMu), so a
// slow runsc never holds up admission, reaps or the status read.
func (p *gvisorPool) available() string {
	if r, done := p.cachedAvailability(); done {
		return r
	}
	p.checkMu.Lock()
	defer p.checkMu.Unlock()
	if r, done := p.cachedAvailability(); done {
		return r
	}
	p.mu.Lock()
	base := p.base
	p.mu.Unlock()
	runsc, version, base, reason := checkRunsc(base)
	p.mu.Lock()
	p.checked = true
	p.retryAt = time.Now().Add(gvisorRetryAfter)
	p.reason = reason
	if reason != "" {
		p.mu.Unlock()
		return reason
	}
	p.runsc, p.version, p.base = runsc, version, base
	swept := p.swept
	p.swept = true
	p.mu.Unlock()
	if !swept {
		// Before ready: no sandbox of this process exists yet.
		p.sweep()
	}
	p.mu.Lock()
	p.ready = true
	p.mu.Unlock()
	return ""
}

// cachedAvailability is the answer without a check: ready, or a negative
// result still in its retry window.
func (p *gvisorPool) cachedAvailability() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		return "", true
	}
	if p.checked && time.Now().Before(p.retryAt) {
		return p.reason, true
	}
	return "", false
}

// checkRunsc finds runsc, reads its version and makes the state dirs.
func checkRunsc(base string) (runsc, version, outBase, reason string) {
	for _, c := range gvisorRunscCandidates() {
		if st, err := os.Stat(c); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0o111 != 0 {
			runsc = c
			break
		}
	}
	if runsc == "" {
		return "", "", base, "no runsc (" + RunscPathEnv + " unset and none in the system profile or on PATH)"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, runsc, "--version").CombinedOutput()
	if err != nil {
		return "", "", base, fmt.Sprintf("runsc --version: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if base == "" {
		base = gvisorStateBase()
	}
	if err := os.MkdirAll(base, 0o711); err != nil {
		return "", "", base, "gVisor state dir: " + err.Error()
	}
	_ = os.Chmod(base, 0o711)
	for _, d := range []string{filepath.Join(base, "state"), filepath.Join(base, "specs")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", "", base, "gVisor state dir: " + err.Error()
		}
	}
	return runsc, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), base, ""
}

// sweep removes what an earlier daemon left. Its runsc run launchers died
// with it (parent-death signal), but not its sandboxes: their sentries are
// ended here (runsc delete --force, then cgroup.kill on each s-* cgroup),
// and their workers have exited at their session socket's EOF. Their state,
// directories and cgroups go too. Nothing resumes across a restart. Runs
// under checkMu, before any sandbox of this process exists.
func (p *gvisorPool) sweep() {
	entries, _ := os.ReadDir(p.stateRoot())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), gvisorSandboxPrefix) {
			_, _ = p.runscCmd(10*time.Second, "delete", "--force", e.Name())
		}
	}
	entries, _ = os.ReadDir(p.base)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), gvisorSandboxPrefix) {
			_ = os.RemoveAll(filepath.Join(p.base, e.Name()))
		}
	}
	specs, _ := os.ReadDir(filepath.Join(p.base, "specs"))
	for _, e := range specs {
		_ = os.Remove(filepath.Join(p.base, "specs", e.Name()))
	}
	if root := os.Getenv(DeskCgroupRootEnv); root != "" {
		sweepSandboxCgroups(root)
	}
}

// sweepSandboxCgroups removes sandbox cgroups no live sandbox holds (a
// crashed daemon's). Called before any sandbox of this process exists.
func sweepSandboxCgroups(root string) {
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), sandboxCgroupPrefix) {
			removeCgroupDir(filepath.Join(root, e.Name()))
		}
	}
}

// gvisorAvailable reports whether gVisor can run desk workers now, and if not
// why.
func gvisorAvailable() (bool, string) {
	r := deskGvisor.available()
	return r == "", r
}

// errDesksMemoryCommitted is the capacity refusal: admitting the worker
// would let the desks' claims exceed the desks cgroup, and then one runaway
// could take the whole class with it.
var errDesksMemoryCommitted = errors.New("desks memory committed")

// desksMemoryMax reads the desks cgroup's memory.max; 0 means no limit (dev,
// tests without a cgroup), and admission then only sizes by budget.
func desksMemoryMax(root string) uint64 {
	if root == "" {
		return 0
	}
	raw, err := os.ReadFile(filepath.Join(root, "memory.max"))
	if err != nil {
		return 0
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// gvisorWorkerClaim is what a worker with this heap share holds in the
// desks memory at most: what it can hold at its address limit
// (gvisorHeapAllowance) and an idle worker's own.
func gvisorWorkerClaim(share uint64) uint64 {
	return gvisorHeapAllowance(share) + gvisorWorkerOverheadBytes
}

// gvisorHeapAllowance is the most a worker with this share is measured to
// hold before its address limit ends it: the share, a quarter more, a heap
// step and a margin.
func gvisorHeapAllowance(share uint64) uint64 {
	return share + share/4 + gvisorAddressStepBytes + gvisorAddressJitterBytes
}

// gvisorShareFor is the largest share whose claim fits workers times in
// room (inverting gvisorWorkerClaim), 0 if none does.
func gvisorShareFor(room, workers uint64) uint64 {
	per := room / workers
	fixed := uint64(gvisorAddressStepBytes + gvisorAddressJitterBytes + gvisorWorkerOverheadBytes)
	if per <= fixed {
		return 0
	}
	return (per - fixed) / 5 * 4
}

// workerShare is a worker's heap share: its desk's budget, at most what
// lets gvisorShareWorkers workers in gvisorShareSandboxes sandboxes fit in
// the desks cgroup, at least gvisorWorkerMinBytes.
func workerShare(budget, desksMax uint64) uint64 {
	if budget == 0 && desksMax == 0 {
		return 0
	}
	share := budget
	if desksMax > 0 {
		fair := uint64(0)
		if room := desksMax; room > gvisorShareSandboxes*gvisorSandboxOverheadBytes {
			fair = gvisorShareFor(room-gvisorShareSandboxes*gvisorSandboxOverheadBytes, gvisorShareWorkers)
		}
		if share == 0 || fair < share {
			share = fair
		}
	}
	if share < gvisorWorkerMinBytes {
		share = gvisorWorkerMinBytes
	}
	return share
}

// errDesksMemoryNeverFits refuses a spawn whose claim alone, with its
// sandbox's, exceeds the desks cgroup: no wait can admit it.
var errDesksMemoryNeverFits = errors.New("desks memory too small")

// admitLocked reserves w's claim (and its sandbox's, for a class with no
// live sandbox) if it fits beside every live claim, joining or registering
// the class's sandbox.
func (p *gvisorPool) admitLocked(w *gvisorWaiter) (gvisorGrant, bool) {
	if w.process {
		// A process-confined worker: its own cgroup, no sandbox.
		if w.desksMax > 0 && p.committed+w.claim > w.desksMax {
			return gvisorGrant{}, false
		}
		p.committed += w.claim
		return gvisorGrant{}, true
	}
	sb := p.sandboxes[w.mapKey]
	if sb != nil && (sb.stopping || sb.dead()) {
		// A dead sandbox's workers are on their way out; a new one serves
		// this worker under a fresh entry.
		delete(p.sandboxes, w.mapKey)
		sb = nil
	}
	need := w.claim
	if sb == nil {
		need += gvisorSandboxOverheadBytes
	}
	if w.desksMax > 0 && p.committed+need > w.desksMax {
		return gvisorGrant{}, false
	}
	p.committed += need
	creator := sb == nil
	if creator {
		id := gvisorSandboxPrefix + w.cls.Key + "-" + randomHex(4)
		sb = &gvisorSandbox{pool: p, mapKey: w.mapKey, key: w.cls.Key, class: w.cls.Class, id: id, bin: w.bin, workerArg: w.worker,
			dir: filepath.Join(p.base, id), log: &tailBuffer{}, done: make(chan struct{}), ready: make(chan struct{})}
		p.sandboxes[w.mapKey] = sb
	}
	sb.workers++
	sb.served++
	sb.liveBytes += w.claim
	return gvisorGrant{sb: sb, creator: creator}, true
}

// grantLocked admits waiters in order (class a first, then by arrival)
// until the first that does not fit: a later, smaller spawn never passes an
// earlier one of its rank. A waiter whose class's sandbox failed to start
// meanwhile is refused with that reason, wherever it waits, rather than
// each retrying a broken runsc. Waiters are poked: the head may have changed.
func (p *gvisorPool) grantLocked() {
	defer p.pokeLocked()
	now := time.Now()
	kept := p.queue[:0]
	for _, w := range p.queue {
		if f, ok := p.failed[w.cls.Key]; ok && now.Before(f.until) && !w.process {
			w.granted <- gvisorGrant{err: errors.New(f.reason)}
			continue
		}
		kept = append(kept, w)
	}
	for i := len(kept); i < len(p.queue); i++ {
		p.queue[i] = nil
	}
	p.queue = kept
	sort.SliceStable(p.queue, func(i, j int) bool {
		if p.queue[i].classA != p.queue[j].classA {
			return p.queue[i].classA
		}
		return p.queue[i].seq < p.queue[j].seq
	})
	for len(p.queue) > 0 {
		w := p.queue[0]
		g, ok := p.admitLocked(w)
		if !ok {
			return
		}
		p.queue = p.queue[1:]
		w.granted <- g
	}
}

// pokeLocked wakes every waiter to look again.
func (p *gvisorPool) pokeLocked() {
	if p.poke != nil {
		close(p.poke)
	}
	p.poke = make(chan struct{})
}

// pokeChanLocked is the channel the next poke closes.
func (p *gvisorPool) pokeChanLocked() <-chan struct{} {
	if p.poke == nil {
		p.poke = make(chan struct{})
	}
	return p.poke
}

// NotifyDeskSessionIdle tells waiting spawns that a desk session went idle:
// the head waiter may now end it (the pressure handler) instead of waiting
// for a reap that only retention would bring.
func NotifyDeskSessionIdle() { deskGvisor.notifyIdle() }

func (p *gvisorPool) notifyIdle() {
	p.mu.Lock()
	if len(p.queue) > 0 {
		p.pokeLocked()
	}
	p.mu.Unlock()
}

// admit queues w and returns its grant: at once if it fits and no spawn of
// its rank or above waits, else when reaps free enough, or an error when ctx
// ends first. While w is the head of the queue and ctx is live, it asks the
// host to end idle sessions (least recently used first), again whenever one
// goes idle; a waiter behind the head, or past its deadline, ends none. p.mu
// must not be held.
func (p *gvisorPool) admit(ctx context.Context, w *gvisorWaiter, role string) (gvisorGrant, time.Duration, error) {
	w.granted = make(chan gvisorGrant, 1)
	p.mu.Lock()
	p.seq++
	w.seq = p.seq
	p.queue = append(p.queue, w)
	p.grantLocked()
	p.mu.Unlock()

	queuedAt := time.Now()
	select {
	case g := <-w.granted:
		return g, 0, g.err
	default:
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, gvisorAdmissionDefaultWait)
		defer cancel()
	}
	for ctx.Err() == nil {
		p.mu.Lock()
		head := len(p.queue) > 0 && p.queue[0] == w
		poke := p.pokeChanLocked()
		p.mu.Unlock()
		select {
		case g := <-w.granted:
			return g, time.Since(queuedAt), g.err
		default:
		}
		if head && ctx.Err() == nil && relieveDeskMemoryPressure() {
			continue
		}
		select {
		case g := <-w.granted:
			return g, time.Since(queuedAt), g.err
		case <-poke:
		case <-ctx.Done():
		}
	}
	p.mu.Lock()
	for i, q := range p.queue {
		if q == w {
			p.queue = append(p.queue[:i], p.queue[i+1:]...)
			break
		}
	}
	// A waiter that leaves may unblock the ones behind it.
	p.grantLocked()
	committed, waiting := p.committed, len(p.queue)
	p.mu.Unlock()
	select {
	case g := <-w.granted:
		// Granted (or refused by its class) as the deadline fell: take it.
		return g, time.Since(queuedAt), g.err
	default:
	}
	return gvisorGrant{}, 0, fmt.Errorf("%w: a %s worker waited %s for %d MiB beside %d MiB claimed of the desks cgroup's %d MiB (%d others waiting): %v",
		errDesksMemoryCommitted, role, time.Since(queuedAt).Round(time.Millisecond), w.claim>>20, committed>>20, w.desksMax>>20, waiting, ctx.Err())
}

// processClaim is a process-confined desk worker's place in the desks
// memory ledger (gVisor unusable for its class): its cgroup's memory.max,
// held from admission to reap.
type processClaim struct {
	pool   *gvisorPool
	bytes  uint64
	queued time.Duration
	once   sync.Once
}

// admitProcess admits a process-confined first-party worker against the
// same ledger and queue as the sandboxes, so a fallback worker is never
// invisible to admission. Its claim is what a sandboxed worker of the desk
// would claim, at most its manifest budget; its cgroup's memory.max is set
// to it (planWorkerCgroup), so the claim bounds what it can hold.
func (p *gvisorPool) admitProcess(ctx context.Context, cls deskSandboxClass, role string, budget uint64, cgroupRoot string) (*processClaim, error) {
	desksMax := desksMemoryMax(cgroupRoot)
	if desksMax == 0 {
		return nil, nil
	}
	claim := gvisorWorkerClaim(workerShare(budget, desksMax))
	if budget > 0 && budget < claim {
		claim = budget
	}
	if claim > desksMax {
		return nil, fmt.Errorf("%w: a %s worker needs %d MiB, more than the desks cgroup's %d MiB", errDesksMemoryNeverFits, role, claim>>20, desksMax>>20)
	}
	w := &gvisorWaiter{classA: cls.Class == TrustClassFirstPartyCustodian, cls: cls, claim: claim, desksMax: desksMax, process: true}
	_, queued, err := p.admit(ctx, w, role)
	if err != nil {
		return nil, err
	}
	return &processClaim{pool: p, bytes: claim, queued: queued}, nil
}

// release returns the claim once (the worker was reaped, or never started).
func (c *processClaim) release() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		c.pool.mu.Lock()
		c.pool.committed -= c.bytes
		c.pool.grantLocked()
		c.pool.mu.Unlock()
	})
}

// attach places one worker in its class's sandbox, starting the sandbox if
// none is live, after admission: the worker's claim (its share plus its
// overhead, and the sandbox's overhead for a new sandbox) must fit in the
// desks cgroup's memory.max beside every live claim (process-confined
// fallback workers' included), so the worker's own address-space limit
// binds before any cgroup does, on every guest size. The desks cgroup sets
// no memory.high (autoputer, MA6.4 r4): admission is its soft bound, and a
// memory.high below the admitted total would throttle the class first. A spawn that does
// not fit now waits in the admission queue (asking the host to end an idle
// session), bounded by ctx; one that can never fit is refused at once.
func (p *gvisorPool) attach(ctx context.Context, cls deskSandboxClass, bin, workerArg, role string, uid uint32, budget uint64, cgroupRoot string, cgroupRequired bool) (*gvisorCell, error) {
	if cgroupRoot == "" && cgroupRequired {
		return nil, fmt.Errorf("required mode needs %s for the sandboxes' memory budgets", DeskCgroupRootEnv)
	}
	desksMax := desksMemoryMax(cgroupRoot)
	share := workerShare(budget, desksMax)
	claim := gvisorWorkerClaim(share)
	if desksMax > 0 && claim+gvisorSandboxOverheadBytes > desksMax {
		return nil, fmt.Errorf("%w: a %s worker needs %d MiB with its sandbox, more than the desks cgroup's %d MiB", errDesksMemoryNeverFits, role, (claim+gvisorSandboxOverheadBytes)>>20, desksMax>>20)
	}

	if r := p.available(); r != "" {
		return nil, errors.New(r)
	}
	p.mu.Lock()
	if f, ok := p.failed[cls.Key]; ok {
		if time.Now().Before(f.until) {
			p.mu.Unlock()
			return nil, errors.New(f.reason)
		}
		delete(p.failed, cls.Key)
	}
	p.mu.Unlock()
	w := &gvisorWaiter{classA: cls.Class == TrustClassFirstPartyCustodian, cls: cls, mapKey: cls.Key + "\x00" + bin, bin: bin, worker: workerArg,
		claim: claim, desksMax: desksMax}
	g, queued, err := p.admit(ctx, w, role)
	if err != nil {
		return nil, err
	}
	sb := g.sb

	if g.creator {
		sb.startErr = sb.start(cgroupRoot)
		close(sb.ready)
		if sb.startErr != nil {
			reason := "gVisor sandbox for class " + cls.Key + " did not start: " + sb.startErr.Error()
			log.Printf("yaegikernel: %s; the class runs under process confinement (first-party) or is refused until %s", reason, time.Now().Add(gvisorRetryAfter).UTC().Format(time.RFC3339))
			p.mu.Lock()
			p.failed[cls.Key] = gvisorFailure{reason: reason, until: time.Now().Add(gvisorRetryAfter)}
			p.mu.Unlock()
		}
	} else {
		<-sb.ready
	}
	c := &gvisorCell{sb: sb, uid: uid, shareBytes: share, claimBytes: claim, queued: queued}
	if sb.startErr != nil {
		c.unclaim(true, false)
		return nil, fmt.Errorf("gVisor sandbox for class %s did not start: %w", cls.Key, sb.startErr)
	}
	// The memory.max follows the live claims; a write that fails refuses the
	// worker rather than run it unbudgeted.
	p.mu.Lock()
	err = sb.setMemoryMaxLocked(false)
	p.mu.Unlock()
	if err != nil {
		c.unclaim(true, false)
		return nil, err
	}
	// The cell's tmp dir is in the sandbox's own memory /tmp, made by the
	// worker itself (0700, its uid) before it confines: nothing a cell
	// writes reaches a host disk. The name is random, so no class-mate can
	// take it first; the worker refuses a dir it did not create.
	c.sandboxDir = filepath.Join("/tmp", "choir-cell-"+safeCellRole(role)+"-"+randomHex(8))
	tag := randomHex(8)
	c.specFile = filepath.Join(p.base, "specs", "exec-"+tag+".json")
	c.pidFile = filepath.Join(p.base, "specs", "pid-"+tag)
	return c, nil
}

// unclaim takes the worker out of its sandbox. Its claim goes back to the
// waiters at once when its end is confirmed; when it is not (hold), the
// claim stays committed, like its uid, until the sandbox stops. The last
// worker out stops the sandbox, outside the pool lock, and the sandbox's
// overhead and every held claim go back only after the stop returns: memory
// is granted to a waiter only once it is free.
func (c *gvisorCell) unclaim(lowerMax, hold bool) {
	sb, p := c.sb, c.sb.pool
	p.mu.Lock()
	sb.workers--
	if hold {
		sb.heldBytes += c.claimBytes
	} else {
		sb.liveBytes -= c.claimBytes
		p.committed -= c.claimBytes
	}
	stop := sb.workers == 0 && !sb.stopping
	if stop {
		sb.stopping = true
		if p.sandboxes[sb.mapKey] == sb {
			delete(p.sandboxes, sb.mapKey)
		}
	} else if lowerMax {
		_ = sb.setMemoryMaxLocked(true)
	}
	held := sb.heldUIDs
	if stop {
		sb.heldUIDs = nil
	}
	if !hold {
		// The freed claim goes to the waiters.
		p.grantLocked()
	}
	p.mu.Unlock()
	if stop {
		sb.stop()
		p.mu.Lock()
		p.committed -= gvisorSandboxOverheadBytes + sb.heldBytes
		sb.liveBytes -= sb.heldBytes
		sb.heldBytes = 0
		p.grantLocked()
		p.mu.Unlock()
		for _, r := range held {
			r()
		}
	}
}

// release ends the worker inside the sandbox (its host-side runsc exec is
// already reaped), removes its tmp dir there, returns its uid once its end is
// confirmed (else when the sandbox stops), and gives back its claim.
func (c *gvisorCell) release(releaseUID func()) {
	c.once.Do(func() {
		confirmed := c.killWorker()
		if confirmed {
			c.removeCellDir()
		}
		_ = os.Remove(c.specFile)
		_ = os.Remove(c.pidFile)
		if releaseUID != nil {
			if confirmed {
				releaseUID()
			} else {
				log.Printf("yaegikernel: gVisor worker (uid %d) in sandbox %s: its end is not confirmed; the uid and its %d MiB claim are held until the sandbox stops", c.uid, c.sb.id, c.claimBytes>>20)
				c.sb.pool.mu.Lock()
				c.sb.heldUIDs = append(c.sb.heldUIDs, releaseUID)
				c.sb.pool.mu.Unlock()
			}
		}
		c.unclaim(true, !confirmed)
	})
}

// tailBuffer keeps the last tailBufferBytes of a sandbox's runsc output: a
// sandbox lives as long as its class has workers, and its log must not grow
// with it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const tailBufferBytes = 16 << 10

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - tailBufferBytes; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// workerPID is the worker's pid inside its sandbox, 0 when it never started.
func (c *gvisorCell) workerPID() int {
	raw, err := os.ReadFile(c.pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0
	}
	return pid
}

// killWorker SIGKILLs the worker inside its sandbox and reports whether its
// end is confirmed (killed, gone from the sandbox's process list, never
// started, or the sandbox is dead). The host process the spawner holds is
// runsc's exec client: killing it alone leaves the worker running, and the
// worker holds the client's stdio, so its reap would wait. Nil-safe: a worker
// under process confinement has no cell.
func (c *gvisorCell) killWorker() bool {
	if c == nil || c.ended.Load() {
		return true
	}
	if c.sb.dead() {
		c.ended.Store(true)
		return true
	}
	pid := c.workerPID()
	if pid <= 0 {
		// No pid file: runsc exec never started the worker.
		_, err := os.Stat(c.pidFile)
		if os.IsNotExist(err) {
			c.ended.Store(true)
			return true
		}
		return false
	}
	p := c.sb.pool
	_, _ = p.runscCmd(10*time.Second, "kill", "--pid", strconv.Itoa(pid), c.sb.id, "KILL")
	// Confirm against the sandbox's own process list, whatever kill said
	// (it fails for a process that already exited).
	deadline := time.Now().Add(3 * time.Second)
	for {
		alive, err := c.sb.pidAlive(pid)
		if (err == nil && !alive) || c.sb.dead() {
			c.ended.Store(true)
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// pidAlive reads the sandbox's process list.
func (sb *gvisorSandbox) pidAlive(pid int) (bool, error) {
	out, err := sb.pool.runscCmd(10*time.Second, "ps", "--format=json", sb.id)
	if err != nil {
		return false, err
	}
	// runsc ps --format=json prints the sandbox's pids: [1,8].
	var pids []int
	if err := json.Unmarshal(bytes.TrimSpace(out), &pids); err != nil {
		return false, fmt.Errorf("runsc ps %q: %w", tailString(string(out), 200), err)
	}
	for _, p := range pids {
		if p == pid {
			return true, nil
		}
	}
	return false, nil
}

// removeCellDir removes the worker's tmp dir from the sandbox's /tmp, as the
// worker's uid (its dir is 0700), once the worker is dead.
func (c *gvisorCell) removeCellDir() {
	if c.sandboxDir == "" || c.sb.dead() || c.workerPID() <= 0 {
		return
	}
	_, _ = c.exec(c.uid, []string{deskSandboxRoleEnv + "=" + deskSandboxRoleCleanup, deskSandboxCleanupEnv + "=" + c.sandboxDir})
}

func (sb *gvisorSandbox) dead() bool {
	select {
	case <-sb.done:
		return true
	default:
		return false
	}
}

// start creates the sandbox's bundle and cgroup and starts it. Runs outside
// the pool lock; concurrent attaches of the class wait on ready. On an error
// the last worker's unclaim stops what was made.
func (sb *gvisorSandbox) start(cgroupRoot string) error {
	p := sb.pool
	rootfs := filepath.Join(sb.dir, "rootfs")
	for _, d := range []struct {
		path string
		mode os.FileMode
	}{{sb.dir, 0o711}, {rootfs, 0o755}, {filepath.Join(rootfs, strings.TrimPrefix(sandboxWorkDir, "/")), 0o755}, {filepath.Join(sb.dir, "bundle"), 0o700}} {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return err
		}
		if err := os.Chmod(d.path, d.mode); err != nil {
			return err
		}
	}
	spec, err := sandboxBundleSpec(sb.bin, sb.workerArg, rootfs)
	if err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(spec, "", " ")
	bundle := filepath.Join(sb.dir, "bundle")
	if err := os.WriteFile(filepath.Join(bundle, "config.json"), raw, 0o600); err != nil {
		return err
	}
	if cgroupRoot != "" {
		p.mu.Lock()
		max := gvisorSandboxOverheadBytes + sb.liveBytes
		p.mu.Unlock()
		cg, err := createSandboxCgroup(cgroupRoot, sandboxCgroupPrefix+sb.key+"-"+randomHex(4), max)
		if err != nil {
			return err
		}
		p.mu.Lock()
		sb.cg = cg
		p.mu.Unlock()
	}
	cmd := exec.Command(p.runsc, append(p.globalFlags(), "run", "--bundle", bundle, sb.id)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.Dir = bundle
	cmd.Stdout, cmd.Stderr = sb.log, sb.log
	// A process left holding the output pipe must not keep dead() false.
	cmd.WaitDelay = 2 * time.Second
	// The parent-death signal ends the runsc run launcher with the daemon;
	// it fires when the starting thread exits, so the start runs on a thread
	// held for the child's life (childthread), never on one the runtime may
	// end. The sandbox itself outlives the launcher: its workers exit at
	// their session socket's EOF, and the next daemon's sweep ends the
	// sentry (runsc delete --force, cgroup.kill on its s-* cgroup). A
	// restart never inherits a sandbox.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if sb.cg != nil {
		cmd.SysProcAttr.UseCgroupFD = true
		cmd.SysProcAttr.CgroupFD = int(sb.cg.dir.Fd())
	}
	if err := childthread.Start(cmd); err != nil {
		return fmt.Errorf("runsc run: %w", err)
	}
	p.mu.Lock()
	sb.cmd = cmd
	sb.started = time.Now()
	p.mu.Unlock()
	go func() { _ = cmd.Wait(); close(sb.done) }()
	deadline := time.Now().Add(gvisorStartTimeout)
	for {
		out, err := p.runscCmd(5*time.Second, "state", sb.id)
		if err == nil {
			var st struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(out, &st) == nil && st.Status == "running" {
				return nil
			}
		}
		select {
		case <-sb.done:
			return fmt.Errorf("runsc run exited before the sandbox ran: %s", tailString(sb.log.String(), 600))
		case <-time.After(15 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the sandbox did not run within %s: %s", gvisorStartTimeout, tailString(sb.log.String(), 600))
		}
	}
}

// stop ends the sandbox and removes its directories and cgroup. Runs outside
// the pool lock.
func (sb *gvisorSandbox) stop() {
	p := sb.pool
	if sb.cmd != nil && sb.cmd.Process != nil {
		if !sb.dead() {
			_, _ = p.runscCmd(10*time.Second, "kill", "--all", sb.id, "KILL")
		}
		_ = syscall.Kill(-sb.cmd.Process.Pid, syscall.SIGKILL)
		_ = sb.cmd.Process.Kill()
		select {
		case <-sb.done:
		case <-time.After(5 * time.Second):
		}
	}
	_, _ = p.runscCmd(10*time.Second, "delete", "--force", sb.id)
	if sb.dir != "" && strings.HasPrefix(filepath.Base(sb.dir), gvisorSandboxPrefix) {
		_ = os.RemoveAll(sb.dir)
	}
	p.mu.Lock()
	cg := sb.cg
	sb.cg = nil
	p.mu.Unlock()
	cg.release()
}

// setMemoryMaxLocked sets the sandbox cgroup's memory.max to its overhead
// plus its live workers' claims. Lowering (at a worker's reap) never goes
// below what the sandbox uses now plus a margin: memory the sentry has not
// yet returned would meet the limit as an OOM of the whole class.
func (sb *gvisorSandbox) setMemoryMaxLocked(lowering bool) error {
	if sb.cg == nil {
		return nil
	}
	want := gvisorSandboxOverheadBytes + sb.liveBytes
	if !lowering && want <= sb.cg.maxBytes {
		return nil
	}
	if lowering {
		if raw, err := os.ReadFile(filepath.Join(sb.cg.path, "memory.current")); err == nil {
			if cur, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64); err == nil && cur+(32<<20) > want {
				want = cur + 32<<20
			}
		}
		if want > sb.cg.maxBytes {
			return nil
		}
	}
	if err := writeCgroupFile(sb.cg.path, "memory.max", strconv.FormatUint(want, 10)); err != nil {
		return fmt.Errorf("sandbox %s memory.max: %w", sb.id, err)
	}
	sb.cg.maxBytes = want
	return nil
}

// createSandboxCgroup makes a sandbox's child of the desks cgroup: no swap,
// a process cap, oom.group (an OOM ends the whole sandbox, never the daemon;
// admission keeps it from being reached), and memory.max from the live
// claims.
func createSandboxCgroup(root, name string, max uint64) (*workerCgroup, error) {
	cg, err := createWorkerCgroup(root, name, max, 0)
	if err != nil {
		return nil, err
	}
	if err := writeCgroupFile(cg.path, "pids.max", strconv.Itoa(gvisorSandboxPidsMax)); err != nil {
		cg.release()
		return nil, fmt.Errorf("sandbox cgroup pids.max: %w", err)
	}
	return cg, nil
}

// ociSpec is the part of the OCI runtime spec the sandbox uses.
type ociSpec struct {
	OCIVersion string       `json:"ociVersion"`
	Process    ociProcess   `json:"process"`
	Root       ociRoot      `json:"root"`
	Hostname   string       `json:"hostname,omitempty"`
	Mounts     []ociMount   `json:"mounts"`
	Linux      ociLinuxPart `json:"linux"`
}

type ociProcess struct {
	User            ociUser          `json:"user"`
	Args            []string         `json:"args"`
	Env             []string         `json:"env"`
	Cwd             string           `json:"cwd"`
	NoNewPrivileges bool             `json:"noNewPrivileges"`
	Capabilities    *ociCapabilities `json:"capabilities"`
	Rlimits         []ociRlimit      `json:"rlimits,omitempty"`
}

type ociRlimit struct {
	Type string `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

// gvisorWorkerStackBytes is a sandboxed worker's RLIMIT_STACK, set before
// it starts: glibc sizes every thread's stack from it (8 MiB by default),
// and each stack is address space under the worker's RLIMIT_AS. Go code runs
// on goroutine stacks; the OS stacks hold only the runtime's g0 frames.
const gvisorWorkerStackBytes = 2 << 20

type ociUser struct {
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

type ociCapabilities struct {
	Bounding    []string `json:"bounding"`
	Effective   []string `json:"effective"`
	Inheritable []string `json:"inheritable"`
	Permitted   []string `json:"permitted"`
	Ambient     []string `json:"ambient"`
}

func noCapabilities() *ociCapabilities {
	return &ociCapabilities{Bounding: []string{}, Effective: []string{}, Inheritable: []string{}, Permitted: []string{}, Ambient: []string{}}
}

type ociRoot struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type ociMount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options,omitempty"`
}

type ociLinuxPart struct {
	Namespaces []ociNamespace `json:"namespaces"`
}

type ociNamespace struct {
	Type string `json:"type"`
}

// nobodyUID runs the sandbox's init and the inspect role.
const nobodyUID = 65534

// sandboxBundleSpec is the sandbox's root container: the filesystem view
// that replaces Landlock, and the pause init.
func sandboxBundleSpec(bin, workerArg, rootfs string) (ociSpec, error) {
	binds, err := binaryClosure(bin)
	if err != nil {
		return ociSpec{}, err
	}
	mounts := []ociMount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev", "mode=1777", "size=" + strconv.Itoa(gvisorSandboxTmpBytes)}},
		// An empty, read-only /sys: gVisor would otherwise give one.
		{Destination: "/sys", Type: "tmpfs", Source: "tmpfs", Options: []string{"ro", "nosuid", "nodev", "noexec"}},
	}
	for _, b := range binds {
		mounts = append(mounts, ociMount{Destination: b, Type: "bind", Source: b, Options: []string{"rbind", "ro", "nosuid", "nodev"}})
	}
	if zi := hostZoneinfo(); zi != "" {
		mounts = append(mounts, ociMount{Destination: sandboxZoneinfoDir, Type: "bind", Source: zi, Options: []string{"rbind", "ro", "nosuid", "nodev", "noexec"}})
	}
	return ociSpec{
		OCIVersion: "1.0.2",
		Process: ociProcess{
			User:            ociUser{UID: nobodyUID, GID: nobodyUID},
			Args:            []string{bin, workerArg},
			Env:             []string{deskSandboxRoleEnv + "=" + deskSandboxRoleInit, "PATH=/nonexistent"},
			Cwd:             "/",
			NoNewPrivileges: true,
			Capabilities:    noCapabilities(),
		},
		Root:     ociRoot{Path: rootfs, Readonly: true},
		Hostname: "desk",
		Mounts:   mounts,
		Linux:    ociLinuxPart{Namespaces: []ociNamespace{{Type: "pid"}, {Type: "ipc"}, {Type: "uts"}, {Type: "mount"}}},
	}, nil
}

// binaryClosure is what the worker binary needs to run: the binary itself,
// and for a dynamically linked one its interpreter and libraries. A path in
// the Nix store is bound as its whole store directory; anything else as the
// file itself. Each is read-only in the sandbox at the same path.
func binaryClosure(bin string) ([]string, error) {
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return nil, fmt.Errorf("desk worker binary: %w", err)
	}
	set := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			// The symlinked path must resolve inside the sandbox too.
			if r != p {
				set[storeTop(p)] = true
			}
			p = r
		}
		set[storeTop(p)] = true
	}
	add(bin)
	add(real)
	// The dynamic closure, breadth first: each object's DT_NEEDED resolved
	// against its own run path, then the interpreter's and the system's
	// library directories.
	seen := map[string]bool{}
	queue := []string{real}
	interpDir := ""
	for len(queue) > 0 {
		obj := queue[0]
		queue = queue[1:]
		if seen[obj] {
			continue
		}
		seen[obj] = true
		f, err := elf.Open(obj)
		if err != nil {
			return nil, fmt.Errorf("desk worker closure %s: %w", obj, err)
		}
		for _, prog := range f.Progs {
			if prog.Type == elf.PT_INTERP {
				b := make([]byte, prog.Filesz)
				if _, err := prog.ReadAt(b, 0); err == nil {
					interp := strings.TrimRight(string(b), "\x00")
					add(interp)
					interpDir = filepath.Dir(interp)
				}
			}
		}
		libs, _ := f.ImportedLibraries()
		var dirs []string
		for _, tag := range []elf.DynTag{elf.DT_RUNPATH, elf.DT_RPATH} {
			if v, err := f.DynString(tag); err == nil {
				for _, s := range v {
					for _, d := range filepath.SplitList(s) {
						dirs = append(dirs, strings.ReplaceAll(d, "$ORIGIN", filepath.Dir(obj)))
					}
				}
			}
		}
		f.Close()
		if interpDir != "" {
			dirs = append(dirs, interpDir)
		}
		dirs = append(dirs, "/lib", "/lib64", "/usr/lib", "/usr/lib64", "/lib/x86_64-linux-gnu", "/lib/aarch64-linux-gnu", "/usr/lib/x86_64-linux-gnu", "/usr/lib/aarch64-linux-gnu")
		for _, lib := range libs {
			found := ""
			for _, d := range dirs {
				if c := filepath.Join(d, lib); fileExists(c) {
					found = c
					break
				}
			}
			if found == "" {
				return nil, fmt.Errorf("desk worker closure: %s needs %s, not found", obj, lib)
			}
			add(found)
			if r, err := filepath.EvalSymlinks(found); err == nil {
				queue = append(queue, r)
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	// A path under another bound path is already covered.
	var kept []string
	for _, p := range out {
		covered := false
		for _, k := range kept {
			if strings.HasPrefix(p, k+"/") {
				covered = true
				break
			}
		}
		if !covered {
			kept = append(kept, p)
		}
	}
	return kept, nil
}

// storeTop maps a path in the Nix store to its store directory.
func storeTop(p string) string {
	const store = "/nix/store/"
	if strings.HasPrefix(p, store) {
		rest := strings.TrimPrefix(p, store)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return store + rest[:i]
		}
	}
	return p
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hostZoneinfo is the time zone database the cells' time package reads
// (ZONEINFO inside the sandbox), or "".
func hostZoneinfo() string {
	for _, c := range []string{os.Getenv("ZONEINFO"), "/etc/zoneinfo", "/usr/share/zoneinfo"} {
		if c == "" {
			continue
		}
		if r, err := filepath.EvalSymlinks(c); err == nil {
			if st, err := os.Stat(r); err == nil && st.IsDir() {
				return r
			}
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func tailString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}

// wrap turns the worker's exec.Cmd (the binary, its argv, environment and
// the session socket as ExtraFiles fd 3) into `runsc exec` into the cell's
// sandbox. The worker's environment and identity go into a process spec the
// spawner alone can read; the host process left is runsc's exec client, which
// lives exactly as long as the worker.
func (c *gvisorCell) wrap(cmd *exec.Cmd, prefix string) error {
	env := sandboxWorkerEnv(cmd.Env, prefix, c.sandboxDir)
	proc := ociProcess{
		User:            ociUser{UID: c.uid, GID: c.uid},
		Args:            append([]string(nil), cmd.Args...),
		Env:             env,
		Cwd:             "/",
		NoNewPrivileges: true,
		Capabilities:    noCapabilities(),
		Rlimits:         []ociRlimit{{Type: "RLIMIT_STACK", Hard: gvisorWorkerStackBytes, Soft: gvisorWorkerStackBytes}},
	}
	raw, err := json.Marshal(proc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.specFile, raw, 0o600); err != nil {
		return fmt.Errorf("gVisor process spec: %w", err)
	}
	p := c.sb.pool
	args := append([]string{p.runsc}, p.globalFlags()...)
	args = append(args, "exec", "--process", c.specFile, "--pass-fd", "3:3", "--internal-pid-file", c.pidFile, c.sb.id)
	cmd.Path = p.runsc
	cmd.Args = args
	// The worker's stdio is donated into the sandbox; once the client exits,
	// its output copy waits at most this long.
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.Dir = "/"
	return nil
}

// sandboxWorkerEnv rewrites the worker environment for the sandbox: its
// tmp dir and allowed root are in-sandbox paths, the sandbox level is
// gvisor, the time zone database is the sandbox's, and the MA6.2 mount step
// is gone (gVisor's view replaces it).
func sandboxWorkerEnv(env []string, prefix, cellDir string) []string {
	prefix = sessionEnvPrefix(prefix)
	drop := map[string]bool{
		prefix + cellMountNSEnv: true, prefix + cellSandboxEnv: true, prefix + cellTmpDirEnv: true,
		prefix + "ALLOWED_ROOT": true, "TMPDIR": true, "ZONEINFO": true, "PATH": true, "MALLOC_ARENA_MAX": true,
	}
	out := make([]string, 0, len(env)+6)
	seen := map[string]int{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] {
			continue
		}
		// The last duplicate wins, as exec.Cmd would.
		if i, ok := seen[k]; ok {
			out[i] = kv
			continue
		}
		seen[k] = len(out)
		out = append(out, kv)
	}
	out = append(out,
		"PATH=/nonexistent",
		"TMPDIR="+cellDir,
		prefix+cellTmpDirEnv+"="+cellDir,
		prefix+"ALLOWED_ROOT="+sandboxWorkDir,
		prefix+cellSandboxEnv+"="+string(cellSandboxGvisor),
	)
	if hostZoneinfo() != "" {
		out = append(out, "ZONEINFO="+sandboxZoneinfoDir)
	}
	// One glibc malloc arena: each further arena reserves 64 MiB of address
	// space, made as threads contend (cgo's thread start mallocs), so the
	// address space a worker needs would swing with its thread count.
	out = append(out, "MALLOC_ARENA_MAX=1")
	return out
}

// inspect runs the worker binary's inspect role inside the cell's sandbox,
// as nobody: the worker's /proc status read from inside (the worker cannot
// forge it). Only a read or write in the worker's own tmp dir (0700) runs as
// the worker's uid.
func (c *gvisorCell) inspect() (sandboxInspectReport, error) {
	return c.inspectWith(nobodyUID, "", "")
}

func (c *gvisorCell) inspectWith(uid uint32, readFile, writeFile string) (sandboxInspectReport, error) {
	pid := c.workerPID()
	if pid <= 0 {
		return sandboxInspectReport{}, fmt.Errorf("worker pid in its sandbox: none in %s", c.pidFile)
	}
	env := []string{deskSandboxRoleEnv + "=" + deskSandboxRoleInspect, deskSandboxInspectPIDEnv + "=" + strconv.Itoa(pid)}
	if readFile != "" {
		env = append(env, deskSandboxInspectReadEnv+"="+readFile)
	}
	if writeFile != "" {
		env = append(env, deskSandboxInspectWriteEnv+"="+writeFile)
	}
	out, err := c.exec(uid, env)
	if err != nil {
		return sandboxInspectReport{}, fmt.Errorf("inspect: %v (%s)", err, tailString(string(out), 300))
	}
	var rep sandboxInspectReport
	trimmed := bytes.TrimSpace(out)
	line := trimmed
	if i := bytes.LastIndexByte(trimmed, '\n'); i >= 0 {
		line = trimmed[i+1:]
	}
	if err := json.Unmarshal(line, &rep); err != nil {
		return sandboxInspectReport{}, fmt.Errorf("inspect output %q: %w", tailString(string(out), 300), err)
	}
	return rep, nil
}

// exec runs the worker binary in the cell's sandbox in one of its sandbox
// roles (env selects it), as uid, and returns its output.
func (c *gvisorCell) exec(uid uint32, env []string) ([]byte, error) {
	proc := ociProcess{
		User:            ociUser{UID: uid, GID: uid},
		Args:            []string{c.sb.bin, c.sb.workerArg},
		Env:             append(env, "PATH=/nonexistent"),
		Cwd:             "/",
		NoNewPrivileges: true,
		Capabilities:    noCapabilities(),
	}
	spec, _ := json.Marshal(proc)
	f := c.specFile + ".role-" + randomHex(4)
	if err := os.WriteFile(f, spec, 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(f)
	return c.sb.pool.runscCmd(20*time.Second, "exec", "--process", f, c.sb.id)
}

// readCellFile reads a file in the worker's tmp dir, as the worker's uid.
func (c *gvisorCell) readCellFile(name string) ([]byte, error) {
	rep, err := c.inspectWith(c.uid, filepath.Join(c.sandboxDir, name), "")
	if err != nil {
		return nil, err
	}
	if rep.FileError != "" {
		return nil, errors.New(rep.FileError)
	}
	return []byte(rep.File), nil
}

// writeCellCanary writes the self-test canary (world-readable) into the
// worker's tmp dir and returns its in-sandbox path.
func (c *gvisorCell) writeCellCanary(name string) (string, error) {
	path := filepath.Join(c.sandboxDir, name)
	rep, err := c.inspectWith(c.uid, "", path)
	if err != nil {
		return "", err
	}
	if rep.FileError != "" {
		return "", errors.New(rep.FileError)
	}
	return path, nil
}

// planGvisorCell is the gVisor branch of planCellConfinement: the worker's
// uid (one per activation), its cell in its class's sandbox, and the wrap of
// its command. cls must be the desk's class.
func planGvisorCell(cfg DeskSessionWorkerConfig, cls deskSandboxClass, bin string, attr *syscall.SysProcAttr) (cellSpawnPlan, error) {
	prefix := sessionEnvPrefix(cfg.EnvPrefix)
	uid := cfg.Confinement.UID
	var release func()
	if uid == 0 {
		var err error
		if uid, release, err = activationUIDs.acquire(); err != nil {
			return cellSpawnPlan{}, err
		}
	}
	var budget uint64
	if d, ok := deskmanifest.Lookup(cfg.Session.Role); ok {
		budget = d.Memory.WorkerMaxBytes
	}
	required := cfg.Confinement.Mode == CellConfinementRequired
	root := cfg.CgroupRoot
	if root == "" {
		root = os.Getenv(DeskCgroupRootEnv)
	}
	cell, err := deskGvisor.attach(cfg.Context, cls, bin, cfg.WorkerArg, cfg.Session.Role, uid, budget, root, required)
	if err != nil {
		if release != nil {
			release()
		}
		return cellSpawnPlan{}, err
	}
	plan := cellSpawnPlan{attr: attr, dir: "/", uid: uid, gvisor: cell, queued: cell.queued}
	plan.release = func() { cell.release(release) }
	plan.env = append(plan.env, prefix+cellUIDEnv+"="+strconv.FormatUint(uint64(uid), 10))
	if as := gvisorAddressLimit(cfg.Session.MemoryLimitBytes, cell.shareBytes); as > 0 {
		plan.env = append(plan.env, prefix+"MEMORY_LIMIT_BYTES="+strconv.FormatUint(as, 10))
	}
	hasLimit := false
	for _, kv := range cfg.ExtraEnv {
		if strings.HasPrefix(kv, "GOMEMLIMIT=") {
			hasLimit = true
		}
	}
	if cell.shareBytes > 0 && !hasLimit {
		// The soft Go limit, three quarters of the worker's heap share, so
		// the collector works before the address-space limit ends it.
		plan.env = append(plan.env, "GOMEMLIMIT="+strconv.FormatUint(cell.shareBytes/4*3, 10))
	}
	plan.wrap = func(cmd *exec.Cmd) error { return cell.wrap(cmd, prefix) }
	return plan, nil
}

// GvisorStatusNow reports the spawner's gVisor state and reads back every
// live sandbox (count, workers, host processes and their resident memory).
// The pool lock is held only to copy the ledger; the runsc check, the /proc
// scan and the cgroup reads run outside it.
func GvisorStatusNow() GvisorStatus {
	p := deskGvisor
	reason := p.available()
	type snap struct {
		r   GvisorSandboxReceipt
		pid int
		cg  string
	}
	var snaps []snap
	st := GvisorStatus{Platform: "systrap", Rootless: p.rootless(), Sandboxes: []GvisorSandboxReceipt{}}
	p.mu.Lock()
	st.Runsc, st.Version = p.runsc, p.version
	st.CommittedBytes, st.Waiting = p.committed, len(p.queue)
	for _, sb := range p.sandboxes {
		if sb.dead() || sb.stopping || sb.cmd == nil || sb.cmd.Process == nil {
			continue
		}
		s := snap{r: GvisorSandboxReceipt{Key: sb.key, Class: string(sb.class), ID: sb.id, StartedAt: sb.started.UTC().Format(time.RFC3339Nano), Workers: sb.workers, Served: sb.served},
			pid: sb.cmd.Process.Pid}
		if sb.cg != nil {
			s.cg = sb.cg.path
		}
		snaps = append(snaps, s)
	}
	p.mu.Unlock()
	if reason != "" {
		st.Reason = reason
	} else {
		st.Available = true
	}
	for _, s := range snaps {
		r := s.r
		r.HostPIDs = processTree(s.pid)
		for _, pid := range r.HostPIDs {
			r.RSSBytes += procKiB(fmt.Sprintf("/proc/%d/status", pid), "VmRSS:") << 10
			r.PSSBytes += procKiB(fmt.Sprintf("/proc/%d/smaps_rollup", pid), "Pss:") << 10
		}
		if s.cg != "" {
			r.Cgroup = s.cg
			if b, err := os.ReadFile(filepath.Join(s.cg, "memory.max")); err == nil {
				r.MemoryMax = strings.TrimSpace(string(b))
			}
			if b, err := os.ReadFile(filepath.Join(s.cg, "memory.current")); err == nil {
				r.MemoryCurrent = strings.TrimSpace(string(b))
			}
		}
		st.Sandboxes = append(st.Sandboxes, r)
	}
	sort.Slice(st.Sandboxes, func(i, j int) bool { return st.Sandboxes[i].Key < st.Sandboxes[j].Key })
	return st
}

// processTree lists root and its descendants, from /proc.
func processTree(root int) []int {
	children := map[int][]int{}
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(raw)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+1:])
		if len(f) < 2 {
			continue
		}
		ppid, _ := strconv.Atoi(f[1])
		children[ppid] = append(children[ppid], pid)
	}
	out := []int{root}
	for i := 0; i < len(out); i++ {
		out = append(out, children[out[i]]...)
	}
	sort.Ints(out)
	return out
}

// procKiB reads a "Key: N kB" line.
func procKiB(path, key string) uint64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, key) {
			f := strings.Fields(strings.TrimPrefix(line, key))
			if len(f) > 0 {
				n, _ := strconv.ParseUint(f[0], 10, 64)
				return n
			}
		}
	}
	return 0
}

// sandboxKeyID names the cell's sandbox: its class key and container id.
func (c *gvisorCell) sandboxKeyID() (string, string) { return c.sb.key, c.sb.id }

// queuedFor is how long the worker's spawn waited for desks memory.
func (c *gvisorCell) queuedFor() time.Duration {
	if c == nil {
		return 0
	}
	return c.queued
}

// heapShare is the heap the worker was admitted with.
func (c *gvisorCell) heapShare() uint64 { return c.shareBytes }

// cgroupPath is the cell's sandbox cgroup, "" without one.
func (c *gvisorCell) cgroupPath() string {
	if c == nil {
		return ""
	}
	c.sb.pool.mu.Lock()
	defer c.sb.pool.mu.Unlock()
	if c.sb.cg == nil {
		return ""
	}
	return c.sb.cg.path
}
