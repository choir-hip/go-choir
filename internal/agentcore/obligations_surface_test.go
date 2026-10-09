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
