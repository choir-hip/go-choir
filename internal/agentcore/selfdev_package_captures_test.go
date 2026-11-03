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

func TestTextureEmbedShowsPackageScenesBeforeRoutes(t *testing.T) {
	var caps []VisualEvidenceCapture
	caps = append(caps, VisualEvidenceCapture{Route: "/"}, VisualEvidenceCapture{Route: "/read.html"})
	for _, theme := range visualcapture.PackageThemes {
		for _, vp := range []string{visualcapture.ViewportDesktop, visualcapture.ViewportPhone} {
			for _, st := range []string{"open-pdf", "empty"} {
				s := visualcapture.Scene{Package: "slides-next", Theme: theme, Viewport: vp, State: st}
				caps = append(caps, VisualEvidenceCapture{Route: s.Route(), Scene: &s})
			}
		}
	}
	got := textureEmbedCaptures(caps, visualcapture.MaxRoutes)
	if len(got) != visualcapture.MaxRoutes {
		t.Fatalf("embed captures %d", len(got))
	}
	// An app candidate's Texture document shows the app it changed, not the
	// signed-out front page: no route capture displaces a package scene.
	for _, c := range got {
		if c.Scene == nil {
			t.Fatalf("route capture %q embedded while package scenes were left out", c.Route)
		}
	}
	for _, theme := range visualcapture.PackageThemes {
		found := false
		for _, c := range got {
			found = found || (c.Scene.Theme == theme && c.Scene.Viewport == visualcapture.ViewportDesktop && c.Scene.State == "open-pdf")
		}
		if !found {
			t.Fatalf("embed dropped %s/desktop/open-pdf", theme)
		}
	}
	// Under the bound everything is kept, package scenes first.
	small := textureEmbedCaptures([]VisualEvidenceCapture{caps[0], caps[2], caps[3]}, visualcapture.MaxRoutes)
	if len(small) != 3 || small[0].Scene == nil || small[1].Scene == nil || small[2].Route != "/" {
		t.Fatalf("small listing order %+v", small)
	}
	// A candidate with no package scenes keeps its route captures as they were.
	routes := []VisualEvidenceCapture{{Route: "/"}, {Route: "/read.html"}}
	if got := textureEmbedCaptures(routes, visualcapture.MaxRoutes); !reflect.DeepEqual(got, routes) {
		t.Fatalf("route-only listing changed: %+v", got)
	}
}

func TestUncapturedPackages(t *testing.T) {
	got := uncapturedPackages([]string{"image", "reader", "slides-next"}, map[string]bool{"reader": true})
	if !reflect.DeepEqual(got, []string{"image", "slides-next"}) {
		t.Fatalf("uncaptured %v", got)
	}
}
