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
	mu         sync.Mutex
	store      *auth.Store
	calls      []string
	computers  []vmctl.AccountComputer
	listErr    error
	fail       map[string]error
	tape       TapeResult
	userAtTape bool
	userAtMail bool
	// onSuspend, when set, runs inside SuspendTakes: a cancel arriving while
	// a pass is in flight.
	onSuspend func()
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
	return f.fail["stop"]
}
func (f *fakeSteps) SuspendTakes(context.Context, string) error {
	f.rec("suspend")
	if f.onSuspend != nil {
		f.onSuspend()
	}
	return f.fail["suspend"]
}
func (f *fakeSteps) RestoreTakes(context.Context, string) error { f.rec("restore"); return nil }
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
func (f *fakeSteps) DestroyComputers(context.Context, Authz) (Counts, error) {
	f.rec("destroy")
	return Counts{"computers": 1}, f.fail["destroy"]
}
func (f *fakeSteps) ForgetGateway(_ context.Context, _ Authz, id string) error {
	f.rec("gateway:" + id)
	return f.fail["gateway"]
}
func (f *fakeSteps) PurgeTape(_ context.Context, a Authz) (TapeResult, error) {
	f.rec("tape")
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
	want := []string{"mail", "data", "destroy", "gateway:computer-abc", "tape"}
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

// The tape step: running waits, a transient refusal waits, a permanent one is
// named in the receipt and the rest finishes.
func TestTapeStepRunningTransientAndPermanent(t *testing.T) {
	e := newEnv(t)
	e.request(t)
	e.clock = e.clock.Add(purgeguard.GracePeriod + time.Minute)
	e.f.tape = TapeResult{Running: true}
	e.pass()
	e.f.tape = TapeResult{Refusal: "computer-abc: head moved within the quiet window"}
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err != nil {
		t.Fatal("finalised while the tape was running or refused for now")
	}
	e.f.tape = TapeResult{Refusal: "computer-abc: has route slot", Permanent: true}
	jobID := func() string { d, _ := e.s.AccountDeletionFor(e.user.ID); return d.JobID }()
	e.pass()
	if _, err := e.s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("a permanent refusal blocked the deletion forever")
	}
	rec, _ := e.s.DeletionReceiptByJob(jobID)
	if rec == nil || !strings.Contains(rec.Summary, "has route slot") {
		t.Fatalf("the refusal is not named in the receipt: %+v", rec)
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
