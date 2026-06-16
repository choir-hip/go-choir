package runtime

const (
	textureRevisionRoleInput     = "input"
	textureRevisionRoleCanonical = "canonical"

	textureInputOriginUserPrompt        = "user_prompt"
	textureInputOriginProcessorHandoff  = "processor_handoff"
	textureInputOriginReconcilerHandoff = "reconciler_handoff"
)

func textureInputOriginForCaller(profile string) string {
	switch canonicalAgentProfile(profile) {
	case AgentProfileProcessor:
		return textureInputOriginProcessorHandoff
	case AgentProfileReconciler:
		return textureInputOriginReconcilerHandoff
	default:
		return ""
	}
}

func wireRevisionIsCanonicalArticle(meta map[string]any) bool {
	if metadataString(meta, "revision_role") == textureRevisionRoleCanonical {
		return true
	}
	if v, ok := meta["article_version"].(bool); ok && v {
		return true
	}
	return false
}

func wireRevisionIsInput(meta map[string]any) bool {
	return metadataString(meta, "revision_role") == textureRevisionRoleInput
}
