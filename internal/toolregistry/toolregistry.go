// Package toolregistry manages the set of available tools for the runtime
// tool-calling loop. Tools are registered once at startup and looked up by
// name during the tool-calling loop when the LLM returns tool_use stop
// reasons.
//
// Extracted from internal/runtime so that the actorruntime adapter and other
// packages can use the tool registry without importing the old runtime.
package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// ToolFunc is the execution contract for in-process tools. Tools are Go
// function calls, not CLI subprocesses (mission constraint: no CLI loop).
// The function receives the raw JSON arguments from the provider and returns
// a text result or an error.
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)

// Tool describes a callable tool plus its LLM-facing schema metadata.
type Tool struct {
	// Name is the unique tool identifier used in LLM tool_use responses.
	Name string `json:"name"`

	// Description is a human-readable summary of what the tool does,
	// included in the system prompt for LLM tool discovery.
	Description string `json:"description,omitempty"`

	// Parameters is the JSON Schema object describing the tool's input
	// parameters. If nil, defaults to an empty object schema.
	Parameters map[string]any `json:"parameters,omitempty"`

	// Func is the Go function that executes the tool. Must be non-nil.
	Func ToolFunc `json:"-"`
}

// Validate checks that the tool has a name and a non-nil function.
func (t Tool) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	if t.Func == nil {
		return fmt.Errorf("tool %q has nil func", t.Name)
	}
	return nil
}

// Definition returns the LLM-facing definition for this tool.
func (t Tool) Definition() provideriface.ToolDefinition {
	return provideriface.ToolDefinition{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  CloneSchemaMap(t.Parameters),
	}
}

// ToolRegistry manages the set of available tools for the runtime loop.
// Tools are registered once at startup and looked up by name during the
// tool-calling loop when the LLM returns tool_use stop reasons.
//
// Thread-safe for concurrent lookup during parallel tool execution.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // sorted names for deterministic catalog output
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// NewToolRegistryWithTools creates a tool registry with the given tools
// pre-registered. Returns an error if any tool fails validation.
func NewToolRegistryWithTools(tools ...Tool) (*ToolRegistry, error) {
	r := NewToolRegistry()
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// MustNewToolRegistry creates a tool registry with the given tools or panics.
func MustNewToolRegistry(tools ...Tool) *ToolRegistry {
	r, err := NewToolRegistryWithTools(tools...)
	if err != nil {
		panic(err)
	}
	return r
}

// Register adds a tool to the registry. Returns an error if the tool fails
// validation or a tool with the same name is already registered.
func (r *ToolRegistry) Register(tool Tool) error {
	if err := tool.Validate(); err != nil {
		return err
	}

	// Default to empty object schema if no parameters specified.
	if len(tool.Parameters) == 0 {
		tool.Parameters = JSONSchemaObject(nil, nil, false)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	r.order = append(r.order, tool.Name)
	sort.Strings(r.order)
	return nil
}

// Lookup returns the tool with the given name, or false if not found.
func (r *ToolRegistry) Lookup(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// Execute runs the named tool with the given arguments. Returns an error
// if the tool is not found or if execution fails.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}
	return tool.Func(ctx, args)
}

// Tools returns all registered tools in sorted order.
func (r *ToolRegistry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// Definitions returns the LLM-facing definitions for all registered tools.
func (r *ToolRegistry) Definitions() []provideriface.ToolDefinition {
	tools := r.Tools()
	out := make([]provideriface.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool.Definition())
	}
	return out
}

// Catalog returns a compact one-line-per-tool description suitable for
// inclusion in the system prompt.
func (r *ToolRegistry) Catalog() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString("Available tools:\n")
	for _, name := range r.order {
		tool := r.tools[name]
		desc := tool.Description
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}
		fmt.Fprintf(&b, "- %s — %s\n", name, desc)
	}
	return b.String()
}

// Size returns the number of registered tools.
func (r *ToolRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// --- Schema helpers ---

// JSONSchemaObject creates a JSON Schema object with the given properties,
// required fields, and additionalProperties setting.
func JSONSchemaObject(properties map[string]any, required []string, additionalProperties bool) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": additionalProperties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// CloneSchemaMap deep-clones a JSON Schema map.
func CloneSchemaMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneSchemaValue(v)
	}
	return out
}

func cloneSchemaValue(v any) any {
	switch vv := v.(type) {
	case map[string]any:
		return CloneSchemaMap(vv)
	case []any:
		out := make([]any, len(vv))
		for i, item := range vv {
			out[i] = cloneSchemaValue(item)
		}
		return out
	default:
		return v
	}
}
