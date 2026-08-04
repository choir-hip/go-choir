package agentcore

import (
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func TestSupervisionAttemptShapeSecondAttemptIsCanonicalRetry(t *testing.T) {
	kind, ordinal, prior := supervisionAttemptShape([]store.SupervisionAttemptLineage{{
		AttemptID: "attempt-1", AttemptKind: "initial", Ordinal: 1, Status: "returned",
	}}, "attempt-2")
	if kind != "retry" || ordinal != 2 || prior == nil || *prior != "attempt-1" {
		t.Fatalf("second attempt = (%q, %d, %v)", kind, ordinal, prior)
	}
}

func TestSupervisionAttemptShapeUsesUnboundedLineage(t *testing.T) {
	attempts := make([]store.SupervisionAttemptLineage, 9)
	for i := range attempts {
		attempts[i] = store.SupervisionAttemptLineage{
			AttemptID: fmt.Sprintf("attempt-%d", i+1), AttemptKind: "retry", Ordinal: uint64(i + 1),
		}
	}
	attempts[0].AttemptKind = "initial"
	kind, ordinal, prior := supervisionAttemptShape(attempts, "attempt-10")
	if kind != "retry" || ordinal != 10 || prior == nil || *prior != "attempt-9" {
		t.Fatalf("unbounded retry = (%q, %d, %v)", kind, ordinal, prior)
	}
}

func TestStoredSupervisionObservedBasePreservesAttemptBase(t *testing.T) {
	fallback := computerevent.SupervisionObservedBase{CanonicalEventHead: "current", IntentRevisionID: "intent-current", ArtifactHeadRevisionID: "artifact-current"}
	base := storedSupervisionObservedBase(map[string]any{runMetadataObservedBase: map[string]string{"canonical_event_head": "start", "intent_revision_id": "intent-start", "artifact_head_revision_id": "artifact-start"}}, fallback)
	if base.CanonicalEventHead != "start" || base.IntentRevisionID != "intent-start" || base.ArtifactHeadRevisionID != "artifact-start" {
		t.Fatalf("observed base = %#v", base)
	}
}
