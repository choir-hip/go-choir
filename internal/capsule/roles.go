package capsule

// AgentRole defines what an agent can do, not what a capsule allows.
// Capsules are execution contexts with full VFS. Roles determine verbs.
type AgentRole string

const (
	RoleSuper      AgentRole = "super"      // spawn/destroy capsules, grant access, diagnostics
	RoleCosuper    AgentRole = "cosuper"    // exec, full VFS within granted capsules
	RoleResearcher AgentRole = "researcher" // read across all capsules, send messages, write to Dolt
)

// VerbSet is the set of broker RPC methods allowed for a role.
// Defined by role, not by *nix read/write permissions.
type VerbSet map[string]bool

// RoleVerbSets maps each role to its allowed broker verbs.
// Host control-plane verbs (spawn, destroy, mint, revoke, commit_transaction,
// inspect_capsule_raw, extract_diff, list_capsules) are NOT broker verbs —
// they are Executor/HostAuthority methods, not routed through a capsule's
// broker. Only in-capsule operations are broker verbs.
var RoleVerbSets = map[AgentRole]VerbSet{
	RoleSuper: {
		// Super has NO broker verbs. Super operates via Executor host methods:
		// spawn_capsule, destroy_capsule, pin_capsule, restart_broker,
		// force_destroy, mint_capability, revoke_capability, commit_transaction,
		// inspect_capsule_raw, extract_diff, list_capsules.
		// These bypass the broker entirely — they are host-side operations.
	},
	RoleCosuper: {
		"exec": true, "read_file": true, "write_file": true, "edit_file": true,
		"list_dir": true, "stat": true, "lstat": true, "readlink": true,
		"mkdir": true, "mkdir_all": true, "remove": true, "remove_all": true,
		"rename": true, "chmod": true, "symlink": true, "truncate": true,
		"file_hash": true, "kill_session": true,
	},
	RoleResearcher: {
		"read_file": true, "list_dir": true, "stat": true, "lstat": true,
		"readlink": true, "file_hash": true,
		// Researcher also gets external access (not broker verbs):
		// send_message, dolt_write — handled outside the broker
	},
}

// HasVerb checks if a role is allowed to perform a given verb.
func (r AgentRole) HasVerb(verb string) bool {
	verbs, ok := RoleVerbSets[r]
	if !ok {
		return false
	}
	return verbs[verb]
}
