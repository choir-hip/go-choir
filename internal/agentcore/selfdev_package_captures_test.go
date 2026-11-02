package agentcore

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// Package captures on the Freeze and listing side (docs/problems/
// selfdev-visual-capture-misses-the-changed-app-2026-11-02.md): a package the
// change touches that no capture shows is a warning at Freeze and a named gap
// in the listing; the Texture embed gets a bounded, representative subset.

func TestFreezeWarnsWhenAChangedPackageHasNoCapture(t *testing.T) {
	paths := []string{"frontend/src/packages/slides-next/App.svelte", "frontend/src/lib/TextureEditor.svelte"}
	root, visual := stagedVisualRoot(t, assetPNG(t))
	out := map[string]any{}
	recordFreezeVisual(nil, root, visual, paths, out)
	warnings, _ := out["visual_warnings"].([]string)
	if !strings.Contains(strings.Join(warnings, "\n"), "choir-capture --package slides-next") {
		t.Fatalf("no package warning: %v", out["visual_warnings"])
	}

	// No evidence at all: still warned.
	out = map[string]any{}
	recordFreezeVisual(nil, root, nil, paths, out)
	if w, _ := out["visual_warnings"].([]string); len(w) != 1 {
		t.Fatalf("no-evidence warning %v", out["visual_warnings"])
	}

	// A package capture of it: no warning.
	scene := visualcapture.Scene{Package: "slides-next", Theme: "london-salmon", Viewport: visualcapture.ViewportDesktop, State: "open-pdf", Fixture: "pdf"}
	visual.Captures[0].Scene = &scene
	visual.Captures[0].Route = scene.Route()
	if got := freezePackageGaps(paths, visual); len(got) != 0 {
		t.Fatalf("gaps %v with a capture of the package", got)
	}
	if got := freezePackageGaps([]string{"internal/x.go"}, nil); len(got) != 0 {
		t.Fatalf("gaps %v for a change outside the packages", got)
	}
}

func TestTextureEmbedKeepsRoutesThenDesktopOpenScenes(t *testing.T) {
	var caps []VisualEvidenceCapture
	caps = append(caps, VisualEvidenceCapture{Route: "/"})
	for _, theme := range visualcapture.PackageThemes {
		for _, vp := range []string{visualcapture.ViewportDesktop, visualcapture.ViewportPhone} {
			for _, st := range []string{"open-pdf", "empty"} {
				s := visualcapture.Scene{Package: "slides-next", Theme: theme, Viewport: vp, State: st}
				caps = append(caps, VisualEvidenceCapture{Route: s.Route(), Scene: &s})
			}
		}
	}
	got := textureEmbedCaptures(caps, visualcapture.MaxRoutes)
	if len(got) != visualcapture.MaxRoutes || got[0].Route != "/" {
		t.Fatalf("embed captures %d, first %q", len(got), got[0].Route)
	}
	var cells []string
	for _, c := range got[1:] {
		cells = append(cells, c.Scene.Theme+"/"+c.Scene.Viewport+"/"+c.Scene.State)
	}
	// Every desktop open scene is kept before anything else.
	for _, theme := range visualcapture.PackageThemes {
		found := false
		for _, c := range cells {
			found = found || c == theme+"/desktop/open-pdf"
		}
		if !found {
			t.Fatalf("embed dropped %s/desktop/open-pdf: %v", theme, cells)
		}
	}
	if small := textureEmbedCaptures(caps[:3], visualcapture.MaxRoutes); !reflect.DeepEqual(small, caps[:3]) {
		t.Fatal("a listing under the bound was changed")
	}
}

func TestUncapturedPackages(t *testing.T) {
	got := uncapturedPackages([]string{"image", "reader", "slides-next"}, map[string]bool{"reader": true})
	if !reflect.DeepEqual(got, []string{"image", "slides-next"}) {
		t.Fatalf("uncaptured %v", got)
	}
}
