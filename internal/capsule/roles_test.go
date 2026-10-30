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
	// Granting a verb the broker cannot serve only moves the failure to
	// "unknown verb"; these three had no handler and no prompt teaches them.
	for _, verb := range []string{"edit_file", "rename", "chmod"} {
		if comanagement[verb] {
			t.Errorf("engineering is granted %s, which the broker does not implement", verb)
		}
	}

	// Every granted verb is one the broker serves: its verb switch in
	// cmd/capsule-broker/main.go (a main package, so listed here). lstat,
	// readlink, mkdir_all, remove_all, symlink, truncate and file_hash were
	// granted with no handler (docs/archive/problems/p5-go-served-0825-engineering-never-freezes-2026-10-15.md,
	// residual 12); exec covers each of them.
	brokerVerbs := map[string]bool{
		"exec": true, "read_file": true, "write_file": true, "list_dir": true, "stat": true,
		"mkdir": true, "remove": true, "kill_session": true, "go_eval": true,
		"init_session": true, "close_session": true, "job_start": true, "job_status": true, "job_stop": true,
	}
	for role, verbs := range RoleVerbSets {
		for verb := range verbs {
			if !brokerVerbs[verb] {
				t.Errorf("%s is granted %s, which the broker does not implement", role, verb)
			}
		}
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

// A role the manifest does not name has no verbs: the table is read from
// internal/deskmanifest and an unknown name fails closed.
func TestUnknownRoleHasNoVerbs(t *testing.T) {
	for _, role := range []AgentRole{"", "root", "app:podcast", "Engineering", "research_child"} {
		for _, verb := range []string{"exec", "read_file", "write_file", "list_dir", "go_eval", "job_start"} {
			if role.HasVerb(verb) {
				t.Errorf("role %q is granted %s", role, verb)
			}
		}
	}
}
