package agentcore

const (
	textureRevisionRoleCanonical = "canonical"

	// Frozen historical profile values remain recognizable while no new
	// processor or reconciler runs can be admitted.
	frozenProcessorProfile  = "processor"
	frozenReconcilerProfile = "reconciler"

	textureInputOriginUserPrompt = "user_prompt"

	// textureMetadataPromptUnixTS is the authoritative owner-prompt reference
	// time for relative temporal language such as "today" or "tomorrow".
	textureMetadataPromptUnixTS = "prompt_unix_ts"
)

// wireDetailKeyResolutionState is the work-item detail key the retired
// processor pipeline wrote; stored wire work items are still read by it.
const wireDetailKeyResolutionState = "resolution_state"
