package vmctl

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// D6 S4, step 0 of make-room: before a refused start hibernates anyone, idle
// computers are asked to return their cold page cache
// (docs/design/density-d6-page-cache-return-2026-10-26.md section 4.4;
// docs/problems/density-station-d-failure-modes-2026-10-26.md D6, M12, M13).
//
// Ways it can fail:
//   - the return covers the shortfall and a computer is hibernated anyway;
//   - M13: a guest's reclaimed bytes are counted as room (pending return is
//     waited for and never counted: only a fresh MemAvailable sample admits);
//   - M13: step 0 overruns its one deadline (calls, settles and samples
//     share it), starts an act with no time left, or goes on after the start
//     was cancelled, or hibernates for a start nobody waits for;
//   - M13: two refused starts squeeze the same computer twice;
//   - M12: the paper, a public_platform computer or a held one is asked;
//   - the pressure trigger off, or observe, or off, still calls a guest or
//     changes what D1 does;
//   - the ranking asks the wrong computers first, or one with no excess.

type mrGuest struct {
	srv    *httptest.Server
	calls  atomic.Int32
	mu     sync.Mutex
	delay  time.Duration
	effect func()
	report string
	order  *crOrder
	name   string
	bodies []string
}

type crOrder struct {
	mu sync.Mutex
	v  []string
}

func (o *crOrder) add(s string) {
	o.mu.Lock()
	o.v = append(o.v, s)
	o.mu.Unlock()
}

func (o *crOrder) get() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.Join(o.v, ",")
}

func newMRGuest(t *testing.T, name string, order *crOrder) *mrGuest {
	g := &mrGuest{name: name, order: order,
		report: `{"outcome":"returned","method":"memory_reclaim","target_bytes":1073741824,"reclaimed_bytes":1073741824,"steps":16,"ms":300}`}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		g.mu.Lock()
		g.bodies = append(g.bodies, string(raw))
		delay, effect, report := g.delay, g.effect, g.report
		g.mu.Unlock()
		if g.order != nil {
			g.order.add(g.name)
		}
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		if effect != nil {
			effect()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, report)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

type stepZeroFixture struct {
	*makeRoomFixture
}

// pressureCR configures step 0 on the fixture: active, the pressure trigger
// only, a budget of budget, the guest RSS read from the fake manager, and
// the settle sleeping nothing.
func pressureCR(t *testing.T, budget time.Duration, mutate ...func(*GuestCacheReturnConfig)) *stepZeroFixture {
	t.Helper()
	f := newMakeRoomFixture(t, makeRoomAdmission())
	cfg := GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"pressure"},
		Scope: "daemon", Idle: 3 * time.Minute, PressureIdle: time.Minute, PressureBudget: budget}
	for _, m := range mutate {
		m(&cfg)
	}
	f.reg.SetGuestCacheReturnConfig(cfg)
	f.reg.cacheReturn.rss = func(vmID string) (cacheReturnRSS, bool) {
		f.mgr.rmu.Lock()
		defer f.mgr.rmu.Unlock()
		v, ok := f.mgr.rss[vmID]
		return cacheReturnRSS{Total: v, Anon: v / 2, File: v / 2}, ok
	}
	f.reg.cacheReturn.avail = func() uint64 { f.mu.Lock(); defer f.mu.Unlock(); return f.avail }
	f.reg.cacheReturn.sleep = func(time.Duration) {}
	return &stepZeroFixture{f}
}

// computer adds a running idle computer behind g with the given resident and
// D4-settled memory (settled 0 is unmeasured).
func (f *stepZeroFixture) computer(vmID string, idle time.Duration, rss, settled uint64, g *mrGuest) *VMOwnership {
	own := f.activeComputer("user-"+vmID, vmID, idle, rss)
	f.reg.mu.Lock()
	own.ComputerURL = g.srv.URL
	own.MemorySettledRSSBytes = settled
	f.reg.mu.Unlock()
	return own
}

func (f *stepZeroFixture) starter() *VMOwnership { return hibernated("b-at-c", "vm-bc", 7847*mib) }

// shortBy sets MemAvailable so the starter is short by short.
func (f *stepZeroFixture) shortBy(short uint64) {
	f.setAvail(4*gib + 1*gib + 7847*mib - short)
}

// returns makes a guest's act free freed bytes of host memory: its RSS falls
// and MemAvailable rises, as free page reporting does.
func (f *stepZeroFixture) returns(g *mrGuest, vmID string, rssAfter, freed uint64) {
	g.effect = func() {
		f.mgr.setRSS(vmID, rssAfter)
		f.addAvail(freed)
	}
}

func claim(bytes uint64) string {
	return `{"outcome":"returned","method":"memory_reclaim","target_bytes":` + strconv.FormatUint(bytes, 10) + `,"reclaimed_bytes":` + strconv.FormatUint(bytes, 10) + `,"steps":8,"ms":100}`
}

// A shortfall a return covers is admitted with no hibernate, naming the
// computer and its RSS fall.
func TestPressureCacheReturnAdmitsWithoutHibernating(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	g := newMRGuest(t, "de", nil)
	f.computer("vm-de", 40*time.Minute, 2*gib, 1*gib, g)
	f.returns(g, "vm-de", 1*gib, 1*gib)
	f.shortBy(448 * mib)

	ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
	if err != nil || ticket == nil {
		t.Fatalf("a start the return covers was refused: %v", err)
	}
	defer ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernated %v although the return covered the shortfall", got)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "admit_after_cache_return" || len(d.CacheReturned) != 1 || d.CacheReturned[0].VMID != "vm-de" ||
		d.CacheReturned[0].RSSFallMiB != 1024 || len(d.MadeRoomVMIDs) != 0 {
		t.Fatalf("decision = %+v, want admit_after_cache_return naming vm-de and a 1024 MiB fall", d)
	}
	if g.calls.Load() != 1 || !strings.Contains(g.bodies[0], `"mode":"idle"`) {
		t.Fatalf("guest calls = %d bodies = %v, want one with mode idle", g.calls.Load(), g.bodies)
	}
	if recs := f.reg.GuestCacheReturnSummary().Recent; len(recs) != 1 || recs[0].Trigger != "pressure" {
		t.Fatalf("records = %+v, want one pressure act", recs)
	}
}

// M13: the guest's claim is not room. It reclaims 2 GiB on paper, the host
// sees nothing: step 0 does not admit, D1's plan runs and hibernates.
func TestPressureCacheReturnNeverCountsAGuestsClaim(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	g := newMRGuest(t, "de", nil)
	g.report = claim(2 * gib)
	f.computer("vm-de", 40*time.Minute, 2*gib, 1*gib, g)
	f.shortBy(448 * mib)

	ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
	if err != nil || ticket == nil {
		t.Fatalf("start refused although D1 could hibernate an idle computer: %v", err)
	}
	defer ticket.finish(true)
	if g.calls.Load() != 1 {
		t.Fatalf("guest calls = %d, want 1", g.calls.Load())
	}
	if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-de" {
		t.Fatalf("hibernates = %v, want D1's plan to hibernate vm-de", got)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "admit_after_make_room" {
		t.Fatalf("decision = %+v, want admit_after_make_room", d)
	}
	// The act is still on the decision's record.
	if len(d.CacheReturned) != 1 || d.CacheReturned[0].VMID != "vm-de" || d.CacheReturned[0].RSSFallMiB != 0 {
		t.Fatalf("decision cache_returned = %+v, want vm-de with no fall", d.CacheReturned)
	}
}

// A return that helps but does not cover the shortfall is followed by D1's
// plan, on a fresh sample.
func TestPressureCacheReturnThatFallsShortLeavesTheRestToMakeRoom(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	g := newMRGuest(t, "de", nil)
	g.report = claim(300 * mib)
	f.computer("vm-de", 40*time.Minute, 2*gib, 1*gib, g)
	f.returns(g, "vm-de", 2*gib-300*mib, 300*mib)
	f.shortBy(2 * gib)

	ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
	if err != nil || ticket == nil {
		t.Fatalf("refused: %v", err)
	}
	defer ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 1 {
		t.Fatalf("hibernates = %v, want one: 300 MiB does not cover 2 GiB", got)
	}
}

// Step 0 ends by its one deadline, and starts no act it has no time for:
// acts of 400 ms under a 1 s budget start at 0 and 400 ms, not at 800 ms
// (a quarter of the budget must remain).
func TestPressureCacheReturnStaysInsideItsDeadline(t *testing.T) {
	budget := 1000 * time.Millisecond
	f := pressureCR(t, budget)
	var guests []*mrGuest
	for i, id := range []string{"vm-a", "vm-b", "vm-c"} {
		g := newMRGuest(t, id, nil)
		g.delay = 400 * time.Millisecond
		g.report = claim(10 * mib) // too little to settle for
		f.computer(id, time.Duration(40+i)*time.Minute, 2*gib, 1*gib, g)
		guests = append(guests, g)
	}
	f.shortBy(5 * gib)
	started := time.Now()
	f.reg.mu.RLock()
	cfg, acfg, sampler := f.reg.pressureReclaim, f.reg.hostAdmission, f.reg.pressureSampler
	f.reg.mu.RUnlock()
	ticket, _, attempted, _ := f.reg.makeRoomCacheReturn(context.Background(), f.starter(), WarmnessClassPrimary, cfg, acfg, sampler, f.reg.guestCacheReturnConfig())
	elapsed := time.Since(started)
	if ticket != nil {
		ticket.finish(true)
		t.Fatal("admitted without room")
	}
	if !attempted {
		t.Fatal("step 0 did not try")
	}
	var n int32
	for _, g := range guests {
		n += g.calls.Load()
	}
	if n != 2 {
		t.Fatalf("%d acts started, want 2 (the third would have under a quarter of the budget left)", n)
	}
	if elapsed > budget+300*time.Millisecond {
		t.Fatalf("step 0 took %s, deadline %s", elapsed, budget)
	}
}

// M13: a guest that never answers cannot hold step 0 past its deadline.
func TestPressureCacheReturnDeadlineCutsAHangingGuest(t *testing.T) {
	budget := 500 * time.Millisecond
	f := pressureCR(t, budget)
	g := newMRGuest(t, "hang", nil)
	g.delay = time.Hour
	f.computer("vm-hang", 40*time.Minute, 2*gib, 1*gib, g)
	f.shortBy(1 * gib)
	started := time.Now()
	ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
	if ticket != nil {
		ticket.finish(true)
	}
	_ = err
	if elapsed := time.Since(started); elapsed > budget+2*time.Second {
		t.Fatalf("the start took %s with a %s step 0 budget", elapsed, budget)
	}
	recs := f.reg.GuestCacheReturnSummary().Recent
	if len(recs) != 1 || recs[0].Outcome != "host_timeout" {
		t.Fatalf("records = %+v, want one host_timeout", recs)
	}
}

// A cancelled start stops step 0 at once and hibernates nothing.
func TestPressureCacheReturnStopsWhenTheStartIsCancelled(t *testing.T) {
	f := pressureCR(t, 10*time.Second)
	g := newMRGuest(t, "slow", nil)
	g.delay = time.Hour
	f.computer("vm-slow", 40*time.Minute, 2*gib, 1*gib, g)
	f.computer("vm-other", 41*time.Minute, 2*gib, 1*gib, newMRGuest(t, "other", nil))
	f.shortBy(1 * gib)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		ticket, err := f.reg.admitHostMemoryCtx(ctx, f.starter(), WarmnessClassPrimary)
		if ticket != nil {
			ticket.finish(true)
		}
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for g.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if g.calls.Load() == 0 {
		t.Fatal("step 0 never reached the guest")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "cancel") {
			t.Fatalf("err = %v, want a cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a cancelled start kept step 0 going")
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("a cancelled start hibernated %v", got)
	}
}

// M13: two refused starts at once. Their step 0s serialize behind the
// make-room slot and no computer receives two acts.
func TestPressureCacheReturnSerializesAndNeverSqueezesOneComputerTwice(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	g := newMRGuest(t, "de", nil)
	g.report = claim(10 * mib)
	f.computer("vm-de", 40*time.Minute, 2*gib, 1*gib, g)
	f.shortBy(448 * mib)
	var wg sync.WaitGroup
	for _, s := range []*VMOwnership{hibernated("b-at-c", "vm-bc", 7847*mib), hibernated("c-at-d", "vm-cd", 7847*mib)} {
		wg.Add(1)
		go func(starter *VMOwnership) {
			defer wg.Done()
			if ticket, _ := f.reg.admitHostMemory(starter, WarmnessClassPrimary); ticket != nil {
				ticket.finish(true)
			}
		}(s)
	}
	wg.Wait()
	if n := g.calls.Load(); n != 1 {
		t.Fatalf("vm-de received %d acts from two refused starts, want 1", n)
	}
}

// M12: the paper, a public platform computer and a held one are never asked;
// premium_always_on is.
func TestPressureCacheReturnNeverAsksProtectedOrHeldButAsksPremium(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	policy := DefaultWarmnessPolicyConfig()
	policy.PaidUserIDs = map[string]bool{"user-vm-premium": true}
	f.reg.SetWarmnessPolicyConfig(policy)
	forbidden := func(id string) *mrGuest {
		g := newMRGuest(t, id, nil)
		g.delay = 0
		return g
	}
	paperG, publicG, heldG, premG := forbidden("paper"), forbidden("public"), forbidden("held"), forbidden("premium")
	paper := f.computer("vm-paper", 40*time.Minute, 4*gib, 1*gib, paperG)
	public := f.computer("vm-public", 40*time.Minute, 4*gib, 1*gib, publicG)
	held := f.computer("vm-held", 40*time.Minute, 4*gib, 1*gib, heldG)
	f.computer("vm-premium", 40*time.Minute, 4*gib, 1*gib, premG)
	f.reg.mu.Lock()
	paper.WarmnessClass = WarmnessClassCriticalProtected
	public.WarmnessClass = WarmnessClassPublicPlatform
	held.HoldStatus = &MaintenanceHold{Reason: "maintenance"}
	f.reg.mu.Unlock()
	f.shortBy(5 * gib)
	f.reg.mu.RLock()
	cfg, acfg, sampler := f.reg.pressureReclaim, f.reg.hostAdmission, f.reg.pressureSampler
	f.reg.mu.RUnlock()
	ticket, _, _, _ := f.reg.makeRoomCacheReturn(context.Background(), f.starter(), WarmnessClassPrimary, cfg, acfg, sampler, f.reg.guestCacheReturnConfig())
	if ticket != nil {
		ticket.finish(true)
	}
	if paperG.calls.Load()+publicG.calls.Load()+heldG.calls.Load() != 0 {
		t.Fatalf("a protected or held computer was asked: paper=%d public=%d held=%d", paperG.calls.Load(), publicG.calls.Load(), heldG.calls.Load())
	}
	if premG.calls.Load() != 1 {
		t.Fatalf("the premium_always_on computer was asked %d times, want 1", premG.calls.Load())
	}
}

// Off, observe, and a trigger list without pressure leave D1 exactly as it
// was and call no guest.
func TestPressureCacheReturnIsOffUnlessActiveWithThePressureTrigger(t *testing.T) {
	for name, mutate := range map[string]func(*GuestCacheReturnConfig){
		"off":              func(c *GuestCacheReturnConfig) { c.Mode = GuestCacheReturnOff },
		"observe":          func(c *GuestCacheReturnConfig) { c.Mode = GuestCacheReturnObserve },
		"no pressure":      func(c *GuestCacheReturnConfig) { c.Triggers = []string{"idle", "hibernate"} },
		"nobody selected":  func(c *GuestCacheReturnConfig) { c.Only = nil },
		"a typo of active": func(c *GuestCacheReturnConfig) { c.Mode = "Active" },
	} {
		f := pressureCR(t, 5*time.Second, mutate)
		g := newMRGuest(t, "de", nil)
		f.computer("vm-de", 40*time.Minute, 2*gib, 1*gib, g)
		f.shortBy(448 * mib)
		ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
		if err != nil || ticket == nil {
			t.Fatalf("%s: D1 did not make room: %v", name, err)
		}
		ticket.finish(true)
		if g.calls.Load() != 0 {
			t.Fatalf("%s: a guest was called", name)
		}
		if d := lastDecision(t, f.reg); d.Decision != "admit_after_make_room" || len(d.CacheReturned) != 0 {
			t.Fatalf("%s: decision = %+v, want D1's unchanged", name, d)
		}
	}
}

// Ranking: biggest excess first (latest RSS less D4's settled RSS), under
// 256 MiB skipped, unmeasured last; at most three.
func TestPressureCacheReturnRanksByHostMeasuredExcess(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	order := &crOrder{}
	mk := func(id string, rss, settled uint64) {
		g := newMRGuest(t, id, order)
		g.report = claim(10 * mib)
		f.computer(id, 40*time.Minute, rss, settled, g)
	}
	mk("small", 1*gib+100*mib, 1*gib)  // excess 100 MiB: skipped
	mk("medium", 1*gib+300*mib, 1*gib) // excess 300 MiB
	mk("large", 3*gib, 1*gib)          // excess 2 GiB
	mk("unmeasured", 2*gib, 0)         // no settled RSS: last
	mk("exact", 1*gib+256*mib, 1*gib)  // excess exactly 256 MiB: kept, but past the cap of three
	f.shortBy(9 * gib)
	f.reg.mu.RLock()
	cfg, acfg, sampler := f.reg.pressureReclaim, f.reg.hostAdmission, f.reg.pressureSampler
	f.reg.mu.RUnlock()
	ticket, _, _, _ := f.reg.makeRoomCacheReturn(context.Background(), f.starter(), WarmnessClassPrimary, cfg, acfg, sampler, f.reg.guestCacheReturnConfig())
	if ticket != nil {
		ticket.finish(true)
	}
	if got := order.get(); got != "large,medium,exact" {
		t.Fatalf("asked %q, want large,medium,exact (small has no excess, unmeasured ranks after them, three at most)", got)
	}
}

// The unmeasured computer is asked when nothing better is left.
func TestPressureCacheReturnAsksAnUnmeasuredComputerLast(t *testing.T) {
	f := pressureCR(t, 5*time.Second)
	order := &crOrder{}
	g1 := newMRGuest(t, "measured", order)
	g1.report = claim(10 * mib)
	g2 := newMRGuest(t, "unmeasured", order)
	g2.report = claim(10 * mib)
	f.computer("vm-m", 40*time.Minute, 3*gib, 1*gib, g1)
	f.computer("vm-u", 90*time.Minute, 2*gib, 0, g2)
	f.shortBy(9 * gib)
	f.reg.mu.RLock()
	cfg, acfg, sampler := f.reg.pressureReclaim, f.reg.hostAdmission, f.reg.pressureSampler
	f.reg.mu.RUnlock()
	ticket, _, _, _ := f.reg.makeRoomCacheReturn(context.Background(), f.starter(), WarmnessClassPrimary, cfg, acfg, sampler, f.reg.guestCacheReturnConfig())
	if ticket != nil {
		ticket.finish(true)
	}
	if got := order.get(); got != "measured,unmeasured" {
		t.Fatalf("asked %q, want measured,unmeasured", got)
	}
}

// The candidate rule under pressure: the idle floor is the pressure floor, a
// computer acted on in this idle period is still a candidate, and one acted
// on in the last minute is not.
func TestPressureCandidatesUseThePressureFloorAndASixtySecondRepeatGuard(t *testing.T) {
	f := newCandidateFixture()
	f.in.Pressure = true
	f.in.IdleFloor = time.Minute
	f.in.SweepHibernates = true
	young := crOwn("young", 30*time.Second)
	ok := crOwn("ok", 90*time.Second)
	acted := crOwn("acted", 10*time.Minute)
	f.acted["acted"] = crNow.Add(-10 * time.Minute) // after its last activity? no: before now, after LastActive (-10m exact)
	repeat := crOwn("repeat", 10*time.Minute)
	f.acted["repeat"] = crNow.Add(-30 * time.Second)
	got := strings.Join(f.pick(young, ok, acted, repeat), ",")
	if got != "acted,ok" {
		t.Fatalf("candidates = %s, want acted (acted before, not in the last minute) and ok", got)
	}
}
