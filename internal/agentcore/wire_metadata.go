package agentcore

import "github.com/yusefmosiah/go-choir/internal/agentprofile"

const (
	textureRevisionRoleInput     = "input"
	textureRevisionRoleCanonical = "canonical"

	// Frozen historical profile values remain recognizable while no new
	// processor or reconciler runs can be admitted.
	frozenProcessorProfile  = "processor"
	frozenReconcilerProfile = "reconciler"

	textureInputOriginUserPrompt        = "user_prompt"
	textureInputOriginProcessorHandoff  = "processor_handoff"
	textureInputOriginReconcilerHandoff = "reconciler_handoff"

	// textureMetadataPromptUnixTS is the authoritative owner-prompt reference
	// time for relative temporal language such as "today" or "tomorrow".
	textureMetadataPromptUnixTS = "prompt_unix_ts"
)

func textureInputOriginForCaller(profile string) string {
	canonicalProfile, _ := agentprofile.Canonical(profile)
	switch canonicalProfile {
	case frozenProcessorProfile:
		return textureInputOriginProcessorHandoff
	case frozenReconcilerProfile:
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
