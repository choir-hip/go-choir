package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The runtime sweep repairs a lost wake without a reboot
// (problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md, "Failure
// modes considered"; the numbers below refer to that list).

// midPollSweepSink runs a sweep while the poll is in flight (failure 3).
type midPollSweepSink struct {
	inFlightProbeSink
	repaired int
	err      error
}

func (s *midPollSweepSink) WriteCaptures(ctx context.Context, owner, computer string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.repaired, s.err = s.rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC())
	return s.inFlightProbeSink.WriteCaptures(ctx, owner, computer, items)
}

func TestSweepRearmsAnOverdueScheduleWithoutAReboot(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, hits := sourceScheduleFixture(t, "rss:sweep", nil)
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:sweep", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	lost := (*wakes)[0]
	setNextDue(t, rt, "owner-1", "rss:sweep", time.Now().UTC().Add(-time.Hour))
	armed := len(*wakes)

	now := time.Now().UTC()
	n, err := rt.sweepOverdueSourceSchedules(ctx, now)
	if err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want one repair", n, err)
	}
	got := getSchedule(t, rt, "owner-1", "rss:sweep")
	if got.Generation != started.Generation+1 || got.State != store.SourceScheduleActive || got.NextDue.Before(now) || got.NextDue.After(now.Add(time.Minute)) {
		t.Fatalf("after the sweep = %+v; want a new tick now", got)
	}
	if len(*wakes) != armed+1 || !(*wakes)[armed].notBefore.Equal(got.NextDue) {
		t.Fatalf("sweep armed %+v; want one wake at the new tick", (*wakes)[armed:])
	}
	// The lost wake is refused; the new one polls once.
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", lost.content); err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[armed].content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d polls after the repair; want exactly one (failure 1)", hits.Load())
	}
}

func TestSweepLeavesHealthySchedulesAlone(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:ok-0", nil)
	ids := []string{"rss:ok-0", "rss:ok-1", "rss:ok-2", "rss:ok-3"}
	addScheduledSources(t, rt, ids[0], ids[1:]...)
	for _, id := range ids {
		if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", id, 10*time.Minute, 5); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	setNextDue(t, rt, "owner-1", "rss:ok-0", now.Add(time.Minute))                   // not due
	setNextDue(t, rt, "owner-1", "rss:ok-1", now.Add(-sourceScheduleOverdueGrace/2)) // due, within the grace
	setNextDue(t, rt, "owner-1", "rss:ok-2", now.Add(-time.Hour))                    // overdue, but its poll is running
	markPollInFlight(t, rt, "owner-1", "rss:ok-2", now.Add(-time.Minute))
	if _, err := rt.StopSourceSchedule(ctx, "owner-1", "rss:ok-3", "owner"); err != nil { // stopped by the owner
		t.Fatal(err)
	}
	before := map[string]store.SourceSchedule{}
	for _, id := range ids {
		before[id] = getSchedule(t, rt, "owner-1", id)
	}
	armed := len(*wakes)
	if n, err := rt.sweepOverdueSourceSchedules(ctx, now); err != nil || n != 0 {
		t.Fatalf("sweep = %d, %v; want nothing to repair", n, err)
	}
	for id, was := range before {
		if got := getSchedule(t, rt, "owner-1", id); got.Generation != was.Generation || !got.NextDue.Equal(was.NextDue) || got.State != was.State {
			t.Fatalf("%s changed: %+v -> %+v (failure 3)", id, was, got)
		}
	}
	if len(*wakes) != armed {
		t.Fatalf("sweep armed %d wakes for healthy schedules", len(*wakes)-armed)
	}
}

func TestSweepRepairsAnAbandonedInFlightMark(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:stuck", nil)
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:stuck", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	setNextDue(t, rt, "owner-1", "rss:stuck", now.Add(-3*time.Hour))
	markPollInFlight(t, rt, "owner-1", "rss:stuck", now.Add(-2*sourcePollInFlightAbandoned))
	armed := len(*wakes)
	if n, err := rt.sweepOverdueSourceSchedules(ctx, now); err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want the stuck schedule repaired (failure 7)", n, err)
	}
	got := getSchedule(t, rt, "owner-1", "rss:stuck")
	if got.Generation != started.Generation+1 || !got.PollInFlightSince.IsZero() || len(*wakes) != armed+1 {
		t.Fatalf("after the sweep = %+v, %d new wakes", got, len(*wakes)-armed)
	}
}

func TestSweepIsIdempotentSoGenerationsDoNotChurn(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:churn", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:churn", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	setNextDue(t, rt, "owner-1", "rss:churn", time.Now().UTC().Add(-time.Hour))
	if n, err := rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC()); err != nil || n != 1 {
		t.Fatalf("first sweep = %d, %v", n, err)
	}
	once := getSchedule(t, rt, "owner-1", "rss:churn")
	armed := len(*wakes)
	// Sweeps in the minutes after the repair, before the new tick is overdue
	// past the grace, change nothing (failure 4).
	for _, after := range []time.Duration{time.Second, 5 * time.Minute, 10 * time.Minute} {
		if n, err := rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC().Add(after)); err != nil || n != 0 {
			t.Fatalf("sweep +%s = %d, %v; want no second repair", after, n, err)
		}
	}
	if got := getSchedule(t, rt, "owner-1", "rss:churn"); got.Generation != once.Generation || len(*wakes) != armed {
		t.Fatalf("generation churn: %+v, %d extra wakes", got, len(*wakes)-armed)
	}
}

func TestSweepDuringAPollDoesNotDoublePoll(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, hits := sourceScheduleFixture(t, "rss:race", nil)
	sink := &midPollSweepSink{inFlightProbeSink: inFlightProbeSink{rt: rt, ownerID: "owner-1", sourceID: "rss:race"}}
	rt.researchDeps.Captures = sink
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:race", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	// The slot is overdue past the grace, and its wake is the one running.
	setNextDue(t, rt, "owner-1", "rss:race", time.Now().UTC().Add(-time.Hour))
	due := getSchedule(t, rt, "owner-1", "rss:race").NextDue
	content, err := json.Marshal(sourcePollDue{SourceID: "rss:race", Generation: started.Generation, Due: due.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	armed := len(*wakes)
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", string(content)); err != nil {
		t.Fatal(err)
	}
	if sink.err != nil || sink.repaired != 0 {
		t.Fatalf("mid-poll sweep = %d, %v; want it to leave the schedule whose poll is running (failure 3)", sink.repaired, sink.err)
	}
	got := getSchedule(t, rt, "owner-1", "rss:race")
	if hits.Load() != 1 || got.Generation != started.Generation || len(*wakes) != armed+1 || got.Totals.Polls != 1 {
		t.Fatalf("race: %d hits, %+v, %d new wakes; want one poll and its one successor", hits.Load(), got, len(*wakes)-armed)
	}
}

func TestSweepFailureStopsThePassAfterTheFirstFailedArm(t *testing.T) {
	ctx := context.Background()
	rt, _, _, _ := sourceScheduleFixture(t, "rss:down-0", nil)
	ids := []string{"rss:down-0", "rss:down-1", "rss:down-2"}
	addScheduledSources(t, rt, ids[0], ids[1:]...)
	for _, id := range ids {
		if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", id, 10*time.Minute, 5); err != nil {
			t.Fatal(err)
		}
		setNextDue(t, rt, "owner-1", id, time.Now().UTC().Add(-time.Hour))
	}
	// Arming fails (the actor log is unavailable): one attempt, then the pass
	// ends, so three schedules cost one failed write, not three (failure 6).
	attempts := 0
	rt.SetScheduleActor(func(context.Context, string, string, string, string, string, string, string, time.Time) error {
		attempts++
		return errors.New("dolt down")
	})
	n, err := rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC())
	if err == nil || n != 0 || attempts != 1 {
		t.Fatalf("sweep = %d, %v after %d arm attempts; want an error after the first", n, err, attempts)
	}
	// A cancelled context (the store is unreachable) is an error, not a repair.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if n, err := rt.sweepOverdueSourceSchedules(cctx, time.Now().UTC()); err == nil || n != 0 {
		t.Fatalf("sweep with an unreachable store = %d, %v; want an error", n, err)
	}
}

func TestSweepLoopBacksOffWhileTheSweepFailsAndResetsWhenItRecovers(t *testing.T) {
	stop := make(chan struct{})
	down := errors.New("dolt down")
	results := []error{down, down, down, down, down, down, nil, down}
	var waits []time.Duration
	calls := 0
	after := func(d time.Duration) <-chan time.Time {
		waits = append(waits, d)
		ch := make(chan time.Time, 1)
		ch <- time.Time{}
		return ch
	}
	sweep := func() (int, error) {
		err := results[calls]
		calls++
		if calls == len(results) {
			close(stop)
		}
		return 0, err
	}
	sourceScheduleSweepLoop(context.Background(), stop, 5*time.Minute, sweep, after)
	want := []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, time.Hour, time.Hour, time.Hour, 5 * time.Minute}
	if len(waits) != len(want) {
		t.Fatalf("waits %v; want %v", waits, want)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("waits %v; want %v (the sweep must back off, not spin: failure 6)", waits, want)
		}
	}
}
