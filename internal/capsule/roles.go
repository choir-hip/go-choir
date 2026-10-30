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
var RoleVerbSets = func() map[AgentRole]VerbSet {
	sets := map[AgentRole]VerbSet{}
	// A role the manifest does not name has no entry, so HasVerb is false
	// for every verb: unknown roles fail closed.
	for _, role := range []AgentRole{RoleManagement, RoleEngineering, RoleResearch} {
		set := VerbSet{}
		if d, ok := deskmanifest.Lookup(string(role)); ok {
			for _, verb := range d.CapsuleVerbs {
				set[verb] = true
			}
		}
		sets[role] = set
	}
	return sets
}()

func (r AgentRole) HasVerb(verb string) bool {
	return RoleVerbSets[r][verb]
}
