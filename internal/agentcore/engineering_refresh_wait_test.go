package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The refresh hold the engineer could not see (live7 trace review, item 3):
// a deploy refresh deferred behind the engineer's frozen candidate held the
// computer on the old build all night, and only management's pack said so.
//
// Ways this could fail, written first:
//   - the engineer's Freeze result says nothing about the update waiting on
//     its candidate;
//   - a lapsed ask (the host stopped asking) is still shown as a wait;
//   - the engineer's run context never carries the wait;
//   - Texture is never told, or is told before the wait has lasted a minute;
//   - another assignment's engineer sees this wait.

func TestPendingRefreshWaitNamesTheUpdateAndTheCandidate(t *testing.T) {
	rt, _, _ := c1eRuntime(t)
	now := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	if w := rt.pendingRefreshWait("a-1", true, now); w != nil {
		t.Fatalf("no ask, no wait: %+v", w)
	}
	rt.noteRefreshAsked("abc123def", now)
	now = now.Add(2 * time.Minute)
	w := rt.pendingRefreshWait("a-1", true, now)
	if w == nil || w.Kind != types.WaitKindPlatformUpdate || w.Class != types.WaitClassPolicy {
		t.Fatalf("wait = %+v", w)
	}
	if !strings.Contains(w.Detail, "an update to abc123def is waiting for this candidate's decision") {
		t.Fatalf("detail = %q", w.Detail)
	}
	if w.Since.Equal(now) || !strings.Contains(w.Sentence, "platform update to abc123def") {
		t.Fatalf("since is the ask's, sentence stamped: %+v", w)
	}
	if live := rt.pendingRefreshWait("a-1", false, now); live == nil || strings.Contains(live.Detail, "candidate") {
		t.Fatalf("an unfrozen engineer is not told of a candidate: %+v", live)
	}
	now = now.Add(refreshAskFresh + time.Minute)
	if w := rt.pendingRefreshWait("a-1", true, now); w != nil {
		t.Fatalf("a lapsed ask is not a wait: %+v", w)
	}
}

func TestFreezeResultAndTextureCarryTheRefreshWait(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-refresh-wait", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	now := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	waitTestClock(rt, &now)
	rec := &types.RunRecord{OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, Metadata: map[string]any{
		"assignment_id": assignment.AssignmentID, "assignment_attempt": int(assignment.Binding.Attempt)}}

	out := map[string]any{}
	rt.noteFreezeRefreshWait(ctx, rec, out)
	if _, ok := out["wait"]; ok {
		t.Fatalf("no ask, no wait: %+v", out)
	}
	rt.noteRefreshAsked("abc123def", now)
	now = now.Add(3 * time.Minute)
	rt.noteRefreshAsked("abc123def", now) // the host's next sweep
	rt.noteFreezeRefreshWait(ctx, rec, out)
	w, _ := out["wait"].(*types.WaitState)
	if w == nil || w.Kind != types.WaitKindPlatformUpdate {
		t.Fatalf("freeze result lacks the wait: %+v", out)
	}
	if s, _ := out["wait_sentence"].(string); !strings.Contains(s, "an update to abc123def is waiting for this candidate's decision") {
		t.Fatalf("wait_sentence = %q", s)
	}
	views := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].Wait == nil || views[0].Wait.Kind != types.WaitKindPlatformUpdate {
		t.Fatalf("management's view: %+v", views)
	}
	relays := waitRelays(t, s, seed)
	if len(relays) != 1 || !strings.Contains(relays[0].Directive.Body, "platform update") {
		t.Fatalf("Texture's line (the ask is older than a minute): %+v", relays)
	}
	// The engineer's run context carries it; another assignment's does not.
	pack := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: agentprofile.Engineering + ":" + assignment.AssignmentID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if pack == nil || len(pack.Waits) != 1 || pack.Waits[0].Wait == nil || pack.Waits[0].Wait.Kind != types.WaitKindPlatformUpdate {
		t.Fatalf("engineer's pack = %+v", pack)
	}
	other := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: agentprofile.Engineering + ":a-other", OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if other != nil {
		for _, v := range other.Waits {
			if v.AssignmentID == assignment.AssignmentID {
				t.Fatalf("another engineer sees this assignment's wait: %+v", v)
			}
		}
	}
}
