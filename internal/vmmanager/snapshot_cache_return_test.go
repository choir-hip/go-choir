package vmmanager

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// D6 S3, the cache return before a snapshot hibernate
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.3, 5.1,
// 9 S3; docs/problems/density-station-d-failure-modes-2026-10-26.md D6).
//
// A snapshot writes the guest's memory, cache included. The host asks the
// fenced guest to return its cold cache first, waits for the Firecracker
// process's RSS to fall, and only then decides Full or Diff and pauses.
//
// Ways it can fail:
//   - M8: the act fails, is refused, times out or answers garbage and the
//     hibernate fails, takes the kill path, or waits for it;
//   - M23: the act is asked of a guest that refused the fence;
//   - M7: the act runs after the pause (in frozen time), before the unplug
//     could shrink the guest, or after the Full or Diff decision; the guest
//     kernel command line gains init_on_free or page poisoning, which would
//     make a discarded free page unsafe for a Diff;
//   - M18: the launch config or the prepare body changes with the option;
//   - M15: with no option a cache return is attempted;
//   - M22: the settle exits before the fall, or past its cap;
//   - M26: the act pushes fence-close to end-of-pause past the fence TTL;
//   - a snapshot taken with the option differs in the calls Firecracker sees.

const gibBytes = int64(1 << 30)

// crSeq is an ordered event log shared by the fakes.
type crSeq struct {
	mu sync.Mutex
	ev []string
}

func (s *crSeq) add(e string) {
	s.mu.Lock()
	s.ev = append(s.ev, e)
	s.mu.Unlock()
}

func (s *crSeq) index(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.ev {
		if strings.HasPrefix(e, prefix) {
			return i
		}
	}
	return -1
}

func (s *crSeq) count(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.ev {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

func (s *crSeq) all() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.ev, ",")
}

// crFixture is the snapshot fixture with the cache return fakes wired:
// every guest, Firecracker, settle-sleep and decision event lands in seq.
type crFixture struct {
	*snapshotFixture
	seq *crSeq
	// clock is the manager's fence clock; advance moves it.
	cmu   sync.Mutex
	clock time.Time
	// rss is the fake process RSS in MiB per poll number (1-based).
	rssAt func(poll int) (uint64, bool)
	polls int
	slept time.Duration
}

func newCRFixture(t *testing.T) *crFixture {
	t.Helper()
	f := &crFixture{snapshotFixture: newSnapshotFixture(t), seq: &crSeq{}, clock: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}
	f.serveLaunchAPI(t)
	f.fc.onCall = func(method, path string) { f.seq.add("fc " + method + " " + path) }
	f.guest.onPrepare = func() { f.seq.add("prepare") }
	f.guest.cacheReturn = f.reportReturned(1 * gibBytes)
	f.mgr.cacheReturnClock = func() time.Time {
		f.cmu.Lock()
		defer f.cmu.Unlock()
		return f.clock
	}
	f.mgr.cacheReturnSleep = func(d time.Duration) {
		f.slept += d
		f.seq.add("settle_sleep")
	}
	f.mgr.cacheReturnRSSReader = func(string) func() (uint64, bool) {
		return func() (uint64, bool) {
			f.polls++
			if f.rssAt == nil {
				return 4000 << 20, true
			}
			return f.rssAt(f.polls)
		}
	}
	f.mgr.onDiffDecision = func() { f.seq.add("diff_decision") }
	return f
}

func (f *crFixture) advance(d time.Duration) {
	f.cmu.Lock()
	f.clock = f.clock.Add(d)
	f.cmu.Unlock()
}

// reportReturned makes the guest answer 200 with a report that claims
// reclaimed bytes.
func (f *crFixture) reportReturned(reclaimed int64) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		f.seq.add("cache_return")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"outcome": "returned", "method": "memory_reclaim", "mode": "hibernate", "scope": "daemon",
			"target_bytes": reclaimed, "reclaimed_bytes": reclaimed, "steps": 16, "ms": 800,
		})
	}
}

func (f *crFixture) hibernate(t *testing.T, opts HibernateOptions) *SnapshotTimeline {
	t.Helper()
	if err := f.mgr.HibernateVMWithOptions(f.vmID, opts); err != nil {
		t.Fatalf("HibernateVMWithOptions: %v", err)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil {
		t.Fatal("no snapshot timeline")
	}
	return tl.Snapshot
}

var withCacheReturn = HibernateOptions{CacheReturn: &CacheReturnOption{Scope: "daemon"}}

// M15: without the option, no cache return call is made, and the
// hibernate is today's: the same calls to Firecracker, a Full or Diff
// snapshot committed.
func TestHibernateWithoutTheOptionMakesNoCacheReturnCall(t *testing.T) {
	f := newCRFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	if n := len(f.guest.cacheReturnBodies); n != 0 {
		t.Fatalf("%d cache return calls without the option", n)
	}
	if got := strings.Join(f.fc.paths(), ","); got != "GET /,PATCH /vm,PUT /snapshot/create" {
		t.Fatalf("calls = %s", got)
	}
	if snap := readHibernateTimeline(t, f.mgr, f.vmID).Snapshot; snap.CacheReturn != nil {
		t.Fatalf("timeline has a cache_return without the option: %+v", snap.CacheReturn)
	}
}

// With the option: one call, in order, and a timeline.
func TestHibernateWithTheOptionReturnsCacheBetweenFenceAndDecision(t *testing.T) {
	f := newCRFixture(t)
	f.rssAt = func(poll int) (uint64, bool) {
		if poll == 1 { // rss0, before the call
			return 4000 << 20, true
		}
		return 3000 << 20, true // the fall, seen on the settle's first polls
	}
	snap := f.hibernate(t, withCacheReturn)
	if snap.Outcome != "snapshot" {
		t.Fatalf("outcome = %q, want snapshot", snap.Outcome)
	}
	if len(f.guest.cacheReturnBodies) != 1 {
		t.Fatalf("%d calls", len(f.guest.cacheReturnBodies))
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(f.guest.cacheReturnBodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "hibernate" || body["scope"] != "daemon" || body["dry_run"] == true {
		t.Fatalf("body = %v", body)
	}
	// M7: prepare < cache return < settle < Full-or-Diff decision < pause <
	// create; nothing of the act after the pause.
	order := []string{"prepare", "cache_return", "settle_sleep", "diff_decision", "fc PATCH /vm", "fc PUT /snapshot/create"}
	last := -1
	for _, e := range order {
		i := f.seq.index(e)
		if i < 0 || i <= last {
			t.Fatalf("event %q at %d after %d; order = %s", e, i, last, f.seq.all())
		}
		last = i
	}
	pause := f.seq.index("fc PATCH /vm")
	f.seq.mu.Lock()
	for i, e := range f.seq.ev {
		if i > pause && (e == "cache_return" || e == "settle_sleep") {
			t.Errorf("%s after the pause: %s", e, strings.Join(f.seq.ev, ","))
		}
	}
	f.seq.mu.Unlock()
	cr := snap.CacheReturn
	if cr == nil || cr.Outcome != "returned" || cr.ReclaimedMiB != 1024 || cr.RSS0MiB != 4000 || cr.RSSEndMiB != 3000 ||
		cr.SettleExit != "fell" || cr.Skipped != "" || cr.Error != "" {
		t.Fatalf("timeline cache_return = %+v", cr)
	}
	// Firecracker saw today's calls.
	if got := strings.Join(f.fc.paths(), ","); got != "GET /,PATCH /vm,PUT /snapshot/create" {
		t.Fatalf("calls = %s", got)
	}
}

// M18: nothing in the launch config or the prepare body changes.
func TestHibernateOptionChangesNeitherLaunchConfigNorPrepareBody(t *testing.T) {
	read := func(f *crFixture) []byte {
		raw, err := os.ReadFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	on, off := newCRFixture(t), newCRFixture(t)
	beforeOn, beforeOff := read(on), read(off)
	if string(beforeOn) != string(beforeOff) {
		t.Fatal("fixture launch configs differ before the hibernate")
	}
	on.hibernate(t, withCacheReturn)
	if err := off.mgr.HibernateVM(off.vmID); err != nil {
		t.Fatal(err)
	}
	if string(read(on)) != string(beforeOn) || string(read(off)) != string(beforeOff) {
		t.Fatal("a hibernate rewrote fc-config.json")
	}
	if len(on.guest.prepareBodies) != 1 || len(off.guest.prepareBodies) != 1 ||
		on.guest.prepareBodies[0] != "{}" || off.guest.prepareBodies[0] != "{}" {
		t.Fatalf("prepare bodies on=%q off=%q, want {} for both", on.guest.prepareBodies, off.guest.prepareBodies)
	}
}

// M7: a discarded free page is only safe for a Diff if the guest assumes
// nothing about free pages; init_on_free and page poisoning would.
func TestGuestKernelCommandLineNeverZeroesOrPoisonsFreePages(t *testing.T) {
	f := newCRFixture(t)
	raw, err := os.ReadFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		BootSource struct {
			BootArgs string `json:"boot_args"`
		} `json:"boot-source"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.BootSource.BootArgs == "" {
		t.Fatal("the fixture's launch config has no boot args to check")
	}
	for _, bad := range []string{"init_on_free=1", "page_poison=1", "init_on_free=on", "page_poison=on"} {
		if strings.Contains(cfg.BootSource.BootArgs, bad) {
			t.Fatalf("boot args carry %s: %s", bad, cfg.BootSource.BootArgs)
		}
	}
	// And the manager's own default parameter set.
	for _, bad := range []string{"init_on_free", "page_poison"} {
		if strings.Contains(DefaultManagerConfig().KernelParams, bad) {
			t.Fatalf("default kernel params carry %s", bad)
		}
	}
}

// M23: the call is made only after a successful prepare.
func TestHibernateMakesNoCacheReturnCallWhenPrepareIsRefused(t *testing.T) {
	for _, status := range []int{http.StatusConflict, http.StatusNotFound, http.StatusInternalServerError} {
		f := newCRFixture(t)
		f.guest.prepareStatus = status
		// The busy skip and the kill path both end without a call.
		_ = f.mgr.HibernateVMWithOptions(f.vmID, withCacheReturn)
		if n := len(f.guest.cacheReturnBodies); n != 0 || f.seq.count("cache_return") != 0 {
			t.Fatalf("prepare answered %d and the act was still asked (%d calls)", status, n)
		}
	}
}

// M8: whatever the act does, the snapshot is taken exactly as without it
// and the timeline records why.
func TestHibernateSurvivesEveryCacheReturnFailure(t *testing.T) {
	cases := map[string]struct {
		handler func(w http.ResponseWriter, r *http.Request)
		wantErr string
	}{
		"503": {func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, "status_503"},
		"409 busy": {func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"outcome":"not_fenced"}`))
		}, "status_409"},
		"500":         {func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, "status_500"},
		"garbage 200": {func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not json")) }, "bad_report"},
		"no route":    {nil, "status_404"},
		"slow": {func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(2 * time.Second):
			case <-r.Context().Done():
			}
		}, "timeout"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCRFixture(t)
			f.guest.cacheReturn = tc.handler
			f.mgr.cacheReturnTimeout = 150 * time.Millisecond
			snap := f.hibernate(t, withCacheReturn)
			if snap.Outcome != "snapshot" || snap.Reason != "" {
				t.Fatalf("outcome %q reason %q: the snapshot must be taken whatever the act does", snap.Outcome, snap.Reason)
			}
			if got := strings.Join(f.fc.paths(), ","); got != "GET /,PATCH /vm,PUT /snapshot/create" {
				t.Fatalf("calls = %s", got)
			}
			cr := snap.CacheReturn
			if cr == nil || cr.Error != tc.wantErr || cr.ReclaimedMiB != 0 {
				t.Fatalf("cache_return = %+v, want error %q", cr, tc.wantErr)
			}
			if f.slept != 0 {
				t.Fatalf("a failed act was waited for (%s)", f.slept)
			}
			if _, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID)); err != nil {
				t.Fatalf("no committed snapshot: %v", err)
			}
		})
	}
}

// M8: the call's bound is 5 s by default.
func TestCacheReturnCallBoundDefaultsToFiveSeconds(t *testing.T) {
	f := newCRFixture(t)
	if got := f.mgr.cacheReturnCallBound(); got != 5*time.Second {
		t.Fatalf("bound = %s, want 5s", got)
	}
}

// M22 through the hibernate: flat for 3 s then falling by half the claim:
// the settle waits for the fall; flat for good it stops at 5 s; a small
// claim is not waited for.
func TestHibernateSettleWaitsForTheFallAndHonoursItsCap(t *testing.T) {
	t.Run("fall after 3 s", func(t *testing.T) {
		f := newCRFixture(t)
		f.rssAt = func(poll int) (uint64, bool) {
			if poll <= 13 { // rss0 plus 12 flat polls
				return 4000 << 20, true
			}
			return 3000 << 20, true
		}
		cr := f.hibernate(t, withCacheReturn).CacheReturn
		if cr.SettleExit != "fell" || f.slept < 3*time.Second || f.slept > 4*time.Second || cr.RSSEndMiB != 3000 {
			t.Fatalf("cache_return = %+v after %s, want fell after about 3.5 s", cr, f.slept)
		}
	})
	t.Run("flat: cap at 5 s", func(t *testing.T) {
		f := newCRFixture(t)
		cr := f.hibernate(t, withCacheReturn).CacheReturn
		if cr.SettleExit != "cap" || f.slept != 5*time.Second || cr.SettleMS != 5000 {
			t.Fatalf("cache_return = %+v after %s, want cap at 5 s", cr, f.slept)
		}
	})
	t.Run("small claim: no wait", func(t *testing.T) {
		f := newCRFixture(t)
		f.guest.cacheReturn = f.reportReturned(100 << 20)
		cr := f.hibernate(t, withCacheReturn).CacheReturn
		if cr.SettleExit != "no_wait" || f.slept != 0 {
			t.Fatalf("cache_return = %+v after %s, want no_wait", cr, f.slept)
		}
	})
}

// M26: the fence's two minutes. Prepare plus unplug that took more than 60 s
// leaves no margin for the act, which is skipped; 59 s still calls it. The
// snapshot proceeds either way.
func TestHibernateSkipsTheActPastTheFenceBudget(t *testing.T) {
	for _, tc := range []struct {
		took     time.Duration
		wantCall bool
	}{{61 * time.Second, false}, {59 * time.Second, true}, {60 * time.Second, true}} {
		f := newCRFixture(t)
		prior := f.guest.onPrepare
		f.guest.onPrepare = func() { prior(); f.advance(tc.took) }
		snap := f.hibernate(t, withCacheReturn)
		called := len(f.guest.cacheReturnBodies) == 1
		if called != tc.wantCall {
			t.Fatalf("prepare took %s: called=%v, want %v", tc.took, called, tc.wantCall)
		}
		cr := snap.CacheReturn
		if !tc.wantCall && (cr == nil || cr.Skipped != "fence_budget") {
			t.Fatalf("prepare took %s: cache_return = %+v, want skipped=fence_budget", tc.took, cr)
		}
		if snap.Outcome != "snapshot" {
			t.Fatalf("prepare took %s: outcome %q, want snapshot", tc.took, snap.Outcome)
		}
		if f.seq.count("fc PUT /snapshot/create") != 1 {
			t.Fatalf("no snapshot after a skipped act: %s", f.seq.all())
		}
	}
}

// observe: the act is asked for a dry run, which frees nothing, so there is
// nothing to wait for.
func TestHibernateDryRunOptionAsksForADryRunAndDoesNotWait(t *testing.T) {
	f := newCRFixture(t)
	f.guest.cacheReturn = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"outcome": "dry_run", "method": "memory_reclaim", "target_bytes": 1 << 30, "reclaimed_bytes": 0})
	}
	snap := f.hibernate(t, HibernateOptions{CacheReturn: &CacheReturnOption{Scope: "root", DryRun: true}})
	var body map[string]any
	if err := json.Unmarshal([]byte(f.guest.cacheReturnBodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if body["dry_run"] != true || body["scope"] != "root" || body["mode"] != "hibernate" {
		t.Fatalf("body = %v", body)
	}
	if f.slept != 0 || snap.CacheReturn == nil || snap.CacheReturn.Outcome != "dry_run" || snap.Outcome != "snapshot" {
		t.Fatalf("slept %s, cache_return %+v", f.slept, snap.CacheReturn)
	}
}

// The request carries no host credential to the guest.
func TestCacheReturnRequestCarriesNoCredential(t *testing.T) {
	f := newCRFixture(t)
	var got http.Header
	f.guest.cacheReturn = func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		f.reportReturned(1<<30)(w, r)
	}
	f.hibernate(t, withCacheReturn)
	for _, name := range []string{"Authorization", "Cookie", "X-Authenticated-User", "X-Choir-Host-Token"} {
		if v := got.Get(name); v != "" {
			t.Fatalf("the request carried %s", name)
		}
	}
	if len(got) == 0 {
		t.Fatal("no request seen")
	}
}

// A hibernate log and timeline carry the numbers the design names.
func TestCacheReturnTimelineJSONNamesTheDesignFields(t *testing.T) {
	f := newCRFixture(t)
	f.hibernate(t, withCacheReturn)
	raw, err := json.Marshal(readHibernateTimeline(t, f.mgr, f.vmID))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"cache_return"`, `"reclaimed_mib"`, `"rss0_mib"`, `"rss_end_mib"`, `"settle_ms"`, `"settle_exit"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("timeline lacks %s: %s", want, raw)
		}
	}
}
