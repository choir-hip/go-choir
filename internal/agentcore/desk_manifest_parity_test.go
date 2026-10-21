package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The manifest's Addresses column is pinned to recordsRoute, the code that
// routes a desk's records today. recordsRoute is not yet generated from the
// manifest (named residual), so this fails if either side changes alone.
func TestManifestAddressesMatchRecordsRoute(t *testing.T) {
	systems := []string{agentprofile.Texture, agentprofile.Research, agentprofile.Management, agentprofile.Engineering}
	for _, from := range systems {
		d, ok := deskmanifest.Lookup(from)
		if !ok {
			t.Fatalf("desk %s missing from the manifest", from)
		}
		addresses := map[string]bool{}
		for _, a := range d.Addresses {
			addresses[a] = true
		}
		for _, to := range systems {
			if to == from {
				continue
			}
			_, routed, err := recordsRoute(from+":subject", "owner-1", to)
			// A route that refuses with an explanation (management must name
			// the Texture inbox) still addresses that desk.
			reaches := routed || err != nil
			if reaches != addresses[to] {
				t.Errorf("desk %s -> %s: recordsRoute reaches=%v, manifest Addresses says %v", from, to, reaches, addresses[to])
			}
		}
	}
}

// recordsDesk is the set of desks whose conversations run on records: the
// system desks the manifest gives a message target or an address.
func TestRecordsDeskMatchesManifest(t *testing.T) {
	for _, name := range []string{agentprofile.Texture, agentprofile.Research, agentprofile.Management, agentprofile.Engineering} {
		if !recordsDesk(name) {
			t.Errorf("recordsDesk(%s) is false", name)
		}
	}
	for _, name := range []string{agentprofile.Conductor, agentprofile.Email, "", "app:x"} {
		if recordsDesk(name) {
			t.Errorf("recordsDesk(%s) is true", name)
		}
	}
}
