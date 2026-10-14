package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// S3 snapshot hibernate, guest fence
// (docs/reports/s3-snapshot-hibernate-design-2026-10-14.md).
//
// Ways this could fail, written first:
//   - the fence closes while a work occurrence is inside, so the snapshot
//     freezes a model call mid-flight and the resumed guest reads a dead
//     connection as the model's answer;
//   - an occurrence slips in between the busy check and the fence closing
//     (the check and the close are not one step);
//   - a busy refusal leaves admission closed, so a guest the host left
//     running refuses work until the TTL;
//   - prepare answers 200 without renewing the capability, so the snapshot
//     freezes a nearly expired token and the woken guest cannot renew;
//   - a host that fenced and then vanished leaves the guest fenced forever;
//   - resumed reopens admission before the clock is stepped, so work stamps
//     records with the hibernate-time wall clock;
//   - resumed answers 200 when the capability renewal failed, so the host
//     keeps a guest that cannot write events;
//   - a guest (not the host peer) can fence or resume a neighbour.

type fakeHibernateCredential struct {
	calls     atomic.Int32
	err       error
	expiresAt time.Time
}

func (f *fakeHibernateCredential) RenewNow(context.Context) (time.Time, error) {
	f.calls.Add(1)
	if f.err != nil {
		return time.Time{}, f.err
	}
	return f.expiresAt, nil
}

func hibernateTestRuntime(t *testing.T) (*Runtime, *fakeHibernateCredential) {
	t.Helper()
	cred := &fakeHibernateCredential{expiresAt: time.Date(2026, 10, 14, 12, 30, 0, 0, time.UTC)}
	rt := &Runtime{running: map[string]context.CancelFunc{}}
	rt.hibernate.credential = cred
	prevSync := syncFilesystems
	syncFilesystems = func() {}
	t.Cleanup(func() { syncFilesystems = prevSync })
	return rt, cred
}

func postHibernate(t *testing.T, handler *APIHandler, path, remoteAddr, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	switch path {
	case "/internal/runtime/hibernate/prepare":
		handler.HandleInternalHibernatePrepare(rec, req)
	case "/internal/runtime/hibernate/resumed":
		handler.HandleInternalHibernateResumed(rec, req)
	default:
		t.Fatalf("unknown path %s", path)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHibernatePrepareRefusesWhileWorkIsInside(t *testing.T) {
	rt, cred := hibernateTestRuntime(t)
	handler := &APIHandler{rt: rt}
	release, err := rt.EnterWork(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["outcome"] != "busy" {
		t.Fatalf("prepare with work inside: %d %v, want 409 busy", code, body)
	}
	if cred.calls.Load() != 0 {
		t.Fatalf("busy prepare renewed the capability")
	}
	release()
	// Admission stayed open: a new occurrence enters without waiting.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	release2, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("busy refusal left admission closed: %v", err)
	}
	release2()
}

func TestHibernatePrepareRefusesWhileRunIsRunning(t *testing.T) {
	rt, _ := hibernateTestRuntime(t)
	rt.running["loop-busy"] = func() {}
	handler := &APIHandler{rt: rt}
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["running_runs"] != float64(1) {
		t.Fatalf("prepare with a running run: %d %v, want 409 running_runs=1", code, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	release, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("busy refusal left admission closed: %v", err)
	}
	release()
}

func TestHibernatePrepareFencesRenewsAndResumedReopens(t *testing.T) {
	rt, cred := hibernateTestRuntime(t)
	handler := &APIHandler{rt: rt}
	var stepped atomic.Value
	prevClock := setSystemClock
	setSystemClock = func(t time.Time) error {
		stepped.Store(t)
		return nil
	}
	t.Cleanup(func() { setSystemClock = prevClock })

	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK || body["status"] != "prepared" {
		t.Fatalf("idle prepare: %d %v, want 200 prepared", code, body)
	}
	if body["capability_expires_at"] != "2026-10-14T12:30:00Z" || cred.calls.Load() != 1 {
		t.Fatalf("prepare did not renew the capability: %v calls=%d", body, cred.calls.Load())
	}

	// Work arriving while fenced waits.
	entered := make(chan struct{})
	go func() {
		release, err := rt.EnterWork(context.Background())
		if err == nil {
			close(entered)
			release()
		}
	}()
	select {
	case <-entered:
		t.Fatal("work entered while the hibernate fence was closed")
	case <-time.After(100 * time.Millisecond):
	}

	host := time.Now().Add(37 * time.Minute)
	code, body = postHibernate(t, handler, "/internal/runtime/hibernate/resumed", "192.0.2.1:4000",
		`{"host_unix_nano":`+jsonInt(host.UnixNano())+`}`)
	if code != http.StatusOK || body["status"] != "resumed" {
		t.Fatalf("resumed: %d %v, want 200 resumed", code, body)
	}
	if got, _ := stepped.Load().(time.Time); !got.Equal(host) {
		t.Fatalf("clock stepped to %v, want host time %v", got, host)
	}
	if ms, _ := body["clock_step_ms"].(float64); ms < float64((36 * time.Minute).Milliseconds()) {
		t.Fatalf("clock_step_ms = %v, want about 37 minutes", body["clock_step_ms"])
	}
	if cred.calls.Load() != 2 {
		t.Fatalf("resumed did not renew the capability (calls=%d)", cred.calls.Load())
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("waiting work did not enter after resume")
	}
}

func TestHibernateResumedKeepsFenceWhenRenewalFails(t *testing.T) {
	rt, cred := hibernateTestRuntime(t)
	handler := &APIHandler{rt: rt}
	prevClock := setSystemClock
	setSystemClock = func(time.Time) error { return nil }
	t.Cleanup(func() { setSystemClock = prevClock })

	if code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`); code != http.StatusOK {
		t.Fatalf("prepare: %d %v", code, body)
	}
	cred.err = errors.New("renewal refused")
	code, _ := postHibernate(t, handler, "/internal/runtime/hibernate/resumed", "192.0.2.1:4000",
		`{"host_unix_nano":`+jsonInt(time.Now().UnixNano())+`}`)
	if code == http.StatusOK {
		t.Fatal("resumed answered 200 although the capability renewal failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := rt.EnterWork(ctx); err == nil {
		t.Fatal("work entered a guest whose resume failed (the host is about to kill it)")
	}
}

func TestHibernatePrepareFailsClosedWhenRenewalFails(t *testing.T) {
	rt, cred := hibernateTestRuntime(t)
	cred.err = errors.New("platform unreachable")
	handler := &APIHandler{rt: rt}
	code, _ := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code == http.StatusOK {
		t.Fatal("prepare answered 200 without a renewed capability")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	release, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("failed prepare left admission closed: %v", err)
	}
	release()
}

func TestHibernateFenceExpiresWithoutResume(t *testing.T) {
	rt, _ := hibernateTestRuntime(t)
	now := time.Now()
	rt.hibernate.now = func() time.Time { return now }
	if _, ok := rt.hibernate.close(hibernateFenceTTL); !ok {
		t.Fatal("fence did not close on an idle guest")
	}
	now = now.Add(hibernateFenceTTL + time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("expired fence still held admission: %v", err)
	}
	release()
}

func TestHibernateEndpointsAreHostPeerOnly(t *testing.T) {
	rt, cred := hibernateTestRuntime(t)
	handler := &APIHandler{rt: rt}
	for _, path := range []string{"/internal/runtime/hibernate/prepare", "/internal/runtime/hibernate/resumed"} {
		code, _ := postHibernate(t, handler, path, "203.0.113.9:4000", `{"host_unix_nano":1}`)
		if code != http.StatusForbidden {
			t.Fatalf("%s from a non-host peer: %d, want 403", path, code)
		}
	}
	if cred.calls.Load() != 0 {
		t.Fatal("a non-host peer reached the capability renewal")
	}
}

func jsonInt(n int64) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}
