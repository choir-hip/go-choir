package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// SDC ARCH-05: a desk that reads untrusted content directly (research, its
// children; Mail) never shares a worker with a privileged desk. The pool
// reuses a session's worker only when sameDeskSessionConfig says the new
// request is for the same desk; a request for another desk under the same pool
// key replaces the worker rather than reusing it.
//
// Ways this could fail, written first: two roles of different taint classes
// compare equal (a research worker is handed to management); a role the
// manifest does not know compares equal to a known one.
func TestPoolNeverSharesAWorkerAcrossTaintClasses(t *testing.T) {
	var roles []string
	for _, name := range deskmanifest.Names() {
		if d, _ := deskmanifest.Lookup(name); d.Carrier == deskmanifest.CarrierDeskWorker && d.Kind != deskmanifest.KindApp {
			roles = append(roles, name)
		}
	}
	if len(roles) < 5 {
		t.Fatalf("manifest worker roles = %v", roles)
	}
	base := yaegikernel.SessionWorkerConfig{ComputerID: "c1", AllowedRoot: "/tmp"}
	for _, a := range roles {
		for _, b := range roles {
			ca, cb := base, base
			ca.Role, cb.Role = a, b
			same := sameDeskSessionConfig(ca, cb)
			da, _ := deskmanifest.Lookup(a)
			db, _ := deskmanifest.Lookup(b)
			if same && a != b {
				t.Errorf("a %s worker would serve a %s request (taint %s and %s)", a, b, da.Taint, db.Taint)
			}
			if !same && a == b {
				t.Errorf("a %s worker is not reused for its own desk", a)
			}
		}
	}
	// An unknown role is never the same desk as a known one.
	for _, known := range roles {
		ck, cu := base, base
		ck.Role, cu.Role = known, "app:never-registered"
		if sameDeskSessionConfig(ck, cu) {
			t.Errorf("an unregistered role shares %s's worker", known)
		}
	}
}
