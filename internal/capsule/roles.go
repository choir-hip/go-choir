package capsule

// AgentRole determines the fixed broker verb set granted by guest core.
type AgentRole string

const (
	RoleSuper      AgentRole = "super"      // lifecycle/authority only; no broker verbs
	RoleCoSuper    AgentRole = "co-super"   // read/write/exec inside one granted capsule
	RoleResearcher AgentRole = "researcher" // read-only inspection across capsules
)

// Actuator route authority (Def 2 route_authority): one flag, three
// consumers. The Executor forwards the host value into the capsule broker
// env, the broker resolves dispatch from its own env, and the host overlay
// builder derives the model-facing schema from the host env. Unset or any
// other value means tools; the wire format is unchanged either way.
const (
	ActuatorEnvVar = "CHOIR_ACTUATOR"
	ActuatorRLM    = "rlm"
	ActuatorTools  = "tools"
)

// EffectiveActuator is the single route authority shared by the broker,
// overlay/schema builder, and reduction gate: guest kernel cmdline
// (choir.actuator=) wins, process env CHOIR_ACTUATOR is the fallback, and
// anything else fails closed to tools.
func EffectiveActuator() string {
	return ReadGuestActuator()
}

// HostSelectsRLM reports whether the activation selected the RLM actuator.
// It shares EffectiveActuator with the broker so cmdline and env cannot
// disagree by construction.
func HostSelectsRLM() bool {
	return EffectiveActuator() == ActuatorRLM
}

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
		"file_hash": true, "kill_session": true, "go_eval": true,
		"get_actuator": true, "init_session": true, "close_session": true,
	},
	RoleResearcher: {
		"read_file": true, "list_dir": true, "stat": true, "lstat": true,
		"readlink": true, "file_hash": true, "go_eval": true,
		"get_actuator": true,
	},
}

func (r AgentRole) HasVerb(verb string) bool {
	return RoleVerbSets[r][verb]
}
