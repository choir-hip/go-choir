package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The operator heal's route
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md).
// Ways it can fail, written before the code:
//
//  1. A caller other than the host peer (loopback: the guest's own children)
//     reads or applies it.
//  2. A GET applies, or a POST only plans.
//  3. A refused heal (a row moved since the plan) is a 500 or a 200.
//  4. The dry run is not machine-readable for the operator.

type fakeStampHealer struct {
	calls []stampHealCall
	err   error
}

type stampHealCall struct {
	computerID    string
	apply, verify bool
}

func (f *fakeStampHealer) HealSelfDevelopmentIntentStamps(_ context.Context, computerID string, apply, verify bool) (agentcore.SelfDevelopmentStampHealReport, error) {
	f.calls = append(f.calls, stampHealCall{computerID, apply, verify})
	mode := "dry_run"
	if apply {
		mode = "applied"
	}
	return agentcore.SelfDevelopmentStampHealReport{
		ComputerID: computerID, Mode: mode, RowsExamined: 1,
		Rows: []selfdev.StampDivergence{{OperationID: "selfdev-1", State: "executing", LiveLastIntentError: "x", Differs: []string{"last_intent_error", "updated_at"}}},
	}, f.err
}

func stampHealRequest(method, rawURL, remote string) *http.Request {
	req := httptest.NewRequest(method, rawURL, nil)
	req.RemoteAddr = remote
	return req
}

func TestOperationStampHealEndpoint(t *testing.T) {
	const host, path = "192.0.2.44:40000", "/internal/self-development/operation-stamp-heal"
	healer := &fakeStampHealer{}
	var serving atomic.Bool
	handler := operationStampHealHandler(healer, "computer-1", func() bool { return serving.Load() })

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		handler(rec, stampHealRequest(method, path, "127.0.0.1:40000"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("loopback %s = %d, want 403", method, rec.Code)
		}
	}
	if len(healer.calls) != 0 {
		t.Fatalf("a refused caller reached the healer: %+v", healer.calls)
	}

	// Until the runtime is serving the route answers 503 and never reaches the
	// healer; the host-peer check still comes first.
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		handler(rec, stampHealRequest(method, path, host))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s before the runtime serves = %d, want 503", method, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodGet, path, "127.0.0.1:40000"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("loopback before the runtime serves = %d, want 403", rec.Code)
	}
	if len(healer.calls) != 0 {
		t.Fatalf("the healer was reached before the runtime served: %+v", healer.calls)
	}
	serving.Store(true)

	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodGet, path+"?apply=1", host))
	var report agentcore.SelfDevelopmentStampHealReport
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &report) != nil || report.Mode != "dry_run" || len(report.Rows) != 1 {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
	}
	if got := healer.calls[0]; got.apply || got.computerID != "computer-1" {
		t.Fatalf("a GET applied or lost the computer id: %+v", got)
	}

	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodPost, path, host))
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &report) != nil || report.Mode != "applied" {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body.String())
	}
	if got := healer.calls[1]; !got.apply || !got.verify {
		t.Fatalf("POST call = %+v, want apply with verify", got)
	}
	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodPost, path+"?verify=0", host))
	if got := healer.calls[2]; !got.apply || got.verify {
		t.Fatalf("POST ?verify=0 call = %+v", got)
	}

	healer.err = selfdev.ErrConflict
	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodPost, path, host))
	if rec.Code != http.StatusConflict {
		t.Fatalf("a refused heal = %d, want 409", rec.Code)
	}
	healer.err = agentcore.ErrStampHealRunning
	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodPost, path, host))
	if rec.Code != http.StatusConflict {
		t.Fatalf("a concurrent heal = %d, want 409", rec.Code)
	}
	for _, unavailable := range []error{selfdev.ErrStampHealUnbound, agentcore.ErrReplayCompletenessUnavailable, fmt.Errorf("probe: %w", agentcore.ErrReplayCompletenessUnavailable)} {
		healer.err = unavailable
		rec = httptest.NewRecorder()
		handler(rec, stampHealRequest(http.MethodGet, path, host))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%v = %d, want 503", unavailable, rec.Code)
		}
	}
	healer.err = errors.New("boom")
	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodPost, path, host))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a failed heal = %d, want 500", rec.Code)
	}
	healer.err = nil
	rec = httptest.NewRecorder()
	handler(rec, stampHealRequest(http.MethodDelete, path, host))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE = %d, want 405", rec.Code)
	}
}
