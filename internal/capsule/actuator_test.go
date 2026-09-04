package capsule

import (
	"testing"
)

func TestParseActuatorFailsClosedToTools(t *testing.T) {
	cases := map[string]string{
		"": trueValue("tools"), "tools": "tools", "TOOLS": "tools",
		"rlm": "rlm", "RLM": "rlm", "  rlm  ": "rlm",
		"bogus": "tools", "rm -rf": "tools",
	}
	for in, want := range cases {
		if got := ParseActuator(in); got != want {
			t.Errorf("ParseActuator(%q) = %q, want %q", in, got, want)
		}
	}
}

func trueValue(s string) string { return s }

func TestResolveGuestActuatorBootBeatsEnv(t *testing.T) {
	if got := ResolveGuestActuator("", "rlm"); got != ActuatorRLM {
		t.Fatalf("env fallback = %q, want rlm", got)
	}
	if got := ResolveGuestActuator("", ""); got != ActuatorTools {
		t.Fatalf("empty default = %q, want tools", got)
	}
	// Machine boot setting wins over a stale forwarded env either way:
	// this is the mechanical rollback path (boot=tools forces tools).
	if got := ResolveGuestActuator("root=/dev/vda choir.actuator=tools quiet", "rlm"); got != ActuatorTools {
		t.Fatalf("boot=tools over env=rlm = %q, want tools", got)
	}
	if got := ResolveGuestActuator("choir.actuator=rlm", "tools"); got != ActuatorRLM {
		t.Fatalf("boot=rlm over env=tools = %q, want rlm", got)
	}
	if got := ResolveGuestActuator("choir.actuator=bogus", "rlm"); got != ActuatorTools {
		t.Fatalf("boot=bogus = %q, want tools (fail closed)", got)
	}
}

func TestHostSelectsRLMUnchanged(t *testing.T) {
	t.Setenv(ActuatorEnvVar, ActuatorRLM)
	if !HostSelectsRLM() {
		t.Fatal("host env rlm must select RLM")
	}
	t.Setenv(ActuatorEnvVar, ActuatorTools)
	if HostSelectsRLM() {
		t.Fatal("host env tools must not select RLM")
	}
}
