package qdrant

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

func CollectionName(ownerID, objectKind string, indexVersion int) string {
	return fmt.Sprintf("choir_%s_%s_v%d", namePart(ownerID), namePart(objectKind), indexVersion)
}

func AliasName(ownerID, objectKind string) string {
	return fmt.Sprintf("choir_%s_%s_active", namePart(ownerID), namePart(objectKind))
}

func PointIDForCanonicalID(canonicalID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("choir:qdrant:"+canonicalID)).String()
}

func namePart(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	if out == "" {
		out = "empty"
	}
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%s_%s", out, hex.EncodeToString(sum[:])[:8])
}
