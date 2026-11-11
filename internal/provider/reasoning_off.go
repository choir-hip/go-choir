package provider

// ReasoningOffHonored reports whether an adapter turns reasoning off for a call
// that carries LLMRequest.ReasoningOff: River (reasoning_effort "none", which
// River documents as off) and Bedrock Haiku 5.5 (thinking disabled). Every other
// adapter ignores the flag and keeps its floor (owner 2026-11-09: thinking is
// never off; owner 2026-11-11: the autoradio role is the one exemption).
func ReasoningOffHonored(providerName, modelID string) bool {
	switch providerName {
	case "river":
		return true
	case "bedrock":
		return bedrockAdaptiveEffort(modelID)
	}
	return false
}

// CacheBreakHonored reports whether an adapter puts an explicit cache
// breakpoint on a Block with CacheBreak: Bedrock does; River and the others
// cache by unchanged prefix, or not at all, and ignore the mark.
func CacheBreakHonored(providerName string) bool { return providerName == "bedrock" }
