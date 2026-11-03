package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Problem 24 (docs/problems/build-refused-for-host-memory-cancels-assignment-2026-11-03.md):
// a build spawn the host could not cover cancelled its assignment, so every
// refusal on a busy host needed a person to re-Cast. The refusal now waits in
// the admission queue. Ways the fix can fail, written before it:
//   W1 the refusal still cancels the open, or a refusal that clears never
//      reaches the spawn saga again;
//   W2 the wait is unbounded, or ends without the host's reason;
//   W3 the retry fires faster than 30 s or slower than a minute;
//   W4 the wait is invisible: nothing on the ledger says what it waits for;
//   W5 a crash boot resumes the waiting request (CLAUDE.md, Restarts End Work);
//   W6 an error that is not the host's refusal (vmctl unreachable, a broken
//      spawn) waits instead of failing, so a real failure hangs 30 minutes;
//   W7 a protected computer waits. Its builds are always admitted by vmctl
//      (internal/vmctl, TestCapsuleBuildReservationProtectedAlwaysAdmittedAndCounted,
//      B5), so it never holds a refusal; W6 pins that nothing else waits.

const hostRefusalReason = "host memory available 10216 MiB; 2161 MiB reserved for starts in flight and live builds; this build needs 2810 MiB above the 4779 MiB floor and 1024 MiB headroom"

func hostRefusalSpawnErr() error {
	return fmt.Errorf("spawn assigned capsule after durable open: %w", &capsule.HostMemoryRefusal{Reason: hostRefusalReason})
}

func decodeSpawnWake(t *testing.T, content string) delegatedAssignmentSpawnDeadline {
	t.Helper()
	var wake delegatedAssignmentSpawnDeadline
	if err := json.Unmarshal([]byte(content), &wake); err != nil {
		t.Fatal(err)
	}
	return wake
}

// W1, W3, W4: refused, then admitted on the retry: the request stays open and
// unbound, one retry is armed 30 to 60 s out carrying when the wait began, the
// wait is on the ledger with its reason, and the retry enters the spawn saga.
func TestEngineeringHostMemoryRefusalWaitsThenProceeds(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	wakes := captureAdmissionWakes(f.rt)
	waiting := f.open(0)
	now := time.Now().UTC()
	f.rt.admissionNow = func() time.Time { return now }
	ctx := context.Background()

	busy := engineeringHostMemoryWait(waiting.AssignmentID, hostRefusalSpawnErr())
	if busy == nil || !errors.Is(busy, ErrEngineeringAdmissionBusy) || busy.Why != engineeringWaitHostMemory {
		t.Fatalf("W1: a host refusal is not a retryable wait: %+v", busy)
	}
	first := delegatedAssignmentSpawnDeadline{AssignmentID: waiting.AssignmentID, Attempt: waiting.Binding.Attempt}
	if err := f.rt.deferEngineeringAdmission(ctx, waiting, first, "objective", busy); err != nil {
		t.Fatalf("W1: the wait was not armed: %v", err)
	}
	if got := f.get(0); got.Disposition != types.EngineeringAssignmentOpen || got.CapsuleDisposition != types.EngineeringCapsuleUnbound {
		t.Fatalf("W1: a refused build left the request %s/%s (%q), want open and unbound", got.Disposition, got.CapsuleDisposition, got.DispositionReason)
	}
	armed := f.spawnWakesFor(*wakes, 0)
	if len(armed) != 1 {
		t.Fatalf("W1: %d retries armed, want one", len(armed))
	}
	if armed[0].HostMemorySince != now.UnixNano() {
		t.Fatalf("W2: the retry does not carry when the wait began: %+v", armed[0])
	}
	if delay := (*wakes)[0].notBefore.Sub((*wakes)[0].at); delay < 29*time.Second || delay > 61*time.Second {
		t.Fatalf("W3: retry in %v, want 30 to 60 s", delay)
	}
	rec, err := f.s.GetCommitmentRecord(ctx, f.seed.OwnerID, f.computer, engineeringHostMemoryWaitRecordID(waiting.AssignmentID, waiting.Binding.Attempt))
	if err != nil || rec == nil {
		t.Fatalf("W4: no ledger record of the wait: %v", err)
	}
	if text := rec.Prediction.Hypothesis + " " + rec.Observation.Excerpt; !strings.Contains(text, "waiting for host memory: ") || !strings.Contains(text, hostRefusalReason) {
		t.Fatalf("W4: the record does not say it waits for host memory and why: %q", text)
	}

	// A second refusal keeps the clock the first one started.
	later := now.Add(5 * time.Minute)
	f.rt.admissionNow = func() time.Time { return later }
	if err := f.rt.deferEngineeringAdmission(ctx, f.get(0), armed[0], "objective", busy); err != nil {
		t.Fatal(err)
	}
	again := f.spawnWakesFor(*wakes, 0)
	if len(again) != 2 || again[1].HostMemorySince != now.UnixNano() || again[1].Retry != 2 {
		t.Fatalf("W2: the second retry restarted the wait or lost its count: %+v", again)
	}
	// A duplicated first wake (or a refusal after another kind of wait)
	// carries no clock; it keeps the first refusal's, from the record.
	if err := f.rt.deferEngineeringAdmission(ctx, f.get(0), first, "objective", busy); err != nil {
		t.Fatal(err)
	}
	if dup := f.spawnWakesFor(*wakes, 0); len(dup) != 3 || dup[2].HostMemorySince != now.UnixNano() {
		t.Fatalf("W2: a duplicated first wake restarted the wait: %+v", dup)
	}
	if strings.Contains(rec.Observation.Excerpt, "durable open") {
		t.Fatalf("W4: the record carries the saga's wrapper, not the host's refusal: %q", rec.Observation.Excerpt)
	}

	// The host has room now: the retry is an ordinary admission attempt and
	// enters the spawn saga (which stops at the absent executor here).
	content, _ := json.Marshal(again[1])
	err = f.rt.HandleDelegatedAssignmentSpawnDeadline(ctx, f.seed.OwnerID, f.computer, EngineeringAdmissionMailbox, string(content))
	if err == nil || !strings.Contains(err.Error(), "capsule authority unavailable") {
		t.Fatalf("W1: the retry did not reach the spawn saga: %v", err)
	}
}

// W2: refused past the bound: the request is cancelled with the host's reason
// and nothing is armed. Just under the bound it still waits.
func TestEngineeringHostMemoryWaitCancelsAtTheBound(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	wakes := captureAdmissionWakes(f.rt)
	waiting := f.open(0)
	now := time.Now().UTC()
	f.rt.admissionNow = func() time.Time { return now }
	ctx := context.Background()
	busy := engineeringHostMemoryWait(waiting.AssignmentID, hostRefusalSpawnErr())

	under := delegatedAssignmentSpawnDeadline{AssignmentID: waiting.AssignmentID, Attempt: waiting.Binding.Attempt, Retry: 40,
		HostMemorySince: now.Add(-engineeringHostMemoryMaxWait + time.Minute).UnixNano()}
	if err := f.rt.deferEngineeringAdmission(ctx, waiting, under, "objective", busy); err != nil {
		t.Fatal(err)
	}
	if got := f.get(0); got.Disposition != types.EngineeringAssignmentOpen || len(f.spawnWakesFor(*wakes, 0)) != 1 {
		t.Fatalf("W2: a request inside the bound was not kept waiting: %s, %d wakes", got.Disposition, len(*wakes))
	}

	*wakes = nil
	past := under
	past.Retry++
	past.HostMemorySince = now.Add(-engineeringHostMemoryMaxWait - time.Second).UnixNano()
	if err := f.rt.deferEngineeringAdmission(ctx, f.get(0), past, "objective", busy); err != nil {
		t.Fatalf("W2: the bound's cancel failed: %v", err)
	}
	got := f.get(0)
	if got.Disposition != types.EngineeringAssignmentCancelled {
		t.Fatalf("W2: a request refused past the bound is %s, want cancelled", got.Disposition)
	}
	if !strings.Contains(got.DispositionReason, hostRefusalReason) || !strings.Contains(got.DispositionReason, "host memory refused the build capsule") {
		t.Fatalf("W2: the cancel does not carry the host's reason: %q", got.DispositionReason)
	}
	if len(*wakes) != 0 {
		t.Fatalf("W2: a cancelled request armed %d wakes", len(*wakes))
	}
}

// W5: a crash boot while the request waits for host memory closes it as
// interrupted and arms nothing; a planned boot keeps it for its one resume.
func TestEngineeringHostMemoryWaitCrashRestartClosesIt(t *testing.T) {
	for _, planned := range []bool{false, true} {
		t.Run(fmt.Sprintf("planned=%t", planned), func(t *testing.T) {
			f := newAdmissionFixture(t, 1)
			wakes := captureAdmissionWakes(f.rt)
			waiting := f.open(0)
			busy := engineeringHostMemoryWait(waiting.AssignmentID, hostRefusalSpawnErr())
			if err := f.rt.deferEngineeringAdmission(context.Background(), waiting,
				delegatedAssignmentSpawnDeadline{AssignmentID: waiting.AssignmentID, Attempt: waiting.Binding.Attempt}, "objective", busy); err != nil {
				t.Fatal(err)
			}
			f.restart(planned)
			*wakes = nil

			f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())

			got := f.get(0)
			armed := f.spawnWakesFor(*wakes, 0)
			if !planned {
				if !got.Disposition.Terminal() || !strings.Contains(got.DispositionReason, engineeringInterruptedByRestart) {
					t.Fatalf("W5: a crash boot left the waiting request %s (%q), want %s", got.Disposition, got.DispositionReason, engineeringInterruptedByRestart)
				}
				if len(armed) != 0 {
					t.Fatalf("W5: a crash boot armed %d spawn wakes for the waiting request", len(armed))
				}
				return
			}
			if got.Disposition != types.EngineeringAssignmentOpen || len(armed) != 1 || armed[0].Boot == 0 {
				t.Fatalf("a planned boot did not keep the waiting request for one resume: %s, %+v", got.Disposition, armed)
			}
		})
	}
}

// W6, W7: only the host's refusal waits. No error, vmctl that cannot be
// asked, or a broken spawn never waits; a protected computer's build is always
// admitted by vmctl, so it never holds a refusal to wait on.
func TestEngineeringHostMemoryOnlyTheHostsRefusalWaits(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("spawn assigned capsule after durable open: capsule state volume has 10 MiB free"),
		fmt.Errorf("spawn assigned capsule after durable open: %w", errors.New("capsule reservations unsupported")),
	} {
		if busy := engineeringHostMemoryWait("assignment-x", err); busy != nil {
			t.Fatalf("W6: %v waits: %+v", err, busy)
		}
	}
}

// W2, review 1: another kind of wait between two host refusals re-arms a
// wake without the clock; the next refusal still runs from the first one, so
// the bound cannot be reset by an intervening operation hold or slot wait.
func TestEngineeringHostMemoryBoundSurvivesAnInterveningWait(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	wakes := captureAdmissionWakes(f.rt)
	waiting := f.open(0)
	t0 := time.Now().UTC()
	f.rt.admissionNow = func() time.Time { return t0 }
	ctx := context.Background()
	host := engineeringHostMemoryWait(waiting.AssignmentID, hostRefusalSpawnErr())
	if err := f.rt.deferEngineeringAdmission(ctx, waiting,
		delegatedAssignmentSpawnDeadline{AssignmentID: waiting.AssignmentID, Attempt: waiting.Binding.Attempt}, "objective", host); err != nil {
		t.Fatal(err)
	}
	other := &engineeringAdmissionBusy{AssignmentID: waiting.AssignmentID, BlockedBy: "assignment-other", Why: "live"}
	f.rt.admissionNow = func() time.Time { return t0.Add(29 * time.Minute) }
	if err := f.rt.deferEngineeringAdmission(ctx, f.get(0), f.spawnWakesFor(*wakes, 0)[0], "objective", other); err != nil {
		t.Fatal(err)
	}
	f.rt.admissionNow = func() time.Time { return t0.Add(31 * time.Minute) }
	if err := f.rt.deferEngineeringAdmission(ctx, f.get(0), f.spawnWakesFor(*wakes, 0)[1], "objective", host); err != nil {
		t.Fatal(err)
	}
	if got := f.get(0); got.Disposition != types.EngineeringAssignmentCancelled || !strings.Contains(got.DispositionReason, hostRefusalReason) {
		t.Fatalf("W2: an intervening wait reset the host memory bound: %s (%q)", got.Disposition, got.DispositionReason)
	}
}
