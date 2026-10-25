package capsule

import (
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// secretBearingRefusal refuses a GoEval source or an Exec command that
// DetectPrivateSecrets flags, before anything runs. The refusal names each
// matched rule, the line it matched on and what the rule matches, never the
// matched text: an unnamed refusal cost the engineer five writes in trace
// review round 4 (P8, docs/evidence/selfdev-trace-reviews-2026-10-25.md).
func secretBearingRefusal(what, payload string) error {
	found := computerevent.LocatePrivateSecrets([]byte(payload))
	if len(found) == 0 {
		return nil
	}
	return fmt.Errorf("capsule: secret-bearing %s cannot produce auditable execution evidence: it matches %s. The %s was not run; change that line so it no longer has this shape (for example rename the identifier, or build the value at run time)", what, computerevent.SecretRefusalDetail(found), what)
}
