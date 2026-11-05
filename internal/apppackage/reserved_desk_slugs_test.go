package apppackage

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// An app desk may not take a name the shell or a built-in app owns
// (deskmanifest.reservedAppSlugs, MA2.5): every shell app name of the
// generated first-party list is reserved there.
func TestDeskManifestReservesEveryShellAndBuiltinName(t *testing.T) {
	for _, list := range [][]string{shellAppNames} {
		for _, n := range list {
			if !deskmanifest.IsReservedAppSlug(n) {
				t.Errorf("%q is a shell app name but an app desk could take it", n)
			}
		}
	}
}
