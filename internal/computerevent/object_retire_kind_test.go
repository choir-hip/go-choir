package computerevent

import (
	"strings"
	"testing"
)

// A Texture revision named by a choir.lifecycle_command (stored_result
// revision_id) is rebuilt by id on replay, so no retirement may remove one.
// The guard is the one-kind rule of ObjectRetireProjection.Validate.
func TestObjectRetireRefusesTextureRevisionsAndLifecycleCommands(t *testing.T) {
	for _, kind := range []string{"choir.texture_revision", "choir.lifecycle_command", "choir.texture_document"} {
		p := ObjectRetireProjection{
			ObjectKind: kind, EventKinds: []string{"x"},
			UpdatedFrom: "2026-11-01T00:00:00Z", UpdatedBefore: "2026-11-02T00:00:00Z",
		}
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), kind) {
			t.Fatalf("retire of %s validated: %v", kind, err)
		}
	}
}
