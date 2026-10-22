package vmctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostboot"
)

// The boot-state route answers internal callers with the detector's fact and
// refuses everyone else: it names the running kernel, so it is never public.
func TestHostBootStateRoute(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("store/k-old/bzImage", "old")
	mustWrite("store/k-new/bzImage", "new")
	mustWrite("store/sys-old/kernel-params", "")
	mustWrite("store/sys-new/kernel-params", "kvm.halt_poll_ns=0")
	for sys, k := range map[string]string{"sys-old": "k-old", "sys-new": "k-new"} {
		if err := os.Symlink(filepath.Join(root, "store", k, "bzImage"), filepath.Join(root, "store", sys, "kernel")); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("proc/cmdline", "loglevel=4")
	if err := os.MkdirAll(filepath.Join(root, "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "store/sys-old"), filepath.Join(root, "run/booted-system")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "store/sys-new"), filepath.Join(root, "run/current-system")); err != nil {
		t.Fatal(err)
	}
	prev := hostBootRoot
	hostBootRoot = root
	t.Cleanup(func() { hostBootRoot = prev })

	h := NewHandler(NewOwnershipRegistry("http://127.0.0.1:9"))

	external := httptest.NewRequest(http.MethodGet, HostBootStatePath, nil)
	external.RemoteAddr = "203.0.113.7:4242"
	rec := httptest.NewRecorder()
	h.HandleHostBootState(rec, external)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("external caller: %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.HandleHostBootState(rec, httptest.NewRequest(http.MethodGet, HostBootStatePath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("internal caller: %d %s", rec.Code, rec.Body.String())
	}
	var st hostboot.State
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Available || !st.RebootPending || len(st.Reasons) != 2 || st.Reasons[0] != hostboot.ReasonKernel || st.Reasons[1] != hostboot.ReasonKernelParams {
		t.Fatalf("boot state = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.HandleHostBootState(rec, httptest.NewRequest(http.MethodPost, HostBootStatePath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d, want 405", rec.Code)
	}
}

// The boot-state journal line is written at start, again every interval, and
// never after the context ends: the lag shows in the journal without anyone
// calling the route (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md,
// 2026-10-22).
func TestHostBootStateLogAtStartAndOnInterval(t *testing.T) {
	prev := hostBootRoot
	hostBootRoot = t.TempDir()
	t.Cleanup(func() { hostBootRoot = prev })

	lines := make(chan string, 16)
	logf := func(format string, args ...any) { lines <- fmt.Sprintf(format, args...) }
	ctx, cancel := context.WithCancel(context.Background())
	StartHostBootStateLog(ctx, 20*time.Millisecond, logf)

	for i := 0; i < 2; i++ {
		select {
		case line := <-lines:
			if !strings.HasPrefix(line, "vmctl: host boot state unavailable: booted system unreadable") {
				t.Fatalf("line %d = %q", i, line)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("line %d never written", i)
		}
	}
	cancel()
	time.Sleep(50 * time.Millisecond)
	for len(lines) > 0 {
		<-lines
	}
	time.Sleep(60 * time.Millisecond)
	if len(lines) != 0 {
		t.Fatalf("logged after cancel: %q", <-lines)
	}
}
