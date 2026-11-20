package actorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
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
	digest := actor.Update{UpdateID: "digest-1", ToAgentID: mailbox, Kind: agentcore.TextureRootDigestKind, Content: `{"v":1,"due":"2026-10-25T11:00:00Z"}`, CreatedAt: boot.Add(-time.Minute)}

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

// Review 247 item 3: ledger changes the actor runtime reconciles by
// trajectory note the document for the Texture root, after their commit
// (the wake is minted post-commit): a work item assigned, a cancellation,
// and a delegated engineering spawn on the document's trajectory.
func TestLedgerChangeWakesNoteTheTextureRoot(t *testing.T) {
	for _, tc := range []struct {
		kind    string
		mailbox func(f *recordsWakeFixture) string
		content func(f *recordsWakeFixture) string
	}{
		{"lifecycle_work_assigned", func(f *recordsWakeFixture) string { return f.agentID }, func(f *recordsWakeFixture) string {
			return `{"work_item_id":"work:` + f.id + `","trajectory_id":"` + f.trajectoryID + `"}`
		}},
		{"lifecycle_cancellation", func(f *recordsWakeFixture) string { return f.agentID }, func(f *recordsWakeFixture) string { return f.trajectoryID }},
		{"delegated_assignment_spawn_deadline", func(*recordsWakeFixture) string { return agentcore.EngineeringAdmissionMailbox }, func(*recordsWakeFixture) string {
			return `{"assignment_id":"assignment-none","attempt":1,"objective":"x"}`
		}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			f := newRecordsWakeFixture(t, "rootnote", false, false)
			var digests []string
			f.env.adapter.Runtime.SetScheduleActor(func(_ context.Context, _, _, to, kind, content, _, _ string, _ time.Time) error {
				if kind == agentcore.TextureRootDigestKind {
					digests = append(digests, content)
				}
				return nil
			})
			handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
			mailbox := tc.mailbox(f)
			_, _ = handler.HandleUpdate(f.env.ctx, mailbox, actor.Update{
				UpdateID: tc.kind + "-1", ToAgentID: scopedActorMailboxID(f.ownerID, f.computerID, mailbox),
				Kind: tc.kind, Content: tc.content(f), TrajectoryID: f.trajectoryID, CreatedAt: time.Now().UTC().Add(time.Second),
			}, nil)
			assertRootDigestNotes(t, f, digests, 1, tc.kind)
		})
	}
}

// Review 248 (LOW): an assigned engineering fate deadline that ran changes
// its document's ledger (it may fail an assignment closed), so it notes the
// document for the root after it commits; a deadline the live saga holds
// (deferred, nothing committed) notes nothing.
func TestFateDeadlineNotesTheTextureRootAfterItCommits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		digests int
	}{{"ran", nil, 1}, {"held", agentcore.ErrAssignmentSagaHeld, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecordsWakeFixture(t, "fatenote", false, false)
			var digests []string
			f.env.adapter.Runtime.SetScheduleActor(func(_ context.Context, _, _, _, kind, content, _, _ string, _ time.Time) error {
				if kind == agentcore.TextureRootDigestKind {
					digests = append(digests, content)
				}
				return nil
			})
			handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
			handler.fateDeadline = func(context.Context, string, string, string, string) error { return tc.err }
			mailbox := "management"
			_, _ = handler.HandleUpdate(f.env.ctx, mailbox, actor.Update{
				UpdateID: "fate-1", ToAgentID: scopedActorMailboxID(f.ownerID, f.computerID, mailbox),
				Kind: "assigned_engineering_fate_deadline", Content: `{"assignment_id":"assignment-none","attempt":1}`,
				TrajectoryID: f.trajectoryID, CreatedAt: time.Now().UTC().Add(time.Second),
			}, nil)
			assertRootDigestNotes(t, f, digests, tc.digests, "fate deadline "+tc.name)
		})
	}
}

// A digest carries a version and its due time and names no documents (what
// is pending is read from the store); the note arms exactly one digest, and a
// handler that noted nothing arms none.
func assertRootDigestNotes(t *testing.T, f *recordsWakeFixture, digests []string, want int, what string) {
	t.Helper()
	if len(digests) != want {
		t.Fatalf("%s on %s armed %d digests %v, want %d", what, f.id, len(digests), digests, want)
	}
	if want == 0 {
		return
	}
	var digest struct {
		V    int      `json:"v"`
		Due  string   `json:"due"`
		Docs []string `json:"docs"`
	}
	if err := json.Unmarshal([]byte(digests[0]), &digest); err != nil || digest.V != 1 || digest.Due == "" || len(digest.Docs) != 0 || strings.Contains(digests[0], f.id) {
		t.Fatalf("%s armed digest %s (%v), want {v:1, due} naming no document", what, digests[0], err)
	}
}
