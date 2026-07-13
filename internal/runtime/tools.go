package runtime

import "github.com/yusefmosiah/go-choir/internal/toolregistry"

type Tool = toolregistry.Tool



// --- Schema helpers (delegated to internal/toolregistry) ---

// jsonSchemaObject creates a JSON Schema object with the given properties,
// required fields, and additionalProperties setting.
func jsonSchemaObject(properties map[string]any, required []string, additionalProperties bool) map[string]any {
	return toolregistry.JSONSchemaObject(properties, required, additionalProperties)
}

// cloneSchemaMap deep-clones a JSON Schema map.
func cloneSchemaMap(in map[string]any) map[string]any {
	return toolregistry.CloneSchemaMap(in)
}

// buildSystemPromptWithTools constructs the system prompt for the tool-calling
// loop by appending the tool catalog to the base system prompt. This gives
// the LLM visibility into available tools without requiring separate tool
// schema negotiation on each turn.
func buildSystemPromptWithTools(basePrompt string, registry *toolregistry.ToolRegistry) string {
	if registry == nil || registry.Size() == 0 {
		return basePrompt
	}
	return basePrompt + "\n\n" + registry.Catalog()
}

func toolProjectionResultJSON(modelOutput any, durableOutput any, metadata map[string]any) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return toolResultJSON(map[string]any{
		"__choir_tool_projection": true,
		"model_output":            modelOutput,
		"durable_output":          durableOutput,
		"projection":              metadata,
	})
}
