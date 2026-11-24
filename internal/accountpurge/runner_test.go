package accountpurge

// Failure list: the header of runner.go (1 to 7). The runner runs against a
// real auth store in a temporary file and a fake of the host services that
// records every call; nothing real is touched.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type fakeSteps struct {
	mu        sync.Mutex
	store     *auth.Store
	calls     []string
	computers []vmctl.AccountComputer
	listErr   error
	fail      map[string]error
	tape      TapeResult
	onTape    func(Authz)
	// preflightErr fails Preflight; destroyFailsAfter makes DestroyComputers
	// report its computers destroyed and then fail.
	preflightErr     error
	destroyFailsOnce bool
	userAtTape       bool
	userAtMail       bool
	// onSuspend, when set, runs inside SuspendTakes: a cancel arriving while
	// a pass is in flight.
	onSuspend func()
	// onStop runs inside StopComputers (a cancel landing between a pass's
	// list and its suspend).
	onStop func()
	// destroyed is what DestroyComputers reports it removed (default: the
	// listed computers).
	destroyed *Destroyed
	// restoreErr fails RestoreTakes.
	restoreErr error
}

func (f *fakeSteps) rec(s string) { f.mu.Lock(); f.calls = append(f.calls, s); f.mu.Unlock() }
func (f *fakeSteps) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}
func (f *fakeSteps) destructive() int {
	n := 0
	for _, p := range []string{"mail", "data", "destroy", "gateway", "tape"} {
		n += f.count(p)
	}
	return n
}

func (f *fakeSteps) Computers(context.Context, string) ([]vmctl.AccountComputer, error) {
	return f.computers, f.listErr
}
func (f *fakeSteps) StopComputers(context.Context, string) error {
	f.rec("stop")
	if f.onStop != nil {
		f.onStop()
	}
	return f.fail["stop"]
}
func (f *fakeSteps) SuspendTakes(context.Context, string) error {
	f.rec("suspend")
	if f.onSuspend != nil {
		f.onSuspend()
	}
	return f.fail["suspend"]
}
func (f *fakeSteps) RestoreTakes(context.Context, string) error {
	f.rec("restore")
	return f.restoreErr
}
func (f *fakeSteps) Preflight(context.Context, Authz) error {
	f.rec("preflight")
	return f.preflightErr
}
func (f *fakeSteps) PurgeMail(_ context.Context, a Authz) (Counts, error) {
	f.rec("mail")
	_, err := f.store.GetUserByID(a.UserID())
	f.userAtMail = err == nil
	return Counts{"aliases": 1}, f.fail["mail"]
}
func (f *fakeSteps) PurgeData(context.Context, Authz) (Counts, error) {
	f.rec("data")
	return Counts{"takes_deleted": 2}, f.fail["data"]
}
func (f *fakeSteps) DestroyComputers(context.Context, Authz) (Counts, Destroyed, error) {
	f.rec("destroy")
	d := Destroyed{}
	for _, c := range f.computers {
		d.ComputerIDs = append(d.ComputerIDs, c.ComputerID)
		d.VMIDs = append(d.VMIDs, c.VMID)
	}
	if f.destroyed != nil {
		d = *f.destroyed
	}
	err := f.fail["destroy"]
	if f.destroyFailsOnce {
		f.destroyFailsOnce = false
		err = errors.New("disk busy on the second computer")
	}
	// Like vmctl, what was destroyed is no longer listed as the account's.
	f.computers = nil
	return Counts{"computers": 1}, d, err
}
func (f *fakeSteps) ForgetGateway(_ context.Context, _ Authz, id string) error {
	f.rec("gateway:" + id)
	return f.fail["gateway"]
}
func (f *fakeSteps) PurgeTape(_ context.Context, a Authz) (TapeResult, error) {
	f.rec("tape")
	if f.onTape != nil {
		f.onTape(a)
	}
	_, err := f.store.GetUserByID(a.UserID())
	f.userAtTape = err == nil
	return f.tape, f.fail["tape"]
}

type env struct {
	r     *Runner
	s     *auth.Store
	f     *fakeSteps
	user  *auth.User
	clock time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	s := auth.TestStore(t)
	u, err := s.CreateUser("purge-user", "purge-user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSteps{store: s, fail: map[string]error{}, computers: []vmctl.AccountComputer{{ComputerID: "computer-abc", VMID: "vm-1", DesktopID: "primary", State: "stopped", Class: "primary"}},
		tape: TapeResult{Counts: Counts{"files_deleted": 9}}}
	e := &env{r: New(s, f), s: s, f: f, user: u, clock: time.Now().UTC()}
	e.r.SetClock(func() time.Time { return e.clock })
	return e
}

func (e *env) request(t *testing.T) {
	t.Helper()
	if _, _, err := e.s.RequestAccountDeletion(e.user.ID, e.clock); err != nil {
		t.Fatal(err)
	}
}

func (e *env) pass() { e.r.Tick(context.Background()) }

// Failure 1: nothing is destroyed before 72 hours, or with no record.
func TestNothingIsDestroyedBeforeTheGracePeriod(t *testing.T) {
	e := newEnv(t)
	e.pass() // no record at all
	if e.f.destructive() != 0 || e.f.count("stop") != 0 {
		t.Fatalf("a call with no record: %v", e.f.calls)
	}
	e.request(t)
	for _, d := range []time.Duration{0, time.Hour, 48 * time.Hour, purgeguard.GracePeriod - time.Second} {
		e.clock = e.clock.Add(0)
		base := e.clock
		e.clock = base.Add(d)
		e.pass()
		e.clock = base
	}
	if e.f.destructive() != 0 {
		t.Fatalf("destructive calls before 72h: %v", e.f.calls)
	}
	if e.f.count("stop") == 0 || e.f.count("suspend") == 0 {
		t.Fatal("a pending account's computer and takes were not held")
	}
	if _, err := e.r.authorize(e.user.ID, e.clock.Add(time.Hour)); err == nil {
		t.Fatal("a capability was made inside the grace period")
	}
	if _, err := e.r.authorize("nobody", e.clock.Add(100*time.Hour)); err == nil {
		t.Fatal("a capability was made with no record")
	}
}

// The whole purge, in order, once each; the user row goes last.
func TestPurgeRunsInOrderAndTheUserRowGoesLast(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	if err := e.r.AtOnce(context.Background(), e.user.ID); err != nil {
		t.Fatal(err)
	}
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if len(d.Computers) != 1 || d.Computers[0] != "computer-abc" {
		t.Fatalf("computers not recorded at once: %v", d.Computers)
	}
	// vmctl forgets the computer once it is destroyed; the record remembers.
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	want := []string{"preflight", "mail", "data", "destroy", "gateway:computer-abc", "gateway:vm-1", "tape"}
	var got []string
	for _, c := range e.f.calls {
		if c != "stop" && c != "suspend" {
			got = append(got, c)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order: %v", got)
	}
	if !e.f.userAtMail || !e.f.userAtTape {
		t.Fatal("the user row was gone before the steps that need it finished")
	}
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("the user row survives the purge")
	}
	if d, _ := e.s.AccountDeletionFor(e.user.ID); d != nil {
		t.Fatal("the job row survives")
	}
	e.pass() // nothing left to do
	if e.f.count("mail") != 1 || e.f.count("tape") != 1 {
		t.Fatalf("a step ran twice: %v", e.f.calls)
	}
}

// Failure 6: the receipt.
func TestReceiptHoldsCountsAndTimesOnly(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	_ = e.r.AtOnce(context.Background(), e.user.ID)
	jobBefore, _ := e.s.AccountDeletionFor(e.user.ID)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	rec, err := e.s.DeletionReceiptByJob(jobBefore.JobID)
	if err != nil || rec == nil {
		t.Fatalf("no receipt: %v", err)
	}
	blob := rec.ID + rec.JobID + rec.UserHash + rec.Summary
	for _, bad := range []string{e.user.ID, "purge-user", "example.com", "@"} {
		if strings.Contains(blob, bad) {
			t.Fatalf("receipt holds %q: %s", bad, blob)
		}
	}
	for _, step := range []string{`"mail"`, `"data"`, `"computers"`, `"gateway"`, `"tape"`, `"takes_deleted":2`} {
		if !strings.Contains(rec.Summary, step) {
			t.Fatalf("receipt lacks %s: %s", step, rec.Summary)
		}
	}
}

// Failure 3: a failed step finalises nothing and is retried from where it was.
func TestAFailedStepFinalisesNothingAndIsRetried(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.fail["data"] = errors.New("corpusd down")
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err != nil {
		t.Fatal("the account was finalised though a step failed")
	}
	if e.f.count("destroy") != 0 || e.f.count("tape") != 0 {
		t.Fatalf("steps after a failure ran: %v", e.f.calls)
	}
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if d.State != auth.DeletionPurging || d.AttemptStart != nil || !strings.Contains(d.LastError, "corpusd down") {
		t.Fatalf("record: %+v", d)
	}
	if _, done := d.Steps["mail"]; !done {
		t.Fatal("the finished step was not recorded")
	}
	delete(e.f.fail, "data")
	e.pass()
	if e.f.count("mail") != 1 {
		t.Fatalf("a finished step ran again: %v", e.f.calls)
	}
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("not finalised after the retry")
	}
}

// The tape step: running waits; ANY refusal halts. A refusal at the tape never
// finalizes the account, whether or not it is called permanent: the user row
// stays, the record stays purging, nothing after the step runs, and the purge
// finishes when the refusal clears (seat 1 and 2, round 2, blocking 2).
func TestATapeRefusalHaltsAndNeverFinalizes(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.tape = TapeResult{Running: true}
	e.pass()
	for _, res := range []TapeResult{
		{Refusal: "computer-abc: head moved within the quiet window"},
		{Refusal: "computer-abc: in registry list"},
		{Refusal: "computer-abc: has route slot", Permanent: true},
		{Refusal: "protected computer: hard refusal list", Permanent: true},
	} {
		e.f.tape = res
		e.pass()
		e.clock = e.clock.Add(100 * time.Hour) // however long it waits, it does not finalize
		e.pass()
		if _, err := e.s.GetUserByID(e.user.ID); err != nil {
			t.Fatalf("finalised past the refusal %q", res.Refusal)
		}
		d, _ := e.s.AccountDeletionFor(e.user.ID)
		if d == nil || d.State != auth.DeletionPurging || !strings.Contains(d.LastError, res.Refusal) {
			t.Fatalf("after %q the record is %+v", res.Refusal, d)
		}
		if _, done := d.Steps["tape"]; done {
			t.Fatalf("the tape step was recorded done past the refusal %q", res.Refusal)
		}
	}
	e.f.tape = TapeResult{Counts: Counts{"files_deleted": 9}}
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("the purge did not finish when the refusal cleared")
	}
}

// Failure 7: a protected account reaches no destructive step.
func TestProtectedAccountReachesNoDestructiveStep(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.f.computers = []vmctl.AccountComputer{
		{ComputerID: "computer-paper", Class: "critical_protected", Refused: "protected class critical_protected"},
		{ComputerID: "computer-abc", Class: "primary"},
	}
	e.clock = e.clock.Add(100 * time.Hour)
	e.pass()
	if e.f.destructive() != 0 {
		t.Fatalf("destructive calls for a protected account: %v", e.f.calls)
	}
	if _, err := e.s.GetUserByID(e.user.ID); err != nil {
		t.Fatal("the protected account was deleted")
	}
	if err := e.r.Precheck(context.Background(), e.user.ID); !errors.Is(err, auth.ErrDeletionRefused) {
		t.Fatalf("precheck: %v", err)
	}
	e.f.computers = []vmctl.AccountComputer{{ComputerID: "computer-abc", Class: "public_platform"}}
	if err := e.r.Precheck(context.Background(), e.user.ID); !errors.Is(err, auth.ErrDeletionRefused) {
		t.Fatalf("class alone must refuse: %v", err)
	}
	e.f.computers = nil
	e.f.listErr = errors.New("vmctl down")
	if err := e.r.Precheck(context.Background(), e.user.ID); err == nil || errors.Is(err, auth.ErrDeletionRefused) {
		t.Fatalf("an unreachable vmctl must fail closed as unavailable: %v", err)
	}
	e.pass()
	if e.f.destructive() != 0 {
		t.Fatal("a purge ran with the computers unlisted")
	}
}

// Failure 2: a cancelled record is not purged, and a new request starts a
// fresh 72 hours.
func TestCancelledDeletionIsNotPurgedAndRestartsTheClock(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	token, err := e.s.OfferDeletionCancel(e.user.ID, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.CancelAccountDeletion(token, e.clock); err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(100 * time.Hour)
	e.pass()
	if e.f.destructive() != 0 {
		t.Fatalf("a cancelled deletion was purged: %v", e.f.calls)
	}
	e.request(t) // asked again
	e.pass()
	if e.f.destructive() != 0 {
		t.Fatal("the new request was purged on the old record's time")
	}
	e.clock = e.clock.Add(purgeguard.GracePeriod - time.Hour)
	e.pass()
	if e.f.destructive() != 0 {
		t.Fatal("purged before the new 72 hours")
	}
}

// Failure 5: a crash leaves an attempt open; boot closes it as interrupted
// and does no work; the next tick starts a new attempt.
func TestBootClosesAnInterruptedAttemptAndDoesNoWork(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	if _, err := e.s.BeginPurge(e.user.ID, e.clock); err != nil { // the crash: no EndAttempt
		t.Fatal(err)
	}
	before := len(e.f.calls)
	e.r.closeInterruptedAttempts()
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if d.AttemptStart != nil || d.LastError != "interrupted by a restart" {
		t.Fatalf("attempt not closed: %+v", d)
	}
	if len(e.f.calls) != before {
		t.Fatal("boot did work")
	}
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("the next tick did not finish the job")
	}
}

// Failure 2: one pass at a time.
func TestPassesDoNotOverlap(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); e.pass() }()
	}
	wg.Wait()
	if e.f.count("mail") != 1 || e.f.count("tape") != 1 {
		t.Fatalf("overlapping passes repeated a step: %v", e.f.calls)
	}
}

// Failure 8 (found on review): a cancel that lands while a pass is holding the
// account's takes back must not leave them suspended for an account that no
// longer has a record. The pass notices the record is gone and restores.
func TestCancelDuringAPassDoesNotLeaveTakesSuspended(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.f.onSuspend = func() {
		token, err := e.s.OfferDeletionCancel(e.user.ID, e.clock)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := e.s.CancelAccountDeletion(token, e.clock); err != nil {
			t.Error(err)
		}
	}
	e.pass()
	if d, _ := e.s.AccountDeletionFor(e.user.ID); d != nil {
		t.Fatal("the cancel did not remove the record")
	}
	if e.f.count("suspend") == 0 {
		t.Fatal("the pass never suspended (the test did not run the race)")
	}
	if e.f.count("restore") == 0 {
		t.Fatalf("takes stayed suspended for an account with no deletion record: %v", e.f.calls)
	}
}

// Failure 9: the disk purge is its own step, logged where an operator reads
// it: the job id, the step name, the computer ids, begin and end, with counts.
// The log never carries the user id or the email address.
func TestDestructiveStepsAreLoggedByJobAndComputerNotByPerson(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	e := newEnv(t)
	e.request(t)
	_ = e.r.AtOnce(context.Background(), e.user.ID)
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	out := buf.String()
	for _, step := range []string{"mail", "data", "computers", "gateway", "tape"} {
		for _, phase := range []string{"begin", "done"} {
			if !strings.Contains(out, "job="+d.JobID+" step="+step+" "+phase) {
				t.Fatalf("no %s line for step %s:\n%s", phase, step, out)
			}
		}
	}
	if !strings.Contains(out, "step=computers begin computers=computer-abc") {
		t.Fatalf("the disk step does not name the computer it will destroy:\n%s", out)
	}
	for _, bad := range []string{e.user.ID, "purge-user", "example.com"} {
		if strings.Contains(out, bad) {
			t.Fatalf("the log names the person (%q):\n%s", bad, out)
		}
	}
}

// A failed step is logged as failed, with the job and the step.
func TestAFailedDestructiveStepIsLoggedAsFailed(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.fail["destroy"] = errors.New("disk busy")
	e.pass()
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if !strings.Contains(buf.String(), "job="+d.JobID+" step=computers failed") {
		t.Fatalf("no failed line:\n%s", buf.String())
	}
}

// V5 (seat 2 B2): a cancel that lands between a pass's list and its suspend
// leaves a tombstone; the pass finds it after its own suspend and serves the
// takes again, so the last act on the takes is a restore, never a suspend.
func TestACancelBetweenTheListAndTheSuspendEndsWithTheTakesServed(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.f.onStop = func() {
		tok, err := e.s.OfferDeletionCancel(e.user.ID, e.clock)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := e.s.CancelAccountDeletion(tok, e.clock); err != nil {
			t.Error(err)
			return
		}
		e.f.rec("restore") // the cancel handler's own restore, before the pass suspends
	}
	e.pass()
	e.f.onStop = nil
	last := ""
	e.f.mu.Lock()
	for _, c := range e.f.calls {
		if c == "suspend" || c == "restore" {
			last = c
		}
	}
	e.f.mu.Unlock()
	if last != "restore" {
		t.Fatalf("the takes were left suspended after a cancel: %v", e.f.calls)
	}
}

// The tombstone a cancel leaves is finished by the loop: takes served, then the
// row gone. A restore that fails keeps the tombstone for the next pass.
func TestTheLoopFinishesACancelledDeletion(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	tok, _ := e.s.OfferDeletionCancel(e.user.ID, e.clock)
	if _, err := e.s.CancelAccountDeletion(tok, e.clock); err != nil {
		t.Fatal(err)
	}
	e.f.restoreErr = errors.New("corpusd down")
	e.pass()
	open, _ := e.s.OpenAccountDeletions(0)
	if len(open) != 1 || e.f.count("restore") != 1 || e.f.destructive() != 0 {
		t.Fatalf("after a failed restore: %d rows, calls %v", len(open), e.f.calls)
	}
	e.f.restoreErr = nil
	e.pass()
	if open, _ := e.s.OpenAccountDeletions(0); len(open) != 0 {
		t.Fatalf("the tombstone survives a successful restore: %+v", open)
	}
	// A tombstone never makes anything destructive eligible, however old.
	e.clock = e.clock.Add(purgeguard.GracePeriod * 2)
	e.pass()
	if e.f.destructive() != 0 || e.f.count("stop") != 0 {
		t.Fatalf("a cancelled account was touched: %v", e.f.calls)
	}
}

// S5 (seat 2): what vmctl actually destroyed joins the record before the
// gateway and the tape, so a computer made after the runner's list is not
// left behind; and the gateway is asked for VM ids too (seat 1 #1).
func TestWhatWasDestroyedIsForgottenAndItsTapePurged(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	_ = e.r.AtOnce(context.Background(), e.user.ID)
	e.f.destroyed = &Destroyed{ComputerIDs: []string{"computer-abc", "computer-late"}, VMIDs: []string{"vm-1", "vm-late"}}
	var tapeIDs []string
	e.f.onTape = func(a Authz) { tapeIDs = a.Computers() }
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	for _, want := range []string{"gateway:computer-abc", "gateway:computer-late", "gateway:vm-1", "gateway:vm-late"} {
		if e.f.count(want) != 1 {
			t.Fatalf("%s not asked: %v", want, e.f.calls)
		}
	}
	if strings.Join(tapeIDs, ",") != "computer-abc,computer-late" {
		t.Fatalf("tape named %v", tapeIDs)
	}
}

// Seat 1 #10: a failed stop does not skip the takes.
func TestAFailedStopStillSuspendsTheTakes(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.f.fail["stop"] = errors.New("vmctl down")
	if err := e.r.AtOnce(context.Background(), e.user.ID); err == nil {
		t.Fatal("a failed stop was not reported")
	}
	if e.f.count("suspend") != 1 {
		t.Fatalf("takes not suspended after a failed stop: %v", e.f.calls)
	}
}

// Round 2, blocking 1: every precondition is checked before anything
// irreversible. A preflight that fails leaves the account whole: pending, with
// no destructive call made, and cancellable.
func TestAFailingPreflightLeavesTheAccountWholeAndCancellable(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	_ = e.r.AtOnce(context.Background(), e.user.ID)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.preflightErr = errors.New("vmctl cannot read the deletion record")
	for i := 0; i < 3; i++ {
		e.pass()
	}
	if e.f.destructive() != 0 {
		t.Fatalf("something irreversible ran after a failed preflight: %v", e.f.calls)
	}
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if d == nil || d.State != auth.DeletionPending {
		t.Fatalf("the record left pending: %+v", d)
	}
	// Still cancellable inside the same moment: the offer needs the clock before
	// purge_after, so move the record's purge time to now+1h as a late runner would see it.
	if _, err := e.s.OfferDeletionCancel(e.user.ID, e.clock.Add(-2*time.Hour)); err != nil {
		t.Fatalf("a pending account whose preflight fails cannot be cancelled: %v", err)
	}
	// When the read works, the purge goes.
	e.f.preflightErr = nil
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("the purge did not run once preflight passed")
	}
	// And preflight came before every destructive call.
	first := map[string]int{}
	for i, c := range e.f.calls {
		if _, ok := first[c]; !ok {
			first[c] = i
		}
	}
	for _, c := range []string{"mail", "data", "destroy", "tape"} {
		if first["preflight"] > first[c] {
			t.Fatalf("%s ran before preflight: %v", c, e.f.calls)
		}
	}
}

// Round 2, blocking 3: the gateway and the tape are driven from the computers
// that were the account's when the purge began (and what vmctl destroyed), not
// from the list taken at the request. A computer that left the account inside
// the 72 hours keeps its live bearer and its tape.
func TestAComputerThatLeftTheAccountInsideThe72HoursIsNotForgotten(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	// At the request the account owns computer-abc and computer-moved.
	e.f.computers = []vmctl.AccountComputer{
		{ComputerID: "computer-abc", VMID: "vm-1", DesktopID: "primary", State: "stopped", Class: "primary"},
		{ComputerID: "computer-moved", VMID: "vm-moved", DesktopID: "second", State: "stopped", Class: "primary"},
	}
	if err := e.r.AtOnce(context.Background(), e.user.ID); err != nil {
		t.Fatal(err)
	}
	// Inside the 72 hours computer-moved is transferred away: vmctl lists one.
	e.f.computers = []vmctl.AccountComputer{{ComputerID: "computer-abc", VMID: "vm-1", DesktopID: "primary", State: "stopped", Class: "primary"}}
	var tapeIDs []string
	e.f.onTape = func(a Authz) { tapeIDs = a.Computers() }
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	for _, bad := range []string{"gateway:computer-moved", "gateway:vm-moved"} {
		if e.f.count(bad) != 0 {
			t.Fatalf("%s: a computer that left the account was forgotten: %v", bad, e.f.calls)
		}
	}
	if e.f.count("gateway:computer-abc") != 1 || e.f.count("gateway:vm-1") != 1 {
		t.Fatalf("the account's own computer was not forgotten: %v", e.f.calls)
	}
	if strings.Join(tapeIDs, ",") != "computer-abc" {
		t.Fatalf("the tape was asked for %v", tapeIDs)
	}
}

// Round 2 (seat 1 #9): a destroy that fails after the first of two computers
// records the first, so the gateway and the tape reach it when the retry
// finishes; and nothing finalizes in between.
func TestAPartialDestroyIsRecordedAndFinishedByTheRetry(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.f.destroyFailsOnce = true
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err != nil {
		t.Fatal("finalised after a failed destroy")
	}
	d, _ := e.s.AccountDeletionFor(e.user.ID)
	if len(d.DestroyComputers) == 0 || d.DestroyComputers[0] != "computer-abc" {
		t.Fatalf("what was destroyed is not recorded: %+v", d)
	}
	e.pass() // the retry: vmctl lists nothing now; the record still names computer-abc
	if e.f.count("gateway:computer-abc") != 1 {
		t.Fatalf("the retry did not reach the gateway for the destroyed computer: %v", e.f.calls)
	}
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("not finalised after the retry")
	}
}

// A computer the account owns when the steps are done is not left behind: its
// steps run again before the user row goes.
func TestAComputerMadeAfterTheListIsNotLeftBehind(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.onTape = func(Authz) { // after destroy, before the end: a new computer appears
		e.f.computers = []vmctl.AccountComputer{{ComputerID: "computer-late", VMID: "vm-late", DesktopID: "x", State: "stopped", Class: "primary"}}
	}
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err != nil {
		t.Fatal("finalised with a computer still owned")
	}
	e.f.onTape = nil
	e.pass()
	if e.f.count("gateway:computer-late") != 1 {
		t.Fatalf("the late computer was not forgotten: %v", e.f.calls)
	}
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("not finalised after the late computer was handled")
	}
}

// The ledger: a finished deletion leaves one line outside auth.db, with the
// hash of the user id and the job, never the user id or the address.
func TestAFinishedDeletionIsWrittenToTheLedgerWithoutThePerson(t *testing.T) {
	e := newEnv(t)
	path := t.TempDir() + "/ledger.jsonl"
	e.r.LedgerPath = path
	e.request(t)
	jobBefore, _ := e.s.AccountDeletionFor(e.user.ID)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.pass()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(raw)
	if !strings.Contains(line, auth.DeletionUserHash(e.user.ID)) || !strings.Contains(line, jobBefore.JobID) {
		t.Fatalf("the ledger line: %s", line)
	}
	for _, bad := range []string{e.user.ID, "purge-user", "example.com", "@"} {
		if strings.Contains(line, bad) {
			t.Fatalf("the ledger names the person (%q): %s", bad, line)
		}
	}
}
