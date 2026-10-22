package agentcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// FP4 round 3, P1
// (docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md).
// On staging B bound 305 s after A's terminal report: the nudge and the
// minute retry are wakes addressed to management's own mailbox, and the
// dispatcher runs one activation per mailbox at a time, so they wait for
// whatever management is doing. When an assignment ends, management is
// always reading its report. Ways the fix can fail, written before it:
//   1. the nudge still waits for management's activation to end;
//   2. the backoff retry or the planned-boot resume still targets
//      management's mailbox, so the slow path keeps the old wait;
//   3. the wake reaches admission on its new mailbox and the handler drops
//      it because the mailbox is not the casting agent;
//   4. the nudge bypasses the one-slot rule (B admitted while A is live).

// admissionDispatchHarness runs a real actor dispatcher over a SQLite kernel
// log, with the runtime's scheduleActor appending to it. Management's mailbox
// is held in one long activation until release is closed.
type admissionDispatchHarness struct {
	f        *admissionFixture
	release  chan struct{}
	inMgmt   chan struct{}
	mu       sync.Mutex
	admitted map[string]admissionRun
	targets  []string
}

type admissionRun struct {
	mailbox string
	at      time.Time
	err     error
}

const holdManagementKind = "test_management_turn"

func newAdmissionDispatchHarness(t *testing.T, f *admissionFixture) *admissionDispatchHarness {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	klog, err := actor.NewSQLiteLog(db)
	if err != nil {
		t.Fatal(err)
	}
	h := &admissionDispatchHarness{f: f, release: make(chan struct{}), inMgmt: make(chan struct{}, 1), admitted: map[string]admissionRun{}}
	handler := actor.HandlerFunc(func(ctx context.Context, mailbox string, u actor.Update, memory []byte) ([]byte, error) {
		switch u.Kind {
		case holdManagementKind:
			h.inMgmt <- struct{}{}
			// Management reading A's report. Bounded far past the test's own
			// 5 s waits, so a broken test fails instead of hanging; a lapse
			// cannot pass the test, which has already judged by then.
			select {
			case <-h.release:
			case <-time.After(time.Minute):
			}
		case delegatedAssignmentSpawnDeadlineUpdateKind:
			err := f.rt.HandleDelegatedAssignmentSpawnDeadline(ctx, f.seed.OwnerID, f.computer, mailbox, u.Content)
			var wake delegatedAssignmentSpawnDeadline
			_ = json.Unmarshal([]byte(u.Content), &wake)
			h.mu.Lock()
			if _, seen := h.admitted[wake.AssignmentID]; !seen {
				h.admitted[wake.AssignmentID] = admissionRun{mailbox: mailbox, at: time.Now(), err: err}
			}
			h.mu.Unlock()
		}
		return memory, nil
	})
	kernel := actor.NewKernelRuntime(klog, klog, handler, actor.Options{}, actor.DispatcherOptions{PollInterval: 20 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	kernel.StartKernel(ctx)
	t.Cleanup(func() {
		h.releaseManagement()
		cancel()
		kernel.Stop()
	})
	var seq int
	f.rt.kernelMode = true
	f.rt.scheduleActor = func(ctx context.Context, _, _, toAgentID, kind, content, trajectoryID, _ string, notBefore time.Time) error {
		h.mu.Lock()
		seq++
		id := fmt.Sprintf("update-%d", seq)
		h.targets = append(h.targets, kind+" -> "+toAgentID)
		h.mu.Unlock()
		return kernel.Send(ctx, actor.Update{UpdateID: id, ToAgentID: toAgentID, Kind: kind, Content: content, TrajectoryID: trajectoryID, NotBefore: notBefore})
	}
	if err := kernel.Send(context.Background(), actor.Update{UpdateID: "management-turn", ToAgentID: f.seed.ParentAgentID, Kind: holdManagementKind}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.inMgmt:
	case <-time.After(10 * time.Second):
		t.Fatal("fixture: management's activation never started")
	}
	return h
}

func (h *admissionDispatchHarness) releaseManagement() {
	select {
	case <-h.release:
	default:
		close(h.release)
	}
}

func (h *admissionDispatchHarness) waitAdmission(assignmentID string, within time.Duration) (admissionRun, bool) {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		run, ok := h.admitted[assignmentID]
		h.mu.Unlock()
		if ok {
			return run, true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return admissionRun{}, false
}

// P1: management is in a long activation (reading A's report) when A ends.
// B's admission runs within seconds, while that activation is still open,
// and gets as far as the spawn saga (which stops at the absent capsule
// executor here, past admission and expiry).
func TestEngineeringAdmissionNudgeDoesNotWaitBehindManagementActivation(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	f.open(0)
	f.bind(0)
	b := f.open(1)
	h := newAdmissionDispatchHarness(t, f)

	ended := time.Now()
	f.finish(0)

	run, ok := h.waitAdmission(b.AssignmentID, 5*time.Second)
	if !ok {
		h.mu.Lock()
		targets := append([]string(nil), h.targets...)
		h.mu.Unlock()
		// Release management: if the nudge then runs, it was waiting
		// behind the activation, not lost.
		released := time.Now()
		h.releaseManagement()
		after, ran := h.waitAdmission(b.AssignmentID, 5*time.Second)
		t.Fatalf("B's admission did not run within 5 s of A's terminal fate while management was in an activation; wakes: %v; after management's activation ended it ran=%v, %s later, on %s",
			targets, ran, after.at.Sub(released).Round(time.Millisecond), after.mailbox)
	}
	if run.err == nil || !strings.Contains(run.err.Error(), "capsule authority unavailable") {
		t.Fatalf("B's admission ran on %s but did not reach the spawn saga: %v", run.mailbox, run.err)
	}
	if run.mailbox == f.seed.ParentAgentID {
		t.Fatalf("B's admission ran on management's mailbox")
	}
	t.Logf("B admitted %s after A ended, on %s", run.at.Sub(ended).Round(time.Millisecond), run.mailbox)
}

// P1, failure 4: off management's mailbox, the nudge still finds the slot
// busy while A is live, and B stays open and unbound.
func TestEngineeringAdmissionWakeOffManagementKeepsTheSlotRule(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	f.open(0)
	f.bind(0)
	b := f.open(1)
	h := newAdmissionDispatchHarness(t, f)

	content, _ := json.Marshal(delegatedAssignmentSpawnDeadline{AssignmentID: b.AssignmentID, Attempt: b.Binding.Attempt, Nudge: "assignment-gone"})
	if err := f.rt.scheduleActor(context.Background(), f.seed.OwnerID, f.computer, engineeringAdmissionMailboxForTest(f), delegatedAssignmentSpawnDeadlineUpdateKind, string(content), b.Binding.TrajectoryID, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	run, ok := h.waitAdmission(b.AssignmentID, 5*time.Second)
	if !ok {
		t.Fatal("the admission mailbox's wake never reached the handler")
	}
	if run.err != nil {
		t.Fatalf("a busy nudge on the admission mailbox errored: %v", run.err)
	}
	if got := f.get(1); got.Disposition != types.EngineeringAssignmentOpen || got.CapsuleDisposition != types.EngineeringCapsuleUnbound {
		t.Fatalf("B was touched while A holds the slot: %s/%s", got.Disposition, got.CapsuleDisposition)
	}
	if got := f.get(0); got.Disposition.Terminal() {
		t.Fatalf("the live assignment was cancelled: %s", got.Disposition)
	}
}

// engineeringAdmissionMailboxForTest is where admission wakes go: any
// mailbox but the casting agent's, read from the nudge the runtime arms.
func engineeringAdmissionMailboxForTest(f *admissionFixture) string {
	var target string
	saved := f.rt.scheduleActor
	f.rt.scheduleActor = func(_ context.Context, _, _, toAgentID, kind, _, _, _ string, _ time.Time) error {
		if kind == delegatedAssignmentSpawnDeadlineUpdateKind {
			target = toAgentID
		}
		return nil
	}
	defer func() { f.rt.scheduleActor = saved }()
	freed := f.get(0)
	freed.Disposition = types.EngineeringAssignmentCancelled
	f.rt.wakeNextQueuedEngineeringRequest(context.Background(), freed)
	if target == "" {
		f.t.Fatal("fixture: no nudge to read the admission mailbox from")
	}
	return target
}

// P1, failure 2: the backoff retry and the planned-boot resume leave
// management's mailbox too.
func TestEngineeringAdmissionRetryAndBootResumeAreNotManagementWakes(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	var targets []string
	f.rt.kernelMode = true
	f.rt.scheduleActor = func(_ context.Context, _, _, toAgentID, kind, _, _, _ string, _ time.Time) error {
		if kind == delegatedAssignmentSpawnDeadlineUpdateKind {
			targets = append(targets, toAgentID)
		}
		return nil
	}
	f.open(0)
	f.bind(0)
	b := f.open(1)
	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, b.AssignmentID, b.Binding.Attempt)
	if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
		t.Fatal(err)
	}
	f.finish(0)
	f.restart(true)
	f.rt.resumeQueuedEngineeringRequestsAtPlannedBoot(context.Background(), f.computer)
	if len(targets) < 3 {
		t.Fatalf("fixture: want a retry, a nudge and a boot resume, got wakes to %v", targets)
	}
	for _, target := range targets {
		if target == f.seed.ParentAgentID {
			t.Fatalf("an admission wake still targets management's mailbox %s: %v", target, targets)
		}
	}
}
