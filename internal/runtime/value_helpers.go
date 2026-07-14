package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func requestDesktopID(r *http.Request) string {
	if r == nil {
		return types.PrimaryDesktopID
	}
	if desktopID := strings.TrimSpace(r.URL.Query().Get("desktop_id")); desktopID != "" {
		return desktopID
	}
	if desktopID := strings.TrimSpace(r.Header.Get("X-Choir-Desktop")); desktopID != "" {
		return desktopID
	}
	return types.PrimaryDesktopID
}

func rawJSONOrFallback(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	return raw
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func digestParts(parts ...string) string {
	return sha256Hex(strings.Join(parts, "\x00"))
}

func computerKindForID(computerID string) string {
	if strings.Contains(strings.ToLower(computerID), "platform") {
		return "platform"
	}
	return "user"
}

func candidateSourceRefForComputer(computerID, kind, candidateID string) string {
	part := safeRefPart(computerID)
	candidatePart := safeRefPart(candidateID)
	if kind == "platform" {
		return "refs/platform-computers/" + part + "/candidates/" + candidatePart
	}
	return "refs/computers/" + part + "/candidates/" + candidatePart
}

func safeRefPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), "-.")
}

func normalizeRouteProfile(profile, ownerID, computerID string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
	}
	if strings.HasPrefix(profile, "route:") {
		legacyID := strings.TrimSpace(strings.TrimPrefix(profile, "route:"))
		if legacyID != "" {
			return safeRefPart(ownerID) + "/" + safeRefPart(legacyID)
		}
		return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
	}
	parts := strings.Split(profile, "/")
	if len(parts) == 2 {
		ownerPart := strings.TrimSpace(parts[0])
		computerPart := strings.TrimSpace(parts[1])
		if ownerPart != "" && computerPart != "" {
			return ownerPart + "/" + computerPart
		}
	}
	return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
}

func firstNonEmptyPromotion(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}
