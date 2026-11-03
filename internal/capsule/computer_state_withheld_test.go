package capsule

import (
	"encoding/json"
	"strings"
	"testing"
)

// A capsule that holds the Claude delegate token gets no /computer layers:
// a correctness refusal until the delegate call moves behind the gateway
// (docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md).
func TestDelegateCapsulesAreWithheldTheComputerState(t *testing.T) {
	cases := []struct {
		name  string
		spec  SpawnSpec
		token string
		want  string
	}{
		{"implementation", SpawnSpec{ComputerState: true, Egress: true}, "", ""},
		{"claude mode", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, "", ComputerStateWithheldDelegate},
		{"token for the broker", SpawnSpec{ComputerState: true, Egress: true}, "a-delegate-token", ComputerStateWithheldDelegate},
		{"token in the env", SpawnSpec{ComputerState: true, Env: []string{"PATH=/bin", ClaudeDelegateBrokerTokenEnv + "=x"}}, "", ComputerStateWithheldDelegate},
		{"empty token in the env", SpawnSpec{ComputerState: true, Env: []string{ClaudeDelegateBrokerTokenEnv + "="}}, "", ""},
	}
	for _, c := range cases {
		if got := computerStateWithheld(c.spec, c.token); got != c.want {
			t.Fatalf("%s: withheld = %q, want %q", c.name, got, c.want)
		}
	}
	r := withheldComputerState(ComputerStateWithheldDelegate)
	if r.Files || r.Store != nil || r.Refused != ComputerStateWithheldDelegate || r.StoreRefused != ComputerStateWithheldDelegate {
		t.Fatalf("withheld receipt = %+v", r)
	}
	raw, _ := json.Marshal(r)
	if !strings.Contains(string(raw), `"computer_state":"withheld: delegate capsule (credential present)"`) {
		t.Fatalf("receipt JSON = %s", raw)
	}
}
