package agentcore

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// D2 S5 item 1: a planned-restart marker is never captured inside a snapshot.
// A resumed guest that crashes within the marker's 15 minutes would boot as
// planned and resume work (design 5.3, M12; review 306 Devin F3). Both
// orderings:
//   - mark, then hibernate: prepare answers busy (planned_restart_pending)
//     while a fresh marker exists;
//   - hibernate, then mark: the planned-restart endpoint and markPlannedRestart
//     answer busy (hibernate_fenced) while the fence holds, before writing
//     anything; and a mark that is inside keeps the fence from closing.
//
// Failure modes, written first:
//   - prepare fences a guest whose marker is on disk;
//   - a stale marker (older than the max age, which the boot ignores) blocks
//     hibernation forever;
//   - the endpoint writes a marker through a closed fence;
//   - the fence closes while a mark is being written;
//   - a refused mark leaves a marker or an open slot behind.

func fencedHibernateRuntime(t *testing.T) *Runtime {
	t.Helper()
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	rt.hibernate.credential = &fakeHibernateCredential{expiresAt: time.Date(2026, 10, 26, 12, 30, 0, 0, time.UTC)}
	prevSync := syncFilesystems
	syncFilesystems = func() {}
	t.Cleanup(func() { syncFilesystems = prevSync })
	return rt
}

func TestHibernatePrepareRefusesWhileAMarkerIsPending(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	handler := &APIHandler{rt: rt}
	if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "abc", RequestedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["outcome"] != "busy" || body["reason"] != "planned_restart_pending" {
		t.Fatalf("prepare with a fresh marker: %d %v; want 409 busy planned_restart_pending", code, body)
	}
	// Admission stayed open.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	release, err := rt.EnterWork(ctx)
	if err != nil {
		t.Fatalf("a refused prepare left the fence closed: %v", err)
	}
	release()

	// A stale marker (older than plannedRestartMaxAge, which the boot ignores)
	// does not block.
	if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "abc", RequestedAt: time.Now().Add(-plannedRestartMaxAge - time.Minute).UTC()}); err != nil {
		t.Fatal(err)
	}
	code, body = postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK || body["status"] != "prepared" {
		t.Fatalf("prepare with a stale marker: %d %v; want 200 prepared", code, body)
	}
	rt.hibernate.lift()

	// A marker that cannot be read counts as pending (fail closed).
	if err := os.WriteFile(PlannedRestartMarkerPath(rt.cfg.StorePath), []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, body = postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["reason"] != "planned_restart_pending" {
		t.Fatalf("prepare with an unreadable marker: %d %v; want 409 planned_restart_pending", code, body)
	}
}

func TestPlannedRestartEndpointRefusedWhileFenced(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	handler := &APIHandler{rt: rt}
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK {
		t.Fatalf("prepare: %d %v", code, body)
	}
	code, resp := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusConflict {
		t.Fatalf("mark through a closed fence: %d %s; want 409", code, resp)
	}
	if want := `"hibernate_fenced"`; !contains(resp, want) {
		t.Fatalf("mark through a closed fence: %s; want reason %s", resp, want)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("a marker was written through a closed fence: %v", err)
	}
	// After the fence lifts the mark goes through, and the slot it took is released.
	rt.hibernate.lift()
	if code, resp := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("mark after the fence lifted: %d %s", code, resp)
	}
	if n, ok := rt.hibernate.close(time.Minute); !ok || n != 0 {
		t.Fatalf("a finished mark left a work slot taken: inflight=%d ok=%v", n, ok)
	}
}

func TestMarkPlannedRestartRefusedWhileFenced(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	if _, ok := rt.hibernate.close(time.Minute); !ok {
		t.Fatal("setup: fence did not close")
	}
	if _, err := rt.markPlannedRestart("apply", "abc"); err == nil {
		t.Fatal("markPlannedRestart succeeded through a closed fence")
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("a marker was written through a closed fence: %v", err)
	}
	rt.hibernate.lift()
	clear, err := rt.markPlannedRestart("apply", "abc")
	if err != nil {
		t.Fatalf("markPlannedRestart after the lift: %v", err)
	}
	if n, ok := rt.hibernate.close(time.Minute); !ok || n != 0 {
		t.Fatalf("markPlannedRestart kept its work slot after the write: inflight=%d ok=%v", n, ok)
	}
	rt.hibernate.lift()
	clear(nil)
}

// The interleaving the design names: a mark is inside, then the fence is
// asked (it answers busy), and after the mark returns it answers busy again
// because a marker exists.
func TestFenceAskedWhileAMarkIsInside(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	handler := &APIHandler{rt: rt}
	release, ok := rt.hibernate.tryEnter()
	if !ok {
		t.Fatal("tryEnter refused on an open fence")
	}
	code, body := postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["outcome"] != "busy" {
		t.Fatalf("prepare while a mark is inside: %d %v; want 409 busy", code, body)
	}
	if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "abc"}); err != nil {
		t.Fatal(err)
	}
	release()
	code, body = postHibernate(t, handler, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["reason"] != "planned_restart_pending" {
		t.Fatalf("prepare after the mark returned: %d %v; want 409 planned_restart_pending", code, body)
	}
}

func TestTryEnterDoesNotWaitAndHonoursExpiry(t *testing.T) {
	var f hibernateFence
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	f.now = func() time.Time { return now }
	f.close(time.Minute)
	start := time.Now()
	if _, ok := f.tryEnter(); ok {
		t.Fatal("tryEnter entered a closed fence")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("tryEnter waited")
	}
	now = now.Add(2 * time.Minute) // the TTL passed without a resume
	release, ok := f.tryEnter()
	if !ok {
		t.Fatal("tryEnter refused an expired fence")
	}
	release()
	release() // idempotent
	if n, ok := f.close(time.Minute); !ok || n != 0 {
		t.Fatalf("release left inflight=%d", n)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
