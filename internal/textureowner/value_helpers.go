package textureowner

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
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

func isTextureAgentID(agentID string) bool {
	return strings.HasPrefix(strings.TrimSpace(agentID), "texture:")
}

func copyStringAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func persistentManagementAgentID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return agentprofile.Management
	}
	return agentprofile.Management + ":" + ownerID
}
