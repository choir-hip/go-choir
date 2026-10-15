package agentcore

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// A lost wake must not leave a standing schedule silent
// (problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md, "Failure
// modes considered"). The numbers below refer to that list.

func setNextDue(t *testing.T, rt *Runtime, ownerID, sourceID string, due time.Time) {
	t.Helper()
	sched := getSchedule(t, rt, ownerID, sourceID)
	sched.NextDue = due
	if err := rt.store.PutSourceSchedule(context.Background(), sched); err != nil {
		t.Fatal(err)
	}
}

// addScheduledSources registers extra RSS sources (same feed as the fixture's
// first) and starts a schedule on each.
func addScheduledSources(t *testing.T, rt *Runtime, first string, ids ...string) {
	t.Helper()
	reg := rt.researchDeps.Sources
	feed := reg.Sources[0]
	for _, id := range ids {
		src := feed
		src.ID = id
		reg.Sources = append(reg.Sources, src)
	}
}

func TestPlannedBootRearmsAnOverdueScheduleAsANewTick(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, hits := sourceScheduleFixture(t, "rss:lost", nil)
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:lost", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	lost := (*wakes)[0]
	// The wake was poisoned during an outage: the saved tick is long past and
	// nothing is queued.
	setNextDue(t, rt, "owner-1", "rss:lost", time.Now().UTC().Add(-2*time.Hour))
	armed := len(*wakes)

	plannedBoot(rt)
	boot := time.Now().UTC()
	rt.recoverSourceSchedulesAtBoot(ctx)

	got := getSchedule(t, rt, "owner-1", "rss:lost")
	if got.State != store.SourceScheduleActive || got.Generation != started.Generation+1 {
		t.Fatalf("after a planned boot = %+v; want active under a new generation so the poisoned wake is refused", got)
	}
	if wait := got.NextDue.Sub(boot); wait < -time.Second || wait > time.Minute {
		t.Fatalf("next tick in %s; want now plus a small stagger, not the stale past tick", wait)
	}
	if len(*wakes) != armed+1 || !(*wakes)[armed].notBefore.Equal(got.NextDue) || (*wakes)[armed].content == lost.content {
		t.Fatalf("planned boot armed %+v; want one new wake at the new tick (failure 1)", (*wakes)[armed:])
	}
	if got.CrashesWithPollInFlight != 0 {
		t.Fatalf("a planned-boot repair touched the crash bound: %+v (failure 5)", got)
	}
	// The old wake is refused; the new tick polls exactly once (failure 1).
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", lost.content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Fatalf("the lost wake polled after the repair (%d hits)", hits.Load())
	}
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[armed].content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("the new tick did not poll once (%d hits)", hits.Load())
	}
}

func TestPlannedBootKeepsAScheduleWithinTheOverdueGraceUntouched(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:grace", nil)
	started, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:grace", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(-sourceScheduleOverdueGrace / 3)
	setNextDue(t, rt, "owner-1", "rss:grace", due)

	plannedBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	got := getSchedule(t, rt, "owner-1", "rss:grace")
	if got.Generation != started.Generation || !got.NextDue.Equal(due) {
		t.Fatalf("a schedule within the grace was re-ticked: %+v", got)
	}
	if len(*wakes) != 2 {
		t.Fatalf("armed %d wakes; want the saved tick armed again", len(*wakes))
	}
}

func TestPlannedBootStaggersTheRepairOfManyOverdueSchedules(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:many-0", nil)
	ids := []string{"rss:many-0", "rss:many-1", "rss:many-2", "rss:many-3"}
	addScheduledSources(t, rt, ids[0], ids[1:]...)
	for _, id := range ids {
		if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", id, 10*time.Minute, 5); err != nil {
			t.Fatal(err)
		}
		setNextDue(t, rt, "owner-1", id, time.Now().UTC().Add(-time.Hour))
	}
	armed := len(*wakes)

	plannedBoot(rt)
	boot := time.Now().UTC()
	rt.recoverSourceSchedulesAtBoot(ctx)
	var offsets []time.Duration
	for _, id := range ids {
		offsets = append(offsets, getSchedule(t, rt, "owner-1", id).NextDue.Sub(boot).Round(time.Second))
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
	for i, off := range offsets {
		want := time.Duration(i) * sourceScheduleRearmStagger
		if off < want-2*time.Second || off > want+3*time.Second {
			t.Fatalf("tick %d due in %s; want about %s (stagger storm, failure 2); all: %v", i, off, want, offsets)
		}
	}
	if len(*wakes) != armed+len(ids) {
		t.Fatalf("armed %d new wakes; want one per schedule", len(*wakes)-armed)
	}
}

func TestPlannedBootDoesNotReviveAClosedSchedule(t *testing.T) {
	ctx := context.Background()
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:closed", nil)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:closed", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	sched := getSchedule(t, rt, "owner-1", "rss:closed")
	sched.State, sched.StateReason = store.SourceScheduleInterruptedByRestart, sourceScheduleCrashLoopWhy
	sched.NextDue = time.Now().UTC().Add(-time.Hour)
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		t.Fatal(err)
	}
	armed := len(*wakes)
	plannedBoot(rt)
	rt.recoverSourceSchedulesAtBoot(ctx)
	if got := getSchedule(t, rt, "owner-1", "rss:closed"); got.State != store.SourceScheduleInterruptedByRestart || got.Generation != sched.Generation || len(*wakes) != armed {
		t.Fatalf("planned boot revived a crash-loop-closed schedule: %+v, %d wakes (failure 5)", got, len(*wakes)-armed)
	}
}
