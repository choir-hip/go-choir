package agentcore

import (
	"bytes"
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// docs/problems/fp4-actor-retry-no-backoff-wake-latency-spawn-mailbox-2026-10-22.md
// F2 and F3, through the outbox sweep that projects an assignment's wakes.
// Ways F2 can fail: the latency line still counts from the assignment's
// open, so a wake minted by a later version reads as the assignment's age;
// the fix moves the body's created_at, which the crash rule reads. Ways F3
// can fail: the open-time spawn wake still goes to the casting agent and
// waits for its turn; it moves but the handler drops it.

type wakeCapture struct {
	mu      sync.Mutex
	targets map[string][]string // kind -> target agent ids, in dispatch order
}

func (c *wakeCapture) dispatch(_ context.Context, _, _, target, kind, _, _, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets[kind] = append(c.targets[kind], target)
	return nil
}

func (c *wakeCapture) of(kind string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.targets[kind]...)
}

// The bind (version 2) mints initial_dispatch three seconds after the open;
// projected at once, its latency line must not read three seconds.
func TestActorWakeLatencyCountsFromTheWakesOwnMint(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	// A computer up for a while, as on staging (2.8 min): the store keeps
	// the open time to the second, and a runtime booted in that same second
	// would close the bind's wake as pre-boot work (recorded in the problem
	// doc as a residual of the crash rule's input).
	rt.bootedAt = time.Now().UTC().Add(-time.Hour)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-wake-mint", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	capture := &wakeCapture{targets: map[string][]string{}}
	rt.SetDispatchActor(capture.dispatch)
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-wake-mint")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	rt.sweepActorWakeOutbox(ctx) // the open's wakes
	time.Sleep(3 * time.Second)
	if _, err := s.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, "run:"+open.AssignmentID, "opaque-wake-mint")); err != nil {
		t.Fatal(err)
	}
	boundAt := time.Now()

	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	rt.sweepActorWakeOutbox(ctx)
	log.SetOutput(previous)

	// The run row and the bind commit each mint one; both must read fresh.
	if got := capture.of("initial_dispatch"); len(got) == 0 {
		t.Fatal("the bind's initial_dispatch was not projected")
	}
	waitedRE := regexp.MustCompile(`waited=(\S+)`)
	for _, line := range strings.Split(buf.String(), "\n") {
		if !strings.Contains(line, "actor wake latency kind=initial_dispatch") {
			continue
		}
		m := waitedRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("latency line without waited: %s", line)
		}
		// Measured from the bind's commit, waited is at most the time since
		// the bind plus the store's second rounding. A slow sweep may log a
		// line (it also logs a dispatch that took 2 s), so the bound moves
		// with it (review round 2, R2-2); on main waited is 3 s more.
		if waited, err := time.ParseDuration(m[1]); err != nil || waited > time.Since(boundAt)+1500*time.Millisecond {
			t.Fatalf("initial_dispatch projected at once reads waited=%s: measured from the assignment's open, not the bind that minted it: %s", m[1], line)
		}
	}
}

// The open commit's spawn wake goes to the admission mailbox, not to the
// casting agent (management), so the first admission attempt does not wait
// for management's turn to end.
func TestOpenTimeSpawnWakeGoesToTheAdmissionMailbox(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-spawn-mailbox", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	capture := &wakeCapture{targets: map[string][]string{}}
	rt.SetDispatchActor(capture.dispatch)
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-spawn-mailbox")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	rt.sweepActorWakeOutbox(ctx)
	got := capture.of(delegatedAssignmentSpawnDeadlineUpdateKind)
	if len(got) != 1 || got[0] != EngineeringAdmissionMailbox {
		t.Fatalf("open-time spawn wake targets = %v, want [%s] (the casting agent is %s)", got, EngineeringAdmissionMailbox, open.Binding.ParentAgentID)
	}
}
