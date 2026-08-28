package textureowner

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
)

// TestStartDefersTextureReconcileUnderMaintenanceHold guards the boot crash-loop
// fix: a computer booted under RUNTIME_MAINTENANCE_HOLD=1 must treat the hold as
// a benign, mutation-fenced state and defer the Texture reconcile (which would
// otherwise attempt run admission and be refused), not return a fatal error to
// the caller that log.Fatalf's the autoputer process. Before the fix this path
// reached the nil store and panicked; after the fix it returns nil immediately.
func TestStartDefersTextureReconcileUnderMaintenanceHold(t *testing.T) {
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	rt := &Handler{Core: &agentcore.Runtime{}}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("held Start must defer the reconcile and return nil, got %v", err)
	}
}
