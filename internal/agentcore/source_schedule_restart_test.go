package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Standing source schedules across restarts (owner ruling 6, 2026-10-14:
// "we need paper publishing 24/7, even after crashes"; restart design
// 2026-10-13 §3.4 option B; station L1 "schedules survive crash";
// problems/planned-refresh-closes-paper-schedules-2026-10-14.md).
// Failure modes pinned, written before the code:
//  1. A crash boot closes the standing schedule (the paper goes silent
//     until an owner act): the 05:48 epoch-cutover refresh and the 23:51
//     hibernate wake both did this.
//  2. A crash boot re-arms the next tick immediately, so a poll that
//     crashes the computer runs again at once (a crash loop).
//  3. A crash boot leaves the pre-boot wake chain live, so the occurrence
//     the crash interrupted runs after the crash (a resume).
//  4. The poll in flight at the crash leaves no visible fate.
//  5. A poll that crashes the computer every time keeps the schedule alive
//     forever (no crash-loop bound).
//  6. Unrelated crashes separated by good polls add up and close a healthy
//     schedule (the bound is not "in a row").
//  7. A crash boot ignores the failure backoff and polls a 429ing feed at
//     the plain interval.
//  8. A planned boot closes a schedule, counts toward the crash bound, or
//     leaves an active schedule with no pending tick.
//  9. A poll in flight is not durable, so a crash cannot be attributed to
//     it.

// inFlightProbeSink reads the schedule while the poll is writing captures,
// i.e. while the poll is in flight.
type inFlightProbeSink struct {
	rt       *Runtime
	ownerID  string
	sourceID string
	seen     store.SourceSchedule
}

func (s *inFlightProbeSink) WriteCaptures(ctx context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.seen, _ = s.rt.store.GetSourceSchedule(ctx, s.ownerID, s.sourceID)
	return sourcegraph.CaptureWriteResult{New: len(items)}, nil
}

func (s *inFlightProbeSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *inFlightProbeSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

func getSchedule(t *testing.T, rt *Runtime, ownerID, sourceID string) store.SourceSchedule {
	t.Helper()
	sched, err := rt.store.GetSourceSchedule(context.Background(), ownerID, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	return sched
}

// markPollInFlight leaves the schedule as a poll that started and never
// finished leaves it: the process died between the in-flight mark and the
// poll's result.
func markPollInFlight(t *testing.T, rt *Runtime, ownerID, sourceID string, since time.Time) {
	t.Helper()
	sched := getSchedule(t, rt, ownerID, sourceID)
	sched.PollInFlightSince = since
	if err := rt.store.PutSourceSchedule(context.Background(), sched); err != nil {
		t.Fatal(err)
	}
}

func crashBoot(rt *Runtime) { WithBootRestart(PlannedRestart{}, false)(rt) }
func plannedBoot(rt *Runtime) {
	WithBootRestart(PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "next"}, true)(rt)
}

func TestSourcePollIsDurablyInFlightWhileItRuns(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:inflight", nil)
	probe := &inFlightProbeSink{rt: rt, ownerID: "owner-1", sourceID: "rss:inflight"}
	rt.researchDeps.Captures = probe
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:inflight", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[0].content); err != nil {
		t.Fatal(err)
	}
	if probe.seen.PollInFlightSince.Before(before.Add(-time.Second)) {
		t.Fatalf("mid-poll schedule = %+v; want a durable in-flight mark (failure 9)", probe.seen)
	}
	if got := getSchedule(t, rt, "owner-1", "rss:inflight"); !got.PollInFlightSince.IsZero() {
		t.Fatalf("finished poll left the in-flight mark: %+v", got)
	}
}

func TestPlannedBootKeepsSchedulesActiveAndRearmsTheSavedTick(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, hits := sourceScheduleFixture(t, "rss:planned", nil)
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:planned", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	saved := (*wakes)[0]
	markPollInFlight(t, rt, "owner-1", "rss:planned", time.Now().UTC().Add(-30*time.Second))

	plannedBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:planned")
	if got.State != store.SourceScheduleActive || got.Generation != started.Generation || !got.NextDue.Equal(started.NextDue) {
		t.Fatalf("after a planned boot = %+v; want the same active tick (failure 8)", got)
	}
	if got.CrashesWithPollInFlight != 0 || !got.PollInFlightSince.IsZero() || got.Totals.Polls != 0 {
		t.Fatalf("a planned boot counted as a crash or closed the occurrence: %+v", got)
	}
	// The saved tick is armed again; the real adapter collapses it into the
	// pending wake (same content, same update id), so a lost wake is repaired
	// and an existing one is not doubled.
	if len(*wakes) != 2 || (*wakes)[1].content != saved.content || !(*wakes)[1].notBefore.Equal(saved.notBefore) {
		t.Fatalf("planned boot armed %+v; want the saved tick again", *wakes)
	}
	// The pre-boot wake still polls after a planned boot (a planned restart
	// may resume).
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", saved.content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("the saved tick did not poll after a planned boot (%d hits)", hits.Load())
	}
}

func TestCrashBootKeepsScheduleClosesThePollInFlightAndArmsTheNextTickAfterTheInterval(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, hits := sourceScheduleFixture(t, "rss:crash", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:crash", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	preBoot := (*wakes)[0]
	markPollInFlight(t, rt, "owner-1", "rss:crash", time.Now().UTC().Add(-30*time.Second))
	before := getSchedule(t, rt, "owner-1", "rss:crash")

	crashBoot(rt)
	boot := time.Now().UTC()
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:crash")
	if got.State != store.SourceScheduleActive || got.StateReason != "" {
		t.Fatalf("after a crash boot = %+v; want the standing schedule active (failure 1)", got)
	}
	if got.Generation != before.Generation+1 {
		t.Fatalf("generation %d -> %d; want a new chain so the pre-boot wake is stale (failure 3)", before.Generation, got.Generation)
	}
	if wait := got.NextDue.Sub(boot); wait < 10*time.Minute-time.Second || wait > 11*time.Minute {
		t.Fatalf("next tick in %s after the crash boot; want the 10 minute interval, not now (failure 2)", wait)
	}
	if len(*wakes) != 2 || !(*wakes)[1].notBefore.Equal(got.NextDue) || (*wakes)[1].kind != sourcePollDueKind || (*wakes)[1].mailbox != "source:rss:crash" {
		t.Fatalf("crash boot armed %+v; want one wake at the new next tick", *wakes)
	}
	if got.CrashesWithPollInFlight != 1 || !got.PollInFlightSince.IsZero() {
		t.Fatalf("crash bound = %d, in flight %s; want 1 and cleared", got.CrashesWithPollInFlight, got.PollInFlightSince)
	}
	if got.Totals.Polls != 1 || len(got.Recent) != 1 || got.Recent[0].Status != store.SourcePollInterruptedByRestart || !strings.Contains(got.Recent[0].Error, sourceScheduleInterruptedWhy) {
		t.Fatalf("usage after the crash = %+v; want the in-flight poll closed with its fate (failure 4)", got.Recent)
	}

	// The pre-boot wake does nothing (the actor handler refuses it anyway;
	// the generation makes it stale even if it were delivered).
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", preBoot.content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 || len(*wakes) != 2 {
		t.Fatalf("the pre-boot wake polled (%d hits) or armed (%d wakes) after a crash", hits.Load(), len(*wakes))
	}
	// The new tick is new work: it polls, and a finished poll clears the
	// crash bound.
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[1].content); err != nil {
		t.Fatal(err)
	}
	got = getSchedule(t, rt, "owner-1", "rss:crash")
	if hits.Load() != 1 || got.CrashesWithPollInFlight != 0 || got.State != store.SourceScheduleActive || len(*wakes) != 3 {
		t.Fatalf("after the post-crash tick: %d hits, %+v, %d wakes", hits.Load(), got, len(*wakes))
	}
}

func TestCrashBootWithNoPollInFlightKeepsTheBackoff(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:idle-crash", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:idle-crash", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	sched := getSchedule(t, rt, "owner-1", "rss:idle-crash")
	sched.ConsecutiveFailures = 2
	sched.CrashesWithPollInFlight = 1
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		t.Fatal(err)
	}

	crashBoot(rt)
	boot := time.Now().UTC()
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:idle-crash")
	if got.State != store.SourceScheduleActive || got.Totals.Polls != 0 {
		t.Fatalf("idle crash boot = %+v; want active with no occurrence closed", got)
	}
	if wait := got.NextDue.Sub(boot); wait < 40*time.Minute-time.Second || wait > 41*time.Minute {
		t.Fatalf("next tick in %s; want the 40 minute backoff of two failures (failure 7)", wait)
	}
	if got.CrashesWithPollInFlight != 0 {
		t.Fatalf("a crash with no poll in flight kept the bound at %d; want it reset (in a row)", got.CrashesWithPollInFlight)
	}
	if len(*wakes) != 2 || !(*wakes)[1].notBefore.Equal(got.NextDue) {
		t.Fatalf("wakes %+v; want the new tick armed", *wakes)
	}
}

func TestScheduleWhosePollIsInFlightAtTwoCrashBootsInARowIsClosed(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:loop", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:loop", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	crashBoot(rt)
	markPollInFlight(t, rt, "owner-1", "rss:loop", time.Now().UTC())
	rt.recoverSourceSchedulesAtBoot(ctx)
	if got := getSchedule(t, rt, "owner-1", "rss:loop"); got.State != store.SourceScheduleActive || got.CrashesWithPollInFlight != 1 {
		t.Fatalf("after the first crash = %+v", got)
	}
	armed := len(*wakes)

	// The post-crash tick starts and the computer crashes again.
	markPollInFlight(t, rt, "owner-1", "rss:loop", time.Now().UTC())
	before := getSchedule(t, rt, "owner-1", "rss:loop")
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:loop")
	if got.State != store.SourceScheduleInterruptedByRestart || got.StateReason != sourceScheduleCrashLoopWhy {
		t.Fatalf("after the second crash in a row = %+v; want closed with the crash-loop cause (failure 5)", got)
	}
	if got.Generation != before.Generation+1 || !got.NextDue.IsZero() || len(*wakes) != armed {
		t.Fatalf("closed schedule kept a tick: %+v, %d wakes (had %d)", got, len(*wakes), armed)
	}
	if got.CrashesWithPollInFlight != 2 || got.Totals.Polls != 2 {
		t.Fatalf("closed schedule = %+v; want both interrupted polls and the count visible", got)
	}

	// An owner act restarts it with a clean bound.
	restarted, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:loop", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.State != store.SourceScheduleActive || restarted.CrashesWithPollInFlight != 0 {
		t.Fatalf("owner restart = %+v; want active with the bound reset", restarted)
	}
}

func TestCrashBoundCountsOnlyCrashesInARow(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:apart", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:apart", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	crashBoot(rt)
	markPollInFlight(t, rt, "owner-1", "rss:apart", time.Now().UTC())
	rt.recoverSourceSchedulesAtBoot(ctx)
	// A good poll between the crashes.
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[len(*wakes)-1].content); err != nil {
		t.Fatal(err)
	}
	markPollInFlight(t, rt, "owner-1", "rss:apart", time.Now().UTC())
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:apart")
	if got.State != store.SourceScheduleActive || got.CrashesWithPollInFlight != 1 {
		t.Fatalf("two crashes apart = %+v; want active with a bound of 1 (failure 6)", got)
	}
}

func TestStoppedScheduleStaysStoppedAcrossACrashBoot(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:stopped", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:stopped", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.StopSourceSchedule(ctx, "owner-1", "rss:stopped", "owner"); err != nil {
		t.Fatal(err)
	}
	armed := len(*wakes)
	crashBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	if got := getSchedule(t, rt, "owner-1", "rss:stopped"); got.State != store.SourceScheduleStopped || len(*wakes) != armed {
		t.Fatalf("a crash boot changed a stopped schedule: %+v, %d wakes", got, len(*wakes))
	}
}
