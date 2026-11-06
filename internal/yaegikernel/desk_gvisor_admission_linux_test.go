//go:build linux

package yaegikernel

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The admission queue (MA6.4 r3), without runsc: a spawn that does not fit
// waits; reaps wake waiters in order, class a first; a deadline refuses by
// name; a spawn that can never fit is refused at once. Ways it could fail:
// a waiter never woken (a lost grant), woken out of order (a burst of class
// b starving class a), granted twice or after it left, the committed sum
// drifting, a race between the deadline and a grant (run under -race).

func admissionTestPool(t *testing.T) *gvisorPool {
	p := newGvisorPool()
	p.base = t.TempDir()
	return p
}

func admissionWaiter(class DeskTrustClass, key string, claim, desksMax uint64) *gvisorWaiter {
	cls := deskSandboxClass{Class: class, Key: key}
	return &gvisorWaiter{classA: class == TrustClassFirstPartyCustodian, cls: cls, mapKey: key + "\x00bin", bin: "bin", worker: "w", claim: claim, desksMax: desksMax}
}

type admitResult struct {
	name   string
	g      gvisorGrant
	queued time.Duration
	err    error
}

func admitAsync(ctx context.Context, p *gvisorPool, name string, w *gvisorWaiter, out chan<- admitResult) {
	go func() {
		g, q, err := p.admit(ctx, w, name)
		out <- admitResult{name, g, q, err}
	}()
}

// releaseGrant gives a granted worker's claim back as a reap does.
func releaseGrant(g gvisorGrant, claim uint64) {
	(&gvisorCell{sb: g.sb, claimBytes: claim}).unclaim(false, false)
}

func waitQueued(t *testing.T, p *gvisorPool, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		p.mu.Lock()
		got := len(p.queue)
		p.mu.Unlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("queue holds %d, want %d", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func expectGrant(t *testing.T, out <-chan admitResult, want string) admitResult {
	t.Helper()
	select {
	case r := <-out:
		if r.err != nil || r.name != want {
			t.Fatalf("got %s (err %v), want a grant for %s", r.name, r.err, want)
		}
		return r
	case <-time.After(5 * time.Second):
		t.Fatalf("no grant, want %s", want)
	}
	return admitResult{}
}

func expectNone(t *testing.T, out <-chan admitResult) {
	t.Helper()
	select {
	case r := <-out:
		t.Fatalf("unexpected result %s (err %v)", r.name, r.err)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestGvisorAdmissionWaitsAndWakesInOrder(t *testing.T) {
	SetDeskMemoryPressureHandler(nil)
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks := uint64(1000 * mib)
	claim := uint64(250 * mib)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Three class-b workers fill the desks memory: 96 + 3 x 250 = 846.
	var held []gvisorGrant
	for i := 0; i < 3; i++ {
		g, q, err := p.admit(ctx, admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
		if err != nil || q != 0 {
			t.Fatalf("worker %d: %v (queued %s)", i, err, q)
		}
		held = append(held, g)
	}
	out := make(chan admitResult, 8)
	// Two research children, then management: management arrives last.
	admitAsync(ctx, p, "child1", admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), out)
	waitQueued(t, p, 1)
	admitAsync(ctx, p, "child2", admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), out)
	waitQueued(t, p, 2)
	admitAsync(ctx, p, "management", admissionWaiter(TrustClassFirstPartyCustodian, "a", claim, desks), out)
	waitQueued(t, p, 3)
	expectNone(t, out)

	// One reap frees 250: management (class a, a new sandbox: 346) is
	// admitted first though it came last; the children wait behind it.
	releaseGrant(held[0], claim)
	m := expectGrant(t, out, "management")
	if m.queued <= 0 {
		t.Fatalf("management queued %s, want the wait recorded", m.queued)
	}
	expectNone(t, out)
	// Each further reap admits the next child, in arrival order.
	releaseGrant(held[1], claim)
	c1 := expectGrant(t, out, "child1")
	expectNone(t, out)
	releaseGrant(held[2], claim)
	c2 := expectGrant(t, out, "child2")
	releaseGrant(c1.g, claim)
	releaseGrant(c2.g, claim)
	releaseGrant(m.g, claim)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed != 0 || len(p.queue) != 0 {
		t.Fatalf("committed %d, queue %d after every worker left", p.committed, len(p.queue))
	}
}

func TestGvisorAdmissionDeadlineRefusesByName(t *testing.T) {
	SetDeskMemoryPressureHandler(nil)
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks, claim := uint64(500*mib), uint64(300*mib)
	g, _, err := p.admit(context.Background(), admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err = p.admit(ctx, admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research_web")
	if !errors.Is(err, errDesksMemoryCommitted) || time.Since(start) < 80*time.Millisecond {
		t.Fatalf("after %s: %v, want the desks-memory refusal at the deadline", time.Since(start), err)
	}
	p.mu.Lock()
	n := len(p.queue)
	p.mu.Unlock()
	if n != 0 {
		t.Fatalf("the refused waiter is still queued (%d)", n)
	}
	releaseGrant(g, claim)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed != 0 {
		t.Fatalf("committed %d: a refused waiter's claim leaked", p.committed)
	}
}

// TestGvisorAdmissionPressureEndsIdle: a waiting spawn asks the host to end
// an idle session, and the freed claim admits it without a reap of its own.
func TestGvisorAdmissionPressureEndsIdle(t *testing.T) {
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks, claim := uint64(500*mib), uint64(300*mib)
	idle, _, err := p.admit(context.Background(), admissionWaiter(TrustClassFirstPartyCustodian, "a", claim, desks), "texture")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	SetDeskMemoryPressureHandler(func() bool {
		calls++
		if calls > 1 {
			return false
		}
		releaseGrant(idle, claim)
		return true
	})
	defer SetDeskMemoryPressureHandler(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	g, q, err := p.admit(ctx, admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil || q <= 0 || calls != 1 {
		t.Fatalf("research: %v queued %s, pressure calls %d", err, q, calls)
	}
	releaseGrant(g, claim)
}

// TestGvisorAdmissionRefusesWhatNeverFits: a claim that alone exceeds the
// desks cgroup is refused at once, before any wait (and before runsc).
func TestGvisorAdmissionRefusesWhatNeverFits(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "memory.max"), []byte(strconv.Itoa(256<<20)), 0o644); err != nil {
		t.Fatal(err)
	}
	p := admissionTestPool(t)
	start := time.Now()
	_, err := p.attach(context.Background(), deskSandboxClass{Class: TrustClassFirstPartyUntrusted, Key: "b"}, "bin", "w", "research", 200000, 384<<20, root, true)
	if !errors.Is(err, errDesksMemoryNeverFits) || time.Since(start) > time.Second {
		t.Fatalf("after %s: %v, want the never-fits refusal at once", time.Since(start), err)
	}
	t.Logf("refused at once: %v", err)
}

// MA6.4 r4. Ways the wait could still fail: a waiter that found no idle
// session at enqueue is never told when one goes idle (it waits for
// retention); a waiter behind the head, or past its deadline, ends idle
// sessions it cannot use; a reaped worker whose end was not confirmed gives
// back memory it may still hold; a sandbox's overhead is granted while the
// sandbox is still stopping; a class whose sandbox failed keeps its queued
// waiters retrying a broken runsc.

// TestGvisorAdmissionIdleNotifyWakesHead: the head waiter asks again for an
// idle session when one goes idle, and is admitted before its deadline.
func TestGvisorAdmissionIdleNotifyWakesHead(t *testing.T) {
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks, claim := uint64(500*mib), uint64(300*mib)
	busy, _, err := p.admit(context.Background(), admissionWaiter(TrustClassFirstPartyCustodian, "a", claim, desks), "texture")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	idle := false
	calls := 0
	SetDeskMemoryPressureHandler(func() bool {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if !idle {
			return false
		}
		idle = false
		releaseGrant(busy, claim)
		return true
	})
	defer SetDeskMemoryPressureHandler(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out := make(chan admitResult, 1)
	admitAsync(ctx, p, "research", admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), out)
	waitQueued(t, p, 1)
	expectNone(t, out)
	// The texture session's cell ends: it is idle now.
	mu.Lock()
	idle = true
	mu.Unlock()
	p.notifyIdle()
	r := expectGrant(t, out, "research")
	if r.queued <= 0 {
		t.Fatalf("queued %s", r.queued)
	}
	releaseGrant(r.g, claim)
}

// TestGvisorAdmissionEvictsOnlyForLiveHead: only the head of the queue ends
// idle sessions, and a waiter past its deadline ends none.
func TestGvisorAdmissionEvictsOnlyForLiveHead(t *testing.T) {
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks, claim := uint64(500*mib), uint64(300*mib)
	held, _, err := p.admit(context.Background(), admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	SetDeskMemoryPressureHandler(func() bool { calls.Add(1); return false })
	defer SetDeskMemoryPressureHandler(nil)

	// A waiter whose deadline has passed is refused without ending anything.
	expired, cancelExpired := context.WithCancel(context.Background())
	cancelExpired()
	if _, _, err := p.admit(expired, admissionWaiter(TrustClassFirstPartyCustodian, "a", claim, desks), "management"); !errors.Is(err, errDesksMemoryCommitted) {
		t.Fatalf("expired waiter: %v", err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("an expired waiter ended idle sessions (%d calls)", n)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := make(chan admitResult, 2)
	admitAsync(ctx, p, "management", admissionWaiter(TrustClassFirstPartyCustodian, "a", claim, desks), out)
	waitQueued(t, p, 1)
	admitAsync(ctx, p, "research_web", admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), out)
	waitQueued(t, p, 2)
	time.Sleep(20 * time.Millisecond)
	base := calls.Load()
	const pokes = 5
	for i := 0; i < pokes; i++ {
		p.notifyIdle()
		time.Sleep(20 * time.Millisecond)
	}
	if n := calls.Load() - base; n < 1 || n > pokes {
		t.Fatalf("%d pressure calls for %d idle notices: want only the head's (1 to %d)", n, pokes, pokes)
	}
	releaseGrant(held, claim)
	m := expectGrant(t, out, "management")
	releaseGrant(m.g, claim)
	c := expectGrant(t, out, "research_web")
	releaseGrant(c.g, claim)
}

// TestGvisorAdmissionHeldClaimUntilSandboxStops: an unconfirmed worker's
// claim stays committed until its sandbox stops, and the sandbox's overhead
// and that claim reach a waiter only after the stop returns.
func TestGvisorAdmissionHeldClaimUntilSandboxStops(t *testing.T) {
	SetDeskMemoryPressureHandler(nil)
	p := admissionTestPool(t)
	// runsc here stands in for a slow `runsc delete`: the stop takes 300 ms.
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary")
	}
	fake := filepath.Join(t.TempDir(), "runsc")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"+sleep+" 0.3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p.runsc = fake
	// A script just written can be busy for exec (ETXTBSY) while another
	// fork holds its fd: wait until it runs for its full 300 ms.
	for i := 0; ; i++ {
		start := time.Now()
		out, err := p.runscCmd(5*time.Second, "delete")
		if err == nil && time.Since(start) >= 250*time.Millisecond {
			break
		}
		if i > 20 {
			t.Fatalf("the stand-in runsc does not run: %v %s", err, out)
		}
		time.Sleep(20 * time.Millisecond)
	}
	const mib = 1 << 20
	desks, claim := uint64(1000*mib), uint64(300*mib)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Two workers in one class-b sandbox: 96 + 600 = 696.
	g1, _, err := p.admit(ctx, admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil {
		t.Fatal(err)
	}
	g2, _, err := p.admit(ctx, admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil || g1.sb != g2.sb {
		t.Fatalf("second worker: %v (same sandbox %v)", err, g1.sb == g2.sb)
	}
	// A class-a spawn needs 96 + 300 + 300 = 696 with a second worker of its
	// own: it waits until both b claims and b's overhead are free.
	out := make(chan admitResult, 2)
	admitAsync(ctx, p, "management", admissionWaiter(TrustClassFirstPartyCustodian, "a", 2*claim, desks), out)
	waitQueued(t, p, 1)

	// Worker 1's kill was not confirmed: its claim stays.
	(&gvisorCell{sb: g1.sb, claimBytes: claim}).unclaim(false, true)
	p.mu.Lock()
	committed := p.committed
	p.mu.Unlock()
	if committed != gvisorSandboxOverheadBytes+2*claim {
		t.Fatalf("committed %d MiB after an unconfirmed reap, want the claim held (%d MiB)", committed>>20, (gvisorSandboxOverheadBytes+2*claim)>>20)
	}
	expectNone(t, out)

	// Worker 2 leaves confirmed: the sandbox stops; nothing is granted until
	// the stop returns.
	stopped := make(chan time.Time, 1)
	go func() {
		(&gvisorCell{sb: g2.sb, claimBytes: claim}).unclaim(false, false)
		stopped <- time.Now()
	}()
	time.Sleep(100 * time.Millisecond)
	p.mu.Lock()
	during := p.committed
	p.mu.Unlock()
	if during < gvisorSandboxOverheadBytes+claim {
		t.Fatalf("committed %d MiB while the sandbox stops: its overhead and the held claim went back early", during>>20)
	}
	expectNone(t, out)
	m := expectGrant(t, out, "management")
	end := <-stopped
	_ = end
	p.mu.Lock()
	after := p.committed
	p.mu.Unlock()
	if after != gvisorSandboxOverheadBytes+2*claim {
		t.Fatalf("committed %d MiB after the stop, want only management's %d MiB", after>>20, (gvisorSandboxOverheadBytes+2*claim)>>20)
	}
	releaseGrant(m.g, 2*claim)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed != 0 {
		t.Fatalf("committed %d at the end", p.committed)
	}
}

// TestGvisorAdmissionFailedClassRefusesQueued: when a class's sandbox fails
// to start, its queued waiters are refused with the reason at once.
func TestGvisorAdmissionFailedClassRefusesQueued(t *testing.T) {
	SetDeskMemoryPressureHandler(nil)
	p := admissionTestPool(t)
	const mib = 1 << 20
	desks, claim := uint64(500*mib), uint64(300*mib)
	held, _, err := p.admit(context.Background(), admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), "research")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := make(chan admitResult, 1)
	admitAsync(ctx, p, "research_web", admissionWaiter(TrustClassFirstPartyUntrusted, "b", claim, desks), out)
	waitQueued(t, p, 1)
	p.mu.Lock()
	p.failed["b"] = gvisorFailure{reason: "gVisor sandbox for class b did not start: test", until: time.Now().Add(time.Minute)}
	p.grantLocked()
	p.mu.Unlock()
	select {
	case r := <-out:
		if r.err == nil || errors.Is(r.err, errDesksMemoryCommitted) {
			t.Fatalf("got %v, want the class failure", r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the queued waiter of a failed class kept waiting")
	}
	releaseGrant(held, claim)
}
