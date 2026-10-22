package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// SL slice 3: "what is owed and why is it not moving" without SSH.
// Failure modes pinned: an exhausted wake is invisible; pending wakes are
// not counted; the surface answers without an authenticated owner.
func TestObligationsSurfaceReportsPendingAndExhaustedWakes(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	exhaustTarget := queueOneActorWake(t, s, "surface-exhaust")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	rt.wakeRetries.now = func() time.Time { return now }
	rt.SetDispatchActor(func(_ context.Context, _, _, to, _, _, _, _ string) error {
		if to == exhaustTarget {
			return errors.New("dispatch refused")
		}
		return errors.New("hold pending")
	})
	for i := 0; i < wakeDispatchMaxAttempts; i++ {
		rt.sweepActorWakeOutbox(ctx)
		now = now.Add(time.Minute)
	}
	// A second wake fails once and is backing off: pending and retrying.
	queueOneActorWake(t, s, "surface-pending")
	rt.sweepActorWakeOutbox(ctx)

	unauth := httptest.NewRecorder()
	NewAPIHandler(rt).HandleObligations(unauth, httptest.NewRequest(http.MethodGet, "/api/runtime/obligations", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated obligations read = %d", unauth.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/obligations", nil)
	req.RemoteAddr = ""
	req.Header.Set("X-Authenticated-User", "user-wake-surface-pending")
	rec := httptest.NewRecorder()
	NewAPIHandler(rt).HandleObligations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("obligations status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body obligationsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Restart.Kind == "" {
		t.Fatalf("restart kind missing: %+v", body.Restart)
	}
	if body.Wakes.Exhausted < 1 || len(body.Wakes.ExhaustedSamples) < 1 {
		t.Fatalf("exhausted wake not visible: %+v", body.Wakes)
	}
	sample := body.Wakes.ExhaustedSamples[0]
	if sample.LastError == "" || sample.Attempts != wakeDispatchMaxAttempts {
		t.Fatalf("exhausted sample lacks why: %+v", sample)
	}
	if body.Wakes.Unprojected < 1 || body.Wakes.Retrying < 1 {
		t.Fatalf("pending, retrying wake not counted: %+v", body.Wakes)
	}
}

// Deferred and in-flight actor work is visible
// (texture-create-occurrence-deferred-never-refires-2026-10-09): the surface
// answered "nothing owed" while a Texture occurrence sat deferred.
func TestObligationsSurfaceReportsActorTape(t *testing.T) {
	rt, _ := testRuntime(t)
	started := time.Now().UTC().Add(-90 * time.Second)
	rt.SetActorObligationsReader(func(context.Context) (ActorObligations, error) {
		return ActorObligations{
			Due: 1, Deferred: 1, MaxDeferCount: 1,
			Samples:  []ActorObligationSample{{UpdateID: "u1", ToAgentID: "texture:d/x", Kind: "initial_dispatch", DeferCount: 1}},
			InFlight: map[string]time.Time{"texture:d/x": started},
		}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/obligations", nil)
	req.RemoteAddr = ""
	req.Header.Set("X-Authenticated-User", "owner-actor-tape")
	rec := httptest.NewRecorder()
	NewAPIHandler(rt).HandleObligations(rec, req)
	var body obligationsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Actors == nil || body.Actors.Due != 1 || body.Actors.Deferred != 1 || len(body.Actors.Samples) != 1 {
		t.Fatalf("actor tape not visible: %s", rec.Body.String())
	}
	if len(body.Actors.InFlight) != 1 || body.Actors.InFlight[0].AgeSeconds < 60 {
		t.Fatalf("in-flight activation age missing: %+v", body.Actors.InFlight)
	}

	rt.SetActorObligationsReader(func(context.Context) (ActorObligations, error) {
		return ActorObligations{}, errors.New("actor log closed")
	})
	rec = httptest.NewRecorder()
	NewAPIHandler(rt).HandleObligations(rec, req)
	body = obligationsResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(body.Errors) == 0 {
		t.Fatalf("actor read error hidden: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

// Drill selfdev-jobs-0101 gap: the apply's restart marker names the
// operation it restarted for, and the surface dropped it. Failure modes
// pinned: M1 a planned boot's marker target is dropped; M2 a crash boot
// serves a target.
func TestObligationsSurfaceServesPlannedRestartTarget(t *testing.T) {
	rt, _ := testRuntime(t)
	read := func() obligationsRestart {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/obligations", nil)
		req.RemoteAddr = ""
		req.Header.Set("X-Authenticated-User", "owner-restart-target")
		rec := httptest.NewRecorder()
		NewAPIHandler(rt).HandleObligations(rec, req)
		var body struct {
			Restart struct {
				Kind   string `json:"kind"`
				Reason string `json:"reason"`
				Target string `json:"target"`
			} `json:"restart"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return obligationsRestart{Kind: body.Restart.Kind, Reason: body.Restart.Reason, Target: body.Restart.Target}
	}

	rt.bootRestart = PlannedRestart{Reason: PlannedRestartSelfDevelopmentApply, Target: "selfdev-op-1"}
	rt.bootRestartPlanned = true
	if got := read(); got.Kind != "planned" || got.Reason != PlannedRestartSelfDevelopmentApply || got.Target != "selfdev-op-1" {
		t.Fatalf("planned boot restart = %+v, want the marker's reason and target", got)
	}

	rt.bootRestart = PlannedRestart{Target: "stale-target"}
	rt.bootRestartPlanned = false
	if got := read(); got.Kind != "crash_or_stop" || got.Target != "" || got.Reason != "" {
		t.Fatalf("crash boot restart = %+v, want no reason or target", got)
	}
}

// An open self-development operation with no transition for longer than
// selfdevStalledAfter is visible as stalled, with its last refusal
// (docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md):
// an operation sat executing for 17 hours while the surface said nothing.
// Failure modes pinned: a stalled operation is invisible; a fresh one is
// called stalled; a settled one is listed; the busy count disagrees with
// /health's selfdev_active_operations.
func TestObligationsSurfaceReportsStalledSelfdevOperations(t *testing.T) {
	rt, s := testRuntime(t)
	computerID := rt.TextureComputerID()
	now := time.Now().UTC()
	seed := func(id, state, lastIntentError string, updated time.Time) {
		if _, err := s.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, desired_head, effective_head, state, last_intent_error, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', '[]', '', '', ?, ?, ?, ?)`,
			id, computerID, "key-"+id, "trajectory-"+id, state, lastIntentError, updated, updated); err != nil {
			t.Fatal(err)
		}
	}
	seed("op-stalled", "executing", "freeze: capsule release must contain executable bin/autoputer", now.Add(-17*time.Hour))
	seed("op-fresh", "awaiting_approval", "", now.Add(-time.Minute))
	seed("op-settled", "failed", "", now.Add(-48*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/obligations", nil)
	req.RemoteAddr = ""
	req.Header.Set("X-Authenticated-User", "user-selfdev-stalled")
	rec := httptest.NewRecorder()
	NewAPIHandler(rt).HandleObligations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("obligations status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body obligationsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := body.Selfdev
	if got.Open != 2 || got.Stalled != 1 || len(got.Operations) != 2 {
		t.Fatalf("selfdev section = %+v; want two open, one stalled", got)
	}
	if got.Busy != rt.activeSelfdevOperations(context.Background()) || got.Busy != 1 {
		t.Fatalf("busy = %d; /health counts %d", got.Busy, rt.activeSelfdevOperations(context.Background()))
	}
	first := got.Operations[0]
	if first.OperationID != "op-stalled" || !first.Stalled || first.LastIntentError == "" || first.IdleSeconds < int((16*time.Hour).Seconds()) {
		t.Fatalf("stalled operation sample = %+v", first)
	}
	if second := got.Operations[1]; second.OperationID != "op-fresh" || second.Stalled {
		t.Fatalf("fresh operation sample = %+v", second)
	}
}
