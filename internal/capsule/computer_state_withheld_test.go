package capsule

import (
	"encoding/json"
	"strings"
	"testing"
)

// The overlay is withheld only from a capsule that holds the owner's raw
// model credential. The claude mode's gateway relay secret, which works only
// at the capsule's own egress proxy, does not withhold it (problem 26,
// docs/problems/delegate-capsules-see-no-computer-state-2026-11-03.md).
//
// Ways this could fail, written before the change:
//  1. claude mode alone still withholds (the bug);
//  2. a relay secret for the broker or in the env still withholds;
//  3. a raw token for the broker passes because it is "just a token";
//  4. a raw token under any Anthropic credential key in the env passes;
//  5. a raw key under an unrelated name passes (value shape, not key);
//  6. a secret that only borrows the relay prefix passes;
//  7. an empty value withholds a capsule that holds nothing;
//  8. a claude-mode spawn without its relay secret gets the state (it
//     must fail closed: every claude-mode spawn carries one);
//  9. a key smuggled in an env entry's name, not its value, passes.
func TestOnlyARawModelCredentialWithholdsTheComputerState(t *testing.T) {
	relay, err := NewClaudeRelaySecret()
	if err != nil {
		t.Fatal(err)
	}
	const raw = "sk-ant-oat01-FAKE-not-a-real-token-0123456789"
	held := ComputerStateWithheldRawCredential
	cases := []struct {
		name  string
		spec  SpawnSpec
		token string
		want  string
	}{
		{"implementation", SpawnSpec{ComputerState: true, Egress: true}, "", ""},
		{"gateway claude mode", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, relay, ""},
		{"claude mode without a relay secret fails closed", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, "", held},
		{"relay secret under the auth token key", SpawnSpec{ComputerState: true, ClaudeDelegate: true, Env: []string{"ANTHROPIC_AUTH_TOKEN=" + relay}}, relay, ""},
		{"raw key as an env name", SpawnSpec{ComputerState: true, ClaudeDelegate: true, Env: []string{raw + "=dummy"}}, relay, held},
		{"raw key as a bare env entry", SpawnSpec{ComputerState: true, ClaudeDelegate: true, Env: []string{raw}}, relay, held},
		{"relay secret in the env", SpawnSpec{ComputerState: true, Env: []string{"PATH=/bin", ClaudeDelegateBrokerTokenEnv + "=" + relay}}, "", ""},
		{"raw token for the broker", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, raw, held},
		{"opaque non-relay token for the broker", SpawnSpec{ComputerState: true, Egress: true}, "a-delegate-token", held},
		{"raw token in the broker env", SpawnSpec{ComputerState: true, Env: []string{"PATH=/bin", ClaudeDelegateBrokerTokenEnv + "=x"}}, "", held},
		{"oauth token in the env", SpawnSpec{ComputerState: true, Env: []string{"CLAUDE_CODE_OAUTH_TOKEN=opaque"}}, relay, held},
		{"api key in the env", SpawnSpec{ComputerState: true, Env: []string{"ANTHROPIC_API_KEY=opaque"}}, "", held},
		{"auth token in the env", SpawnSpec{ComputerState: true, Env: []string{"ANTHROPIC_AUTH_TOKEN=opaque"}}, "", held},
		{"raw key under another name", SpawnSpec{ComputerState: true, ClaudeDelegate: true, Env: []string{"SOMETHING=" + raw}}, relay, held},
		{"borrowed relay prefix", SpawnSpec{ComputerState: true, ClaudeDelegate: true}, ClaudeRelaySecretPrefix + "not-hex", held},
		{"empty token in the env", SpawnSpec{ComputerState: true, Env: []string{ClaudeDelegateBrokerTokenEnv + "=", "ANTHROPIC_API_KEY="}}, "", ""},
	}
	for _, c := range cases {
		if got := computerStateWithheld(c.spec, c.token); got != c.want {
			t.Fatalf("%s: withheld = %q, want %q", c.name, got, c.want)
		}
	}
	r := withheldComputerState(held)
	if r.Files || r.Refused != held {
		t.Fatalf("withheld receipt = %+v", r)
	}
	encoded, _ := json.Marshal(r)
	if !strings.Contains(string(encoded), `"computer_state":"withheld: raw model credential present"`) {
		t.Fatalf("receipt JSON = %s", encoded)
	}
	if strings.Contains(string(encoded), raw) || strings.Contains(string(encoded), relay) {
		t.Fatalf("the receipt names a credential (%d bytes)", len(encoded))
	}
}

// A minted relay secret is recognized; nothing else is.
func TestIsClaudeRelaySecret(t *testing.T) {
	relay, err := NewClaudeRelaySecret()
	if err != nil {
		t.Fatal(err)
	}
	if !IsClaudeRelaySecret(relay) {
		t.Fatalf("a minted relay secret was not recognized")
	}
	for _, s := range []string{"", ClaudeRelaySecretPrefix, ClaudeRelaySecretPrefix + strings.Repeat("A", 64), ClaudeRelaySecretPrefix + strings.Repeat("a", 63), relay + "0", "sk-ant-" + relay} {
		if IsClaudeRelaySecret(s) {
			t.Fatalf("%q was taken for a relay secret", s)
		}
	}
}
