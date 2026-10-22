package textureowner

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A validation refusal of the owner's prompt is the owner's to correct: a 400
// that names the validator's reason, not a 500 "failed to submit prompt" with
// the reason only in the guest log (staging 2026-10-22 16:28 UTC). The errors
// below are wrapped the way the production path wraps them: the store's
// "lifecycle start: prepare initial revision: %w", then the handoff's
// "start Texture lifecycle: %w".
func TestPromptBarSubmitFailureStatusAndReason(t *testing.T) {
	invalid := fmt.Errorf("start Texture lifecycle: %w",
		fmt.Errorf("lifecycle start: prepare initial revision: %w",
			fmt.Errorf("%w: doc.content.paragraph.content[135] contains legacy unresolved numbered citation syntax; use a source_ref node", store.ErrInvalidTextureRevision)))
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
		wantHas    string
		wantNot    string
	}{
		{
			name: "validation refusal is a 400 with the validator's reason", err: invalid,
			wantStatus: http.StatusBadRequest, wantHas: "contains legacy unresolved numbered citation syntax", wantNot: "failed to submit prompt",
		},
		{name: "command conflict stays 409", err: fmt.Errorf("x: %w", agentcore.ErrPromptCommandConflict), wantStatus: http.StatusConflict, wantMsg: "command identity conflicts with the stored request"},
		{name: "lifecycle conflict stays 409", err: fmt.Errorf("x: %w", store.ErrLifecycleCommandConflict), wantStatus: http.StatusConflict, wantMsg: "command identity conflicts with the stored request"},
		{name: "concurrent change stays 409", err: fmt.Errorf("x: %w", store.ErrConcurrentStateChange), wantStatus: http.StatusConflict, wantMsg: "command identity conflicts with the stored request"},
		{name: "pre-genesis stays 503", err: fmt.Errorf("x: %w", agentcore.ErrPreGenesis), wantStatus: http.StatusServiceUnavailable, wantMsg: "computer initializing"},
		{name: "anything else stays 500 without internals", err: errors.New("dolt: connection reset by peer"), wantStatus: http.StatusInternalServerError, wantMsg: "failed to submit prompt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, msg := promptBarSubmitFailure(tc.err)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (message %q)", status, tc.wantStatus, msg)
			}
			if tc.wantMsg != "" && msg != tc.wantMsg {
				t.Fatalf("message = %q, want %q", msg, tc.wantMsg)
			}
			if tc.wantHas != "" && !strings.Contains(msg, tc.wantHas) {
				t.Fatalf("message = %q, want it to carry %q", msg, tc.wantHas)
			}
			if tc.wantNot != "" && strings.Contains(msg, tc.wantNot) {
				t.Fatalf("message = %q, must not carry %q", msg, tc.wantNot)
			}
			if strings.Contains(msg, "start Texture lifecycle") || strings.Contains(msg, "prepare initial revision") {
				t.Fatalf("message = %q leaks the internal wrapping", msg)
			}
		})
	}
}
