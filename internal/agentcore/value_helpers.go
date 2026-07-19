package agentcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	return firstNonEmpty(values...)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
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

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataStringSlice(value any) []string {
	var values []string
	switch typed := value.(type) {
	case []string:
		values = typed
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeCoSuperSlot(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "implementation", "implementer", "worker", "writer", "builder":
		return "implementation"
	case "verifier", "verify", "reviewer", "review":
		return "verifier"
	default:
		return ""
	}
}

func splitTypedWorkerUpdateRef(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	for _, separator := range []string{":", "="} {
		before, after, ok := strings.Cut(ref, separator)
		if !ok {
			continue
		}
		key := normalizeWorkerUpdateRefKey(before)
		value := strings.TrimSpace(after)
		if key == "" || value == "" || strings.ContainsAny(value, " \t\r\n") {
			return "", ""
		}
		return key, value
	}
	return "", ""
}

func normalizeWorkerUpdateRefKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "source_service_item", "source_item", "item_id":
		return "source_service_item"
	case "content_id", "content_item", "content_item_id":
		return "content_id"
	case "evidence", "evidence_id":
		return "evidence"
	case "command", "command_output", "cmd_output", "shell_command":
		return "command_output"
	case "shell", "shell_session", "terminal_session":
		return "shell_session"
	case "diff", "diff_hunk", "patch_hunk":
		return "diff_hunk"
	case "patch":
		return "patch"
	case "test", "tests", "test_run", "test_result":
		return "test_run"
	case "app_change_package", "change_package", "package":
		return "app_change_package"
	case "screenshot", "image_artifact":
		return "screenshot"
	case "video_artifact", "video_proof":
		return "video_artifact"
	case "benchmark", "benchmark_log":
		return "benchmark_log"
	case "file", "file_artifact", "artifact":
		return "file_artifact"
	default:
		return ""
	}
}

func executionTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "command_output", "shell_session", "diff_hunk", "patch", "test_run", "app_change_package", "screenshot", "video_artifact", "benchmark_log", "file_artifact":
		return true
	default:
		return false
	}
}

func looksLikeArtifactPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	return strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.Contains(value, "/")
}

func isHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func decodeRevisionMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil
	}
	return metadata
}

func sanitizeExportPart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "run"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "run"
}

func intMapValue(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case uint64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		return 0
	}
}
