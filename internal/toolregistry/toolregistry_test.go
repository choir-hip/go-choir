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
func TestMustNewToolRegistryPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNewToolRegistry did not panic for invalid tool")
		}
	}()
	MustNewToolRegistry(Tool{})
}
