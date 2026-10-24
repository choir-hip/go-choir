package autoputer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// The guest's credential step: an envelope past its life is fatal for that
// boot (a restart reads the same envelope); a platform blip stays transient
// (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md, item 2).
func TestCredentialStepFatalOnlyForEnvelopeLifetime(t *testing.T) {
	cases := []struct {
		err   error
		fatal bool
	}{
		{platform.ErrCredentialEnvelopeLifetime, true},
		{fmt.Errorf("exchange: %w", platform.ErrCredentialEnvelopeLifetime), true},
		{errors.New("guest credential: exchange: dial tcp 10.0.0.1:8090: connection refused"), false},
		{errors.New("guest credential: exchange refused: status 503"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := credentialStepFatal(c.err); got != c.fatal {
			t.Fatalf("credentialStepFatal(%v) = %v, want %v", c.err, got, c.fatal)
		}
	}
}
