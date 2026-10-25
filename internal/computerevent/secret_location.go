package computerevent

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Naming a secret refusal (P8, docs/evidence/selfdev-trace-reviews-2026-10-25.md
// round 4). A refusal that names no rule leaves the author guessing; one that
// names the rule and the line, never the matched text, lets them fix the line.

// SecretLocation is where a private-secret pattern first matched a payload:
// the pattern's kind and the 1-based line the match starts on.
type SecretLocation struct {
	Kind string
	Line int
}

// LocatePrivateSecrets is DetectPrivateSecrets with the line of each kind's
// first match. It reports the same kinds, in the same order.
func LocatePrivateSecrets(payload []byte) []SecretLocation {
	first := map[string]int{}
	for _, pattern := range privateSecretPatterns {
		loc := pattern.expression.FindIndex(payload)
		if loc == nil {
			continue
		}
		if prev, seen := first[pattern.kind]; !seen || loc[0] < prev {
			first[pattern.kind] = loc[0]
		}
	}
	out := make([]SecretLocation, 0, len(first))
	for kind, offset := range first {
		out = append(out, SecretLocation{Kind: kind, Line: bytes.Count(payload[:offset], []byte("\n")) + 1})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// secretRuleDescriptions say what each private-secret pattern matches, by
// token class: never a matched value.
var secretRuleDescriptions = map[string]string{
	"private_key":           "a PEM private key block",
	"private_key_header":    "a PEM private key header (-----BEGIN ... PRIVATE KEY-----)",
	"authorization_bearer":  "the word Bearer followed by 12 or more token characters",
	"credential_assignment": "one of the words api_key, access_token, auth_token, token, secret or password (any case, also at the end of a longer name such as cfgToken or \"token\" in quotes) followed by : or = and then 8 or more characters with no space, comma, semicolon or quote",
	"openai_key":            "an sk- prefix followed by 16 or more key characters",
	"github_token":          "a ghp_ or github_pat_ prefix followed by 16 or more characters",
	"google_api_key":        "an AIza prefix followed by 20 or more key characters",
}

// DescribeSecretRule says what a private-secret kind matches, or "" for an
// unknown kind.
func DescribeSecretRule(kind string) string {
	return secretRuleDescriptions[kind]
}

// SecretRefusalDetail names each located rule, its line and what it
// matches, for a refusal message.
func SecretRefusalDetail(found []SecretLocation) string {
	parts := make([]string, 0, len(found))
	for _, f := range found {
		parts = append(parts, fmt.Sprintf("%s at line %d (%s)", f.Kind, f.Line, DescribeSecretRule(f.Kind)))
	}
	return strings.Join(parts, "; ")
}
