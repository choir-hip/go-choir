package runtime

import "strings"

const (
	textureAgentRevisionTaskType     = "texture_agent_revision"
	legacyVTextAgentRevisionTaskType = "vtext_agent_revision"
)

func isTextureAgentRevisionTaskType(value string) bool {
	switch strings.TrimSpace(value) {
	case textureAgentRevisionTaskType, legacyVTextAgentRevisionTaskType:
		return true
	default:
		return false
	}
}
