package agentcore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Problem 22 (docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md):
// pool's rebase candidate froze, reached awaiting_approval, and vmctl's
// deferred-refresh sweep restarted pool onto a newer deploy 31 seconds later.
// The owner's approval then failed on booted_base_moved, and the standing
// consensus attempt (an in-memory goroutine with no resumer) died with the
// guest. A candidate that waits for its decision now holds a planned refresh,
// bounded so a forgotten candidate cannot block updates for ever.
//
// Ways this could fail, written first:
//   - a frozen, verified or awaiting_approval candidate does not hold the
//     refresh, so a deploy throws away a finished candidate (the incident);
//   - allow_busy (critical_protected, public_platform) overrides the hold, so
//     an always-on computer still loses its candidate;
//   - the refusal does not say why, so vmctl's pending list shows a bare
//     "busy" and the operator cannot tell a held candidate from running work;
//   - a candidate older than the cap still holds, so a forgotten candidate
//     blocks every update for ever;
//   - a candidate whose updated_at keeps moving (intent-error stamps) holds
//     for ever, so the cap must also bound the operation's whole life;
//   - an accepted (decided, not yet applied) operation stops holding at the
//     cap, so a refresh lands mid-apply;
//   - the hold leaks into the restore restart or the snapshot busy reads,
//     which are owner acts and storage bounds, not deploys.

func seedDecisionHoldOperation(t *testing.T, s *store.Store, computerID, id, state string, created, updated time.Time) {
	t.Helper()
	if _, err := s.DB().ExecContext(context.Background(), `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		id, computerID, "key-"+id, strings.Repeat("d", 64), "trajectory-"+id, strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), state, created, updated); err != nil {
		t.Fatal(err)
	}
}

func decisionHoldRuntime(t *testing.T) (*Runtime, *store.Store) {
	t.Helper()
	rt, s := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	rt.cfg.ComputerID = "computer-decision-hold"
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	return rt, s
}

func assertNoPlannedRestartMarker(t *testing.T, rt *Runtime) {
	t.Helper()
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err == nil {
		t.Fatal("refusal wrote a marker")
	} else if !os.IsNotExist(err) {
		t.Fatalf("marker unreadable: %v", err)
	}
}

func TestPlannedRestartHeldWhileCandidateAwaitsDecision(t *testing.T) {
	for _, state := range []string{selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval} {
		for _, url := range []string{"/internal/runtime/planned-restart", "/internal/runtime/planned-restart?allow_busy=1"} {
			t.Run(state+" "+url, func(t *testing.T) {
				rt, s := decisionHoldRuntime(t)
				now := time.Now().UTC()
				seedDecisionHoldOperation(t, s, rt.cfg.ComputerID, "selfdev-held-"+state, state, now.Add(-20*time.Minute), now.Add(-time.Minute))
				code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, url, `{"target":"761d94c5"}`)
				if code != 409 {
					t.Fatalf("candidate %s: status=%d body=%s, want 409", state, code, body)
				}
				for _, want := range []string{`"outcome":"busy"`, `"reason":"selfdev_awaiting_decision"`, `"selfdev_awaiting_decision":1`, "selfdev-held-" + state} {
					if !strings.Contains(body, want) {
						t.Fatalf("candidate %s: body=%s, want it to name %s", state, body, want)
					}
				}
				assertNoPlannedRestartMarker(t, rt)
			})
		}
	}
}

func TestPlannedRestartNotHeldByCandidatePastTheCap(t *testing.T) {
	now := time.Now().UTC()
	cases := map[string]struct{ created, updated time.Time }{
		"awaiting longer than the cap":      {now.Add(-3 * time.Hour), now.Add(-selfdevDecisionHoldCap - time.Minute)},
		"updated lately but older than cap": {now.Add(-selfdevDecisionHoldLifetime - time.Minute), now.Add(-time.Minute)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rt, s := decisionHoldRuntime(t)
			seedDecisionHoldOperation(t, s, rt.cfg.ComputerID, "selfdev-stale", selfdev.StateAwaitingApproval, c.created, c.updated)
			code, body := postPlannedRestart(t, &APIHandler{rt: rt}, "192.0.2.1:4000", `{"target":"761d94c5"}`)
			if code != 200 {
				t.Fatalf("stale candidate: status=%d body=%s, want 200 (a forgotten candidate must not block updates)", code, body)
			}
		})
	}
}

func TestPlannedRestartHeldByAcceptedOperationPastTheCap(t *testing.T) {
	rt, s := decisionHoldRuntime(t)
	now := time.Now().UTC()
	old := now.Add(-selfdevDecisionHoldLifetime - time.Hour)
	seedDecisionHoldOperation(t, s, rt.cfg.ComputerID, "selfdev-accepted", selfdev.StateAccepted, old, old)
	code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	if code != 409 || !strings.Contains(body, `"selfdev_active_operations":1`) {
		t.Fatalf("accepted, not applied: status=%d body=%s, want 409 naming selfdev_active_operations", code, body)
	}
	assertNoPlannedRestartMarker(t, rt)

	// Applied: the hold ends and the refresh goes ahead.
	if _, err := s.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET state=? WHERE operation_id=?`, selfdev.StateApplied, "selfdev-accepted"); err != nil {
		t.Fatal(err)
	}
	code, body = postPlannedRestart(t, &APIHandler{rt: rt}, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != 200 {
		t.Fatalf("applied: status=%d body=%s, want 200", code, body)
	}
}

func TestDecisionHoldDoesNotLeakIntoRestoreOrSnapshotBusy(t *testing.T) {
	rt, s := decisionHoldRuntime(t)
	now := time.Now().UTC()
	seedDecisionHoldOperation(t, s, rt.cfg.ComputerID, "selfdev-awaiting", selfdev.StateAwaitingApproval, now, now)
	work, err := rt.plannedRestartBusy(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if work.unsafe() || work.SelfdevActive != 0 {
		t.Fatalf("an awaiting candidate made the work unsafe (%+v): the restore restart and snapshot reads must not wait on it", work)
	}
	if work.SelfdevAwaitingDecision != 1 {
		t.Fatalf("work = %+v, want one candidate awaiting its decision", work)
	}
}
