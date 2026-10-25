package computerevent

import (
	"slices"
	"strings"
	"testing"
)

// P8 (docs/evidence/selfdev-trace-reviews-2026-10-25.md round 4): the
// capsule's "secret-bearing Go source" refusal named no rule, and the engineer
// spent five writes (calls 53-58) guessing. Ways naming it can fail, written
// first: the location disagrees with DetectPrivateSecrets (a kind refused but
// not named, or named but not refused); the line is wrong; the description
// carries the matched value; a kind has no description.
func TestLocatePrivateSecretsNamesTheRuleAndLineNotTheValue(t *testing.T) {
	const value = "s3cr3tvalue9"
	src := "package main\n\nfunc f() {\n\tx := 1\n\tcfg := map[string]string{\"token\": \"" + value + "\"}\n\t_ = cfg\n}\n"
	found := LocatePrivateSecrets([]byte(src))
	if len(found) != 1 || found[0].Kind != "credential_assignment" || found[0].Line != 5 {
		t.Fatalf("located %+v, want credential_assignment at line 5", found)
	}
	if d := DescribeSecretRule(found[0].Kind); d == "" || strings.Contains(d, value) || !strings.Contains(d, "token") {
		t.Fatalf("description %q", d)
	}
	if msg := SecretRefusalDetail(found); strings.Contains(msg, value) || !strings.Contains(msg, "credential_assignment") || !strings.Contains(msg, "line 5") {
		t.Fatalf("detail %q", msg)
	}
	for _, payload := range []string{
		"", "no secrets here", src,
		"-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
		"Authorization: Bearer abcdefghijklmnop\nsk-abcdefghijklmnopqrst\nghp_abcdefghijklmnopqrst\nAIzaabcdefghijklmnopqrstuv\npassword=hunter2abc",
	} {
		var kinds []string
		for _, l := range LocatePrivateSecrets([]byte(payload)) {
			kinds = append(kinds, l.Kind)
			if DescribeSecretRule(l.Kind) == "" {
				t.Errorf("kind %s has no description", l.Kind)
			}
			if l.Line < 1 {
				t.Errorf("kind %s line %d", l.Kind, l.Line)
			}
		}
		want := DetectPrivateSecrets([]byte(payload))
		slices.Sort(kinds)
		if !slices.Equal(kinds, want) && !(len(kinds) == 0 && len(want) == 0) {
			t.Errorf("located %v, detected %v for %q", kinds, want, payload)
		}
	}
}
