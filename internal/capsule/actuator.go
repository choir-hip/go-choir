package capsule

import (
	"os"
	"strings"
)

// Boot kernel parameter carrying the actuator route into the guest microVM.
// The machine boot setting is rendered as `choir.actuator=rlm|tools` on the
// guest kernel command line; the in-guest broker reads it at startup. Host
// process env CHOIR_ACTUATOR remains the local-dev override. Unset or any
// other value fails closed to tools: mechanical rollback is flipping the
// boot setting (or the env) back to tools, no code change.
const BootActuatorParam = "choir.actuator"

// ParseActuator normalizes a raw actuator value. Anything but rlm is tools.
func ParseActuator(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), ActuatorRLM) {
		return ActuatorRLM
	}
	return ActuatorTools
}

// ActuatorFromCmdline extracts choir.actuator=<value> from a kernel command
// line. Reports false when the parameter is absent.
func ActuatorFromCmdline(cmdline string) (string, bool) {
	for _, field := range strings.Fields(cmdline) {
		name, value, ok := strings.Cut(field, "=")
		if !ok || name != BootActuatorParam {
			continue
		}
		return ParseActuator(value), true
	}
	return "", false
}

// ResolveGuestActuator is the in-guest route authority: kernel cmdline
// (machine boot setting) wins, host-forwarded env is the fallback, default
// is tools. Pure function for testing; ReadGuestActuator adds I/O.
func ResolveGuestActuator(cmdline, env string) string {
	if value, ok := ActuatorFromCmdline(cmdline); ok {
		return value
	}
	if strings.TrimSpace(env) != "" {
		return ParseActuator(env)
	}
	return ActuatorTools
}

// ReadGuestActuator observes the live guest boot parameter, falling back to
// the forwarded environment. On non-Linux (host dev) /proc/cmdline is absent
// and the env decides, preserving local-dev behavior byte-identically.
func ReadGuestActuator() string {
	var cmdline string
	if raw, err := os.ReadFile("/proc/cmdline"); err == nil {
		cmdline = string(raw)
	}
	return ResolveGuestActuator(cmdline, os.Getenv(ActuatorEnvVar))
}
