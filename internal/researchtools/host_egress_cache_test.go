package researchtools

import (
	"context"
	"testing"
)

// docs/problems/research-fetch-url-has-no-address-guard-2026-10-10.md
// (adjacent finding): the egress tool table was cached under the address of
// a value receiver, so every call missed and added an entry. Failure mode
// pinned: repeated calls on one deps set grow the cache.
func TestHostEgressToolTableIsBuiltOncePerDependencies(t *testing.T) {
	deps := &Dependencies{}
	size := func() int {
		egressTableMu.Lock()
		defer egressTableMu.Unlock()
		return len(egressTables)
	}
	before := size()
	for i := 0; i < 5; i++ {
		_, _ = deps.HostEgress(context.Background(), "no_such_action", nil)
	}
	if grown := size() - before; grown != 1 {
		t.Fatalf("five calls on one deps set grew the cache by %d entries, want 1", grown)
	}
}
