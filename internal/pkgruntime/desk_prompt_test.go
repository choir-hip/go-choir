package pkgruntime

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime/runtimetest"
)

const deskOnlyManifest = `{
  "schema": "choir.package/v2",
  "name": "notes-desk",
  "version": "1.0.0",
  "display": {"title": "Notes desk", "icon": "N", "description": "A desk from another publisher"},
  "kinds": [], "uses_kinds": [],
  "contributes": {"desks": [{"name": "notes-desk", "prompt": "prompt.md"}]},
  "capabilities": [{"grant": "desk.act"}]
}
`

// A desk-only release from another publisher builds with no app: the desk's
// prompt rides as a verified data file of the install, and a missing or empty
// prompt refuses the build.
func TestBuildKeepsAThirdPartyDesksPromptAsADataFile(t *testing.T) {
	apppackage.SetPlatformSource(apppackage.ShippedPlatform)
	t.Cleanup(func() { apppackage.SetPlatformSource(nil) })

	rec, patch := runtimetest.Release(t, "owner-a", "notes-desk", map[string]string{
		"choir-package.json": deskOnlyManifest,
		"prompt.md":          "You keep the owner's notes tidy.\n",
	})
	m, err := Builder{}.Build(context.Background(), rec, patch)
	if err != nil {
		t.Fatalf("a desk-only release did not build: %v", err)
	}
	if len(m.Bytes) != 0 || len(m.Files) != 1 || m.Files[0].Path != "prompt.md" || string(m.Files[0].Bytes) != "You keep the owner's notes tidy.\n" {
		t.Fatalf("module = %+v", m)
	}
}
