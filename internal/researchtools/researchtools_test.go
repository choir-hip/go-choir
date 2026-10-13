package researchtools

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestRegisterInstallsResearchToolsWithCanonicalSchemas(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{}); err != nil {
		t.Fatalf("register research tools: %v", err)
	}

	wantNames := []string{
		"read",
		"search_wire_corpus",
	}
	tools := registry.Tools()
	gotNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		gotNames = append(gotNames, tool.Name)
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("tool %q has empty description", tool.Name)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("registered tools = %v, want %v", gotNames, wantNames)
	}

	read, _ := registry.Lookup("read")
	if !reflect.DeepEqual(read.Parameters["required"], []string{"url"}) {
		t.Fatalf("read required = %#v", read.Parameters["required"])
	}

	if err := Register(registry, Dependencies{}); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate Register error = %v", err)
	}
}
