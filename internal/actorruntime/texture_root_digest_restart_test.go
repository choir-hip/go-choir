package actorruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
)

// PD3 S1 failure mode 7: a crash leaves the root's digest to resume at boot.
// A texture_root_digest recorded before a markerless boot is
// interrupted_by_restart and reaches no handler, so it starts no root run
// (the handler here has no Texture owner bound: reaching it would fail
// differently). After a planned restart the same digest is delivered.
func TestPreBootTextureRootDigestIsInterruptedAfterCrash(t *testing.T) {
	if agentcore.TextureRootDigestKind != "texture_root_digest" {
		t.Fatalf("digest kind = %q", agentcore.TextureRootDigestKind)
	}
	if !agentcore.RestartWorkKind(agentcore.TextureRootDigestKind) {
		t.Fatal("texture_root_digest is not a restart work kind: a crash boot would deliver a pre-boot digest")
	}
	boot := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	mailbox := scopedActorMailboxID("owner-1", "computer-1", agentcore.TextureRootDigestMailbox)
	digest := actor.Update{UpdateID: "digest-1", ToAgentID: mailbox, Kind: agentcore.TextureRootDigestKind, Content: `{"v":1,"docs":["doc-b"]}`, CreatedAt: boot.Add(-time.Minute)}

	crash := &actorHandler{bootAt: boot}
	if _, err := crash.HandleUpdate(context.Background(), mailbox, digest, nil); !errors.Is(err, actor.ErrDurableInvalid) {
		t.Fatalf("pre-boot digest after a crash: err=%v, want durable-invalid interrupted_by_restart", err)
	}
	if crash.preBootWorkOccurrence(mailbox, actor.Update{ToAgentID: mailbox, Kind: agentcore.TextureRootDigestKind, CreatedAt: boot.Add(time.Second)}) {
		t.Fatal("a post-boot digest was treated as interrupted: the next commit's digest must run")
	}
	planned := &actorHandler{bootAt: boot, plannedBoot: true}
	if planned.preBootWorkOccurrence(mailbox, digest) {
		t.Fatal("a pre-boot digest was interrupted after a planned restart")
	}
}
