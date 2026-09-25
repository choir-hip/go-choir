package capsule

import "testing"

func TestRoleVerbSets(t *testing.T) {
	// Management should have no broker verbs.
	if len(RoleVerbSets[RoleManagement]) != 0 {
		t.Errorf("super should have no broker verbs, got %d", len(RoleVerbSets[RoleManagement]))
	}

	// Cosuper should have exec, read_file, write_file, etc.
	comanagement := RoleVerbSets[RoleEngineering]
	if !comanagement["exec"] {
		t.Error("cosuper should have exec verb")
	}
	if !comanagement["read_file"] {
		t.Error("cosuper should have read_file verb")
	}
	if !comanagement["write_file"] {
		t.Error("cosuper should have write_file verb")
	}
	if !comanagement["edit_file"] {
		t.Error("cosuper should have edit_file verb")
	}

	// Research should have read_file but not exec or write_file.
	research := RoleVerbSets[RoleResearch]
	if !research["read_file"] {
		t.Error("researcher should have read_file verb")
	}
	if research["exec"] {
		t.Error("researcher should NOT have exec verb")
	}
	if research["write_file"] {
		t.Error("researcher should NOT have write_file verb")
	}

	// Go evaluation goes through the SAME broker as Bash. Engineering gets both
	// (Go + Bash); Research gets Go-only (no exec); Management gets neither.
	if !comanagement["go_eval"] {
		t.Error("cosuper should have go_eval verb (Go + Bash through one broker)")
	}
	if !research["go_eval"] {
		t.Error("researcher should have go_eval verb (Go-only profile)")
	}
	if RoleVerbSets[RoleManagement]["go_eval"] {
		t.Error("super should NOT have go_eval verb")
	}
}
