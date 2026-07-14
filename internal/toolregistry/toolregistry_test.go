package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func noopToolFunc(context.Context, json.RawMessage) (string, error) {
	return "", nil
}

func TestToolRegistryRegister(t *testing.T) {
	registry := NewToolRegistry()
	tool := Tool{
		Name:        "read_file",
		Description: "Read a file from the sandbox filesystem",
		Func: func(context.Context, json.RawMessage) (string, error) {
			return "file contents", nil
		},
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}
	if got := registry.Size(); got != 1 {
		t.Fatalf("Size() = %d, want 1", got)
	}
}

func TestToolRegistryRegisterDuplicate(t *testing.T) {
	registry := NewToolRegistry()
	tool := Tool{Name: "duplicate", Func: noopToolFunc}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registry.Register(tool); err == nil {
		t.Fatal("duplicate registration succeeded")
	}
	if got := registry.Size(); got != 1 {
		t.Fatalf("Size() after duplicate = %d, want 1", got)
	}
}

func TestToolRegistryRegisterValidateNoName(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Func: noopToolFunc}); err == nil {
		t.Fatal("registered tool with no name")
	}
	if got := registry.Size(); got != 0 {
		t.Fatalf("Size() after invalid registration = %d, want 0", got)
	}
}

func TestToolRegistryRegisterValidateNilFunc(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "nil_func"}); err == nil {
		t.Fatal("registered tool with nil func")
	}
	if got := registry.Size(); got != 0 {
		t.Fatalf("Size() after invalid registration = %d, want 0", got)
	}
}

func TestToolRegistryExecute(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "echo",
		Func: func(_ context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := registry.Execute(context.Background(), "echo", json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `{"message":"hello"}`; got != want {
		t.Fatalf("Execute output = %q, want %q", got, want)
	}
}

func TestToolRegistryExecuteNotFound(t *testing.T) {
	if _, err := NewToolRegistry().Execute(context.Background(), "nonexistent", nil); err == nil {
		t.Fatal("Execute succeeded for nonexistent tool")
	}
}

func TestToolRegistryExecuteError(t *testing.T) {
	registry := MustNewToolRegistry(Tool{
		Name: "fail",
		Func: func(context.Context, json.RawMessage) (string, error) {
			return "", errors.New("tool failure")
		},
	})

	if _, err := registry.Execute(context.Background(), "fail", nil); err == nil {
		t.Fatal("Execute discarded tool error")
	}
}

func TestToolRegistryLookup(t *testing.T) {
	registry := MustNewToolRegistry(Tool{Name: "findme", Func: noopToolFunc})

	found, ok := registry.Lookup("findme")
	if !ok {
		t.Fatal("Lookup did not find registered tool")
	}
	if found.Name != "findme" {
		t.Fatalf("Lookup name = %q, want findme", found.Name)
	}
	if _, ok := registry.Lookup("nonexistent"); ok {
		t.Fatal("Lookup found nonexistent tool")
	}
}

func TestToolRegistryCatalog(t *testing.T) {
	registry := MustNewToolRegistry(
		Tool{Name: "read_file", Description: "Read a file", Func: noopToolFunc},
		Tool{Name: "list_files", Description: "List files", Func: noopToolFunc},
	)

	want := "Available tools:\n- list_files — List files\n- read_file — Read a file\n"
	if got := registry.Catalog(); got != want {
		t.Fatalf("Catalog() = %q, want %q", got, want)
	}
}

func TestToolRegistryDefinitions(t *testing.T) {
	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
	registry := MustNewToolRegistry(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters:  parameters,
		Func:        noopToolFunc,
	})

	definitions := registry.Definitions()
	if len(definitions) != 1 {
		t.Fatalf("Definitions() length = %d, want 1", len(definitions))
	}
	definition := definitions[0]
	if definition.Name != "test_tool" || definition.Description != "A test tool" {
		t.Fatalf("Definitions()[0] = %#v", definition)
	}
	definition.Parameters["type"] = "changed"
	definition.Parameters["properties"].(map[string]any)["path"].(map[string]any)["type"] = "number"
	if parameters["type"] != "object" || parameters["properties"].(map[string]any)["path"].(map[string]any)["type"] != "string" {
		t.Fatalf("Definitions() exposed mutable registry schema: %#v", parameters)
	}
}

func TestToolRegistryDefaultParameters(t *testing.T) {
	registry := MustNewToolRegistry(Tool{Name: "no_params", Func: noopToolFunc})
	found, ok := registry.Lookup("no_params")
	if !ok {
		t.Fatal("Lookup did not find registered tool")
	}
	want := `{"additionalProperties":false,"properties":{},"type":"object"}`
	got, err := ResultJSON(found.Parameters)
	if err != nil {
		t.Fatalf("encode default parameters: %v", err)
	}
	if got != want {
		t.Fatalf("default parameters = %s, want %s", got, want)
	}
}

func TestNewToolRegistryWithTools(t *testing.T) {
	registry, err := NewToolRegistryWithTools(
		Tool{Name: "tool_a", Func: noopToolFunc},
		Tool{Name: "tool_b", Func: noopToolFunc},
	)
	if err != nil {
		t.Fatalf("NewToolRegistryWithTools: %v", err)
	}
	if got := registry.Size(); got != 2 {
		t.Fatalf("Size() = %d, want 2", got)
	}
}

func TestMustNewToolRegistryPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNewToolRegistry did not panic for invalid tool")
		}
	}()
	MustNewToolRegistry(Tool{})
}

func TestToolRegistryToolsSorted(t *testing.T) {
	registry := NewToolRegistry()
	for _, name := range []string{"z_tool", "a_tool", "m_tool"} {
		if err := registry.Register(Tool{Name: name, Func: noopToolFunc}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	tools := registry.Tools()
	if len(tools) != 3 {
		t.Fatalf("Tools() length = %d, want 3", len(tools))
	}
	if tools[0].Name != "a_tool" || tools[1].Name != "m_tool" || tools[2].Name != "z_tool" {
		t.Fatalf("Tools() order = %q, %q, %q", tools[0].Name, tools[1].Name, tools[2].Name)
	}
}

func TestToolValidate(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr bool
	}{
		{name: "valid tool", tool: Tool{Name: "valid", Func: noopToolFunc}},
		{name: "empty name", tool: Tool{Func: noopToolFunc}, wantErr: true},
		{name: "nil func", tool: Tool{Name: "nil_func"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.tool.Validate(); (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, want error %v", err, test.wantErr)
			}
		})
	}
}

func TestToolDefinition(t *testing.T) {
	parameters := map[string]any{"type": "object"}
	definition := (Tool{
		Name:        "test",
		Description: "desc",
		Parameters:  parameters,
		Func:        noopToolFunc,
	}).Definition()

	if definition.Name != "test" || definition.Description != "desc" || definition.Parameters["type"] != "object" {
		t.Fatalf("Definition() = %#v", definition)
	}
	definition.Parameters["type"] = "changed"
	if parameters["type"] != "object" {
		t.Fatalf("Definition() did not clone parameters: %#v", parameters)
	}
}

func TestResultJSON(t *testing.T) {
	t.Run("JSON exact bytes", func(t *testing.T) {
		got, err := ResultJSON(map[string]any{"status": "ok", "count": 2})
		if err != nil {
			t.Fatalf("ResultJSON: %v", err)
		}
		if want := `{"count":2,"status":"ok"}`; got != want {
			t.Fatalf("ResultJSON = %s, want %s", got, want)
		}
	})

	t.Run("HTML is JSON escaped", func(t *testing.T) {
		got, err := ResultJSON("<html>&</html>")
		if err != nil {
			t.Fatalf("ResultJSON: %v", err)
		}
		if want := `"\u003chtml\u003e\u0026\u003c/html\u003e"`; got != want {
			t.Fatalf("ResultJSON = %s, want %s", got, want)
		}
	})

	t.Run("nil", func(t *testing.T) {
		got, err := ResultJSON(nil)
		if err != nil {
			t.Fatalf("ResultJSON: %v", err)
		}
		if got != "null" {
			t.Fatalf("ResultJSON(nil) = %q, want null", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		if got, err := ResultJSON(make(chan int)); err == nil || got != "" {
			t.Fatalf("ResultJSON(unsupported) = %q, %v; want empty output and error", got, err)
		}
	})
}

func TestProjectionResultJSON(t *testing.T) {
	t.Run("exact bytes", func(t *testing.T) {
		got, err := ProjectionResultJSON(
			map[string]any{"summary": "compact"},
			map[string]any{"raw": "full"},
			map[string]any{"type": "test_projection"},
		)
		if err != nil {
			t.Fatalf("ProjectionResultJSON: %v", err)
		}
		want := `{"__choir_tool_projection":true,"durable_output":{"raw":"full"},"model_output":{"summary":"compact"},"projection":{"type":"test_projection"}}`
		if got != want {
			t.Fatalf("ProjectionResultJSON = %s, want %s", got, want)
		}
	})

	t.Run("nil metadata", func(t *testing.T) {
		got, err := ProjectionResultJSON("compact", "full", nil)
		if err != nil {
			t.Fatalf("ProjectionResultJSON: %v", err)
		}
		want := `{"__choir_tool_projection":true,"durable_output":"full","model_output":"compact","projection":{}}`
		if got != want {
			t.Fatalf("ProjectionResultJSON = %s, want %s", got, want)
		}
	})

	t.Run("error", func(t *testing.T) {
		if got, err := ProjectionResultJSON(make(chan int), nil, nil); err == nil || got != "" {
			t.Fatalf("ProjectionResultJSON(unsupported) = %q, %v; want empty output and error", got, err)
		}
	})
}

func TestJSONSchemaObject(t *testing.T) {
	properties := map[string]any{"path": map[string]any{"type": "string"}}
	schema := JSONSchemaObject(properties, []string{"path"}, false)
	got, err := ResultJSON(schema)
	if err != nil {
		t.Fatalf("encode schema: %v", err)
	}
	want := `{"additionalProperties":false,"properties":{"path":{"type":"string"}},"required":["path"],"type":"object"}`
	if got != want {
		t.Fatalf("JSONSchemaObject = %s, want %s", got, want)
	}

	empty := JSONSchemaObject(nil, nil, true)
	got, err = ResultJSON(empty)
	if err != nil {
		t.Fatalf("encode empty schema: %v", err)
	}
	want = `{"additionalProperties":true,"properties":{},"type":"object"}`
	if got != want {
		t.Fatalf("JSONSchemaObject(nil) = %s, want %s", got, want)
	}
}

func TestCloneSchemaMap(t *testing.T) {
	properties := map[string]any{
		"path": map[string]any{"type": "string"},
		"tags": []any{map[string]any{"type": "string"}},
	}
	schema := JSONSchemaObject(properties, []string{"path"}, false)
	clone := CloneSchemaMap(schema)

	clone["type"] = "changed"
	clone["properties"].(map[string]any)["path"].(map[string]any)["type"] = "number"
	clone["properties"].(map[string]any)["tags"].([]any)[0].(map[string]any)["type"] = "number"
	if schema["type"] != "object" || properties["path"].(map[string]any)["type"] != "string" || properties["tags"].([]any)[0].(map[string]any)["type"] != "string" {
		t.Fatalf("CloneSchemaMap mutated its input: %#v", schema)
	}
	if CloneSchemaMap(nil) != nil {
		t.Fatal("CloneSchemaMap(nil) did not return nil")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	registry := MustNewToolRegistry(Tool{
		Name:        "read_file",
		Description: "Read one file.",
		Func:        noopToolFunc,
	})
	if got, want := BuildSystemPrompt("Base prompt.", registry), "Base prompt.\n\nAvailable tools:\n- read_file — Read one file.\n"; got != want {
		t.Fatalf("BuildSystemPrompt = %q, want %q", got, want)
	}
	if got := BuildSystemPrompt("Base prompt.", nil); got != "Base prompt." {
		t.Fatalf("BuildSystemPrompt with nil registry = %q", got)
	}
	if got := BuildSystemPrompt("Base prompt.", NewToolRegistry()); got != "Base prompt." {
		t.Fatalf("BuildSystemPrompt with empty registry = %q", got)
	}
}
