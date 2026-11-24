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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
}

func (a Authz) UserID() string         { return a.userID }
func (a Authz) JobID() string          { return a.jobID }
func (a Authz) RequestedAt() time.Time { return a.requestedAt }
func (a Authz) Computers() []string    { return append([]string(nil), a.computers...) }

// Counts are numbers only; they go into the receipt.
type Counts map[string]int64

// TapeResult is what the tape step reports.
type TapeResult struct {
	Running   bool
	Counts    Counts
	Refusal   string // named, not hidden
	Permanent bool   // a refusal waiting will not change
}

// Steps is the host side of the runner, over HTTP in production and faked in
// tests. Every destructive method takes the Authz.
type Steps interface {
	// Computers lists the account's computers and which are refused.
	Computers(ctx context.Context, userID string) ([]vmctl.AccountComputer, error)
	StopComputers(ctx context.Context, userID string) error
	SuspendTakes(ctx context.Context, userID string) error
	RestoreTakes(ctx context.Context, userID string) error

	PurgeMail(ctx context.Context, a Authz) (Counts, error)
	PurgeData(ctx context.Context, a Authz) (Counts, error)
	DestroyComputers(ctx context.Context, a Authz) (Counts, error)
	ForgetGateway(ctx context.Context, a Authz, computerID string) error
	PurgeTape(ctx context.Context, a Authz) (TapeResult, error)
}

// Runner is the loop and the auth.DeletionHost.
type Runner struct {
	store *auth.Store
	steps Steps
	now   func() time.Time
	// Interval is the tick; zero is one minute.
	Interval time.Duration
	mu       sync.Mutex // one pass at a time
}

// New makes a runner.
func New(store *auth.Store, steps Steps) *Runner {
	return &Runner{store: store, steps: steps, now: func() time.Time { return time.Now().UTC() }}
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
// record. It records the computers the first time they are seen.
func (r *Runner) assertPending(ctx context.Context, d *auth.AccountDeletion) error {
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
	if len(ids) != len(d.Computers) {
		if err := r.store.RecordDeletionComputers(d.UserID, ids, r.now()); err != nil {
			return err
		}
	}
	if err := r.steps.StopComputers(ctx, d.UserID); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	if err := r.steps.SuspendTakes(ctx, d.UserID); err != nil {
		return fmt.Errorf("suspend takes: %w", err)
	}
	// A cancel can land while this pass was working from a list read before
	// it. The record is the truth: if it is gone, the takes this pass just
	// held back are served again, so a cancelled account is never left
	// suspended with nothing that names why.
	if cur, err := r.store.AccountDeletionFor(d.UserID); err == nil && cur == nil {
		log.Printf("accountpurge: job=%s cancelled during a pass; takes restored", d.JobID)
		return r.steps.RestoreTakes(ctx, d.UserID)
	}
	if _, done := d.Steps["at_once"]; !done {
		return r.store.RecordDeletionStep(d.UserID, "at_once", auth.DeletionStep{DoneAt: r.now()})
	}
	return nil
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
		if d.State == auth.DeletionPending && now.Before(d.PurgeAfter) {
			if err := r.assertPending(ctx, d); err != nil {
				log.Printf("accountpurge: job=%s pending: %v", d.JobID, err)
			}
			continue
		}
		if err := r.purge(ctx, d); err != nil {
			log.Printf("accountpurge: job=%s purge: %v", d.JobID, err)
		}
	}
}

// authorize is the only maker of an Authz: it re-reads the record and applies
// the rule at the moment of the call.
func (r *Runner) authorize(userID string, now time.Time) (Authz, error) {
	if err := r.store.PurgeEligible(userID, now); err != nil {
		return Authz{}, err
	}
	d, err := r.store.AccountDeletionFor(userID)
	if err != nil || d == nil {
		return Authz{}, purgeguard.ErrNoRecord
	}
	return Authz{userID: d.UserID, jobID: d.JobID, requestedAt: d.RequestedAt, computers: append([]string(nil), d.Computers...)}, nil
}

// purge runs the destructive steps of one record whose grace period ended.
func (r *Runner) purge(ctx context.Context, d *auth.AccountDeletion) (err error) {
	now := r.now()
	a, err := r.authorize(d.UserID, now)
	if err != nil {
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
	ids := mergeIDs(d.Computers, computerIDs(computers))
	if err := r.store.RecordDeletionComputers(d.UserID, ids, now); err != nil {
		return err
	}
	if _, err := r.store.BeginPurge(d.UserID, now); err != nil {
		return err
	}
	a.computers = ids
	defer func() {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_ = r.store.EndAttempt(d.UserID, msg, r.now())
	}()
	fresh, err := r.store.AccountDeletionFor(d.UserID)
	if err != nil || fresh == nil {
		return errors.New("record vanished")
	}
	done := fresh.Steps

	// Each destructive step is logged on its own where an operator reads it:
	// the job id (never the user id or the address), the step, the computers
	// it acts on, and how it ended. The disk of a computer goes in the
	// "computers" step and nowhere else.
	computersDetail := " computers=" + strings.Join(a.computers, ",")
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
	if err := run("computers", computersDetail, func() (Counts, error) { return r.steps.DestroyComputers(ctx, a) }); err != nil {
		return err
	}
	if err := run("gateway", computersDetail, func() (Counts, error) {
		var n int64
		for _, id := range a.computers {
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
		log.Printf("accountpurge: job=%s step=tape begin%s", d.JobID, computersDetail)
		res, err := r.steps.PurgeTape(ctx, a)
		if err != nil {
			log.Printf("accountpurge: job=%s step=tape failed%s: %v", d.JobID, computersDetail, err)
			return fmt.Errorf("tape: %w", err)
		}
		switch {
		case res.Running:
			return nil // the next tick polls
		case res.Refusal != "" && !res.Permanent:
			log.Printf("accountpurge: job=%s step=tape waiting%s: %s", d.JobID, computersDetail, res.Refusal)
			return fmt.Errorf("tape: not yet: %s", res.Refusal)
		}
		log.Printf("accountpurge: job=%s step=tape done%s counts=%v refused=%q", d.JobID, computersDetail, res.Counts, res.Refusal)
		if err := r.store.RecordDeletionStep(d.UserID, "tape", auth.DeletionStep{DoneAt: r.now(), Counts: res.Counts, Refused: res.Refusal}); err != nil {
			return err
		}
	}

	fresh, err = r.store.AccountDeletionFor(d.UserID)
	if err != nil || fresh == nil {
		return errors.New("record vanished")
	}
	summary, err := receiptSummary(fresh)
	if err != nil {
		return err
	}
	rec, err := r.store.FinalizeAccountDeletion(d.UserID, summary, r.now())
	if err != nil {
		return err
	}
	log.Printf("accountpurge: job=%s receipt=%s purged", rec.JobID, rec.ID)
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
