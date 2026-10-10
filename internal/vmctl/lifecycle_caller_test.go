package vmctl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Phase 0 step 1 (docs/vmctl-360-review-2026-10-10.md): a lifecycle call that
// can destroy a computer names its caller, so vmctl's log joins the caller to
// the kill's destruction receipt. Failure mode pinned: the tag set on the
// caller's context never reaches vmctl, leaving the kill unattributed again.
func TestClientSendsLifecycleCallerFromContext(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get(LifecycleCallerHeader)
		_ = json.NewEncoder(w).Encode(map[string]any{"user_id": "u", "desktop_id": "primary", "computer_id": "c", "vm_id": "vm-1"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	ctx := WithLifecycleCaller(context.Background(), "proxy.compute-recovery")
	_, _ = client.RefreshDesktopContext(ctx, "u", "primary")
	if caller := <-got; caller != "proxy.compute-recovery" {
		t.Fatalf("refresh caller header = %q, want proxy.compute-recovery", caller)
	}

	_, _ = client.RefreshDesktopContext(context.Background(), "u", "primary")
	if caller := <-got; caller != "" {
		t.Fatalf("untagged call sent caller %q", caller)
	}
}
