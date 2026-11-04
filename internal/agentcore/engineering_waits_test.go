package agentcore

// One typed wait record per engineering assignment, shown to management
// and Texture (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md).
// Failure modes, written before the code:
//
//  W1 an admission wait or a job wait leaves no current record that
//     management can read (it cannot tell "blocked" from "stuck");
//  W2 Texture gets one line per check instead of one per wait, or a line for
//     a wait shorter than the announce threshold;
//  W3 the wait's end is never told, so the owner's line says "waiting"
//     forever, or the minutes waited are lost;
//  W4 the relay is minted with progress_to_texture off;
//  W5 the round's minutes waited by kind are never recorded when the
//     assignment ends, or the tracker keeps the entry forever;
//  W6 a busy refusal maps to the wrong class (a policy wait reads as
//     contention) or to no kind.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func waitTestClock(rt *Runtime, at *time.Time) {
	rt.admissionNow = func() time.Time { return *at }
}

func waitRelays(t *testing.T, s *store.Store, seed store.EngineeringAssignmentSeed) []types.CommitmentRecord {
	t.Helper()
	var out []types.CommitmentRecord
	for _, rec := range relayTextureRecords(t, s, seed) {
		if strings.HasPrefix(rec.RecordID, "progress:wait:") {
			out = append(out, rec)
		}
	}
	return out
}

func TestAssignmentWaitIsShownToManagementAndOnceToTexture(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-waits", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)

	busy := &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: "host memory", Why: engineeringWaitHostMemory,
		HostMemoryReason: "host memory refused the build capsule: 900 MiB over the floor"}
	rt.observeAdmissionWait(ctx, assignment, busy, now, 30*time.Second)
	views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].Wait == nil || views[0].Wait.Kind != types.WaitKindHostMemory || views[0].Wait.Sentence == "" {
		t.Fatalf("W1: views = %+v", views)
	}
	if got := waitRelays(t, s, seed); len(got) != 0 {
		t.Fatalf("W2: a wait under a minute was relayed: %+v", got)
	}
	for i := 0; i < 4; i++ {
		now = now.Add(30 * time.Second)
		rt.observeAdmissionWait(ctx, assignment, busy, now, 30*time.Second)
	}
	relays := waitRelays(t, s, seed)
	if len(relays) != 1 || relays[0].SubjectRefs["wait_phase"] != types.WaitRelayStart ||
		!strings.Contains(relays[0].Directive.Body, "host memory") || !types.IsEngineeringProgressRelay(relays[0]) {
		t.Fatalf("W2: one start line per wait: %+v", relays)
	}
	now = now.Add(7 * time.Minute)
	rt.endAssignmentWait(ctx, assignment.AssignmentID, assignment.Binding.Attempt, "admitted")
	relays = waitRelays(t, s, seed)
	if len(relays) != 2 {
		t.Fatalf("W3: start and end lines: %+v", relays)
	}
	var end types.CommitmentRecord
	for _, r := range relays {
		if r.SubjectRefs["wait_phase"] == types.WaitRelayEnd {
			end = r
		}
	}
	if !strings.Contains(end.Directive.Body, "Waited 9 min for host memory") || end.SubjectRefs["waited_s:host_memory"] != "540" {
		t.Fatalf("W3: end line = %q refs %v", end.Directive.Body, end.SubjectRefs)
	}
	views = rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].Wait != nil || views[0].WaitedMinutes["host_memory"] != 9 {
		t.Fatalf("W3: after the end: %+v", views)
	}
	rt.closeAssignmentWaits(ctx, assignment)
	if views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID); len(views) != 0 {
		t.Fatalf("W5: the tracker kept a closed assignment: %+v", views)
	}
	rec, err := s.GetCommitmentRecord(ctx, seed.OwnerID, seed.ComputerID, assignmentWaitSummaryRecordID(assignment.AssignmentID, assignment.Binding.Attempt))
	if err != nil || rec == nil || !strings.Contains(rec.Prediction.Hypothesis, "waited 9 min: host memory 9 min") || rec.SubjectRefs["waited_s:host_memory"] != "540" {
		t.Fatalf("W5: summary record = %+v %v", rec, err)
	}
}

func TestAssignmentWaitRelaysNothingWhenTheSwitchIsOff(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-waits-off", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, false)
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	busy := &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: "a-0", Why: "live"}
	rt.observeAdmissionWait(ctx, assignment, busy, now, 10*time.Second)
	now = now.Add(3 * time.Minute)
	rt.observeAdmissionWait(ctx, assignment, busy, now, 10*time.Second)
	if got := waitRelays(t, s, seed); len(got) != 0 {
		t.Fatalf("W4: relayed with the switch off: %+v", got)
	}
	// The switch turned on mid-wait: the next check announces the wait
	// (a start), and an end never comes without its start (review r1 F9).
	relaySwitch(t, s, seed, true)
	rt.endAssignmentWait(ctx, assignment.AssignmentID, assignment.Binding.Attempt, "admitted")
	if got := waitRelays(t, s, seed); len(got) != 0 {
		t.Fatalf("W4: an end line without its start: %+v", got)
	}
	if views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID); len(views) != 1 || views[0].WaitedMinutes["engineering_slot"] != 3 {
		t.Fatalf("management still sees the wait with the switch off: %+v", views)
	}
}

func TestJobWaitEventsFollowTheAssignment(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-waits-job", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	notify := map[string]string{jobNotifyOwner: seed.OwnerID, jobNotifyComputer: seed.ComputerID, jobNotifyInbox: run.AgentID, jobNotifyRun: run.RunID}
	w := &types.WaitState{Kind: types.WaitKindHostMemory, Class: types.WaitClassContention, Subject: "job j-1", Resource: "host memory", Since: now}
	rt.handleCapsuleJobWait(capsule.JobWaitEvent{AgentRunID: run.RunID, JobID: "j-1", Notify: notify, Wait: w})
	views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].AssignmentID != assignment.AssignmentID || views[0].Wait == nil {
		t.Fatalf("W1 job: views = %+v", views)
	}
	now = now.Add(2 * time.Minute)
	rt.handleCapsuleJobWait(capsule.JobWaitEvent{AgentRunID: run.RunID, JobID: "j-1", Notify: notify, Wait: w, Ended: true, Waited: 2 * time.Minute, Outcome: "started"})
	views = rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].Wait != nil || views[0].WaitedMinutes["host_memory"] != 2 {
		t.Fatalf("W3 job: views = %+v", views)
	}
}

func TestAdmissionBusyWaitStateClasses(t *testing.T) {
	now := time.Now()
	for why, want := range map[string]struct {
		kind  types.WaitKind
		class types.WaitClass
	}{
		engineeringWaitHostMemory:     {types.WaitKindHostMemory, types.WaitClassContention},
		"live":                        {types.WaitKindEngineeringSlot, types.WaitClassPolicy},
		"queued_ahead":                {types.WaitKindQueuedAhead, types.WaitClassPolicy},
		"candidate_pending":           {types.WaitKindCandidatePending, types.WaitClassPolicy},
		"operation_open":              {types.WaitKindSelfdevOperation, types.WaitClassPolicy},
		"carry_unreadable":            {types.WaitKindReadError, types.WaitClassPolicy},
		engineeringWaitRefresh:        {types.WaitKindPlatformUpdate, types.WaitClassPolicy},
		engineeringWaitSourceSettling: {types.WaitKindSourceSettling, types.WaitClassPolicy},
	} {
		w := admissionBusyWaitState(types.EngineeringAssignment{AssignmentID: "a-1"}, &engineeringAdmissionBusy{AssignmentID: "a-1", BlockedBy: "x", Why: why, RefreshTarget: "abc"}, now, now, 10*time.Second)
		if w == nil || w.Kind != want.kind || w.Class != want.class || w.Resource == "" || w.CheckEverySec != 10 {
			t.Errorf("W6 %s: %+v", why, w)
		}
	}
	if w := admissionBusyWaitState(types.EngineeringAssignment{}, &engineeringAdmissionBusy{Why: engineeringWaitHostMemory}, now, now, time.Second); w.Until.IsZero() {
		t.Error("W6: a host memory wait names its bound")
	}
}

// Engineering's own view. Failure modes:
//  E1 choir.Start's result for a waiting job says only "waiting" (no record,
//     no sentence, no choice);
//  E2 a turn parked on a job still waiting for host memory gives up at the
//     job's wall clock from now, before the job has even started;
//  E3 management's frame lacks the waits.

func TestJobStartResultCarriesTheTypedWait(t *testing.T) {
	now := time.Now()
	w := &types.WaitState{Kind: types.WaitKindHostMemory, Class: types.WaitClassContention, Subject: "job j-1", Resource: "host memory",
		NeedBytes: 1 << 30, HaveBytes: 0, HasHave: true, Since: now}
	w.Stamp(now)
	out := map[string]any{}
	jobStartFields(out, yaegikernel.JobView{ID: "j-1", State: capsulejob.StateWaiting, Reason: "waiting for host memory: text", Wait: w})
	if out["wait"] == nil || !strings.Contains(out["waiting"].(string), "needs 1024 MiB") || !strings.Contains(out["next"].(string), "choir.Stop") {
		t.Fatalf("E1: %+v", out)
	}
	out = map[string]any{}
	jobStartFields(out, yaegikernel.JobView{ID: "j-1", State: capsulejob.StateWaiting, Reason: "waiting for host memory: text"})
	if out["waiting"] != "waiting for host memory: text" {
		t.Fatalf("E1 without a record: %+v", out)
	}
}

func TestParkOnAWaitingJobCoversItsWaitBound(t *testing.T) {
	now := time.Now()
	wall := now.Add(20 * time.Minute)
	view := yaegikernel.JobView{State: capsulejob.StateWaiting, WallClockSec: 1200,
		Wait: &types.WaitState{Kind: types.WaitKindHostMemory, Since: now, Until: now.Add(30 * time.Minute)}}
	if got := jobParkUntil(view, wall, now); !got.Equal(now.Add(50 * time.Minute)) {
		t.Fatalf("E2: deadline %s, want the wait bound plus the wall clock", got.Sub(now))
	}
	running := yaegikernel.JobView{State: capsulejob.StateRunning}
	if got := jobParkUntil(running, wall, now); !got.Equal(wall) {
		t.Fatalf("a running job keeps its deadline: %s", got.Sub(now))
	}
}

func TestManagementPackCarriesWaits(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-waits-pack", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	rt.observeAdmissionWait(ctx, assignment, &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: "a-0", Why: "live"}, now, 10*time.Second)
	pack := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: persistentManagementAgentID(seed.OwnerID), OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if pack == nil || len(pack.Waits) != 1 || pack.Waits[0].Wait == nil || pack.Waits[0].Wait.Kind != types.WaitKindEngineeringSlot {
		t.Fatalf("E3: pack waits = %+v", pack)
	}
	other := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: "texture:x", OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if other != nil && len(other.Waits) != 0 {
		t.Fatalf("only management's pack carries the waits: %+v", other.Waits)
	}
}

// Review r1 (DeepSeek F2, F6): a late job-wait event never brings back an
// ended assignment, and the round's start ends only an admission wait.
func TestWaitTrackerIgnoresEndedAssignmentsAndKeepsJobWaitsAtRoundStart(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-waits-late", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	notify := map[string]string{jobNotifyOwner: seed.OwnerID, jobNotifyComputer: seed.ComputerID, jobNotifyInbox: run.AgentID, jobNotifyRun: run.RunID}
	w := &types.WaitState{Kind: types.WaitKindHostMemory, Class: types.WaitClassContention, Subject: "job j-1", Resource: "host memory", Since: now}
	rt.handleCapsuleJobWait(capsule.JobWaitEvent{AgentRunID: run.RunID, JobID: "j-1", Notify: notify, Wait: w})
	rt.endAdmissionWait(ctx, assignment.AssignmentID, assignment.Binding.Attempt, "the round started")
	if views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID); len(views) != 1 || views[0].Wait == nil {
		t.Fatalf("F6: the round's start ended a job's wait: %+v", views)
	}
	ended := assignment
	ended.Disposition = types.EngineeringAssignmentCancelled
	rt.closeAssignmentWaits(ctx, ended)
	rt.observeAssignmentWait(ctx, ended, func(since time.Time) *types.WaitState { return w.Clone() })
	if views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID); len(views) != 0 {
		t.Fatalf("F2: an ended assignment came back: %+v", views)
	}
}
