package capsule

import "testing"

func TestRoleVerbSets(t *testing.T) {
	// Super should have no broker verbs.
	if len(RoleVerbSets[RoleSuper]) != 0 {
		t.Errorf("super should have no broker verbs, got %d", len(RoleVerbSets[RoleSuper]))
	}

	// Cosuper should have exec, read_file, write_file, etc.
	cosuper := RoleVerbSets[RoleCoSuper]
	if !cosuper["exec"] {
		t.Error("cosuper should have exec verb")
	}
	if !cosuper["read_file"] {
		t.Error("cosuper should have read_file verb")
	}
	if !cosuper["write_file"] {
		t.Error("cosuper should have write_file verb")
	}
	if !cosuper["edit_file"] {
		t.Error("cosuper should have edit_file verb")
	}

	// Researcher should have read_file but not exec or write_file.
	researcher := RoleVerbSets[RoleResearcher]
	if !researcher["read_file"] {
		t.Error("researcher should have read_file verb")
	}
	if researcher["exec"] {
		t.Error("researcher should NOT have exec verb")
	}
	if researcher["write_file"] {
		t.Error("researcher should NOT have write_file verb")
	}
}

func TestHasVerb(t *testing.T) {
	if !RoleCoSuper.HasVerb("exec") {
		t.Error("cosuper should have exec verb")
	}
	if RoleResearcher.HasVerb("exec") {
		t.Error("researcher should NOT have exec verb")
	}
	if RoleSuper.HasVerb("exec") {
		t.Error("super should NOT have exec verb (host-side only)")
	}
	// Unknown role.
	if AgentRole("unknown").HasVerb("exec") {
		t.Error("unknown role should not have any verbs")
	}
}
