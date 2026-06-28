package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

func objectiveFingerprint(ownerID, trajectoryID, parentRunID, objective string) string {
	parts := []string{
		strings.TrimSpace(ownerID),
		strings.TrimSpace(trajectoryID),
		strings.TrimSpace(parentRunID),
		normalizeObjectiveText(objective),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeObjectiveText(raw string) string {
	terms := []string{}
	var b strings.Builder
	lastSpace := false
	flush := func() {
		token := strings.TrimSpace(b.String())
		if token == "" {
			return
		}
		terms = append(terms, token)
		b.Reset()
	}
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			flush()
			lastSpace = true
		}
	}
	flush()
	return strings.Join(terms, " ")
}
