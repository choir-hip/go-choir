package capsule

import "github.com/yusefmosiah/go-choir/internal/deskmanifest"

// AgentRole determines the fixed broker verb set granted by guest core.
type AgentRole string

const (
	RoleManagement  AgentRole = "management"  // lifecycle/authority only; no broker verbs
	RoleEngineering AgentRole = "engineering" // read/write/exec inside one granted capsule
	RoleResearch    AgentRole = "research"    // read-only inspection across capsules
)

// VerbSet is a fixed role policy. Capability payloads carry a copy for audit,
// but authorization always consults RoleVerbSets rather than trusting payload.
type VerbSet map[string]bool

// Only verbs the broker serves are granted. edit_file, rename, chmod, lstat,
// readlink, mkdir_all, remove_all, symlink, truncate and file_hash had no
// handler and no prompt or cell surface teaching them; write_file replaces
// an existing read-only file itself (yaegikernel.WriteFileReplacing) and exec
// covers the rest. A grant without a handler only turns a role check into
// "unknown verb"
// (docs/archive/problems/p5-go-served-0825-engineering-never-freezes-2026-10-15.md).
// RoleVerbSets is computed from the live desk registry on each call (F4): a
// desk registered after init has an entry too, and a role the manifest does not
// name has none, so HasVerb is false for every verb and unknown roles fail
// closed. Run roles (management task, research child) have no broker verbs.
func RoleVerbSets() map[AgentRole]VerbSet {
	sets := map[AgentRole]VerbSet{}
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind == deskmanifest.KindRole {
			continue
		}
		set := VerbSet{}
		for _, verb := range d.CapsuleVerbs {
			set[verb] = true
		}
		sets[AgentRole(name)] = set
	}
	return sets
}

func (r AgentRole) HasVerb(verb string) bool {
	d, ok := deskmanifest.Lookup(string(r))
	if !ok || d.Kind == deskmanifest.KindRole {
		return false
	}
	for _, v := range d.CapsuleVerbs {
		if v == verb {
			return true
		}
	}
	return false
}

// MutatesCapsule reports whether the role holds capsule.mutate: it may bind a
// capsule, run a job in it and write inside it (MA4.10: the capsule driver
// asks the grant, never the desk's name). An unknown role fails closed.
func (r AgentRole) MutatesCapsule() bool { return deskmanifest.MutatesCapsule(string(r)) }

// ReadsCapsules reports whether the role holds capsule.read: read-only
// inspection across capsules.
func (r AgentRole) ReadsCapsules() bool { return deskmanifest.ReadsCapsules(string(r)) }
