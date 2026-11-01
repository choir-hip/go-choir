package vmctl

import (
	"context"
	"strings"
	"testing"
	"time"
)

// D6 S5, the host half of busy_cold: under host pressure, make-room's step 0
// also asks the always-busy computers the busy-cold switch names to return
// a capped share of their cold cache while they work
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.4, 5.5,
// 9 S5; owner decision 2026-11-01, section 12).
//
// Ways it can fail, written first:
//   - the host calls busy_cold on a computer the switch does not name (the
//     paper by class, a premium computer by "*" in the idle selector, a
//     computer named only in VMCTL_GUEST_CACHE_RETURN_ONLY);
//   - the idle trigger's loop calls busy_cold (only the pressure path and
//     the periodic busy_cold pass may);
//   - the global switch off, or no busy-cold entries, still calls a guest;
//   - a typo in an entry's mode ("on", "Active") turns it on;
//   - observe (globally or per entry) writes: the body must carry dry_run;
//   - a held computer, one with no ComputerURL, one under a lifecycle claim,
//     the starter itself, one already in flight or unsupported at this boot,
//     or one whose D4 settled RSS is not captured yet is asked;
//   - two refused starts close together squeeze one always-busy computer
//     twice (the busy-cold repeat guard is five minutes);
//   - busy_cold needs the idle selector's pressure trigger, so turning it on
//     for the paper would also turn S4 on for the idle ring;
//   - the record and the journal line lack the mode and the guest's refault
//     readings, so the act cannot be proven from vmctl's journal.

const paperID = "computer-1355b83bced1613eb048ee28d66688a8"

// busyColdCR is a step-0 fixture whose idle selector has the idle trigger
// only (as staging does) and whose busy-cold switch names the given entries.
func busyColdCR(t *testing.T, entries map[string]string) *stepZeroFixture {
	t.Helper()
	return pressureCR(t, 5*time.Second, func(c *GuestCacheReturnConfig) {
		c.Only = []string{"computer-59c902c5d46caecc0a180a8557164e72"}
		c.Triggers = []string{"idle"}
		c.BusyCold = entries
	})
}

func (f *stepZeroFixture) paper(g *mrGuest) *VMOwnership {
	own := f.computer("vm-paper", 0, 7800*mib, 1*gib, g) // working: last active now
	f.reg.mu.Lock()
	own.ComputerID = paperID
	own.WarmnessClass = WarmnessClassCriticalProtected
	f.reg.mu.Unlock()
	return own
}

func (f *stepZeroFixture) stepZero() {
	f.reg.mu.RLock()
	cfg, acfg, sampler := f.reg.pressureReclaim, f.reg.hostAdmission, f.reg.pressureSampler
	f.reg.mu.RUnlock()
	ticket, _, _, _ := f.reg.makeRoomCacheReturn(context.Background(), f.starter(), WarmnessClassPrimary, cfg, acfg, sampler, f.reg.guestCacheReturnConfig())
	if ticket != nil {
		ticket.finish(true)
	}
}

// The parse: only id=active and id=observe enter; anything else is dropped.
func TestBusyColdEntriesParseExactWordsOnly(t *testing.T) {
	cfg := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{
		Mode:     "active",
		BusyCold: " computer-a=active, computer-b=observe,computer-c=on,computer-d,=active,computer-e=Active,computer-f=off ",
	})
	if len(cfg.BusyCold) != 2 || cfg.BusyCold["computer-a"] != GuestCacheReturnActive || cfg.BusyCold["computer-b"] != GuestCacheReturnObserve {
		t.Fatalf("entries = %v, want exactly a=active and b=observe", cfg.BusyCold)
	}
	if def := DefaultGuestCacheReturnConfig(); len(def.BusyCold) != 0 {
		t.Fatalf("the code default names %v, want nobody", def.BusyCold)
	}
}

// The effective mode: the global switch is a ceiling (off kills every
// entry, observe makes every entry a dry run); a computer is matched by VM
// id or computer id.
func TestBusyColdModeIsCappedByTheGlobalSwitch(t *testing.T) {
	own := &VMOwnership{VMID: "vm-paper", ComputerID: paperID}
	entries := map[string]string{paperID: GuestCacheReturnActive}
	for global, want := range map[string]string{
		GuestCacheReturnActive:  GuestCacheReturnActive,
		GuestCacheReturnObserve: GuestCacheReturnObserve,
		GuestCacheReturnOff:     GuestCacheReturnOff,
	} {
		if got := busyColdModeFor(GuestCacheReturnConfig{Mode: global, BusyCold: entries}, own); got != want {
			t.Errorf("global %s: mode %s, want %s", global, got, want)
		}
	}
	if got := busyColdModeFor(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, BusyCold: map[string]string{"vm-paper": GuestCacheReturnObserve}}, own); got != GuestCacheReturnObserve {
		t.Errorf("by VM id: %s, want observe", got)
	}
	if got := busyColdModeFor(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, BusyCold: map[string]string{"computer-other": GuestCacheReturnActive}}, own); got != GuestCacheReturnOff {
		t.Errorf("unnamed: %s, want off", got)
	}
}

// Under pressure the named paper is asked, with mode busy_cold, although it
// is critical_protected and working, and although the idle selector has no
// pressure trigger; the act is recorded as busy_cold with the guest's
// refault readings in the journal line.
func TestBusyColdStepZeroAsksTheNamedPaper(t *testing.T) {
	f := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.report = `{"outcome":"returned","method":"memory_reclaim","target_bytes":1073741824,"reclaimed_bytes":1073741824,"steps":16,"ms":400,` +
		`"refault_limit_per_s":32,"refault_rate_before_per_s":0.5,"refault_rate_after_per_s":0.7,` +
		`"before":{"workingset_refault_file":1000},"after":{"workingset_refault_file":1042}}`
	f.paper(g)
	f.returns(g, "vm-paper", 6800*mib, 1*gib)
	f.shortBy(512 * mib)

	ticket, err := f.reg.admitHostMemory(f.starter(), WarmnessClassPrimary)
	if err != nil || ticket == nil {
		t.Fatalf("a start the paper's return covers was refused: %v", err)
	}
	defer ticket.finish(true)
	if g.calls.Load() != 1 || !strings.Contains(g.bodies[0], `"mode":"busy_cold"`) || !strings.Contains(g.bodies[0], `"dry_run":false`) {
		t.Fatalf("paper calls %d bodies %v, want one busy_cold act", g.calls.Load(), g.bodies)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "admit_after_cache_return" || len(d.MadeRoomVMIDs) != 0 {
		t.Fatalf("decision %+v, want admit_after_cache_return with nobody hibernated", d)
	}
	recs := f.reg.GuestCacheReturnSummary().Recent
	if len(recs) != 1 || recs[0].Mode != "busy_cold" || recs[0].Trigger != "pressure" || recs[0].GuestRefaultsBefore != 1000 || recs[0].GuestRefaultsAfter != 1042 {
		t.Fatalf("records %+v, want one busy_cold pressure act with refault counts", recs)
	}
	line := recs[0].JournalLine()
	for _, want := range []string{"mode=busy_cold", "guest_refaults_before=1000", "guest_refaults_after=1042", "guest_refault_rate_before=0.5", "guest_refault_rate_after=0.7", "guest_refault_limit=32", "guest_target_mib=1024"} {
		if !strings.Contains(line, want) {
			t.Fatalf("journal line lacks %q: %s", want, line)
		}
	}
}

// The idle rule's 256 MiB excess floor does not apply: the paper's D4
// settled RSS is taken after its cache grew, so its excess is near zero
// while its cold cache is gigabytes. It is still asked.
func TestBusyColdHasNoExcessFloor(t *testing.T) {
	f := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	paper := f.paper(g)
	f.reg.mu.Lock()
	paper.MemorySettledRSSBytes = 7800 * mib // settled at its peak
	f.reg.mu.Unlock()
	f.shortBy(5 * gib)
	f.stepZero()
	if g.calls.Load() != 1 {
		t.Fatalf("the paper with no host-measured excess was asked %d times, want 1", g.calls.Load())
	}
}

// The host never calls busy_cold on a computer outside the switch: the
// paper unnamed (with the idle selector "*" and the pressure trigger on), a
// premium computer named only in the idle selector, and with the global
// switch off nobody at all.
func TestBusyColdNeverCallsAComputerOutsideTheSwitch(t *testing.T) {
	f := pressureCR(t, 5*time.Second, func(c *GuestCacheReturnConfig) {
		c.BusyCold = map[string]string{"computer-someone-else": GuestCacheReturnActive}
	})
	g := newMRGuest(t, "paper", nil)
	f.paper(g)
	f.shortBy(5 * gib)
	f.stepZero()
	if g.calls.Load() != 0 {
		t.Fatalf("the unnamed paper was asked %d times", g.calls.Load())
	}

	off := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	off.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnOff, Only: []string{"*"}, Triggers: []string{"pressure"},
		BusyCold: map[string]string{paperID: GuestCacheReturnActive}})
	g2 := newMRGuest(t, "paper", nil)
	off.paper(g2)
	off.shortBy(5 * gib)
	off.stepZero()
	if g2.calls.Load() != 0 {
		t.Fatalf("with the switch off the paper was asked %d times", g2.calls.Load())
	}

	// Every busy_cold body names a computer of the busy-cold switch.
	mixed := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	idleG := newMRGuest(t, "de", nil)
	mixed.computer("vm-de", 40*time.Minute, 3*gib, 1*gib, idleG)
	mixed.shortBy(9 * gib)
	mixed.stepZero()
	for _, body := range idleG.bodies {
		if strings.Contains(body, "busy_cold") {
			t.Fatalf("a computer outside the busy-cold switch got %s", body)
		}
	}
}

// The idle loop never calls busy_cold: only the pressure path and the
// periodic busy_cold pass (guest_cache_return_busy_cold_periodic_test.go) do.
func TestBusyColdIdleLoopNeverCallsIt(t *testing.T) {
	f := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	f.reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Only: []string{"*"}, Triggers: []string{"idle", "hibernate", "pressure"},
		Scope: "daemon", Idle: time.Minute, BusyCold: map[string]string{paperID: GuestCacheReturnActive}})
	g := newMRGuest(t, "paper", nil)
	paper := f.paper(g)
	f.reg.mu.Lock()
	paper.LastActiveAt = f.now.Add(-2 * time.Hour) // even an idle-looking paper
	f.reg.mu.Unlock()
	for i := 0; i < 3; i++ {
		f.reg.runGuestCacheReturnPass(context.Background())
	}
	if !f.reg.waitGuestCacheReturn(5 * time.Second) {
		t.Fatal("acts did not finish")
	}
	if g.calls.Load() != 0 {
		t.Fatalf("the idle loop called the paper %d times", g.calls.Load())
	}
}

// observe, globally or per entry, sends dry_run.
func TestBusyColdObserveIsADryRun(t *testing.T) {
	for name, cfg := range map[string]func(*GuestCacheReturnConfig){
		"entry observe":  func(c *GuestCacheReturnConfig) { c.BusyCold = map[string]string{paperID: GuestCacheReturnObserve} },
		"global observe": func(c *GuestCacheReturnConfig) { c.Mode = GuestCacheReturnObserve },
	} {
		t.Run(name, func(t *testing.T) {
			f := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
			c := f.reg.guestCacheReturnConfig()
			cfg(&c)
			f.reg.SetGuestCacheReturnConfig(c)
			g := newMRGuest(t, "paper", nil)
			g.report = `{"outcome":"dry_run","method":"memory_reclaim","target_bytes":1073741824}`
			f.paper(g)
			f.shortBy(5 * gib)
			f.stepZero()
			if g.calls.Load() != 1 || !strings.Contains(g.bodies[0], `"dry_run":true`) || !strings.Contains(g.bodies[0], `"mode":"busy_cold"`) {
				t.Fatalf("calls %d bodies %v, want one busy_cold dry run", g.calls.Load(), g.bodies)
			}
		})
	}
}

// Two refused starts a minute apart ask the paper once: the busy-cold
// repeat guard is five minutes.
func TestBusyColdRepeatGuardIsFiveMinutes(t *testing.T) {
	f := busyColdCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.report = claim(10 * mib)
	f.paper(g)
	f.shortBy(5 * gib)
	f.stepZero()
	*f.now = f.now.Add(time.Minute)
	f.stepZero()
	if g.calls.Load() != 1 {
		t.Fatalf("paper asked %d times within five minutes, want 1", g.calls.Load())
	}
	*f.now = f.now.Add(5 * time.Minute)
	f.stepZero()
	if g.calls.Load() != 2 {
		t.Fatalf("paper asked %d times after five minutes, want 2", g.calls.Load())
	}
}

// The candidate rule as a pure function: held, no URL, claimed, the
// starter, in flight, unsupported, uncaptured and not-active computers are
// never busy-cold candidates; the named, running paper is.
func TestBusyColdCandidateRule(t *testing.T) {
	now := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	mk := func(id string) *VMOwnership {
		return &VMOwnership{VMID: id, ComputerID: "computer-" + id, State: VMStateActive, ComputerURL: "http://10.0.0.9:8085",
			LastActiveAt: now, WarmnessClass: WarmnessClassCriticalProtected}
	}
	ok, held, nourl, claimed, inflight, unsup, uncapt, hib, unnamed := mk("ok"), mk("held"), mk("nourl"), mk("claimed"), mk("inflight"), mk("unsup"), mk("uncapt"), mk("hib"), mk("unnamed")
	held.HoldStatus = &MaintenanceHold{Reason: "maintenance"}
	nourl.ComputerURL = " "
	hib.State = VMStateHibernated
	entries := map[string]string{}
	for _, o := range []*VMOwnership{ok, held, nourl, claimed, inflight, unsup, uncapt, hib} {
		entries[o.ComputerID] = GuestCacheReturnActive
	}
	got := busyColdCandidates(cacheReturnCandidateInput{
		Now: now, Config: GuestCacheReturnConfig{Mode: GuestCacheReturnActive, BusyCold: entries},
		Ownerships:  []*VMOwnership{ok, held, nourl, claimed, inflight, unsup, uncapt, hib, unnamed},
		Claimed:     func(o *VMOwnership) bool { return o.VMID == "claimed" },
		InFlight:    func(id string) bool { return id == "inflight" },
		Unsupported: func(id string, _ int64) bool { return id == "unsup" },
		Uncaptured:  func(id string) bool { return id == "uncapt" },
		LastAct:     func(string) (time.Time, bool) { return time.Time{}, false },
	})
	if len(got) != 1 || got[0].VMID != "ok" {
		var ids []string
		for _, o := range got {
			ids = append(ids, o.VMID)
		}
		t.Fatalf("candidates %v, want only ok", ids)
	}
}
