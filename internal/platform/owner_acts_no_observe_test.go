package platform

import (
	"net/http"
	"testing"
)

// Failure list (W3): owner acts have no observe mode. CHOIR_OWNER_ACT_MODE is
// not read, so a host that still carries it set to observe (or any value, or
// unset) refuses an owner mode write that carries no verified receipt.
//  1. TestOwnerActEnvNoLongerSelectsObserve: with the variable set to observe
//     the authority corpusd builds from its environment refuses a receiptless
//     mode write (403) and does not apply it. Before the change it logged
//     "would refuse" and returned 200.
func TestOwnerActEnvNoLongerSelectsObserve(t *testing.T) {
	for _, value := range []string{"observe", "", "enforce", "off"} {
		t.Run("mode="+value, func(t *testing.T) {
			t.Setenv("CHOIR_OWNER_ACT_MODE", value)
			f := newModeAuthorityFixture(t)
			f.modes.ConfigureOwnerActs(LoadOwnerActAuthorityFromEnv())
			arm := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeApply, ExpectedGeneration: 0, IdempotencyKey: "arm-1"}
			code, _, body := f.ownerWith(t, arm, "")
			if code != http.StatusForbidden {
				t.Fatalf("a mode write with no receipt under CHOIR_OWNER_ACT_MODE=%q = %d %s, want 403", value, code, body)
			}
		})
	}
}
