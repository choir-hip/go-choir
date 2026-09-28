package computerevent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type SecretHandle struct {
	Kind   string `json:"kind"`
	Handle string `json:"handle"`
}

type secretPattern struct {
	kind       string
	expression *regexp.Regexp
}

var privateSecretPatterns = []secretPattern{
	{kind: "private_key", expression: regexp.MustCompile(`(?s)-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----.*?-----END (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)},
	{kind: "private_key_header", expression: regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)},
	{kind: "authorization_bearer", expression: regexp.MustCompile(`(?i)Bearer[ \t]+[A-Za-z0-9._~+/=-]{12,}`)},
	{kind: "credential_assignment", expression: regexp.MustCompile(`(?i)(?:api[_-]?key|access[_-]?token|auth[_-]?token|token|secret|password)[\"']?[ \t]*[:=][ \t]*[\"']?[^\s,;\"']{8,}`)},
	{kind: "openai_key", expression: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`)},
	{kind: "github_token", expression: regexp.MustCompile(`\b(?:ghp_|github_pat_)[A-Za-z0-9_]{16,}\b`)},
	{kind: "google_api_key", expression: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}\b`)},
}

// binarySecretPatterns is the refusal-safe subset for binary payloads.
// DetectPrivateSecrets is a redaction-grade detector: over-matching only
// costs extra encryption. Used as a refusal predicate (capsule release
// staging) on binary content it rejects genuine compiled artifacts —
// keyword regexes (credential_assignment, openai_key, even
// authorization_bearer) match adjacent literals in linker string blobs and
// Go symbol names like sk-session…. Binary content therefore scans only
// with the byte-prefix patterns whose fixed-format specificity (PEM
// headers, provider token prefixes) cannot collide with compiled content.
var binarySecretPatterns = []secretPattern{
	privateSecretPatterns[0],
	privateSecretPatterns[1],
	privateSecretPatterns[5],
	privateSecretPatterns[6],
}

// DetectBinarySecrets reports the secret kinds found in binary payload
// content using the refusal-safe structural subset.
func DetectBinarySecrets(payload []byte) []string {
	return detectSecrets(payload, binarySecretPatterns)
}

func DetectPrivateSecrets(payload []byte) []string {
	return detectSecrets(payload, privateSecretPatterns)
}

// refusalCredentialAssignment captures the assigned value so the refusal
// detector can drop matches whose value is program code, not a credential:
// the capture is [\"']?(value) where value is the same charset as the
// original pattern's `[^\s,;\"']{8,}` tail.
var refusalCredentialAssignment = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|auth[_-]?token|token|secret|password)[\"']?[ \t]*[:=][ \t]*[\"']?([^\s,;\"']{8,})`)

// credentialValueLooksLikeCode reports whether a credential_assignment match
// value is program structure rather than a secret. Machine-generated and
// vendored text (minified JS, vendored pdf.js, compiled string dumps)
// produces credential-shaped tokens that are identifiers, member accesses,
// or literals — the class of content the release secret scan must not
// refuse. A lowercase unquoted string of 8+ word-chars is the remaining
// secret shape (`api_key=abcdefghijklmnop`, `password=hunter2abc`).
func credentialValueLooksLikeCode(key, value string) bool {
	switch {
	case strings.HasPrefix(value, "0x"), strings.HasPrefix(value, "0X"):
		return true // hex literal (bitmask/enum constant)
	case strings.ContainsAny(value, ".()[]/\\\\{}<>"):
		return true // member access, call, index, or path expression
	case strings.HasPrefix(value, "0"), strings.HasPrefix(value, "$"):
		return true // numeric literal or interpolation start
	}
	if strings.EqualFold(key, value) {
		return true // self-assignment (`password = password`, `token = token`)
	}
	// camelCase / identifier: any uppercase in the value means code.
	for _, r := range value {
		if 'A' <= r && r <= 'Z' {
			return true
		}
	}
	return false
}

// DetectRefusalSecrets is the refusal-grade detector for text payloads.
// Unlike DetectPrivateSecrets (redaction-grade, over-match is cheap) it keeps
// precision high enough to gate a capsule release: private-key blocks and
// provider token prefixes, plus credential_assignment only when the assigned
// value is not obviously program code. Vendored build output — minified JS,
// pdf.js enums like `PASSWORD: 0x0002000`, `token = this.getToken()` — is the
// same false-positive class as compiled binaries, for which the binary
// subset already exists.
func DetectRefusalSecrets(payload []byte) []string {
	kinds := make(map[string]struct{})
	for _, pattern := range privateSecretPatterns {
		if pattern.kind != "credential_assignment" && pattern.expression.Match(payload) {
			kinds[pattern.kind] = struct{}{}
		}
	}
	for _, match := range refusalCredentialAssignment.FindAllSubmatch(payload, -1) {
		if len(match) > 2 && !credentialValueLooksLikeCode(string(match[1]), string(match[2])) {
			kinds["credential_assignment"] = struct{}{}
			break
		}
	}
	result := make([]string, 0, len(kinds))
	for kind := range kinds {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}

func detectSecrets(payload []byte, patterns []secretPattern) []string {
	kinds := make(map[string]struct{})
	for _, pattern := range patterns {
		if pattern.expression.Match(payload) {
			kinds[pattern.kind] = struct{}{}
		}
	}
	result := make([]string, 0, len(kinds))
	for kind := range kinds {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}

type secretMatch struct {
	start int
	end   int
	kind  string
}

func redactPrivatePayload(key []byte, payload []byte) ([]byte, []SecretHandle, error) {
	if len(key) != chachaKeySize {
		return nil, nil, fmt.Errorf("secret redaction: invalid key")
	}
	matches := make([]secretMatch, 0)
	for _, pattern := range privateSecretPatterns {
		for _, location := range pattern.expression.FindAllIndex(payload, -1) {
			matches = append(matches, secretMatch{start: location[0], end: location[1], kind: pattern.kind})
		}
	}
	if len(matches) == 0 {
		return append([]byte(nil), payload...), []SecretHandle{}, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})
	redacted := make([]byte, 0, len(payload))
	handles := make([]SecretHandle, 0, len(matches))
	cursor := 0
	for _, match := range matches {
		if match.start < cursor {
			continue
		}
		redacted = append(redacted, payload[cursor:match.start]...)
		handle := secretHandle(key, match.kind, payload[match.start:match.end])
		redacted = append(redacted, handle...)
		handles = append(handles, SecretHandle{Kind: match.kind, Handle: string(handle)})
		cursor = match.end
	}
	redacted = append(redacted, payload[cursor:]...)
	return redacted, handles, nil
}

const chachaKeySize = 32

func secretHandle(key []byte, kind string, secret []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("choir-secret-handle-v1\x00"))
	_, _ = mac.Write([]byte(kind))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(secret)
	digest := mac.Sum(nil)
	return []byte("secret-handle:v1:" + kind + ":" + hex.EncodeToString(digest))
}
