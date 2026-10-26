package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Automatic checkpoints (docs/design/apply-checkpoint-without-replay-2026-10-26.md,
// 4.1, 4.2 and slice 3). The owner: "We shouldn't have to bind checkpoints
// manually. It shouldn't be a cli command but done automatically after n
// events or time."
//
// The cadence is a standing intent; each bind is an occurrence (the schedule
// rule of CLAUDE.md, restarts end work). Nothing resumes a bind at boot: the
// loop starts after Start returns, waits out an initial delay, and counts from
// the newest checkpoint the platform holds. A bind that crashes the computer
// twice turns the cadence off, with that cause, until the owner reopens it.

const (
	// defaultCheckpointEveryEvents is today's checkpointd cadence, whose
	// rebuild cost is measured (43 to 56 s), and keeps any restore tail under
	// a quarter of MaxRecoveryTailEvents.
	defaultCheckpointEveryEvents uint64 = 2500
	// defaultCheckpointEvery bounds how old the newest restore point of a
	// quiet computer can be.
	defaultCheckpointEvery = 60 * time.Minute

	envCheckpointEveryEvents = "CHOIR_CHECKPOINT_EVERY_EVENTS"
	envCheckpointEvery       = "CHOIR_CHECKPOINT_EVERY"

	// checkpointCadenceAttemptLimit is how many unfinished ticks close the
	// cadence: a tick that crashed the computer twice.
	checkpointCadenceAttemptLimit = 2
	checkpointCadenceAttemptKey   = "checkpoint_cadence"

	// A boot settles before the first tick; ticks are a minute apart.
	cadenceDefaultInitialDelay = 2 * time.Minute
	cadenceDefaultTickEvery    = time.Minute

	// A bind holds appends while it extracts the live store. Binds are spaced
	// by at least a minute and by twenty times the hold the last one cost, so
	// the hold is at most about 5% of the computer's time whatever its size
	// (design R2: the extract's time on the paper is not yet measured).
	cadenceMinSpacing  = time.Minute
	cadenceDutyFactor  = 20
	cadenceBackoffBase = 5 * time.Minute
	cadenceBackoffMax  = 60 * time.Minute
)

type checkpointCadenceConfig struct {
	// EveryEvents binds after this many events since the newest checkpoint;
	// zero turns the event trigger off.
	EveryEvents uint64
	// Every binds after this long since the newest checkpoint, when at least
	// one event has committed since; zero turns the time trigger off.
	Every time.Duration
}

func (c checkpointCadenceConfig) disabled() bool { return c.EveryEvents == 0 && c.Every == 0 }

// checkpointCadenceConfigFromEnv reads the two overrides the host can write to
// the guest's environment file. An unparsable or negative value keeps the
// default, loudly: a typo must not turn automatic checkpoints off.
func checkpointCadenceConfigFromEnv(getenv func(string) string) checkpointCadenceConfig {
	cfg := checkpointCadenceConfig{EveryEvents: defaultCheckpointEveryEvents, Every: defaultCheckpointEvery}
	if raw := strings.TrimSpace(getenv(envCheckpointEveryEvents)); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
			cfg.EveryEvents = n
		} else {
			log.Printf("runtime: %s=%q is not a count; keeping %d", envCheckpointEveryEvents, raw, cfg.EveryEvents)
		}
	}
	if raw := strings.TrimSpace(getenv(envCheckpointEvery)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
			cfg.Every = d
		} else {
			log.Printf("runtime: %s=%q is not a duration; keeping %s", envCheckpointEvery, raw, cfg.Every)
		}
	}
	return cfg
}

// checkpointMark is the newest published checkpoint: its head sequence and
// when it was published.
type checkpointMark struct {
	Sequence uint64
	At       time.Time
}

// due reports whether a checkpoint is due and why. No event since the newest
// checkpoint is never due: there are no empty checkpoints.
func (c checkpointCadenceConfig) due(headSequence uint64, last checkpointMark, now time.Time) (bool, string) {
	if headSequence <= last.Sequence {
		return false, ""
	}
	if c.EveryEvents > 0 && headSequence-last.Sequence >= c.EveryEvents {
		return true, "events"
	}
	if c.Every > 0 && !last.At.IsZero() && now.Sub(last.At) >= c.Every {
		return true, "time"
	}
	return false, ""
}

type cadenceOutcome string

const (
	cadenceIdle    cadenceOutcome = "idle"
	cadenceBound   cadenceOutcome = "bound"
	cadenceFailed  cadenceOutcome = "failed"
	cadenceOff     cadenceOutcome = "off"
	cadenceSkipped cadenceOutcome = "skipped"
	cadenceBackoff cadenceOutcome = "backoff"
)

// CheckpointCadenceStatus is what the checkpoint listing shows of the cadence.
type CheckpointCadenceStatus struct {
	// State is on, off (closed with Cause until the owner reopens it) or
	// disabled (configured off).
	State             string `json:"state"`
	Cause             string `json:"cause,omitempty"`
	EveryEvents       uint64 `json:"every_events"`
	Every             string `json:"every"`
	LastBoundAt       string `json:"last_bound_at,omitempty"`
	LastBoundSequence uint64 `json:"last_bound_sequence,omitempty"`
	// LastBindMillis is how long the last bind held appends.
	LastBindMillis int64  `json:"last_bind_millis,omitempty"`
	NotBefore      string `json:"not_before,omitempty"`
}

type checkpointCadence struct {
	tickMu sync.Mutex // serializes ticks
	mu     sync.Mutex // guards the fields below

	// Seams. Nil means the real platform listing and the real bind.
	cfg          *checkpointCadenceConfig
	initialDelay time.Duration
	tickEvery    time.Duration
	last         func(ctx context.Context) (checkpointMark, bool, error)
	bind         func(ctx context.Context) (uint64, error)
	// measure replaces the clock a bind is timed with (tests).
	measure func() time.Duration

	// mark is the newest checkpoint as last read from the platform, moved to
	// the bound head when this runtime binds.
	mark      checkpointMark
	markKnown bool
	firstTick time.Time

	notBefore time.Time
	failures  int
	off       string

	lastBindDuration  time.Duration
	lastBoundAt       time.Time
	lastBoundSequence uint64

	stop chan struct{}
}

func (c *checkpointCadence) config() checkpointCadenceConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil {
		cfg := checkpointCadenceConfigFromEnv(os.Getenv)
		c.cfg = &cfg
	}
	return *c.cfg
}

func (rt *Runtime) cadenceKey() string { return rt.cfg.ComputerID }

// checkpointCadenceTick is one evaluation: bind if a checkpoint is due. It is
// the only place the cadence binds, and it is new work every time.
func (rt *Runtime) checkpointCadenceTick(ctx context.Context, now time.Time) (cadenceOutcome, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return cadenceSkipped, nil
	}
	c := &rt.cadence
	c.tickMu.Lock()
	defer c.tickMu.Unlock()

	cfg := c.config()
	if cfg.disabled() {
		return cadenceSkipped, nil
	}
	// Boot settles first, and a held or applying computer binds its own
	// checkpoints: the cadence never competes with either.
	if rt.bootInProgress.Load() || rt.maintenanceHeld() || rt.SelfDevelopmentApplyMaterializing(ctx) {
		return cadenceSkipped, nil
	}
	c.mu.Lock()
	lastFn, bindFn := c.last, c.bind
	off, notBefore := c.off, c.notBefore
	if c.firstTick.IsZero() {
		c.firstTick = now
	}
	c.mu.Unlock()
	if lastFn == nil || bindFn == nil {
		if rt.ownerRecoveryControl == nil {
			return cadenceSkipped, nil // no platform control: nothing to publish to
		}
		if lastFn == nil {
			lastFn = rt.cadenceLastCheckpoint
		}
		if bindFn == nil {
			bindFn = rt.cadenceBindCheckpoint
		}
	}
	if off != "" {
		return cadenceOff, nil
	}
	if now.Before(notBefore) {
		return cadenceBackoff, nil
	}

	head, err := rt.store.Head(ctx, rt.cfg.ComputerID)
	if err != nil {
		return cadenceSkipped, fmt.Errorf("checkpoint cadence: read head: %w", err)
	}
	if head == nil {
		return cadenceSkipped, nil
	}
	// The platform is asked only when a checkpoint could be due: the newest
	// checkpoint is cached, and a head at or below it is never due.
	refresh := func() (checkpointMark, error) {
		mark, found, err := lastFn(ctx)
		if err != nil {
			return checkpointMark{}, fmt.Errorf("checkpoint cadence: read newest checkpoint: %w", err)
		}
		if !found {
			c.mu.Lock()
			mark = checkpointMark{At: c.firstTick}
			if !rt.bootedAt.IsZero() {
				mark.At = rt.bootedAt
			}
			c.mu.Unlock()
		}
		c.mu.Lock()
		c.mark, c.markKnown = mark, true
		c.mu.Unlock()
		return mark, nil
	}
	c.mu.Lock()
	mark, known := c.mark, c.markKnown
	c.mu.Unlock()
	fresh := false
	if !known {
		if mark, err = refresh(); err != nil {
			return cadenceSkipped, err
		}
		fresh = true
	}
	if due, _ := cfg.due(head.Sequence, mark, now); !due {
		return cadenceIdle, nil
	}
	if !fresh {
		if mark, err = refresh(); err != nil {
			return cadenceSkipped, err
		}
		if due, _ := cfg.due(head.Sequence, mark, now); !due {
			return cadenceIdle, nil
		}
	}

	// A start is counted before the bind and forgotten after it returns: only
	// a bind that never returned (the computer crashed in it) stays counted.
	storePath := rt.cfg.StorePath
	if err := spendSettlementAttempt(storePath, checkpointCadenceAttemptKey, rt.cadenceKey(), checkpointCadenceAttemptLimit); err != nil {
		switch {
		case errors.Is(err, ErrGuestApplyAttemptsSpent):
			cause := fmt.Sprintf("a checkpoint bind did not finish on %d starts (the computer crashed in it); an owner act reopens it: choir computer resume-checkpoints", checkpointCadenceAttemptLimit)
			rt.closeCheckpointCadence(cause)
			return cadenceOff, nil
		case errors.Is(err, errSettlementUnbounded):
			rt.closeCheckpointCadence("no durable attempt count, so a crashing bind could not be bounded")
			return cadenceOff, nil
		default:
			return cadenceSkipped, fmt.Errorf("checkpoint cadence: %w", err)
		}
	}
	started := time.Now()
	sequence, bindErr := bindFn(ctx)
	took := time.Since(started)
	if c.measure != nil {
		took = c.measure()
	}
	if err := clearGuestAttempt(storePath, checkpointCadenceAttemptKey, rt.cadenceKey()); err != nil {
		log.Printf("runtime: checkpoint cadence: forget the finished start: %v", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if bindErr != nil {
		c.failures++
		wait := cadenceBackoffBase << (c.failures - 1)
		if wait > cadenceBackoffMax || wait <= 0 {
			wait = cadenceBackoffMax
		}
		c.notBefore = now.Add(wait)
		log.Printf("runtime: checkpoint cadence: bind failed (%d in a row), next try after %s: %v", c.failures, wait, bindErr)
		return cadenceFailed, bindErr
	}
	c.failures = 0
	spacing := cadenceMinSpacing
	if hold := took * cadenceDutyFactor; hold > spacing {
		spacing = hold
	}
	c.notBefore = now.Add(spacing)
	c.mark, c.markKnown = checkpointMark{Sequence: sequence, At: now}, true
	c.lastBindDuration, c.lastBoundAt, c.lastBoundSequence = took, now, sequence
	log.Printf("runtime: checkpoint cadence: bound at sequence %d (held appends %s)", sequence, took.Round(time.Millisecond))
	return cadenceBound, nil
}

func (rt *Runtime) closeCheckpointCadence(cause string) {
	c := &rt.cadence
	c.mu.Lock()
	c.off = cause
	c.mu.Unlock()
	log.Printf("runtime: checkpoint cadence off: %s", cause)
}

// cadenceLastCheckpoint reads the newest checkpoint the platform holds for
// this computer, the one authority for "since the last checkpoint". A
// checkpoint the platform cannot place on the tape reads as sequence zero,
// which makes the next checkpoint due: it is replaced by one it can place.
func (rt *Runtime) cadenceLastCheckpoint(ctx context.Context) (checkpointMark, bool, error) {
	list, err := rt.ownerRecoveryControl.ListCheckpoints(ctx, rt.cfg.ComputerID, 1)
	if err != nil {
		return checkpointMark{}, false, err
	}
	if len(list.Checkpoints) == 0 {
		return checkpointMark{}, false, nil
	}
	newest := list.Checkpoints[0]
	at, _ := time.Parse(time.RFC3339Nano, newest.CreatedAt)
	return checkpointMark{Sequence: newest.AcceptedSequence, At: at}, true, nil
}

// cadenceBindCheckpoint is the bind the apply uses, run for the owner's
// recovery evidence class, and the sequence it bound.
func (rt *Runtime) cadenceBindCheckpoint(ctx context.Context) (uint64, error) {
	report, err := rt.BindCheckpointRestoreSet(ctx, rt.cfg.ComputerID)
	if err != nil {
		return 0, err
	}
	if report.PublishedCheckpoint == nil {
		return 0, errors.New("the bind published nothing")
	}
	return report.BoundSequence, nil
}

// startCheckpointCadence starts the loop. It is called after Start's boot
// phases and never binds before the initial delay: a restart resumes no work.
func (rt *Runtime) startCheckpointCadence(ctx context.Context) {
	if rt == nil {
		return
	}
	c := &rt.cadence
	c.mu.Lock()
	if c.stop != nil {
		c.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	c.stop = stop
	delay, every := c.initialDelay, c.tickEvery
	if delay == 0 && c.tickEvery == 0 {
		delay = cadenceDefaultInitialDelay
	}
	if every <= 0 {
		every = cadenceDefaultTickEvery
	}
	c.mu.Unlock()
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			if _, err := rt.checkpointCadenceTick(ctx, time.Now().UTC()); err != nil {
				log.Printf("runtime: checkpoint cadence: %v", err)
			}
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (rt *Runtime) stopCheckpointCadence() {
	if rt == nil {
		return
	}
	c := &rt.cadence
	c.mu.Lock()
	stop := c.stop
	c.stop = nil
	c.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// resumeCheckpointCadence is the owner act that reopens a closed cadence: it
// forgets the unfinished starts and the backoff. Nothing reopens it by itself.
func (rt *Runtime) resumeCheckpointCadence() error {
	if rt == nil {
		return errors.New("runtime unavailable")
	}
	if err := clearGuestAttempt(rt.cfg.StorePath, checkpointCadenceAttemptKey, rt.cadenceKey()); err != nil {
		return err
	}
	c := &rt.cadence
	c.mu.Lock()
	c.off, c.failures, c.notBefore = "", 0, time.Time{}
	c.mu.Unlock()
	log.Printf("runtime: checkpoint cadence reopened by the owner")
	return nil
}

// CheckpointCadenceStatus is the cadence as the listing shows it. A cadence
// closed by two unfinished starts reads off even before its next tick.
func (rt *Runtime) CheckpointCadenceStatus() CheckpointCadenceStatus {
	cfg := rt.cadence.config()
	c := &rt.cadence
	c.mu.Lock()
	defer c.mu.Unlock()
	status := CheckpointCadenceStatus{State: "on", EveryEvents: cfg.EveryEvents, Every: cfg.Every.String(), LastBoundSequence: c.lastBoundSequence, LastBindMillis: c.lastBindDuration.Milliseconds()}
	if !c.lastBoundAt.IsZero() {
		status.LastBoundAt = c.lastBoundAt.UTC().Format(time.RFC3339Nano)
	}
	if !c.notBefore.IsZero() {
		status.NotBefore = c.notBefore.UTC().Format(time.RFC3339Nano)
	}
	switch {
	case cfg.disabled():
		status.State = "disabled"
	case c.off != "":
		status.State, status.Cause = "off", c.off
	default:
		if n, err := peekGuestAttempts(rt.cfg.StorePath, checkpointCadenceAttemptKey, rt.cadenceKey()); err == nil && n >= checkpointCadenceAttemptLimit {
			status.State = "off"
			status.Cause = fmt.Sprintf("a checkpoint bind did not finish on %d starts (the computer crashed in it); an owner act reopens it: choir computer resume-checkpoints", n)
		}
	}
	return status
}
