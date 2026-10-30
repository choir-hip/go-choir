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
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md).
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
//
// Re-credential on resume (design note items 31-36):
//   - resumed with an envelope still renews the frozen capability (refused
//     hours after hibernate) instead of installing the delivered one;
//   - the envelope is installed but its consumption receipt is never appended
//     as key_revoked, so the frozen capability stays valid and the guest keeps
//     an epoch-E token the next boot revokes under it;
//   - an install or key_revoked failure still reopens admission;
//   - the envelope is echoed in a response;
//   - prepare does not advertise credential_resume, so the host never
//     delivers and the old deadline keeps applying.

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

type fakeResumeCredential struct {
	envelopes []string
	err       error
	expiresAt time.Time
}

func (f *fakeResumeCredential) InstallResumeEnvelope(_ context.Context, envelope string) (time.Time, error) {
	f.envelopes = append(f.envelopes, envelope)
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

// resumeCredentialRuntime fences a runtime whose credential accepts delivered
// envelopes; reconciled counts key_revoked reconciliations.
func resumeCredentialRuntime(t *testing.T) (*Runtime, *fakeHibernateCredential, *fakeResumeCredential, *atomic.Int32) {
	t.Helper()
	rt, renew := hibernateTestRuntime(t)
	install := &fakeResumeCredential{expiresAt: time.Date(2026, 10, 14, 16, 10, 0, 0, time.UTC)}
	rt.hibernate.resumeCredential = install
	reconciled := &atomic.Int32{}
	rt.hibernate.reconcile = func(context.Context) error {
		reconciled.Add(1)
		return nil
	}
	prevClock := setSystemClock
	setSystemClock = func(time.Time) error { return nil }
	t.Cleanup(func() { setSystemClock = prevClock })
	return rt, renew, install, reconciled
}

const testResumeEnvelope = "eyJ2ZXJzaW9uIjoxfQ"

func TestHibernatePrepareAdvertisesCredentialResume(t *testing.T) {
	rt, _, _, _ := resumeCredentialRuntime(t)
	code, body := postHibernate(t, &APIHandler{rt: rt}, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK || body["credential_resume"] != true {
		t.Fatalf("prepare: %d %v, want credential_resume=true", code, body)
	}
	plain, _ := hibernateTestRuntime(t)
	_, body = postHibernate(t, &APIHandler{rt: plain}, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if _, ok := body["credential_resume"]; ok {
		t.Fatalf("a guest that cannot install envelopes advertised credential_resume: %v", body)
	}
}

func TestHibernateResumedInstallsDeliveredCredential(t *testing.T) {
	rt, renew, install, reconciled := resumeCredentialRuntime(t)
	handler := &APIHandler{rt: rt}
	if code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`); code != http.StatusOK {
		t.Fatalf("prepare: %d %v", code, body)
	}
	renewsAfterPrepare := renew.calls.Load()
	// The frozen capability is long past the renewal grace: renewal fails.
	renew.err = errors.New("renewal refused: expired")
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/resumed", "192.0.2.1:4000",
		`{"host_unix_nano":`+jsonInt(time.Now().Add(5*time.Hour).UnixNano())+`,"credential_envelope":"`+testResumeEnvelope+`"}`)
	if code != http.StatusOK || body["status"] != "resumed" || body["credential"] != "installed" {
		t.Fatalf("resumed with envelope: %d %v, want 200 resumed credential=installed", code, body)
	}
	if len(install.envelopes) != 1 || install.envelopes[0] != testResumeEnvelope {
		t.Fatalf("installed envelopes = %v", install.envelopes)
	}
	if renew.calls.Load() != renewsAfterPrepare {
		t.Fatal("resumed with an envelope renewed the frozen capability")
	}
	if reconciled.Load() != 1 {
		t.Fatalf("key_revoked reconciliations = %d, want 1", reconciled.Load())
	}
	if body["capability_expires_at"] != "2026-10-14T16:10:00Z" {
		t.Fatalf("capability_expires_at = %v", body["capability_expires_at"])
	}
	raw, _ := json.Marshal(body)
	if bytes.Contains(raw, []byte(testResumeEnvelope)) {
		t.Fatal("resumed response echoes the credential envelope")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("admission not reopened after an installed resume: %v", err)
	}
	release()
}

func TestHibernateResumedKeepsFenceWhenCredentialInstallFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(rt *Runtime, install *fakeResumeCredential)
	}{
		{"install refused", func(_ *Runtime, install *fakeResumeCredential) { install.err = errors.New("exchange refused") }},
		{"key_revoked append failed", func(rt *Runtime, _ *fakeResumeCredential) {
			rt.hibernate.reconcile = func(context.Context) error { return errors.New("append refused") }
		}},
		{"guest cannot install", func(rt *Runtime, _ *fakeResumeCredential) { rt.hibernate.resumeCredential = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, _, install, _ := resumeCredentialRuntime(t)
			handler := &APIHandler{rt: rt}
			if code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`); code != http.StatusOK {
				t.Fatalf("prepare: %d %v", code, body)
			}
			tc.setup(rt, install)
			code, body := postHibernate(t, handler, "/internal/runtime/hibernate/resumed", "192.0.2.1:4000",
				`{"host_unix_nano":`+jsonInt(time.Now().UnixNano())+`,"credential_envelope":"`+testResumeEnvelope+`"}`)
			if code == http.StatusOK {
				t.Fatalf("resumed answered 200 although the credential was not installed: %v", body)
			}
			raw, _ := json.Marshal(body)
			if bytes.Contains(raw, []byte(testResumeEnvelope)) {
				t.Fatal("refusal echoes the credential envelope")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if _, err := rt.EnterWork(ctx); err == nil {
				t.Fatal("work entered a guest whose re-credential failed (the host is about to kill it)")
			}
		})
	}
}

func jsonInt(n int64) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}
