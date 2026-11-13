package vmctl

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The host's clear of a planned-restart marker is best effort, but a stalled
// withdrawal or a guest-owned marker it left alone must be visible in the
// log (review 3): one line each, no change in behaviour.
func TestTheHostsClearLogsAGuestRefusalAndAKeptAnswer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"refusal", http.StatusServiceUnavailable, `{"error":"a planned-restart request is still in flight"}`, "503"},
		{"kept", http.StatusOK, `{"status":"kept","reason":"a planned-restart marker for platform_update is not the host's refresh"}`, "kept"},
		{"cleared", http.StatusOK, `{"status":"cleared"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(log.Writer())
			httpGuestMarker{client: srv.Client()}.clear(context.Background(), VMOwnership{VMID: "vm-clear-log", ComputerURL: srv.URL})
			out := buf.String()
			if tc.want == "" {
				if strings.Contains(out, "clear planned-restart marker") {
					t.Fatalf("a clean clear logged: %s", out)
				}
				return
			}
			if !strings.Contains(out, "vm-clear-log") || !strings.Contains(out, tc.want) {
				t.Fatalf("log = %q, want a line naming the computer and %q", out, tc.want)
			}
		})
	}
}
