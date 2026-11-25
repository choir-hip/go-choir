// Package accountpurge runs account deletions after the auth service has
// recorded them (docs/design/account-create-delete-2026-11-23.md, with the
// owner's rulings of 2026-11-23).
//
// The auth service writes the deletion record and revokes access in one
// transaction (internal/auth/deletion_store.go). This package is what the
// record is for: a loop, in the auth process, that re-reads the table each
// tick and does what the table says.
//
//   - Pending record, any age: the account's computers stay stopped and its
//     takes stay suspended (re-asserted every tick, never assumed).
//   - Pending record whose 72 hours have passed: the purge. The runner moves
//     the record to purging (cancel is refused from then on) and runs the
//     destructive steps in order: mail, platform data, the computers, the
//     gateway identity, the tape, then one transaction that deletes the user
//     row and writes the receipt.
//
// Restarts (CLAUDE.md "Restarts"). A deletion is a standing intent of the
// person, not agent work, and the loop is a re-evaluation of a table, not a
// resumer: it runs on a timer at all times, and a boot does nothing special
// except one thing, which is to close an attempt that a crash left open as
// "interrupted by a restart" in the record. The next tick then starts a new
// attempt from the steps the record shows done. Every destructive step is
// idempotent, and the order is the crash-safety argument: the user row, which
// the cancel path and every sign-in check read, goes last and in the same
// transaction as the receipt, so a crash anywhere leaves a record the next
// tick finishes, never a half-deleted account with nothing that names it. No
// planned-restart marker is needed because no agent work is resumed.
//
// Every destructive call carries a capability only authorize() can make, and
// authorize() re-reads the record and applies purgeguard: no record, or one
// younger than 72 hours, gets no capability. The services that hold the data
// apply the same rule again (vmctl reads the record; corpusd, maild and the
// gateway receive its request time).
//
// Ways this could go wrong, written before the code (tests in
// runner_test.go):
//  1. a destructive step runs for a record younger than 72 hours, or with no
//     record, or for a cancelled one;
//  2. a purge runs twice at once, or a cancelled-then-requested account is
//     purged on the old record's time;
//  3. a step's failure finalises the account anyway, or a refusal is hidden;
//  4. the user row goes before the steps that need its id are recorded;
//  5. a crash leaves an attempt that nothing closes, or a boot resumes work
//     with no record;
//  6. the receipt holds an email, an id, or content;
//  7. a protected account (the paper, the owner) reaches a destructive step.
package accountpurge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Authz is the capability a destructive step requires. Its fields are
// unexported, so only authorize() makes one.
type Authz struct {
	userID      string
	jobID       string
	requestedAt time.Time
	computers   []string
	vmIDs       []string
}

func (a Authz) UserID() string         { return a.userID }
func (a Authz) JobID() string          { return a.jobID }
func (a Authz) RequestedAt() time.Time { return a.requestedAt }
func (a Authz) Computers() []string    { return append([]string(nil), a.computers...) }
func (a Authz) VMIDs() []string        { return append([]string(nil), a.vmIDs...) }

// Destroyed is what vmctl reports it removed: the product computer ids and the
// VM ids behind them.
type Destroyed struct {
	ComputerIDs []string
	VMIDs       []string
}

// alertAttempts is how many failed attempts of one purge pass before every
// further failure is logged at ALERT level: a purge that cannot finish stays
// in purging (it must never finalize past a refusal), so it must be loud.
const alertAttempts = 5

// stuckAlertAfter is how long past its purge time a record may stay before
// every failed pass is logged at ALERT level whatever the attempt count: a
// preflight that fails never counts an attempt, so age is the alarm.
const stuckAlertAfter = time.Hour

// Counts are numbers only; they go into the receipt.
type Counts map[string]int64

// TapeResult is what the tape step reports.
type TapeResult struct {
	Running   bool
	Counts    Counts
	Refusal   string // named, not hidden: ANY refusal halts the purge
	Permanent bool   // a refusal waiting will not change (reported; it halts all the same)
}

// Steps is the host side of the runner, over HTTP in production and faked in
// tests. Every destructive method takes the Authz.
type Steps interface {
	// Computers lists the account's computers and which are refused.
	Computers(ctx context.Context, userID string) ([]vmctl.AccountComputer, error)
	StopComputers(ctx context.Context, userID string) error
	SuspendTakes(ctx context.Context, userID string) error
	RestoreTakes(ctx context.Context, userID string) error

	// Preflight checks, before anything irreversible, every precondition of
	// the steps that can refuse (vmctl: the deletion record is readable and
	// old enough, no computer is protected, held or running a capsule). It
	// touches nothing.
	Preflight(ctx context.Context, a Authz) error

	// ForgetGatewayAtOnce forgets a computer's gateway identity at the request,
	// so the computer's provider key ends with the person's access. It needs
	// no capability: it destroys nothing the host cannot import again, and
	// the gateway still refuses a protected computer.
	ForgetGatewayAtOnce(ctx context.Context, userID, computerID string) error

	PurgeMail(ctx context.Context, a Authz) (Counts, error)
	PurgeData(ctx context.Context, a Authz) (Counts, error)
	DestroyComputers(ctx context.Context, a Authz) (Counts, Destroyed, error)
	ForgetGateway(ctx context.Context, a Authz, computerID string) error
	PurgeTape(ctx context.Context, a Authz) (TapeResult, error)
}

// Runner is the loop and the auth.DeletionHost.
type Runner struct {
	store        *auth.Store
	steps        Steps
	now          func() time.Time
	checkAccount func(string) error
	// Interval is the tick; zero is one minute.
	Interval time.Duration
	mu       sync.Mutex // one pass at a time
	// LedgerPath, when set, is a file the runner appends one line to for every
	// finished deletion: the hash of the user id, the job id and the time. It
	// lives outside auth.db and outside what the backup copies, so that after
	// a restore from backup the accounts that were deleted since can be found
	// and deleted again (cmd/deletion-restore-check). It holds no email and no
	// user id.
	LedgerPath string
	// ledgered is "job/state" of the ledger lines this process has written.
	ledgered map[string]bool
}

// New makes a runner.
func New(store *auth.Store, steps Steps, checkAccount func(string) error) *Runner {
	return &Runner{store: store, steps: steps, checkAccount: checkAccount, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock replaces the clock (tests).
func (r *Runner) SetClock(now func() time.Time) { r.now = now }

var _ auth.DeletionHost = (*Runner)(nil)

// Precheck refuses an account that owns a computer an account deletion never
// touches, and fails closed when the computers cannot be listed.
func (r *Runner) Precheck(ctx context.Context, userID string) error {
	computers, err := r.steps.Computers(ctx, userID)
	if err != nil {
		return err
	}
	for _, c := range computers {
		if c.Refused != "" || purgeguard.ClassRefused(c.Class) || c.Held {
			return fmt.Errorf("%w: %s", auth.ErrDeletionRefused, c.Refused)
		}
	}
	return nil
}

// AtOnce stops the account's computers and suspends its takes, and records
// the computers so the later steps still know them after vmctl forgets them.
func (r *Runner) AtOnce(ctx context.Context, userID string) error {
	d, err := r.store.AccountDeletionFor(userID)
	if err != nil {
		return err
	}
	if d == nil {
		return auth.ErrNoDeletion
	}
	return r.assertPending(ctx, d)
}

// Restore serves the account's takes again after a cancel.
func (r *Runner) Restore(ctx context.Context, userID string) error {
	return r.steps.RestoreTakes(ctx, userID)
}

// assertPending is the idempotent "stopped and suspended" state of a pending
// record. It records the computers the first time they are seen. Stopping the
// computers and holding the takes back are independent: one failing never
// skips the other, and both errors are reported.
func (r *Runner) assertPending(ctx context.Context, d *auth.AccountDeletion) error {
	if err := r.checkAccount(d.UserID); err != nil {
		return err
	}
	computers, err := r.steps.Computers(ctx, d.UserID)
	if err != nil {
		return err
	}
	for _, c := range computers {
		if c.Refused != "" {
			// The account became protected after the request. Nothing is
			// stopped or destroyed; the record stays and the error names it.
			return fmt.Errorf("%w: %s", auth.ErrDeletionRefused, c.Refused)
		}
	}
	ids := mergeIDs(d.Computers, computerIDs(computers))
	vmIDs := mergeIDs(d.VMIDs, vmIDsOf(computers))
	if len(ids) != len(d.Computers) || len(vmIDs) != len(d.VMIDs) {
		if err := r.store.RecordDeletionComputers(d.UserID, ids, vmIDs, r.now()); err != nil {
			return err
		}
	}
	var errs []error
	if err := r.steps.StopComputers(ctx, d.UserID); err != nil {
		errs = append(errs, fmt.Errorf("stop: %w", err))
	}
	if err := r.steps.SuspendTakes(ctx, d.UserID); err != nil {
		errs = append(errs, fmt.Errorf("suspend takes: %w", err))
	} else if cur, err := r.store.AccountDeletionFor(d.UserID); err == nil && cur == nil {
		// A cancel can land while this pass was working from a list read
		// before it. A cancel leaves a tombstone, which is no live record: if
		// there is none, the takes this pass just held back are served again,
		// so a cancelled account is never left suspended.
		log.Printf("accountpurge: job=%s cancelled during a pass; takes restored", d.JobID)
		if err := r.steps.RestoreTakes(ctx, d.UserID); err != nil {
			errs = append(errs, fmt.Errorf("restore takes: %w", err))
		}
		return errors.Join(errs...)
	}
	if _, done := d.Steps["at_once"]; !done {
		// The computer's gateway identity ends with the person's access, not 72
		// hours later. Once (the step is recorded); a cancel is followed by the
		// host importing the computer's own token again on the next resolve.
		for _, id := range mergeIDs(ids, vmIDs) {
			if err := r.steps.ForgetGatewayAtOnce(ctx, d.UserID, id); err != nil {
				errs = append(errs, fmt.Errorf("forget gateway %s: %w", id, err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if _, done := d.Steps["at_once"]; !done {
		return r.store.RecordDeletionStep(d.UserID, "at_once", auth.DeletionStep{DoneAt: r.now()})
	}
	return nil
}

// finishCancelled serves a cancelled account's takes again and deletes the
// tombstone the cancel left; a failure keeps the tombstone for the next pass.
func (r *Runner) finishCancelled(ctx context.Context, d *auth.AccountDeletion) error {
	if err := r.steps.RestoreTakes(ctx, d.UserID); err != nil {
		return fmt.Errorf("restore takes: %w", err)
	}
	log.Printf("accountpurge: job=%s cancelled; takes served again", d.JobID)
	return r.store.FinishCancelledDeletion(d.UserID)
}

func computerIDs(cs []vmctl.AccountComputer) []string {
	var out []string
	for _, c := range cs {
		if strings.TrimSpace(c.ComputerID) != "" {
			out = append(out, c.ComputerID)
		}
	}
	return out
}

func vmIDsOf(cs []vmctl.AccountComputer) []string {
	var out []string
	for _, c := range cs {
		if strings.TrimSpace(c.VMID) != "" {
			out = append(out, c.VMID)
		}
	}
	return out
}

func mergeIDs(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range append(append([]string{}, a...), b...) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// Start runs the loop until ctx ends. A boot first closes an attempt a crash
// left open.
func (r *Runner) Start(ctx context.Context) {
	r.closeInterruptedAttempts()
	interval := r.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			r.Tick(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (r *Runner) closeInterruptedAttempts() {
	open, err := r.store.OpenAccountDeletions(0)
	if err != nil {
		log.Printf("accountpurge: boot: list: %v", err)
		return
	}
	for _, d := range open {
		if d.AttemptStart != nil {
			log.Printf("accountpurge: job=%s attempt interrupted by a restart; the next pass starts a new one", d.JobID)
			_ = r.store.EndAttempt(d.UserID, "interrupted by a restart", r.now())
		}
	}
}

// Tick is one pass over the table.
func (r *Runner) Tick(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	open, err := r.store.OpenAccountDeletions(0)
	if err != nil {
		log.Printf("accountpurge: list: %v", err)
		return
	}
	now := r.now()
	for _, d := range open {
		if ctx.Err() != nil {
			return
		}
		if d.State == auth.DeletionCancelled {
			if err := r.finishCancelled(ctx, d); err != nil {
				log.Printf("accountpurge: job=%s cancelled: %v", d.JobID, err)
			}
			continue
		}
		if d.State == auth.DeletionPending {
			// Held every pass while pending, including past the purge time
			// when the purge cannot begin yet (its preflight fails): the
			// account is still the person's to cancel, and its computers stay
			// stopped and its takes held back until the purge begins.
			if err := r.assertPending(ctx, d); err != nil {
				log.Printf("accountpurge: job=%s pending: %v", d.JobID, err)
			}
			if now.Before(d.PurgeAfter) {
				continue
			}
		}
		if err := r.purge(ctx, d); err != nil {
			log.Printf("accountpurge: job=%s purge: %v", d.JobID, err)
		}
	}
}

// authorize is the only maker of an Authz: it re-reads the record and applies
// the rule at the moment of the call.
func (r *Runner) authorize(userID string, now time.Time) (Authz, error) {
	if err := r.checkAccount(userID); err != nil {
		return Authz{}, err
	}
	if err := r.store.PurgeEligible(userID, now); err != nil {
		return Authz{}, err
	}
	d, err := r.store.AccountDeletionFor(userID)
	if err != nil || d == nil {
		return Authz{}, purgeguard.ErrNoRecord
	}
	return Authz{userID: d.UserID, jobID: d.JobID, requestedAt: d.RequestedAt, computers: append([]string(nil), d.DestroyComputers...), vmIDs: append([]string(nil), d.DestroyVMIDs...)}, nil
}

// purge runs the destructive steps of one record whose grace period ended.
//
// The order is the safety argument. First everything that can refuse is
// checked and nothing is touched (the record is old enough, no computer is
// protected, vmctl can read the record it will rely on); a refusal here leaves
// the account whole, pending and cancellable. Only then does the record move to
// purging and the steps run, each re-checking the rule through a fresh
// capability. A refusal or failure at any step HALTS: the record stays
// purging, nothing after it runs, and the user row is never deleted past it.
//
// The gateway and the tape are driven from the destroy set, not from the list
// taken at the request: the computers vmctl listed as the account's own when
// this purge began, plus those it reported destroying. A computer that left
// the account inside the 72 hours is in neither.
func (r *Runner) purge(ctx context.Context, d *auth.AccountDeletion) (err error) {
	now := r.now()
	if _, err := r.authorize(d.UserID, now); err != nil {
		return err
	}
	// Refuse a protected account before anything is destroyed: the whole
	// account, from the live list, not only from the record.
	computers, lerr := r.steps.Computers(ctx, d.UserID)
	if lerr != nil {
		return fmt.Errorf("list computers: %w", lerr)
	}
	for _, c := range computers {
		if c.Refused != "" {
			_ = r.store.EndAttempt(d.UserID, "refused: "+c.Refused, now)
			return fmt.Errorf("%w: %s", auth.ErrDeletionRefused, c.Refused)
		}
	}
	if err := r.store.RecordDeletionComputers(d.UserID, mergeIDs(d.Computers, computerIDs(computers)), mergeIDs(d.VMIDs, vmIDsOf(computers)), now); err != nil {
		return err
	}
	// The preflight is asked about the computers listed now; nothing is written
	// to the record until it has passed (a pass that fails leaves no destroy set
	// behind to outlive the account's ownership).
	a, err := r.authorize(d.UserID, now)
	if err != nil {
		return err
	}
	a.computers, a.vmIDs = computerIDs(computers), vmIDsOf(computers)
	// Preflight: nothing irreversible has happened and the record is still
	// pending, so a failure here is a pause, not damage. The record stays
	// cancellable: the state, not the clock, is the boundary.
	if err := r.steps.Preflight(ctx, a); err != nil {
		r.alarmStuck(d, now, "preflight keeps failing", err)
		_ = r.store.EndAttempt(d.UserID, "preflight: "+err.Error(), now)
		return fmt.Errorf("preflight: %w", err)
	}
	// Written after the preflight and before anything is destroyed: a crash
	// after vmctl destroys a computer still leaves its id here. While the record
	// is pending nothing has been destroyed, so the set is the current listing
	// (a stale id is dropped); once purging it only grows.
	if d.State == auth.DeletionPending {
		err = r.store.ReplaceDeletionDestroySet(d.UserID, computerIDs(computers), vmIDsOf(computers), now)
	} else {
		err = r.store.RecordDeletionDestroySet(d.UserID, computerIDs(computers), vmIDsOf(computers), now)
	}
	if err != nil {
		return err
	}
	if _, err := r.store.BeginPurge(d.UserID, now); err != nil {
		return err
	}
	defer func() {
		msg := ""
		if err != nil {
			msg = err.Error()
			r.alarmStuck(d, r.now(), fmt.Sprintf("purge halted at attempt %d and stays purging until this is fixed", d.Attempts+1), err)
		}
		_ = r.store.EndAttempt(d.UserID, msg, r.now())
	}()
	// The ledger has the account from the moment its purge begins: a host lost
	// while the purge is half done must not restore the account unnoticed. A
	// ledger that cannot be written halts here, before the first removal.
	if err := r.writeLedger(d, "purging"); err != nil {
		// Only the pass that began the purge knows nothing was removed: a
		// step is recorded after it succeeds, so a record already purging
		// when this pass read it may have had data removed by a step that
		// failed afterwards (maild commits its rows, then a directory removal
		// fails). That record stays purging (fail closed); this one goes back
		// to pending, still the person's to cancel.
		if d.State == auth.DeletionPending {
			if rerr := r.store.ReturnToPending(d.UserID, r.now()); rerr != nil {
				return errors.Join(err, rerr)
			}
		}
		return err
	}
	fresh, err := r.store.AccountDeletionFor(d.UserID)
	if err != nil || fresh == nil {
		return errors.New("record vanished")
	}
	done := fresh.Steps
	a, err = r.authorize(d.UserID, r.now())
	if err != nil {
		return err
	}

	// Each destructive step is logged on its own where an operator reads it:
	// the job id (never the user id or the address), the step, the computers
	// it acts on, and how it ended. The disk of a computer goes in the
	// "computers" step and nowhere else.
	detail := func() string { return " computers=" + strings.Join(a.computers, ",") }
	run := func(name, detail string, f func() (Counts, error)) error {
		if _, ok := done[name]; ok {
			return nil
		}
		// Each destructive step re-checks the rule through a fresh
		// capability: nothing carries one across a step.
		if _, err := r.authorize(d.UserID, r.now()); err != nil {
			return err
		}
		log.Printf("accountpurge: job=%s step=%s begin%s", d.JobID, name, detail)
		c, err := f()
		if err != nil {
			log.Printf("accountpurge: job=%s step=%s failed%s: %v", d.JobID, name, detail, err)
			return fmt.Errorf("%s: %w", name, err)
		}
		log.Printf("accountpurge: job=%s step=%s done%s counts=%v", d.JobID, name, detail, c)
		return r.store.RecordDeletionStep(d.UserID, name, auth.DeletionStep{DoneAt: r.now(), Counts: c})
	}

	// The order: what needs the user's id and the computers first, the
	// computers' removal from the registry before the tape (the tape sweep
	// proves a computer destroyed by its absence), the user row last.
	if err := run("mail", "", func() (Counts, error) { return r.steps.PurgeMail(ctx, a) }); err != nil {
		return err
	}
	if err := run("data", "", func() (Counts, error) { return r.steps.PurgeData(ctx, a) }); err != nil {
		return err
	}
	if err := run("computers", detail(), func() (Counts, error) {
		counts, gone, err := r.steps.DestroyComputers(ctx, a)
		// What vmctl actually removed joins the destroy set even when the step
		// failed halfway: the gateway and the tape must reach the computers
		// that are already gone.
		if len(gone.ComputerIDs) > 0 || len(gone.VMIDs) > 0 {
			a.computers = mergeIDs(a.computers, gone.ComputerIDs)
			a.vmIDs = mergeIDs(a.vmIDs, gone.VMIDs)
			if rerr := r.store.RecordDeletionDestroySet(d.UserID, gone.ComputerIDs, gone.VMIDs, r.now()); rerr != nil {
				return nil, errors.Join(err, rerr)
			}
		}
		if err != nil {
			return nil, err
		}
		return counts, nil
	}); err != nil {
		return err
	}
	// The gateway keys a credential by the VM it authenticates; the product
	// computer id is asked for too (it matches the identity's recorded
	// product computer).
	if err := run("gateway", detail(), func() (Counts, error) {
		var n int64
		for _, id := range mergeIDs(a.computers, a.vmIDs) {
			if err := r.steps.ForgetGateway(ctx, a, id); err != nil {
				return nil, err
			}
			n++
		}
		return Counts{"identities": n}, nil
	}); err != nil {
		return err
	}
	if _, ok := done["tape"]; !ok {
		if _, err := r.authorize(d.UserID, r.now()); err != nil {
			return err
		}
		log.Printf("accountpurge: job=%s step=tape begin%s", d.JobID, detail())
		res, err := r.steps.PurgeTape(ctx, a)
		if err != nil {
			log.Printf("accountpurge: job=%s step=tape failed%s: %v", d.JobID, detail(), err)
			return fmt.Errorf("tape: %w", err)
		}
		switch {
		case res.Running:
			return nil // the next tick polls
		case res.Refusal != "":
			// A refusal HALTS. The tape of a computer the sweep will not
			// destroy is personal data that is still there; the purge never
			// finalizes past it (the user row is what lets anyone find the
			// account to finish it). The record stays purging, the log says
			// why every minute, and the refusal clears when its cause does
			// (a quiet window closing, a registry entry going).
			log.Printf("accountpurge: job=%s step=tape refused%s permanent=%v: %s", d.JobID, detail(), res.Permanent, res.Refusal)
			return fmt.Errorf("tape refused: %s", res.Refusal)
		}
		log.Printf("accountpurge: job=%s step=tape done%s counts=%v", d.JobID, detail(), res.Counts)
		if err := r.store.RecordDeletionStep(d.UserID, "tape", auth.DeletionStep{DoneAt: r.now(), Counts: res.Counts}); err != nil {
			return err
		}
	}

	// Last check before the user row goes: a computer the account owns now
	// (one made after the list) is not left behind. Its steps run again.
	if left, err := r.steps.Computers(ctx, d.UserID); err != nil {
		return fmt.Errorf("list computers before the end: %w", err)
	} else if len(left) > 0 {
		if err := r.store.RecordDeletionDestroySet(d.UserID, computerIDs(left), vmIDsOf(left), r.now()); err != nil {
			return err
		}
		if err := r.store.ClearDeletionSteps(d.UserID, "computers", "gateway", "tape"); err != nil {
			return err
		}
		return fmt.Errorf("the account owns %d computer(s) made since the list; their steps run again", len(left))
	}

	fresh, err = r.store.AccountDeletionFor(d.UserID)
	if err != nil || fresh == nil {
		return errors.New("record vanished")
	}
	summary, err := receiptSummary(fresh)
	if err != nil {
		return err
	}
	// Durable before the transaction that deletes the user row: a crash between
	// the two leaves a ledger line and a record (the next pass finishes), never
	// a deleted account that no ledger knows.
	if _, err := r.authorize(d.UserID, r.now()); err != nil {
		return err
	}
	if err := r.writeLedger(d, "finishing"); err != nil {
		return err
	}
	rec, err := r.store.FinalizeAccountDeletion(d.UserID, summary, r.now())
	if err != nil {
		return err
	}
	log.Printf("accountpurge: job=%s receipt=%s purged", rec.JobID, rec.ID)
	return nil
}

// alarmStuck logs a stuck pass at ALERT level when it has failed enough times
// or has been stuck long enough, by what state the record is in.
func (r *Runner) alarmStuck(d *auth.AccountDeletion, now time.Time, what string, err error) {
	age := now.Sub(d.PurgeAfter)
	if d.Attempts < alertAttempts && age < stuckAlertAfter {
		return
	}
	if d.State == auth.DeletionPending {
		log.Printf("accountpurge: ALERT job=%s %s, stuck for %s past the deadline; the account is untouched and still cancellable: %v", d.JobID, what, age.Round(time.Minute), err)
		return
	}
	log.Printf("accountpurge: ALERT job=%s %s, stuck for %s past the deadline; the account's data may be partly removed and it cannot be cancelled (docs/runbooks/account-deletion-stuck.md): %v", d.JobID, what, age.Round(time.Minute), err)
}

// ledgerLine is one line of the ledger: the hash of the user id and the job,
// with the stage the purge had reached ("purging" when the record moved to
// purging, "finishing" just before the user row is deleted).
type ledgerLine struct {
	UserHash string `json:"user_hash"`
	JobID    string `json:"job_id"`
	State    string `json:"state"`
	At       string `json:"at"`
}

// writeLedger appends one line and makes it durable (file and directory
// fsynced) before it returns; a failure is an error, so the caller stops
// instead of going on without the line. Idempotent per job and state within a
// process (a stuck purge does not add a line a minute; a duplicate line is
// harmless, the reader keeps one per hash). With no LedgerPath it does nothing.
// trimTornLedgerTail cuts a last line that a crash left without its newline,
// so the next line does not run into it (that line is written again: the
// runner appends the line of each state on every pass until it lands).
func trimTornLedgerTail(path string) error {
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("deletion ledger %s is a symlink; it is not written through", path)
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || len(raw) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	if raw[len(raw)-1] == '\n' {
		return nil
	}
	keep := bytes.LastIndexByte(raw, '\n') + 1
	log.Printf("accountpurge: the deletion ledger ends in a torn line (%d bytes); it is cut", len(raw)-keep)
	return os.Truncate(path, int64(keep))
}

func (r *Runner) writeLedger(d *auth.AccountDeletion, state string) error {
	if r.LedgerPath == "" {
		return nil
	}
	key := d.JobID + "/" + state
	if r.ledgered[key] {
		return nil
	}
	raw, _ := json.Marshal(ledgerLine{UserHash: auth.DeletionUserHash(d.UserID), JobID: d.JobID, State: state, At: r.now().UTC().Format(time.RFC3339)})
	err := func() error {
		if err := trimTornLedgerTail(r.LedgerPath); err != nil {
			return err
		}
		f, err := os.OpenFile(r.LedgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(raw, '\n')); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if dir, err := os.Open(filepath.Dir(r.LedgerPath)); err == nil {
			_ = dir.Sync()
			_ = dir.Close()
		}
		return nil
	}()
	if err != nil {
		log.Printf("accountpurge: ALERT job=%s the deletion ledger could not be written (%s): %v; the purge stops here because a restore from backup could bring this account back unnoticed", d.JobID, r.LedgerPath, err)
		return fmt.Errorf("deletion ledger: %w", err)
	}
	if r.ledgered == nil {
		r.ledgered = map[string]bool{}
	}
	r.ledgered[key] = true
	return nil
}

// receiptSummary is counts and times only: the step names, when each was
// done, its counts, and any refusal named. No user id, no email, no content.
func receiptSummary(d *auth.AccountDeletion) (string, error) {
	type step struct {
		DoneAt  string           `json:"done_at"`
		Counts  map[string]int64 `json:"counts,omitempty"`
		Refused string           `json:"refused,omitempty"`
	}
	out := struct {
		RequestedAt string          `json:"requested_at"`
		Computers   int             `json:"computers"`
		Attempts    int             `json:"attempts"`
		Steps       map[string]step `json:"steps"`
	}{RequestedAt: d.RequestedAt.UTC().Format(time.RFC3339), Computers: len(d.Computers), Attempts: d.Attempts, Steps: map[string]step{}}
	for name, s := range d.Steps {
		out.Steps[name] = step{DoneAt: s.DoneAt.UTC().Format(time.RFC3339), Counts: s.Counts, Refused: s.Refused}
	}
	raw, err := json.Marshal(out)
	return string(raw), err
}
