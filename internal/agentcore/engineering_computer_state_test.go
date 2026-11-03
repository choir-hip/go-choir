package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Only an implementation assignment's capsule gets the computer's state;
// verification never depends on /computer
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, S2 failure mode 8).
func TestOnlyImplementationSpawnsGetComputerState(t *testing.T) {
	if !engineeringSpawnGetsComputerState(types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentImplementation}) {
		t.Fatal("an implementation assignment did not get the computer state")
	}
	for _, kind := range []types.EngineeringAssignmentKind{types.EngineeringAssignmentVerification, ""} {
		if engineeringSpawnGetsComputerState(types.EngineeringAssignmentBinding{Kind: kind}) {
			t.Fatalf("kind %q got the computer state", kind)
		}
	}
	// The spec the spawn saga builds carries it.
	impl := engineeringAssignmentSpawnSpec("c", "run:a", types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentImplementation}, capsule.SourcePreflight{}, 1, 1, 1, false)
	verify := engineeringAssignmentSpawnSpec("c", "run:a", types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentVerification}, capsule.SourcePreflight{}, 1, 1, 1, false)
	if !impl.ComputerState || verify.ComputerState {
		t.Fatalf("spawn specs: implementation %t, verification %t", impl.ComputerState, verify.ComputerState)
	}
	// Claude mode (problem 26): an implementation capsule still asks for the
	// state, carries no credential in its spec, and a verification capsule
	// is unchanged.
	claudeImpl := engineeringAssignmentSpawnSpec("c", "run:a", types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentImplementation}, capsule.SourcePreflight{}, 1, 1, 1, true)
	claudeVerify := engineeringAssignmentSpawnSpec("c", "run:a", types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentVerification}, capsule.SourcePreflight{}, 1, 1, 1, true)
	if !claudeImpl.ComputerState || !claudeImpl.ClaudeDelegate || len(claudeImpl.Env) != 0 || claudeVerify.ComputerState {
		t.Fatalf("claude-mode spawn specs: implementation %+v, verification computer state %t", claudeImpl, claudeVerify.ComputerState)
	}
}

// Only an implementation capsule is told about /computer; a verification
// capsule keeps the mock bullet (S3 review, DeepSeek 7).
func TestBrowserRecipeNamesComputerStateOnlyForImplementation(t *testing.T) {
	impl, verify := capsuleBrowserRecipeFor(true), capsuleBrowserRecipeFor(false)
	if !strings.Contains(impl, "/computer/files") || strings.Contains(impl, "- Mock the backend with context.route for signed-in views.\n") {
		t.Fatalf("implementation recipe: %s", impl)
	}
	if strings.Contains(verify, "/computer") || !strings.Contains(verify, "- Mock the backend with context.route for signed-in views.\n") {
		t.Fatalf("verification recipe: %s", verify)
	}
	if !strings.Contains(impl, "\n- Link, run and unlink inside one job") {
		t.Fatal("the bullet after the replacement lost its line break")
	}
	// luna Slides round 7: two implementation engineers with a loopback
	// quoted the networkless line and took no shots.
	if strings.Contains(impl, "networkless") || !strings.Contains(impl, "it has a loopback") {
		t.Fatalf("implementation recipe still tells the engineer it may be networkless: %s", impl)
	}
	if !strings.Contains(verify, "In a networkless capsule (every verification capsule) Vite cannot bind") {
		t.Fatalf("verification recipe lost the networkless line: %s", verify)
	}
}
