package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The Autopaper source schedule (W1c,
// docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// each scheduled source is a chain of durable source_poll_due wakes. A wake
// polls the source with no model call, records the poll's usage on the
// schedule, and arms the next wake.
//
// A schedule is a standing owner intent; a poll is an occurrence (owner
// ruling 6, 2026-10-14: "we need paper publishing 24/7, even after
// crashes"; restart design 2026-10-13 §3.4 option B). A planned boot leaves
// the pending wake to fire. A crash boot closes only the poll in flight and
// arms the schedule's next tick one interval after boot, as new work under a
// new generation; a schedule whose poll was in flight at
// sourceScheduleCrashLoopLimit crash boots in a row is closed with that
// cause, so a poll that crashes the computer cannot loop.
const (
	sourcePollDueKind            = "source_poll_due"
	defaultSourcePollInterval    = 15 * time.Minute
	minSourcePollInterval        = time.Minute
	maxSourcePollBackoff         = 24 * time.Hour
	sourceScheduleInterruptedWhy = "interrupted by a restart"
	sourceScheduleCrashLoopLimit = 2
	sourceScheduleCrashLoopWhy   = "closed: its poll was in flight at 2 crash restarts in a row; an owner act restarts it"
)

// sourceScheduleMu serializes schedule read-modify-write in this process.
var sourceScheduleMu sync.Mutex

type sourcePollDue struct {
	SourceID   string `json:"source_id"`
	Generation int64  `json:"generation"`
	Due        int64  `json:"due_unix"`
}

// sourcePollMailbox gives each source its own mailbox, so one source's polls
// run one at a time and a slow feed never holds a desk's mailbox.
func sourcePollMailbox(sourceID string) string { return "source:" + sourceID }

func (rt *Runtime) armSourcePoll(ctx context.Context, sched store.SourceSchedule) error {
	if rt.scheduleActor == nil {
		return fmt.Errorf("source schedule: durable wakes are unavailable")
	}
	content, err := json.Marshal(sourcePollDue{SourceID: sched.SourceID, Generation: sched.Generation, Due: sched.NextDue.Unix()})
	if err != nil {
		return err
	}
	return rt.scheduleActor(ctx, sched.OwnerID, sched.ComputerID, sourcePollMailbox(sched.SourceID),
		sourcePollDueKind, string(content), "", "", sched.NextDue)
}

// StartSourceSchedule starts (or restarts) polling sourceID every interval.
// The first poll is due now. Any earlier chain for the source goes stale.
// A test_fixture source is refused; StartSourceScheduleRun marks a fixture
// run.
func (rt *Runtime) StartSourceSchedule(ctx context.Context, ownerID, computerID, sourceID string, interval time.Duration, maxItems int) (store.SourceSchedule, error) {
	return rt.StartSourceScheduleRun(ctx, ownerID, computerID, sourceID, interval, maxItems, false)
}

// StartSourceScheduleRun is StartSourceSchedule with the fixture-run mark:
// a source whose tos_class is test_fixture starts only when fixtureRun is
// set, so test data never becomes a standing schedule by accident (L2a
// R2-7).
func (rt *Runtime) StartSourceScheduleRun(ctx context.Context, ownerID, computerID, sourceID string, interval time.Duration, maxItems int, fixtureRun bool) (store.SourceSchedule, error) {
	if rt == nil || rt.store == nil {
		return store.SourceSchedule{}, fmt.Errorf("source schedule: store unavailable")
	}
	sourceID = strings.TrimSpace(sourceID)
	var err error
	var reg sources.Registry
	if rt.researchDeps != nil && rt.researchDeps.Sources != nil {
		reg = *rt.researchDeps.Sources
	} else if reg, err = sources.DefaultRegistry(); err != nil {
		return store.SourceSchedule{}, err
	}
	source, ok := reg.Lookup(sourceID)
	if !ok {
		return store.SourceSchedule{}, fmt.Errorf("unknown source %q", sourceID)
	}
	isFixture := strings.TrimSpace(source.TOSClass) == sources.TOSClassTestFixture
	if isFixture && !fixtureRun {
		return store.SourceSchedule{}, fmt.Errorf("source %q is a %s; a schedule on it starts only when the request is marked as a fixture run (fixture_run)", sourceID, sources.TOSClassTestFixture)
	}
	if interval <= 0 {
		interval = defaultSourcePollInterval
	}
	if interval < minSourcePollInterval {
		interval = minSourcePollInterval
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, err := rt.store.GetSourceSchedule(ctx, ownerID, sourceID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.SourceSchedule{}, err
	}
	now := time.Now().UTC()
	sched.OwnerID, sched.ComputerID, sched.SourceID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), sourceID
	sched.IntervalSeconds, sched.MaxItems, sched.FixtureRun = int64(interval/time.Second), maxItems, isFixture
	sched.State, sched.StateReason = store.SourceScheduleActive, ""
	sched.Generation++
	sched.NextDue, sched.ConsecutiveFailures, sched.UpdatedAt = now, 0, now
	sched.CrashesWithPollInFlight = 0
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		return store.SourceSchedule{}, err
	}
	if err := rt.armSourcePoll(ctx, sched); err != nil {
		sched.State, sched.StateReason = store.SourceScheduleStopped, "could not arm the first poll: "+err.Error()
		_ = rt.store.PutSourceSchedule(ctx, sched)
		return sched, err
	}
	return sched, nil
}

// StopSourceSchedule stops sourceID's schedule; its pending wake goes stale.
func (rt *Runtime) StopSourceSchedule(ctx context.Context, ownerID, sourceID, reason string) (store.SourceSchedule, error) {
	if rt == nil || rt.store == nil {
		return store.SourceSchedule{}, fmt.Errorf("source schedule: store unavailable")
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, err := rt.store.GetSourceSchedule(ctx, ownerID, strings.TrimSpace(sourceID))
	if err != nil {
		return store.SourceSchedule{}, err
	}
	sched.State, sched.StateReason = store.SourceScheduleStopped, strings.TrimSpace(reason)
	sched.Generation++
	sched.NextDue, sched.UpdatedAt = time.Time{}, time.Now().UTC()
	return sched, rt.store.PutSourceSchedule(ctx, sched)
}

// HandleSourcePollDue runs one scheduled poll. A wake for a schedule that is
// no longer active, or from an older generation, does nothing.
func (rt *Runtime) HandleSourcePollDue(ctx context.Context, ownerID, computerID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("source schedule: store unavailable")
	}
	var due sourcePollDue
	if err := json.Unmarshal([]byte(content), &due); err != nil {
		return fmt.Errorf("source schedule: decode wake: %w", err)
	}
	current := func() (store.SourceSchedule, bool, error) {
		sched, err := rt.store.GetSourceSchedule(ctx, ownerID, due.SourceID)
		if errors.Is(err, store.ErrNotFound) {
			return sched, false, nil
		}
		if err != nil {
			return sched, false, err
		}
		return sched, sched.State == store.SourceScheduleActive && sched.Generation == due.Generation, nil
	}
	started := time.Now().UTC()
	sourceScheduleMu.Lock()
	sched, live, err := current()
	if err == nil && live && sched.NextDue.Unix() == due.Due {
		// The poll is in flight from here until it records its result, and
		// durably so: a crash boot that finds the mark closes this poll and
		// counts the crash against the schedule (the crash-loop bound).
		sched.PollInFlightSince, sched.UpdatedAt = started, started
		err = rt.store.PutSourceSchedule(ctx, sched)
	}
	sourceScheduleMu.Unlock()
	if err != nil || !live {
		return err
	}
	if sched.NextDue.Unix() != due.Due {
		// A wake of the current generation whose slot already ran: a replay,
		// or a retry after the schedule was saved but the actor commit that
		// carried its successor failed. It does not poll again; it re-arms the
		// saved next poll, which dedupes if the successor already exists
		// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
		return rt.armSourcePoll(ctx, sched)
	}

	usage := store.SourcePollUsage{At: started}
	pendingAfter := sched.PendingArrivals
	if rt.researchDeps == nil {
		usage.Status, usage.Error = "error", "research dependencies are not configured"
	} else {
		report, pollErr := researchtools.PollSourceOnce(ctx, *rt.researchDeps, ownerID, computerID, sched.SourceID, sched.MaxItems)
		usage = store.SourcePollUsage{
			At: started, Status: report.Status, StatusCode: report.StatusCode, Fetched: report.Fetched,
			NotModified: report.NotModified, ItemBytes: report.ItemBytes, New: report.New,
			Repeated: report.Repeated, Skipped: report.Skipped, Deferred: report.Deferred, PollMS: report.PollMS,
			TotalMS: report.TotalMS, Error: report.Error,
		}
		if pollErr != nil {
			usage.Status, usage.Error = "error", pollErr.Error()
		}
		// With intake.labels on, Jev labels the new captures before the
		// arrival names them (L2a A1). Labels are their own objects and
		// never gate the arrival. Under event wakes they retire: management
		// reads captures on event records, which name the stories that
		// hold the event (event layer §18), so a label call per capture
		// would scale calls with captures for nothing.
		eventWakes := rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEvents) &&
			rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEventWakes)
		if len(report.Captured) > 0 && !eventWakes && rt.intakeLabelsEnabled(ctx, ownerID, computerID) {
			usage.Labelled, usage.Unlabelled = rt.labelCaptures(ctx, ownerID, computerID, sched.SourceID, report.Captured)
		}
		// With intake.events on, the new captures queue for the event
		// layer, which resolves every source's captures together onto
		// world events off this path (event_layer.go). It never gates the
		// arrival. With intake.event_wakes on too, the arrival is held:
		// it names the captures and wakes no one, and the captures reach
		// management on their events' records (event layer §15).
		// Pending captures from an earlier poll are held too, so the
		// arrival never wakes management while production waits on events
		// (code review, Gemini B3); any never queued are released by the
		// sweep.
		rt.enqueueEventResolution(ctx, ownerID, computerID, sched.SourceID, report.Captured)
		held := eventWakes
		// The poll's new captures reach management as one record (Cut 2
		// 2b), after any the source still has pending, at most the poll's
		// limit. Captures a failed append could not announce stay pending
		// on the schedule for the next poll (L2a A5, defect 2).
		limit := report.Limit
		if limit <= 0 {
			limit = researchtools.EffectivePollMaxItems(sched.MaxItems)
		}
		var arrivalErr error
		pendingAfter, arrivalErr = rt.announceCaptures(ctx, ownerID, computerID, sched.SourceID, sched.PendingArrivals, report.Captured, limit, held)
		if arrivalErr != nil {
			log.Printf("source schedule: %s: %v; %d captures pending for the next poll", sched.SourceID, arrivalErr, len(pendingAfter))
		}
		// Held captures a crash, a stop or the setting's rollback stranded
		// go to management (event_wakes.go), from this live poll.
		rt.sweepHeldCaptures(ctx, ownerID, computerID, time.Now().UTC())
	}

	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, live, err = current()
	if err != nil {
		return err
	}
	live = live && sched.NextDue.Unix() == due.Due
	sched.RecordPoll(usage)
	// The poll finished without crashing the computer.
	sched.PollInFlightSince, sched.CrashesWithPollInFlight = time.Time{}, 0
	// The pending list rides the schedule write below; if that write fails,
	// the error is logged and returned as before, and the list is lost with
	// it only when the store itself is down.
	var dropped int
	sched.PendingArrivals, dropped = boundPendingArrivals(pendingAfter)
	if dropped > 0 {
		sched.ArrivalsClosed += dropped
		sched.ArrivalsClosedReason = "pending arrivals over the bound"
		log.Printf("source schedule: %s: %d oldest pending captures closed unannounced (pending bound %d)", sched.SourceID, dropped, sourceArrivalPendingLimit)
	}
	now := time.Now().UTC()
	if usage.Error != "" {
		sched.ConsecutiveFailures++
		log.Printf("source schedule: %s poll failed (%d in a row): %s", sched.SourceID, sched.ConsecutiveFailures, usage.Error)
	} else {
		sched.ConsecutiveFailures = 0
	}
	interval := sourcePollInterval(sched)
	sched.UpdatedAt = now
	put := func() error {
		err := rt.store.PutSourceSchedule(ctx, sched)
		if err != nil && len(sched.PendingArrivals) > 0 {
			log.Printf("source schedule: %s: %d pending captures not saved: %v", sched.SourceID, len(sched.PendingArrivals), err)
		}
		return err
	}
	if !live {
		// Stopped or restarted while this poll ran: keep the usage and the
		// pending list, arm nothing.
		return put()
	}
	sched.NextDue = now.Add(interval)
	if err := put(); err != nil {
		return err
	}
	return rt.armSourcePoll(ctx, sched)
}

// sourcePollInterval is the wait before a schedule's next poll: its
// interval, doubled for each failure in a row, at most maxSourcePollBackoff.
func sourcePollInterval(sched store.SourceSchedule) time.Duration {
	interval := time.Duration(sched.IntervalSeconds) * time.Second
	if interval < minSourcePollInterval {
		interval = minSourcePollInterval
	}
	for i := 0; i < sched.ConsecutiveFailures && interval < maxSourcePollBackoff; i++ {
		interval *= 2
	}
	if interval > maxSourcePollBackoff {
		interval = maxSourcePollBackoff
	}
	return interval
}

// recoverSourceSchedulesAtBoot carries every schedule across this boot
// (owner rule, CLAUDE.md "Restarts End Work (Crash) Or Resume It"; ruling 6,
// 2026-10-14: standing intents survive a crash, their occurrences do not).
//
// A planned boot may resume: an active schedule keeps its generation and its
// saved tick, which is armed again (it collapses into the pending wake, so an
// existing one is not doubled); its pending arrivals are named by that poll.
// A schedule more than sourceScheduleOverdueGrace overdue has lost its wake
// (re-arming the same tick would collapse into the lost one, see
// problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md), so it is
// re-armed as a new tick under a new generation, staggered per schedule.
//
// A crash boot never resumes an occurrence. For an active schedule:
//   - a poll in flight is closed (recorded with the status
//     interrupted_by_restart) and the crash counts against the schedule;
//     with no poll in flight the count resets, since it counts crashes in a
//     row;
//   - at sourceScheduleCrashLoopLimit the schedule is closed as
//     interrupted_by_restart with that cause: its tick crashes the computer;
//   - otherwise it stays active under a new generation (the pre-boot wake,
//     which the actor handler refuses, is stale too) and its next tick is
//     armed one interval (with any failure backoff) after boot, never at
//     once: new work on a computer that has stayed up that long.
//
// A crash boot also closes every schedule's pending arrivals (L2a A5,
// defect 2) with the fate "interrupted by a restart": they are counted in
// ArrivalsClosed and their ids logged, never announced after the crash.
func (rt *Runtime) recoverSourceSchedulesAtBoot(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	_, planned := rt.BootWasPlannedRestart()
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	schedules, err := rt.store.ListSourceSchedules(ctx, "")
	if err != nil {
		log.Printf("source schedule: boot: list schedules: %v", err)
		return
	}
	now := time.Now().UTC()
	repaired := 0
	for _, sched := range schedules {
		if planned {
			rt.recoverSourceScheduleAfterPlannedBoot(ctx, sched, now, &repaired)
		} else {
			rt.recoverSourceScheduleAfterCrash(ctx, sched)
		}
	}
}

func (rt *Runtime) recoverSourceScheduleAfterPlannedBoot(ctx context.Context, sched store.SourceSchedule, now time.Time, repaired *int) {
	if !sched.PollInFlightSince.IsZero() {
		// The planned restart cut a poll; its wake is redelivered (a planned
		// boot may resume) and marks it again. Not a crash: nothing counts.
		sched.PollInFlightSince, sched.UpdatedAt = time.Time{}, now
		if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
			log.Printf("source schedule: planned boot: %s: %v", sched.SourceID, err)
			return
		}
	}
	if sched.State != store.SourceScheduleActive || sched.NextDue.IsZero() {
		return
	}
	if sourceScheduleOverdue(sched, now) {
		// The saved tick is far in the past with nothing running: its wake was
		// lost (poisoned or dropped in an outage), and re-arming the same tick
		// would collapse into that lost wake. Make it a new tick instead.
		if err := rt.rearmOverdueSourceSchedule(ctx, sched, now, *repaired, "planned boot"); err != nil {
			log.Printf("source schedule: planned boot: %s: re-arm the overdue schedule as a new tick: %v (the schedule list marks it overdue)", sched.SourceID, err)
			return
		}
		*repaired++
		return
	}
	if err := rt.armSourcePoll(ctx, sched); err != nil {
		log.Printf("source schedule: planned boot: %s: re-arm the saved tick: %v (the schedule list marks it overdue)", sched.SourceID, err)
	}
}

// rearmOverdueSourceSchedule restarts an overdue active schedule's chain as a
// new tick: the generation goes up, so a stale, lost, or poisoned wake of the
// old chain is refused by HandleSourcePollDue, and the first poll of the new
// chain is due now plus index*sourceScheduleRearmStagger so many repaired
// sources do not poll at once. The caller holds sourceScheduleMu and has
// judged the schedule overdue with no live poll. The crash bound is left
// alone: a repaired tick that crashes the computer is counted by the crash
// boot like any other.
func (rt *Runtime) rearmOverdueSourceSchedule(ctx context.Context, sched store.SourceSchedule, now time.Time, index int, cause string) error {
	was := sched.NextDue
	sched.Generation++
	sched.NextDue = now.Add(time.Duration(index) * sourceScheduleRearmStagger)
	sched.PollInFlightSince, sched.UpdatedAt = time.Time{}, now
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		return err
	}
	log.Printf("source schedule: %s: %s: overdue since %s with no wake; re-armed as generation %d, next tick %s",
		sched.SourceID, cause, was.Format(time.RFC3339), sched.Generation, sched.NextDue.Format(time.RFC3339))
	return rt.armSourcePoll(ctx, sched)
}

func (rt *Runtime) recoverSourceScheduleAfterCrash(ctx context.Context, sched store.SourceSchedule) {
	active, pending, inFlight := sched.State == store.SourceScheduleActive, sched.PendingArrivals, sched.PollInFlightSince
	if !active && len(pending) == 0 && inFlight.IsZero() {
		return
	}
	now := time.Now().UTC()
	if !inFlight.IsZero() {
		sched.RecordPoll(store.SourcePollUsage{At: inFlight, Status: store.SourcePollInterruptedByRestart, Error: sourceScheduleInterruptedWhy})
		sched.PollInFlightSince = time.Time{}
	}
	closed := false
	if active {
		if inFlight.IsZero() {
			sched.CrashesWithPollInFlight = 0
		} else {
			sched.CrashesWithPollInFlight++
		}
		sched.Generation++
		if sched.CrashesWithPollInFlight >= sourceScheduleCrashLoopLimit {
			sched.State, sched.StateReason = store.SourceScheduleInterruptedByRestart, sourceScheduleCrashLoopWhy
			sched.NextDue, closed = time.Time{}, true
		} else {
			sched.NextDue = now.Add(sourcePollInterval(sched))
		}
	}
	if len(pending) > 0 {
		sched.PendingArrivals = nil
		sched.ArrivalsClosed += len(pending)
		sched.ArrivalsClosedReason = sourceScheduleInterruptedWhy
	}
	sched.UpdatedAt = now
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		log.Printf("source schedule: crash boot: %s: %v", sched.SourceID, err)
		return
	}
	if !inFlight.IsZero() {
		log.Printf("source schedule: crash boot: %s: poll in flight since %s closed: %s (%d crash boots in a row with a poll in flight)",
			sched.SourceID, inFlight.Format(time.RFC3339), sourceScheduleInterruptedWhy, sched.CrashesWithPollInFlight)
	}
	if len(pending) > 0 {
		log.Printf("source schedule: crash boot: %s: %d pending captures closed unannounced: %s: %v",
			sched.SourceID, len(pending), sourceScheduleInterruptedWhy, arrivalIDs(pending))
	}
	switch {
	case closed:
		log.Printf("source schedule: crash boot: %s: paper source stopped: %s", sched.SourceID, sourceScheduleCrashLoopWhy)
	case active:
		if err := rt.armSourcePoll(ctx, sched); err != nil {
			log.Printf("source schedule: crash boot: %s: arm the next tick: %v (the schedule list marks it overdue)", sched.SourceID, err)
			return
		}
		log.Printf("source schedule: crash boot: %s stays active; next tick %s (generation %d)", sched.SourceID, sched.NextDue.Format(time.RFC3339), sched.Generation)
	}
}

func arrivalIDs(items []types.CaptureArrival) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.CaptureID
	}
	return ids
}

// sourceScheduleOverdueGrace is how long past its next poll an active
// schedule may go before the list calls it overdue: a wake whose re-arm kept
// failing leaves a schedule active with nothing queued, and only this flag
// shows it (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
const sourceScheduleOverdueGrace = 15 * time.Minute

// sourceScheduleRearmStagger spaces the first polls of schedules repaired in
// one pass, so a paper with dozens of sources does not poll them all at once.
const sourceScheduleRearmStagger = 15 * time.Second

const (
	// sourceScheduleSweepInterval is how often the runtime looks for an
	// overdue schedule with no live poll. A pass is one list read; it polls
	// nothing and makes no model call.
	sourceScheduleSweepInterval = 5 * time.Minute
	// sourceScheduleSweepMaxWait bounds the backoff while passes keep failing
	// (the store is down): 5m, 10m, 20m, 40m, then hourly.
	sourceScheduleSweepMaxWait = time.Hour
	// sourcePollInFlightAbandoned is how old an in-flight mark must be before
	// the sweep treats it as left behind (a panicked or hung handler) rather
	// than a poll still running. A poll has bounded HTTP timeouts.
	sourcePollInFlightAbandoned = time.Hour
)

// sourceScheduleNeedsRepair reports whether an active schedule is past the
// overdue grace with no live poll: its wake was lost.
func sourceScheduleNeedsRepair(sched store.SourceSchedule, now time.Time) bool {
	if !sourceScheduleOverdue(sched, now) {
		return false
	}
	return sched.PollInFlightSince.IsZero() || now.Sub(sched.PollInFlightSince) > sourcePollInFlightAbandoned
}

// sweepOverdueSourceSchedules re-arms every schedule whose wake was lost, as
// rearmOverdueSourceSchedule does at a planned boot. It runs under
// sourceScheduleMu, so it cannot interleave with a poll's in-flight mark or
// result: a poll that started first is skipped (marked in flight), and a wake
// that starts after is refused by the new generation. The first failure ends
// the pass, so an outage costs one failed write, not one per schedule.
func (rt *Runtime) sweepOverdueSourceSchedules(ctx context.Context, now time.Time) (int, error) {
	if rt == nil || rt.store == nil || rt.scheduleActor == nil {
		return 0, nil
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	schedules, err := rt.store.ListSourceSchedules(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("list schedules: %w", err)
	}
	repaired := 0
	for _, sched := range schedules {
		if !sourceScheduleNeedsRepair(sched, now) {
			continue
		}
		if err := rt.rearmOverdueSourceSchedule(ctx, sched, now, repaired, "sweep"); err != nil {
			return repaired, fmt.Errorf("re-arm %s: %w", sched.SourceID, err)
		}
		repaired++
	}
	return repaired, nil
}

// sourceScheduleSweepLoop runs sweep every base interval, doubling the wait
// after each failed pass up to sourceScheduleSweepMaxWait and resetting it on
// a clean pass. after is time.After in production.
func sourceScheduleSweepLoop(ctx context.Context, stop <-chan struct{}, base time.Duration, sweep func() (int, error), after func(time.Duration) <-chan time.Time) {
	failures := 0
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-after(sourceScheduleSweepWait(base, failures)):
		}
		_, err := func() (n int, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("sweep panicked: %v", r)
				}
			}()
			return sweep()
		}()
		if err != nil {
			failures++
			log.Printf("source schedule: sweep failed (%d in a row): %v; next pass in %s", failures, err, sourceScheduleSweepWait(base, failures))
			continue
		}
		if failures > 0 {
			log.Printf("source schedule: sweep recovered after %d failed passes", failures)
		}
		failures = 0
	}
}

// sourceScheduleSweepWait is the wait before the next pass: base, doubled for
// each failed pass in a row, at most sourceScheduleSweepMaxWait.
func sourceScheduleSweepWait(base time.Duration, failures int) time.Duration {
	wait := base
	for i := 0; i < failures && wait < sourceScheduleSweepMaxWait; i++ {
		wait *= 2
	}
	if wait > sourceScheduleSweepMaxWait {
		wait = sourceScheduleSweepMaxWait
	}
	return wait
}

// startSourceScheduleSweep starts the periodic lost-wake sweep. It does
// nothing without durable wakes. Stop ends it.
func (rt *Runtime) startSourceScheduleSweep(ctx context.Context) {
	if rt == nil || rt.store == nil || rt.scheduleActor == nil || rt.sourceSweepStop != nil {
		return
	}
	stop := make(chan struct{})
	rt.sourceSweepStop = stop
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		sourceScheduleSweepLoop(ctx, stop, sourceScheduleSweepInterval, func() (int, error) {
			return rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC())
		}, time.After)
	}()
}

func (rt *Runtime) stopSourceScheduleSweep() {
	if rt.sourceSweepStop != nil {
		close(rt.sourceSweepStop)
		rt.sourceSweepStop = nil
	}
}

type sourceScheduleView struct {
	store.SourceSchedule
	Overdue bool `json:"overdue"`
}

func sourceScheduleOverdue(sched store.SourceSchedule, now time.Time) bool {
	return sched.State == store.SourceScheduleActive && !sched.NextDue.IsZero() &&
		now.After(sched.NextDue.Add(sourceScheduleOverdueGrace))
}

type sourceScheduleRequest struct {
	SourceID        string `json:"source_id"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	MaxItems        int    `json:"max_items,omitempty"`
	Reason          string `json:"reason,omitempty"`
	// FixtureRun marks a schedule on a test_fixture source (L2a R2-7).
	FixtureRun bool `json:"fixture_run,omitempty"`
}

// HandleSourceSchedules serves /api/autopaper/sources/schedules: GET lists
// the owner's schedules, POST starts one, DELETE (?source_id=) stops one.
func (h *APIHandler) HandleSourceSchedules(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	switch r.Method {
	case http.MethodGet:
		schedules, err := rt.store.ListSourceSchedules(r.Context(), ownerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		now := time.Now().UTC()
		views := make([]sourceScheduleView, 0, len(schedules))
		for _, sched := range schedules {
			views = append(views, sourceScheduleView{SourceSchedule: sched, Overdue: sourceScheduleOverdue(sched, now)})
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedules": views})
	case http.MethodPost:
		var req sourceScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON body"})
			return
		}
		sched, err := rt.StartSourceScheduleRun(r.Context(), ownerID, rt.TextureComputerID(), req.SourceID,
			time.Duration(req.IntervalMinutes)*time.Minute, req.MaxItems, req.FixtureRun)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched})
	case http.MethodDelete:
		sched, err := rt.StopSourceSchedule(r.Context(), ownerID, r.URL.Query().Get("source_id"), r.URL.Query().Get("reason"))
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no schedule for that source"})
			return
		}
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
