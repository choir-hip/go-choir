package runtime

const (
	vtextRevisionRoleInput     = "input"
	vtextRevisionRoleCanonical = "canonical"

	vtextInputOriginUserPrompt       = "user_prompt"
	vtextInputOriginProcessorHandoff = "processor_handoff"
	vtextInputOriginReconcilerHandoff = "reconciler_handoff"
)

func vtextInputOriginForCaller(profile string) string {
	switch canonicalAgentProfile(profile) {
	case AgentProfileProcessor:
		return vtextInputOriginProcessorHandoff
	case AgentProfileReconciler:
		return vtextInputOriginReconcilerHandoff
	default:
		return ""
	}
}

func wireRevisionIsCanonicalArticle(meta map[string]any) bool {
	if metadataString(meta, "revision_role") == vtextRevisionRoleCanonical {
		return true
	}
	if v, ok := meta["article_version"].(bool); ok && v {
		return true
	}
	return false
}

func wireRevisionIsInput(meta map[string]any) bool {
	return metadataString(meta, "revision_role") == vtextRevisionRoleInput
}
