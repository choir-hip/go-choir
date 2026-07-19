package computerevent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
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
	{kind: "authorization_bearer", expression: regexp.MustCompile(`(?i)Bearer[ \t]+[A-Za-z0-9._~+/=-]{12,}`)},
	{kind: "credential_assignment", expression: regexp.MustCompile(`(?i)(?:api[_-]?key|access[_-]?token|auth[_-]?token|secret|password)[ \t]*[:=][ \t]*[^\s,;"']{8,}`)},
	{kind: "openai_key", expression: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`)},
	{kind: "github_token", expression: regexp.MustCompile(`\b(?:ghp_|github_pat_)[A-Za-z0-9_]{16,}\b`)},
	{kind: "google_api_key", expression: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}\b`)},
}

func DetectPrivateSecrets(payload []byte) []string {
	kinds := make(map[string]struct{})
	for _, pattern := range privateSecretPatterns {
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
