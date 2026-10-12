package agentcore

import (
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

func normalizeEngineeringSlot(raw string) string {
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
	case "capsule_bundle", "bundle", "package":
		return "capsule_bundle"
	case "screenshot", "image_artifact":
		return "screenshot"
	case "video_artifact", "video_proof":
		return "video_artifact"
	case "benchmark", "benchmark_log":
		return "benchmark_log"
	case "file", "file_artifact", "artifact":
		return "file_artifact"
	case "operation", "operation_id", "self_development_operation":
		return "operation"
	case "receipt", "receipt_id", "consensus_receipt":
		return "receipt"
	case "event_head", "head", "canonical_event_head":
		return "event_head"
	default:
		return ""
	}
}
