package capsule

// AgentRole determines the fixed broker verb set granted by guest core.
type AgentRole string

const (
	RoleSuper      AgentRole = "super"      // lifecycle/authority only; no broker verbs
	RoleCoSuper    AgentRole = "co-super"   // read/write/exec inside one granted capsule
	RoleResearcher AgentRole = "researcher" // read-only inspection across capsules
)

// VerbSet is a fixed role policy. Capability payloads carry a copy for audit,
// but authorization always consults RoleVerbSets rather than trusting payload.
type VerbSet map[string]bool

var RoleVerbSets = map[AgentRole]VerbSet{
	RoleSuper: {},
	RoleCoSuper: {
		"exec": true, "read_file": true, "write_file": true, "edit_file": true,
		"list_dir": true, "stat": true, "lstat": true, "readlink": true,
		"mkdir": true, "mkdir_all": true, "remove": true, "remove_all": true,
		"rename": true, "chmod": true, "symlink": true, "truncate": true,
		"file_hash": true, "kill_session": true,
	},
	RoleResearcher: {
		"read_file": true, "list_dir": true, "stat": true, "lstat": true,
		"readlink": true, "file_hash": true,
	},
}

func (r AgentRole) HasVerb(verb string) bool {
	return RoleVerbSets[r][verb]
}
