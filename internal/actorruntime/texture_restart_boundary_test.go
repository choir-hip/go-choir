package actorruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
)

// Owner rule (AGENTS.md "Restarts End Work (Crash) Or Resume It"): a crash
// restart never resumes work, for any desk; a planned update restart may.
// Failure modes pinned: a pre-boot work occurrence of any desk runs after a
// crash boot; a planned boot interrupts work it should resume; the boundary
// swallows post-boot work, a pre-boot cancel, or a fail-closed deadline; a
// handler without a boot instant interrupts.
func TestPreBootWorkOccurrenceIsInterruptedAfterCrashForEveryDesk(t *testing.T) {
	h := &actorHandler{bootAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	before := h.bootAt.Add(-time.Minute)
	for _, agent := range []string{"texture:doc-1", "management:root", "engineering:root", "research:worker-1"} {
		mailbox := scopedActorMailboxID("owner-1", "computer-1", agent)
		for _, kind := range []string{"initial_dispatch", "coagent_result", "channel_message", "owner_revision", "lifecycle_work_assigned", "delegated_assignment_spawn_deadline", "fresh_mint_management_resume_deadline"} {
			_, err := h.HandleUpdate(context.Background(), mailbox, actor.Update{UpdateID: "u-" + kind, ToAgentID: mailbox, Kind: kind, CreatedAt: before}, nil)
			if !errors.Is(err, actor.ErrDurableInvalid) {
				t.Fatalf("pre-boot %s %s after crash: err=%v, want durable-invalid interrupted_by_restart", agent, kind, err)
			}
		}
		if h.preBootWorkOccurrence(mailbox, actor.Update{ToAgentID: mailbox, Kind: "coagent_result", CreatedAt: h.bootAt.Add(time.Second)}) {
			t.Fatalf("post-boot %s occurrence treated as interrupted", agent)
		}
		for _, kind := range []string{"cancel", "lifecycle_cancellation", "activation_budget_deadline", "cell_terminal_deadline", "assigned_engineering_fate_deadline", "engineering_progress_overdue_deadline", "reactivated_management_resume_deadline", "selfdev_materialization_retry"} {
			if h.preBootWorkOccurrence(mailbox, actor.Update{ToAgentID: mailbox, Kind: kind, CreatedAt: before}) {
				t.Fatalf("pre-boot %s %s treated as interrupted; closing work must still apply", agent, kind)
			}
		}
	}
	texture := scopedActorMailboxID("owner-1", "computer-1", "texture:doc-1")
	if (&actorHandler{}).preBootWorkOccurrence(texture, actor.Update{ToAgentID: texture, Kind: "coagent_result", CreatedAt: before}) {
		t.Fatal("handler without a boot instant must not interrupt")
	}
}

func TestPreBootWorkOccurrenceResumesAfterPlannedRestart(t *testing.T) {
	h := &actorHandler{bootAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), plannedBoot: true}
	before := h.bootAt.Add(-time.Minute)
	for _, agent := range []string{"texture:doc-1", "management:root", "engineering:root"} {
		mailbox := scopedActorMailboxID("owner-1", "computer-1", agent)
		if h.preBootWorkOccurrence(mailbox, actor.Update{ToAgentID: mailbox, Kind: "coagent_result", CreatedAt: before}) {
			t.Fatalf("pre-boot %s occurrence interrupted after a planned restart", agent)
		}
	}
}
