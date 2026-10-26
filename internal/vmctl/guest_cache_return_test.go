package vmctl

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// D6 S2, the vmctl idle trigger for the guest cache return act
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.2, 4.5,
// 4.6, 9 S2; docs/problems/density-station-d-failure-modes-2026-10-26.md D6).
//
// Ways it can fail, written first:
//   - M16: a typo, a case change or a near word turns it on;
//   - M1: a busy computer, or one idle under 3 minutes, is acted on; the
//     guest is probed before the cheap rules have filtered it;
//   - M6: a computer D4 has not captured the settled RSS of is acted on
//     (open reservation, or an ended window still awaiting capture), so D4
//     under-charges every later wake;
//   - M10: the same computer is acted on every sweep pass; a premium
//     computer (never idle-hibernated) is never acted on again;
//   - M12: the paper (critical_protected), a public_platform computer or a
//     held computer is acted on;
//   - M24: a computer with no ComputerURL is called;
//   - M15: with the switch off a guest is called, or a rollback to off needs
//     more than the switch;
//   - observe really acts (no dry_run);
//   - M9: an old guest without the route is retried every sweep, or never
//     again after it is rebooted onto a new image;
//   - M19: the request carries a credential to the guest;
//   - M11: an act races a lifecycle claim, runs twice on one computer, or
//     more than two run at once;
//   - M17: the guest's own report drives a decision;
//   - M14: a restart (a new registry) resumes or replays an act;
//   - a host timeout leaves the in-flight entry for ever.

const crMiB = uint64(1 << 20)

func TestGuestCacheReturnModeFailsClosed(t *testing.T) {
	for value, want := range map[string]string{
		"active": GuestCacheReturnActive, "observe": GuestCacheReturnObserve, "off": GuestCacheReturnOff,
		"Active": GuestCacheReturnOff, "ACTIVE": GuestCacheReturnOff, "on": GuestCacheReturnOff,
		"true": GuestCacheReturnOff, "1": GuestCacheReturnOff, "actve": GuestCacheReturnOff,
		"observe ": GuestCacheReturnOff, " active": GuestCacheReturnOff, "": GuestCacheReturnOff,
		"yes": GuestCacheReturnOff, "enabled": GuestCacheReturnOff,
	} {
		if got := NormalizeGuestCacheReturnMode(value); got != want {
			t.Errorf("mode %q = %q, want %q", value, got, want)
		}
	}
	if def := DefaultGuestCacheReturnConfig(); def.Mode != GuestCacheReturnOff || len(def.Only) != 0 {
		t.Fatalf("default config = %+v, want off and nobody selected", def)
	}
}

func TestGuestCacheReturnConfigParsesAndClamps(t *testing.T) {
	cfg := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{
		Mode: "observe", Only: " comp-a, class:test ,,*", Triggers: "idle, hibernate,bogus",
		Scope: "root", Idle: "5m", PressureIdle: "30s", PressureBudget: "20s",
	})
	if cfg.Mode != GuestCacheReturnObserve || strings.Join(cfg.Only, "|") != "comp-a|class:test|*" {
		t.Fatalf("mode/selector = %+v", cfg)
	}
	if !cfg.TriggerOn("idle") || !cfg.TriggerOn("hibernate") || cfg.TriggerOn("pressure") || cfg.TriggerOn("bogus") {
		t.Fatalf("triggers = %v", cfg.Triggers)
	}
	if cfg.Scope != "root" || cfg.Idle != 5*time.Minute || cfg.PressureIdle != 30*time.Second || cfg.PressureBudget != 20*time.Second {
		t.Fatalf("scope/durations = %+v", cfg)
	}
	// Defaults.
	def := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Mode: "active"})
	if !def.TriggerOn("idle") || def.TriggerOn("hibernate") || def.TriggerOn("pressure") || def.Scope != "daemon" ||
		def.Idle != 3*time.Minute || def.PressureIdle != time.Minute || def.PressureBudget != 12*time.Second || len(def.Only) != 0 {
		t.Fatalf("defaults = %+v", def)
	}
	// Clamps: 1m to 9m, 0 to 9m, 2s to 30s.
	lo := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Idle: "1s", PressureIdle: "-1s", PressureBudget: "100ms"})
	if lo.Idle != time.Minute || lo.PressureIdle != 0 || lo.PressureBudget != 2*time.Second {
		t.Fatalf("lower clamps = %+v", lo)
	}
	hi := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Idle: "2h", PressureIdle: "1h", PressureBudget: "10m"})
	if hi.Idle != 9*time.Minute || hi.PressureIdle != 9*time.Minute || hi.PressureBudget != 30*time.Second {
		t.Fatalf("upper clamps = %+v", hi)
	}
	// Malformed values are the defaults, an unknown scope is daemon.
	bad := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Idle: "soon", PressureIdle: "x", PressureBudget: "y", Scope: "cgroup"})
	if bad.Idle != 3*time.Minute || bad.PressureIdle != time.Minute || bad.PressureBudget != 12*time.Second || bad.Scope != "daemon" {
		t.Fatalf("malformed = %+v", bad)
	}
	// A trigger list with no valid word selects no trigger (fails closed),
	// while an unset list is the default.
	none := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Triggers: "bogus"})
	if none.TriggerOn("idle") || none.TriggerOn("hibernate") || none.TriggerOn("pressure") {
		t.Fatalf("a list of unknown triggers selected %v", none.Triggers)
	}
}

// --- the candidate rule, pure ---------------------------------------------

var crNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type candidateFixture struct {
	in      cacheReturnCandidateInput
	probes  []string
	claimed map[string]bool
	open    map[string]bool
	acted   map[string]time.Time
	unsup   map[string]int64
	flying  map[string]bool
	busy    map[string]bool
}

func newCandidateFixture() *candidateFixture {
	f := &candidateFixture{claimed: map[string]bool{}, open: map[string]bool{}, acted: map[string]time.Time{},
		unsup: map[string]int64{}, flying: map[string]bool{}, busy: map[string]bool{}}
	f.in = cacheReturnCandidateInput{
		Now:       crNow,
		Config:    GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"idle"}},
		Policy:    DefaultWarmnessPolicyConfig(),
		IdleFloor: 3 * time.Minute,
		Limit:     100,
		Claimed:   func(o *VMOwnership) bool { return f.claimed[o.VMID] },
		Uncaptured: func(vmID string) bool {
			return f.open[vmID]
		},
		InFlight:    func(vmID string) bool { return f.flying[vmID] },
		LastAct:     func(vmID string) (time.Time, bool) { t, ok := f.acted[vmID]; return t, ok },
		Unsupported: func(vmID string, epoch int64) bool { e, ok := f.unsup[vmID]; return ok && e == epoch },
		Busy: func(o *VMOwnership) bool {
			f.probes = append(f.probes, o.VMID)
			return f.busy[o.VMID]
		},
	}
	return f
}

func crOwn(vmID string, idle time.Duration) *VMOwnership {
	return &VMOwnership{VMID: vmID, ComputerID: "comp-" + vmID, UserID: "user-" + vmID, DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, Epoch: 3,
		ComputerURL: "http://10.0.0.9:8085", LastActiveAt: crNow.Add(-idle)}
}

func (f *candidateFixture) pick(owns ...*VMOwnership) []string {
	f.in.Ownerships = owns
	f.probes = nil
	var ids []string
	for _, own := range cacheReturnCandidates(f.in) {
		ids = append(ids, own.VMID)
	}
	sort.Strings(ids)
	return ids
}

func TestCacheReturnCandidatesIdleFloorAndState(t *testing.T) {
	f := newCandidateFixture()
	hibernated := crOwn("hib", 10*time.Minute)
	hibernated.State = VMStateHibernated
	noActivity := crOwn("never", 0)
	noActivity.LastActiveAt = time.Time{}
	got := f.pick(crOwn("old", 4*time.Minute), crOwn("young", 2*time.Minute), crOwn("exact", 3*time.Minute), hibernated, noActivity)
	if strings.Join(got, ",") != "exact,old" {
		t.Fatalf("candidates = %v, want exact (idle exactly 3m) and old", got)
	}
}

// M12: the protected classes, a held computer, a computer under a lifecycle
// claim and one in flight are never candidates.
func TestCacheReturnCandidatesExcludeProtectedHeldClaimedAndInFlight(t *testing.T) {
	f := newCandidateFixture()
	paper := crOwn("paper", 10*time.Minute)
	paper.WarmnessClass = WarmnessClassCriticalProtected
	public := crOwn("public", 10*time.Minute)
	public.WarmnessClass = WarmnessClassPublicPlatform
	held := crOwn("held", 10*time.Minute)
	held.HoldStatus = &MaintenanceHold{Reason: "maintenance"}
	claimed := crOwn("claimed", 10*time.Minute)
	f.claimed["claimed"] = true
	flying := crOwn("flying", 10*time.Minute)
	f.flying["flying"] = true
	premium := crOwn("premium", 10*time.Minute)
	f.in.Policy = DefaultWarmnessPolicyConfig()
	f.in.Policy.PaidUserIDs = map[string]bool{"user-premium": true}
	plain := crOwn("plain", 10*time.Minute)
	got := f.pick(paper, public, held, claimed, flying, premium, plain)
	if strings.Join(got, ",") != "plain,premium" {
		t.Fatalf("candidates = %v, want plain and premium_always_on only", got)
	}
}

// M24: no ComputerURL, no call, and no probe either.
func TestCacheReturnCandidatesNeedAComputerURL(t *testing.T) {
	f := newCandidateFixture()
	noURL := crOwn("nourl", 10*time.Minute)
	noURL.ComputerURL = "  "
	got := f.pick(noURL, crOwn("url", 10*time.Minute))
	if strings.Join(got, ",") != "url" {
		t.Fatalf("candidates = %v, want url only", got)
	}
	for _, id := range f.probes {
		if id == "nourl" {
			t.Fatal("the guest with no address was probed")
		}
	}
}

// M24 pin: the shared probe answers busy for an empty URL (density D3a), so
// the rule above is a second layer, not the only one.
func TestSharedBusyProbeFailsClosedWithoutAComputerURL(t *testing.T) {
	if reading := httpGuestHealth()(&VMOwnership{VMID: "vm", ComputerURL: ""}); !reading.Busy {
		t.Fatal("httpGuestHealth answered idle for a computer with no ComputerURL")
	}
}

// M6: D4's capture comes first.
func TestCacheReturnCandidatesWaitForD4SettledCapture(t *testing.T) {
	f := newCandidateFixture()
	f.open["waiting"] = true // an open reservation, or an ended window awaiting capture
	got := f.pick(crOwn("waiting", 10*time.Minute), crOwn("captured", 10*time.Minute))
	if strings.Join(got, ",") != "captured" {
		t.Fatalf("candidates = %v, want the captured computer only", got)
	}
}

// M10: once per idle period; a premium computer, which the sweep never
// hibernates, again after 30 minutes.
func TestCacheReturnCandidatesActOncePerIdlePeriod(t *testing.T) {
	f := newCandidateFixture()
	f.in.SweepHibernates = true
	later := crOwn("later", 10*time.Minute) // active 10m ago
	f.acted["later"] = crNow.Add(-2 * time.Minute)
	earlier := crOwn("earlier", 4*time.Minute) // acted before its last activity
	f.acted["earlier"] = crNow.Add(-20 * time.Minute)
	fresh := crOwn("fresh", 10*time.Minute)
	got := f.pick(later, earlier, fresh)
	if strings.Join(got, ",") != "earlier,fresh" {
		t.Fatalf("candidates = %v, want earlier (new idle period) and fresh", got)
	}

	f = newCandidateFixture()
	f.in.SweepHibernates = true
	f.in.Policy.PaidUserIDs = map[string]bool{"user-p29": true, "user-p31": true}
	p29, p31 := crOwn("p29", 60*time.Minute), crOwn("p31", 60*time.Minute)
	f.acted["p29"] = crNow.Add(-29 * time.Minute)
	f.acted["p31"] = crNow.Add(-31 * time.Minute)
	got = f.pick(p29, p31)
	if strings.Join(got, ",") != "p31" {
		t.Fatalf("premium candidates = %v, want p31 only (re-armed every 30 minutes)", got)
	}
	// An ordinary computer acted on 31 minutes ago in the same idle period is
	// not re-armed: the sweep hibernates it.
	f = newCandidateFixture()
	f.in.SweepHibernates = true
	plain := crOwn("plain", 60*time.Minute)
	f.acted["plain"] = crNow.Add(-31 * time.Minute)
	if got = f.pick(plain); len(got) != 0 {
		t.Fatalf("an ordinary computer was re-armed: %v", got)
	}
}

// M9: unsupported until the next boot.
func TestCacheReturnCandidatesSkipUnsupportedUntilTheNextBoot(t *testing.T) {
	f := newCandidateFixture()
	f.unsup["old"] = 3
	got := f.pick(crOwn("old", 10*time.Minute))
	if len(got) != 0 {
		t.Fatalf("an unsupported guest was a candidate: %v", got)
	}
	rebooted := crOwn("old", 10*time.Minute)
	rebooted.Epoch = 4
	if got = f.pick(rebooted); len(got) != 1 {
		t.Fatalf("a new boot did not clear unsupported: %v", got)
	}
}

func TestCacheReturnCandidatesSelector(t *testing.T) {
	policy := DefaultWarmnessPolicyConfig()
	policy.PaidUserIDs = map[string]bool{"user-prem": true}
	policy.TestComputerIDs = map[string]bool{"comp-test": true}
	owns := func() []*VMOwnership {
		return []*VMOwnership{crOwn("plain", 10*time.Minute), crOwn("prem", 10*time.Minute), crOwn("test", 10*time.Minute)}
	}
	for only, want := range map[string]string{
		"":                    "",
		"comp-plain":          "plain",
		"vm-x":                "",
		"plain":               "plain", // a VM id
		"class:test":          "test",
		"class:primary":       "plain",
		"class:premium":       "prem",
		"*":                   "plain,prem,test",
		"comp-prem,comp-test": "prem,test",
		"class:Test":          "", // exact words
	} {
		f := newCandidateFixture()
		f.in.Policy = policy
		f.in.Config.Only = nil
		for _, tok := range strings.Split(only, ",") {
			if tok != "" {
				f.in.Config.Only = append(f.in.Config.Only, tok)
			}
		}
		if got := strings.Join(f.pick(owns()...), ","); got != want {
			t.Errorf("selector %q picked %q, want %q", only, got, want)
		}
	}
}

// The probe is the last rule: called only for computers that passed every
// other rule, and no more than Limit of them pass.
func TestCacheReturnCandidatesProbeLastAndLimit(t *testing.T) {
	f := newCandidateFixture()
	f.busy["a-busy"] = true
	f.in.Limit = 2
	f.claimed["claimed"] = true
	f.open["open"] = true
	got := f.pick(
		crOwn("claimed", 10*time.Minute), crOwn("open", 10*time.Minute), crOwn("young", time.Minute),
		crOwn("a", 10*time.Minute), crOwn("a-busy", 10*time.Minute), crOwn("b", 10*time.Minute), crOwn("c", 10*time.Minute),
	)
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("candidates = %v, want a and b (Limit 2, busy skipped)", got)
	}
	for _, id := range f.probes {
		switch id {
		case "claimed", "open", "young":
			t.Fatalf("%s was probed although it failed a cheaper rule (probes %v)", id, f.probes)
		}
	}
	if got := strings.Join(f.probes, ","); got != "a,a-busy,b" {
		t.Fatalf("probes = %s, want a,a-busy,b: stop probing once the limit is met", got)
	}
}

// --- the settle wait ------------------------------------------------------

func TestWaitRSSFallWaitsForTheFallNotForStability(t *testing.T) {
	// Flat for 3 s (12 polls), then falling by half the reclaimed bytes.
	polls := 0
	read := func() (cacheReturnRSS, bool) {
		polls++
		if polls <= 12 {
			return cacheReturnRSS{Total: 4000 * crMiB}, true
		}
		return cacheReturnRSS{Total: 3000 * crMiB}, true
	}
	var slept time.Duration
	got := waitRSSFall(read, func(d time.Duration) { slept += d }, cacheReturnRSS{Total: 4000 * crMiB}, 1500*crMiB, 5*time.Second)
	if got.Exit != "fell" || got.RSS.Total != 3000*crMiB || slept < 3*time.Second || slept > 4*time.Second {
		t.Fatalf("settle = %+v after %s, want fell at 3000 MiB after about 3 s", got, slept)
	}
	// Flat for 5 s: the cap.
	slept = 0
	got = waitRSSFall(func() (cacheReturnRSS, bool) { return cacheReturnRSS{Total: 4000 * crMiB}, true },
		func(d time.Duration) { slept += d }, cacheReturnRSS{Total: 4000 * crMiB}, 1500*crMiB, 5*time.Second)
	if got.Exit != "cap" || slept != 5*time.Second || got.MS != 5000 {
		t.Fatalf("flat settle = %+v after %s, want cap at 5 s", got, slept)
	}
	// Under 128 MiB reclaimed: no wait at all.
	slept = 0
	got = waitRSSFall(func() (cacheReturnRSS, bool) { return cacheReturnRSS{Total: 4000 * crMiB}, true },
		func(d time.Duration) { slept += d }, cacheReturnRSS{Total: 4000 * crMiB}, 100*crMiB, 5*time.Second)
	if got.Exit != "no_wait" || slept != 0 {
		t.Fatalf("small settle = %+v after %s, want no_wait", got, slept)
	}
	// A VM that is gone ends the wait.
	got = waitRSSFall(func() (cacheReturnRSS, bool) { return cacheReturnRSS{}, false },
		func(time.Duration) {}, cacheReturnRSS{Total: 4000 * crMiB}, 1500*crMiB, 5*time.Second)
	if got.Exit != "gone" {
		t.Fatalf("gone settle = %+v", got)
	}
	// Falling but still moving: wait for the last two readings to agree.
	vals := []uint64{3900, 3500, 3100, 3000, 3000}
	i := 0
	got = waitRSSFall(func() (cacheReturnRSS, bool) {
		v := vals[min(i, len(vals)-1)]
		i++
		return cacheReturnRSS{Total: v * crMiB}, true
	}, func(time.Duration) {}, cacheReturnRSS{Total: 4000 * crMiB}, 1500*crMiB, 5*time.Second)
	if got.Exit != "fell" || got.RSS.Total != 3000*crMiB {
		t.Fatalf("moving settle = %+v, want fell at 3000", got)
	}
}

// --- the registry pass ----------------------------------------------------

type crGuest struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	bodies  []map[string]any
	headers []http.Header
	status  int
	report  string
	block   chan struct{} // when non-nil, handlers wait on it
	started chan struct{}
	calls   atomic.Int32
}

func newCRGuest(t *testing.T) *crGuest {
	g := &crGuest{t: t, status: 200, started: make(chan struct{}, 16),
		report: `{"outcome":"returned","method":"memory_reclaim","target_bytes":1048576000,"reclaimed_bytes":943718400,"steps":15,"ms":900}`}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		g.mu.Lock()
		g.bodies = append(g.bodies, body)
		g.headers = append(g.headers, r.Header.Clone())
		status, report, block := g.status, g.report, g.block
		g.mu.Unlock()
		if r.URL.Path != "/internal/runtime/memory/cache-return" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		select {
		case g.started <- struct{}{}:
		default:
		}
		if block != nil {
			select {
			case <-block:
			case <-r.Context().Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, report)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

type crFixture struct {
	t    *testing.T
	reg  *OwnershipRegistry
	now  *time.Time
	mgr  *rssVMManager
	rss  map[string]cacheReturnRSS
	mu   sync.Mutex
	busy map[string]bool
	// probed counts the busy probes per VM.
	probed map[string]int
}

func newCRFixture(t *testing.T, mode string) *crFixture {
	t.Helper()
	mgr := &rssVMManager{processControlVMManager: newProcessControlVMManager(), rss: map[string]uint64{}}
	reg, now := hostCapacityRegistry(t, mgr)
	f := &crFixture{t: t, reg: reg, now: now, mgr: mgr, rss: map[string]cacheReturnRSS{}, busy: map[string]bool{}, probed: map[string]int{}}
	reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{
		Mode: mode, Only: []string{"*"}, Triggers: []string{"idle"}, Scope: "daemon",
		Idle: 3 * time.Minute, PressureIdle: time.Minute, PressureBudget: 12 * time.Second,
	})
	reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.probed[own.VMID]++
		return f.busy[own.VMID]
	})
	reg.cacheReturn.rss = func(vmID string) (cacheReturnRSS, bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		v, ok := f.rss[vmID]
		return v, ok
	}
	reg.cacheReturn.avail = func() uint64 { return 20 * 1024 * crMiB }
	reg.cacheReturn.sleep = func(time.Duration) {}
	return f
}

func (f *crFixture) setRSS(vmID string, total uint64) {
	f.mu.Lock()
	f.rss[vmID] = cacheReturnRSS{Total: total * crMiB, Anon: total / 2 * crMiB, File: total / 2 * crMiB}
	f.mu.Unlock()
}

// running adds an active computer idle for idle, behind guest.
func (f *crFixture) running(vmID string, idle time.Duration, g *crGuest) *VMOwnership {
	own := &VMOwnership{VMID: vmID, ComputerID: "comp-" + vmID, UserID: "user-" + vmID, DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, Epoch: 5, LastActiveAt: f.now.Add(-idle)}
	if g != nil {
		own.ComputerURL = g.srv.URL
	}
	addOwnership(f.reg, own)
	f.setRSS(vmID, 4000)
	return own
}

func (f *crFixture) pass() int {
	f.t.Helper()
	started := f.reg.runGuestCacheReturnPass(context.Background())
	if !f.reg.waitGuestCacheReturn(10 * time.Second) {
		f.t.Fatal("cache return acts did not finish")
	}
	return started
}

func (f *crFixture) records() []CacheReturnRecord {
	return f.reg.GuestCacheReturnSummary().Recent
}

// M15: off never calls a guest across a full sweep tick, and observe does.
func TestCacheReturnOffNeverCallsAGuestAcrossASweepTick(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnOff)
	f.running("vm-a", 10*time.Minute, g)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.reg.StartIdleSweeper(ctx, 5*time.Millisecond, func(context.Context, string, string) error { return nil })
	time.Sleep(200 * time.Millisecond)
	if n := g.calls.Load(); n != 0 {
		t.Fatalf("a guest was called %d times with the switch off", n)
	}
	if recs := f.records(); len(recs) != 0 {
		t.Fatalf("off recorded %d acts", len(recs))
	}
	// The same fixture with observe calls it within a few ticks.
	f.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnObserve, Only: []string{"*"}, Triggers: []string{"idle"}})
	deadline := time.Now().Add(3 * time.Second)
	for g.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if g.calls.Load() == 0 {
		t.Fatal("observe never called the guest from the sweep")
	}
}

func TestCacheReturnWithNobodySelectedOrNoIdleTriggerCallsNothing(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-a", 10*time.Minute, g)
	f.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Triggers: []string{"idle"}})
	if f.pass() != 0 || g.calls.Load() != 0 {
		t.Fatal("an empty selector acted")
	}
	f.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"hibernate"}})
	if f.pass() != 0 || g.calls.Load() != 0 {
		t.Fatal("a trigger list without idle acted in the idle pass")
	}
}

func TestCacheReturnObserveSendsDryRunAndActiveDoesNot(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnObserve)
	f.running("vm-a", 10*time.Minute, g)
	if f.pass() != 1 {
		t.Fatal("observe did not act on an idle computer")
	}
	if len(g.bodies) != 1 || g.bodies[0]["dry_run"] != true || g.bodies[0]["mode"] != "idle" || g.bodies[0]["scope"] != "daemon" {
		t.Fatalf("observe body = %v, want idle/daemon with dry_run true", g.bodies)
	}

	g2 := newCRGuest(t)
	f2 := newCRFixture(t, GuestCacheReturnActive)
	f2.running("vm-a", 10*time.Minute, g2)
	f2.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"idle"}, Scope: "root"})
	f2.pass()
	if len(g2.bodies) != 1 || g2.bodies[0]["dry_run"] == true || g2.bodies[0]["scope"] != "root" {
		t.Fatalf("active body = %v, want scope root and no dry_run", g2.bodies)
	}
}

// M19: no credential goes to the guest.
func TestCacheReturnRequestCarriesNoCredential(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-a", 10*time.Minute, g)
	f.pass()
	if len(g.headers) != 1 {
		t.Fatalf("%d requests", len(g.headers))
	}
	h := g.headers[0]
	for _, name := range []string{hostauth.Header, "Authorization", "Cookie", "X-Authenticated-User", "X-Gateway-Token"} {
		if v := h.Get(name); v != "" {
			t.Fatalf("the request carried %s", name)
		}
	}
}

// M9: a 404 marks the guest unsupported; the next pass does not call it; a
// new boot (a new epoch) clears it.
func TestCacheReturn404MarksUnsupportedUntilTheNextBoot(t *testing.T) {
	g := newCRGuest(t)
	g.status = http.StatusNotFound
	f := newCRFixture(t, GuestCacheReturnActive)
	own := f.running("vm-a", 10*time.Minute, g)
	f.pass()
	if g.calls.Load() != 1 {
		t.Fatalf("first pass made %d calls", g.calls.Load())
	}
	recs := f.records()
	if len(recs) != 1 || recs[0].Outcome != "unsupported" {
		t.Fatalf("records = %+v, want one unsupported", recs)
	}
	*f.now = f.now.Add(time.Hour)
	f.pass()
	if g.calls.Load() != 1 {
		t.Fatalf("an unsupported guest was called again (%d calls)", g.calls.Load())
	}
	f.reg.mu.Lock()
	own.Epoch++ // a new boot
	own.LastActiveAt = f.now.Add(-10 * time.Minute)
	f.reg.mu.Unlock()
	f.pass()
	if g.calls.Load() != 2 {
		t.Fatalf("a new boot did not clear unsupported (%d calls)", g.calls.Load())
	}
}

// M10: once per idle period.
func TestCacheReturnActsOncePerIdlePeriod(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	own := f.running("vm-a", 10*time.Minute, g)
	f.pass()
	*f.now = f.now.Add(2 * time.Minute)
	f.pass()
	f.pass()
	if g.calls.Load() != 1 {
		t.Fatalf("%d calls in one idle period, want 1", g.calls.Load())
	}
	// New activity, then idle again for 3 minutes: a new period.
	f.reg.mu.Lock()
	own.LastActiveAt = f.now.Add(time.Minute)
	f.reg.mu.Unlock()
	*f.now = f.now.Add(5 * time.Minute)
	f.pass()
	if g.calls.Load() != 2 {
		t.Fatalf("%d calls after a new idle period, want 2", g.calls.Load())
	}
}

// M1: a computer the guest probe says is busy is not called; the probe runs
// only for computers that pass the cheap rules.
func TestCacheReturnSkipsABusyComputerAndProbesLast(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-busy", 10*time.Minute, g)
	f.running("vm-young", time.Minute, g)
	f.busy["vm-busy"] = true
	f.pass()
	if g.calls.Load() != 0 {
		t.Fatal("a busy computer was acted on")
	}
	if f.probed["vm-busy"] != 1 || f.probed["vm-young"] != 0 {
		t.Fatalf("probes = %v, want vm-busy once and vm-young never", f.probed)
	}
}

// M6 through the real ledger: an open reservation, or a window that ended
// and awaits the sampler's capture, holds the act off.
func TestCacheReturnWaitsForTheLedgerToCaptureD4(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.reg.SetHostAdmissionConfig(measuredAdmission())
	f.running("vm-open", 10*time.Minute, g)
	f.running("vm-ended", 10*time.Minute, g)
	f.running("vm-free", 10*time.Minute, g)
	led := &f.reg.admissionLedger
	led.mu.Lock()
	led.initLocked()
	led.reservations[1] = &admissionReservation{id: 1, vmID: "vm-open", need: 2 * gib, startedAt: *f.now, settledAt: f.now.Add(-time.Minute)}
	led.reservations[2] = &admissionReservation{id: 2, vmID: "vm-ended", need: 2 * gib, startedAt: f.now.Add(-20 * time.Minute), settledAt: f.now.Add(-10 * time.Minute)}
	led.mu.Unlock()
	f.pass()
	if g.calls.Load() != 1 {
		t.Fatalf("%d calls, want only vm-free", g.calls.Load())
	}
	// The sampler captures the ended window; vm-ended is then a candidate.
	f.mgr.setRSS("vm-ended", 900*crMiB)
	f.mgr.setRSS("vm-open", 900*crMiB)
	f.mgr.setRSS("vm-free", 900*crMiB)
	f.reg.sampleVMMemory()
	led.mu.Lock()
	_, stillOpen := led.reservations[1]
	_, stillExpired := led.expired["vm-ended"]
	led.mu.Unlock()
	if !stillOpen || stillExpired {
		t.Fatalf("fixture: open=%v expired=%v, want the open reservation kept and the ended one captured", stillOpen, stillExpired)
	}
	f.pass()
	if g.calls.Load() != 2 {
		t.Fatalf("%d calls after the capture, want vm-ended added", g.calls.Load())
	}
}

// M11: a lifecycle claim is skipped; at most two acts at once; one per VM.
func TestCacheReturnBoundsConcurrencyAndSkipsClaims(t *testing.T) {
	g := newCRGuest(t)
	g.block = make(chan struct{})
	f := newCRFixture(t, GuestCacheReturnActive)
	for _, id := range []string{"vm-1", "vm-2", "vm-3", "vm-4"} {
		f.running(id, 10*time.Minute, g)
	}
	f.running("vm-claimed", 10*time.Minute, g)
	f.reg.mu.Lock()
	f.reg.lifecycle[ownershipKey("user-vm-claimed", PrimaryDesktopID)] = lifecycleHibernate
	f.reg.mu.Unlock()

	started := f.reg.runGuestCacheReturnPass(context.Background())
	if started != 2 {
		t.Fatalf("started %d acts, want 2 (host-wide bound)", started)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-g.started:
		case <-time.After(5 * time.Second):
			t.Fatal("the acts never reached the guest")
		}
	}
	// A second pass while both are in flight starts none, and never a second
	// act on one of the two.
	if again := f.reg.runGuestCacheReturnPass(context.Background()); again != 0 {
		t.Fatalf("a pass during two in-flight acts started %d more", again)
	}
	close(g.block)
	if !f.reg.waitGuestCacheReturn(10 * time.Second) {
		t.Fatal("acts did not finish")
	}
	// Next pass takes the others, never the claimed one.
	f.pass()
	seen := map[string]int{}
	for _, rec := range f.records() {
		seen[rec.VMID]++
	}
	if seen["vm-claimed"] != 0 {
		t.Fatalf("the computer under a lifecycle claim was acted on: %v", seen)
	}
	for _, id := range []string{"vm-1", "vm-2", "vm-3", "vm-4"} {
		if seen[id] != 1 {
			t.Fatalf("%s acted %d times, want once: %v", id, seen[id], seen)
		}
	}
}

// A host timeout is recorded and clears the in-flight entry.
func TestCacheReturnHostTimeoutIsRecordedAndFreesTheSlot(t *testing.T) {
	g := newCRGuest(t)
	g.block = make(chan struct{}) // never released
	f := newCRFixture(t, GuestCacheReturnActive)
	f.reg.cacheReturn.httpTimeout = 100 * time.Millisecond
	f.running("vm-a", 10*time.Minute, g)
	f.pass()
	recs := f.records()
	if len(recs) != 1 || recs[0].Outcome != "host_timeout" {
		t.Fatalf("records = %+v, want one host_timeout", recs)
	}
	if n := f.reg.cacheReturn.inFlightCount(); n != 0 {
		t.Fatalf("%d acts still in flight after the timeout", n)
	}
}

// The guest's own refusals are recorded under their words.
func TestCacheReturnRecordsGuestRefusals(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{409, `{"outcome":"busy","reason":"desk_pending"}`, "busy"},
		{409, `{"outcome":"in_progress"}`, "in_progress"},
		{500, `oops`, "http_500"},
		{200, `not json`, "bad_report"},
	} {
		g := newCRGuest(t)
		g.status, g.report = tc.status, tc.body
		f := newCRFixture(t, GuestCacheReturnActive)
		f.running("vm-a", 10*time.Minute, g)
		f.pass()
		recs := f.records()
		if len(recs) != 1 || recs[0].Outcome != tc.want {
			t.Fatalf("status %d: records = %+v, want outcome %s", tc.status, recs, tc.want)
		}
	}
}

// The record: host measurements before and after, the guest's claim kept
// apart, a journal-ready line.
func TestCacheReturnRecordCarriesHostMeasurementsAndTheGuestClaim(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-a", 10*time.Minute, g)
	reads := 0
	f.reg.cacheReturn.rss = func(string) (cacheReturnRSS, bool) {
		reads++
		if reads == 1 {
			return cacheReturnRSS{Total: 4000 * crMiB, Anon: 1000 * crMiB, File: 3000 * crMiB}, true
		}
		return cacheReturnRSS{Total: 3100 * crMiB, Anon: 1000 * crMiB, File: 2100 * crMiB}, true
	}
	avails := []uint64{10000 * crMiB, 10900 * crMiB}
	ai := 0
	f.reg.cacheReturn.avail = func() uint64 { v := avails[min(ai, 1)]; ai++; return v }
	f.pass()
	recs := f.records()
	if len(recs) != 1 {
		t.Fatalf("records = %+v", recs)
	}
	r := recs[0]
	if r.Outcome != "returned" || r.Trigger != "idle" || r.Mode != "idle" || r.Scope != "daemon" || r.VMID != "vm-a" {
		t.Fatalf("record = %+v", r)
	}
	if r.GuestTargetMiB != 1000 || r.GuestReclaimedMiB != 900 {
		t.Fatalf("guest claim = target %d reclaimed %d MiB, want 1000 and 900", r.GuestTargetMiB, r.GuestReclaimedMiB)
	}
	if r.RSSBeforeMiB != 4000 || r.RSSAfterMiB != 3100 || r.RSSAnonBeforeMiB != 1000 || r.RSSFileBeforeMiB != 3000 || r.RSSFileAfterMiB != 2100 {
		t.Fatalf("host RSS = %+v", r)
	}
	if r.HostAvailBeforeMiB != 10000 || r.HostAvailAfterMiB != 10900 {
		t.Fatalf("host available = %d -> %d", r.HostAvailBeforeMiB, r.HostAvailAfterMiB)
	}
	if r.SettleExit == "" {
		t.Fatal("the record has no settle exit")
	}
	line := r.JournalLine()
	for _, want := range []string{"guest cache return VM vm-a", "trigger=idle", "mode=idle", "scope=daemon", "outcome=returned",
		"guest_target_mib=1000", "guest_reclaimed_mib=900", "rss_before_mib=4000", "rss_after_mib=3100",
		"rss_anon_before_mib=1000", "rss_file_after_mib=2100", "host_avail_before_mib=10000", "host_avail_after_mib=10900", "settle_exit="} {
		if !strings.Contains(line, want) {
			t.Fatalf("journal line %q lacks %q", line, want)
		}
	}
	if f.reg.GuestCacheReturnSummary().LastAt["vm-a"] == "" {
		t.Fatal("health has no last_cache_return_at for vm-a")
	}
}

// M17: a guest that claims 4 GiB reclaimed while the host RSS does not move
// changes nothing but the record.
func TestCacheReturnGuestClaimDrivesNothing(t *testing.T) {
	g := newCRGuest(t)
	g.report = `{"outcome":"returned","method":"memory_reclaim","target_bytes":4294967296,"reclaimed_bytes":4294967296,"steps":64,"ms":900}`
	f := newCRFixture(t, GuestCacheReturnActive)
	own := f.running("vm-a", 10*time.Minute, g)
	f.mgr.setRSS("vm-a", 4000*crMiB)
	f.reg.sampleVMMemory()
	f.reg.mu.Lock()
	beforeOwn := *own
	f.reg.mu.Unlock()
	beforeAdmission := f.reg.HostAdmissionSummary()
	f.pass()
	recs := f.records()
	if len(recs) != 1 || recs[0].GuestReclaimedMiB != 4096 || recs[0].RSSBeforeMiB != recs[0].RSSAfterMiB {
		t.Fatalf("record = %+v, want the claim kept and an unmoved host RSS", recs)
	}
	f.reg.mu.RLock()
	afterOwn := *f.reg.vmByID["vm-a"]
	f.reg.mu.RUnlock()
	if afterOwn.State != beforeOwn.State || afterOwn.MemoryPeakRSSBytes != beforeOwn.MemoryPeakRSSBytes ||
		afterOwn.MemorySettledRSSBytes != beforeOwn.MemorySettledRSSBytes || afterOwn.StoppedBy != beforeOwn.StoppedBy {
		t.Fatalf("the ownership changed: before %+v after %+v", beforeOwn, afterOwn)
	}
	afterAdmission := f.reg.HostAdmissionSummary()
	if afterAdmission.ReservedBytes != beforeAdmission.ReservedBytes || len(afterAdmission.Recent) != len(beforeAdmission.Recent) {
		t.Fatalf("admission changed: %+v -> %+v", beforeAdmission, afterAdmission)
	}
}

// M14: a registry that starts over (a vmctl restart) has no in-flight or
// last-act state and performs no act at construction.
func TestCacheReturnRestartResumesAndReplaysNothing(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-a", 10*time.Minute, g)
	f.pass()
	if g.calls.Load() != 1 {
		t.Fatalf("first registry made %d calls", g.calls.Load())
	}
	before := g.calls.Load()
	fresh := NewOwnershipRegistry("http://127.0.0.1:8085")
	fresh.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"idle"}})
	if n := fresh.cacheReturn.inFlightCount(); n != 0 {
		t.Fatalf("a new registry has %d acts in flight", n)
	}
	if recs := fresh.GuestCacheReturnSummary().Recent; len(recs) != 0 || len(fresh.GuestCacheReturnSummary().LastAt) != 0 {
		t.Fatalf("a new registry remembers acts: %+v", recs)
	}
	time.Sleep(50 * time.Millisecond)
	if g.calls.Load() != before {
		t.Fatal("constructing a registry made a guest call")
	}
}

// Health: the ring holds 20, newest last; the mode shows even when off.
func TestCacheReturnSummaryRingAndMode(t *testing.T) {
	f := newCRFixture(t, GuestCacheReturnObserve)
	s := f.reg.GuestCacheReturnSummary()
	if s.Mode != GuestCacheReturnObserve || strings.Join(s.Only, ",") != "*" || len(s.Triggers) != 1 {
		t.Fatalf("summary = %+v", s)
	}
	for i := 0; i < 25; i++ {
		f.reg.cacheReturn.record(CacheReturnRecord{VMID: "vm", Outcome: "returned", GuestReclaimedMiB: uint64(i)})
	}
	recent := f.reg.GuestCacheReturnSummary().Recent
	if len(recent) != 20 || recent[0].GuestReclaimedMiB != 5 || recent[19].GuestReclaimedMiB != 24 {
		t.Fatalf("ring = %d entries, first %d last %d, want the newest 20", len(recent), recent[0].GuestReclaimedMiB, recent[len(recent)-1].GuestReclaimedMiB)
	}
	f.reg.SetGuestCacheReturnConfig(DefaultGuestCacheReturnConfig())
	if f.reg.GuestCacheReturnSummary().Mode != GuestCacheReturnOff {
		t.Fatal("health does not show the off rollback")
	}
}

// Health shows the switch and the recent acts under memory.cache_return,
// and never probes a guest to do it.
func TestHealthShowsCacheReturnWithoutProbing(t *testing.T) {
	g := newCRGuest(t)
	f := newCRFixture(t, GuestCacheReturnActive)
	f.running("vm-a", 10*time.Minute, g)
	f.pass()
	probed := f.probed["vm-a"]
	rec := httptest.NewRecorder()
	NewHandler(f.reg).HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		Memory struct {
			CacheReturn struct {
				Mode   string `json:"mode"`
				Recent []struct {
					VMID    string `json:"vm_id"`
					Outcome string `json:"outcome"`
				} `json:"recent"`
				LastAt map[string]string `json:"last_cache_return_at"`
			} `json:"cache_return"`
		} `json:"memory"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cr := body.Memory.CacheReturn
	if cr.Mode != "active" || len(cr.Recent) != 1 || cr.Recent[0].VMID != "vm-a" || cr.LastAt["vm-a"] == "" {
		t.Fatalf("health memory.cache_return = %+v", cr)
	}
	if f.probed["vm-a"] != probed || g.calls.Load() != 1 {
		t.Fatal("health probed or called a guest")
	}
}
