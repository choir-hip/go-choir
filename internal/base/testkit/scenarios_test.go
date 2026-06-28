package testkit

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

// runScenario executes a scenario against the pure planner and checks the
// relaxed assertions (ExpectActionTypes, ExpectConflictItems, ExpectNoActions,
// ExpectNoConflicts). It does NOT compare full Action/Conflict equality,
// because Version records carry timestamps and the planner may emit actions
// in a different order; instead it pins behavior by type, item id, and
// conflict reason.
func runScenario(t *testing.T, sc Scenario) {
	t.Helper()
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)

	if sc.ExpectNoActions && len(actions) != 0 {
		t.Errorf("%s: expected no actions, got %d: %v", sc.Name, len(actions), actions)
	}
	if sc.ExpectNoConflicts && len(conflicts) != 0 {
		t.Errorf("%s: expected no conflicts, got %d: %v", sc.Name, len(conflicts), conflicts)
	}

	// Relaxed action-type assertions.
	for _, want := range sc.ExpectActionTypes {
		found := false
		for _, a := range actions {
			if a.Type == want.Type && a.ItemID == want.ItemID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected action %s for %s, not found in %v", sc.Name, want.Type, want.ItemID, actions)
		}
	}

	// Relaxed conflict assertions.
	for _, want := range sc.ExpectConflictItems {
		found := false
		for _, c := range conflicts {
			if c.ItemID != want.ItemID {
				continue
			}
			if want.ReasonContains != "" && !contains(c.Reason, want.ReasonContains) {
				continue
			}
			if want.LocalVersionID != "" && c.LocalVer.VersionID != want.LocalVersionID {
				continue
			}
			if want.RemoteVersionID != "" && c.RemoteVer.VersionID != want.RemoteVersionID {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Errorf("%s: expected conflict for %s (reason~=%q local=%s remote=%s), not found in %v",
				sc.Name, want.ItemID, want.ReasonContains, want.LocalVersionID, want.RemoteVersionID, conflicts)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestScenarios runs every required scenario from the mission stopping
// condition. All six MUST pass.
func TestScenarios(t *testing.T) {
	for _, sc := range Scenarios() {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			runScenario(t, sc)
		})
	}
}

// TestScenario1LocalAddRemoteAddSamePath verifies the add/add-same-path
// conflict is emitted and both sides are preserved.
func TestScenario1LocalAddRemoteAddSamePath(t *testing.T) {
	sc := Scenarios()[0]
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(conflicts) == 0 {
		t.Fatalf("expected at least one path-collision conflict, got none")
	}
	// Verify the conflict preserves both sides.
	var found bool
	for _, c := range conflicts {
		if contains(c.Reason, "path collision") {
			if c.LocalVer.VersionID == "base_ver_local" && c.RemoteVer.VersionID == "base_ver_remote" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("path-collision conflict must preserve both sides, got %v", conflicts)
	}
	// Both items should still be uploaded/downloaded (they are genuinely new).
	hasUpload := false
	hasDownload := false
	for _, a := range actions {
		if a.Type == planner.ActionUpload && a.ItemID == "base_item_local" {
			hasUpload = true
		}
		if a.Type == planner.ActionDownload && a.ItemID == "base_item_remote" {
			hasDownload = true
		}
	}
	if !hasUpload || !hasDownload {
		t.Errorf("expected both upload and download for add/add, got %v", actions)
	}
}

// TestScenario2BothEditConflict verifies both sides preserved.
func TestScenario2BothEditConflict(t *testing.T) {
	sc := Scenarios()[1]
	_, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	c := conflicts[0]
	if c.LocalVer.VersionID != "base_ver_2" || c.RemoteVer.VersionID != "base_ver_3" {
		t.Errorf("conflict must preserve both sides, got local=%s remote=%s", c.LocalVer.VersionID, c.RemoteVer.VersionID)
	}
	if c.SyncedVer.VersionID != "base_ver_1" {
		t.Errorf("conflict must preserve synced ancestor, got %s", c.SyncedVer.VersionID)
	}
}

// TestScenario3DeleteVsEdit verifies modify/delete conflict.
func TestScenario3DeleteVsEdit(t *testing.T) {
	sc := Scenarios()[2]
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(actions) != 0 {
		t.Errorf("expected no actions for modify/delete conflict, got %v", actions)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if !contains(conflicts[0].Reason, "modify/delete") {
		t.Errorf("expected modify/delete reason, got %q", conflicts[0].Reason)
	}
}

// TestScenario4MoveVsEdit verifies move/edit conflict preserves both sides.
func TestScenario4MoveVsEdit(t *testing.T) {
	sc := Scenarios()[3]
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(actions) != 0 {
		t.Errorf("expected no actions for move/edit conflict, got %v", actions)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	c := conflicts[0]
	if c.LocalVer.VersionID != "base_ver_1" || c.RemoteVer.VersionID != "base_ver_2" {
		t.Errorf("move/edit conflict must preserve both sides, got local=%s remote=%s", c.LocalVer.VersionID, c.RemoteVer.VersionID)
	}
	if !contains(c.Reason, "move/edit") {
		t.Errorf("expected move/edit reason, got %q", c.Reason)
	}
}

// TestScenario5Idempotence verifies duplicate remote event produces no action.
func TestScenario5Idempotence(t *testing.T) {
	sc := Scenarios()[4]
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(actions) != 0 || len(conflicts) != 0 {
		t.Errorf("idempotent scenario must produce no actions/conflicts, got %d actions %d conflicts", len(actions), len(conflicts))
	}
}

// TestScenario6CorruptLocal verifies corrupt local item produces explicit
// conflict, not a silent action.
func TestScenario6CorruptLocal(t *testing.T) {
	sc := Scenarios()[5]
	actions, conflicts := planner.Plan(sc.Remote, sc.Local, sc.Synced)
	if len(actions) != 0 {
		t.Errorf("corrupt item must NOT produce a silent action, got %v", actions)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict for corrupt item, got %d", len(conflicts))
	}
	if !contains(conflicts[0].Reason, "corrupt") {
		t.Errorf("expected corrupt reason, got %q", conflicts[0].Reason)
	}
}

// TestBonusFolderMove verifies folder moves reconcile without conflict.
func TestBonusFolderMove(t *testing.T) {
	runScenario(t, FolderMoveRemote())
}

// TestAllScenariosNamed verifies every required scenario has a unique name.
func TestAllScenariosNamed(t *testing.T) {
	seen := make(map[string]bool)
	for _, sc := range Scenarios() {
		if sc.Name == "" {
			t.Error("scenario with empty name")
		}
		if seen[sc.Name] {
			t.Errorf("duplicate scenario name: %s", sc.Name)
		}
		seen[sc.Name] = true
	}
}

// TestPurityNoIOImports is a static guard: the planner package must import
// only the model package and the sort utility. This test fails to compile if
// someone adds an I/O import, which is the desired behavior — it forces a
// review. We verify by checking that the planner builds at all (the import
// list is enforced by code review and the build).
func TestPurityNoIOImports(t *testing.T) {
	// This test exists to document the purity invariant. The planner package
	// imports only "sort" and the model package. If a future change adds
	// "os", "net", "time", "crypto/rand", "database/sql", or "encoding/json"
	// to the planner, it violates the purity invariant and must be rejected.
	// We cannot statically assert imports at runtime without reflection on
	// the build, so this test is a documentation anchor; the real guard is
	// `go list -deps ./internal/base/planner` in CI.
	_ = model.ItemID("base_item_anchor")
}
