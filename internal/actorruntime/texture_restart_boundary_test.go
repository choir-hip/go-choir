package actorruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
)

// A guest restart is a system failure, not a wake (owner rule,
// docs/problems/texture-zombie-activations-revising-forever-2026-10-09.md).
// Failure modes pinned: a Texture occurrence recorded before this boot runs
// (or defers and retries) after the restart; the boundary swallows
// post-boot Texture work, a pre-boot cancel, or another desk's occurrence.
func TestPreBootTextureOccurrenceIsInterruptedNotRun(t *testing.T) {
	h := &actorHandler{bootAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	texture := scopedActorMailboxID("owner-1", "computer-1", "texture:doc-1")
	before := h.bootAt.Add(-time.Minute)
	for _, kind := range []string{"initial_dispatch", "coagent_result", "channel_message", "owner_revision", "lifecycle_work_assigned"} {
		_, err := h.HandleUpdate(context.Background(), texture, actor.Update{UpdateID: "u-" + kind, ToAgentID: texture, Kind: kind, CreatedAt: before}, nil)
		if !errors.Is(err, actor.ErrDurableInvalid) {
			t.Fatalf("pre-boot Texture %s: err=%v, want durable-invalid interrupted_by_restart", kind, err)
		}
	}
	if h.preBootTextureOccurrence(texture, actor.Update{ToAgentID: texture, Kind: "coagent_result", CreatedAt: h.bootAt.Add(time.Second)}) {
		t.Fatal("post-boot Texture occurrence treated as interrupted")
	}
	if h.preBootTextureOccurrence(texture, actor.Update{ToAgentID: texture, Kind: "cancel", CreatedAt: before}) {
		t.Fatal("pre-boot cancel treated as interrupted; cancellation must still apply")
	}
	research := scopedActorMailboxID("owner-1", "computer-1", "research:worker-1")
	if h.preBootTextureOccurrence(research, actor.Update{ToAgentID: research, Kind: "coagent_result", CreatedAt: before}) {
		t.Fatal("non-Texture occurrence treated as interrupted")
	}
	if (&actorHandler{}).preBootTextureOccurrence(texture, actor.Update{ToAgentID: texture, Kind: "coagent_result", CreatedAt: before}) {
		t.Fatal("handler without a boot instant must not interrupt")
	}
}
