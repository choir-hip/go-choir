package capsule

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

func TestRoleVerbSets(t *testing.T) {
	// Management should have no broker verbs.
	if len(RoleVerbSets()[RoleManagement]) != 0 {
		t.Errorf("super should have no broker verbs, got %d", len(RoleVerbSets()[RoleManagement]))
	}

	// Cosuper should have exec, read_file, write_file, etc.
	comanagement := RoleVerbSets()[RoleEngineering]
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
	for role, verbs := range RoleVerbSets() {
		for verb := range verbs {
			if !brokerVerbs[verb] {
				t.Errorf("%s is granted %s, which the broker does not implement", role, verb)
			}
		}
	}

	// Research should have read_file but not exec or write_file.
	research := RoleVerbSets()[RoleResearch]
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
	if RoleVerbSets()[RoleManagement]["go_eval"] {
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

// RoleVerbSets is the fold of the desk registry (MA2.5, F4): every root desk has
// an entry, one registered after init included, the sets equal the manifest rows, and a run role has none.
func TestRoleVerbSetsFoldTheRegistry(t *testing.T) {
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		set, ok := RoleVerbSets()[AgentRole(name)]
		if d.Kind == deskmanifest.KindRole {
			if ok {
				t.Errorf("run role %s has a broker verb set", name)
			}
			continue
		}
		if d.Kind == deskmanifest.KindApp {
			continue // registered after init by other tests
		}
		if !ok || len(set) != len(d.CapsuleVerbs) {
			t.Errorf("%s: broker set %d verbs, manifest %d", name, len(set), len(d.CapsuleVerbs))
		}
		for _, v := range d.CapsuleVerbs {
			if !set[v] {
				t.Errorf("%s lacks %s", name, v)
			}
		}
	}
	if AgentRole("texture").HasVerb("exec") {
		t.Error("texture holds a broker verb")
	}
}

func TestRoleVerbSetsSeeLateRegisteredDesks(t *testing.T) {
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:rolesets", Verbs: []string{"Cast"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62700, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	defer deskmanifest.RetireApp("app:rolesets")
	set, ok := RoleVerbSets()["app:rolesets"]
	if !ok || len(set) != 0 {
		t.Fatalf("late app desk set = %v, %v", set, ok)
	}
	if AgentRole("app:rolesets").HasVerb("exec") || AgentRole("nobody").HasVerb("exec") || AgentRole("management_task").HasVerb("exec") {
		t.Fatal("a desk without capsule verbs holds one")
	}
	deskmanifest.RetireApp("app:rolesets")
	if _, ok := RoleVerbSets()["app:rolesets"]; ok {
		t.Fatal("a retired desk keeps an entry")
	}
}

// MA4.10: the capsule's role checks ask the grant, not the name. A role that
// holds capsule.mutate may bind a job and a capsule; a role that does not, and
// an unknown role, may not.
func TestAgentRoleMutatesCapsuleIsTheGrant(t *testing.T) {
	if !RoleEngineering.MutatesCapsule() {
		t.Error("engineering does not mutate a capsule")
	}
	for _, r := range []AgentRole{RoleManagement, RoleResearch, "", "no-such-role"} {
		if r.MutatesCapsule() {
			t.Errorf("%q mutates a capsule", r)
		}
	}
	if !RoleResearch.ReadsCapsules() || RoleManagement.ReadsCapsules() {
		t.Error("ReadsCapsules is not capsule.read")
	}
}
