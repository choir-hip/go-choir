package store

// After the vocabulary cutover, the object-graph write guard must accept
// every live desk role agentprofile registers. S2a added management_task to
// agentprofile but not to the V2 live set, and every management task on the
// paper was refused at "persist agent"
// (docs/archive/problems/s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md).
// Package tests never run with the cutover active, so nothing caught it.
//
// Failure modes this guards: a role registered in agentprofile but missing
// from the fence (the S2a miss); the guard inactive in the test (vacuous
// pass), checked by also refusing an unknown role.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestVocabWriteGuardAcceptsEveryRegisteredRoleAfterCutover(t *testing.T) {
	s := openTestStore(t)
	s.vocabCutover.Store(true)
	agent := func(role string) []objectgraph.Object {
		meta, _ := json.Marshal(map[string]string{"role": role})
		return []objectgraph.Object{{ObjectKind: objectgraph.ObjectKind("choir.agent"), Metadata: meta, Body: []byte(`{}`)}}
	}
	roles := []string{
		agentprofile.Management, agentprofile.Engineering, agentprofile.Research,
		agentprofile.Texture, agentprofile.Conductor, agentprofile.Email,
		agentprofile.ManagementTask, "verifier", "verifier_multimodal",
	}
	for _, role := range roles {
		if _, err := agentprofile.Canonical(role); err != nil {
			t.Fatalf("test lists %q, which agentprofile does not register: %v", role, err)
		}
		if err := s.vocabWriteGuard(context.Background(), agent(role), nil); err != nil {
			t.Errorf("write guard refused registered role %q after cutover: %v", role, err)
		}
	}
	if err := s.vocabWriteGuard(context.Background(), agent("boss"), nil); err == nil {
		t.Fatal("write guard accepted an unknown role: the guard is not active in this test")
	}
}
