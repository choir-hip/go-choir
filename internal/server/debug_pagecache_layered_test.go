//go:build linux

package server

import (
	"path/filepath"
	"strings"
	"testing"
)

// Layered guest image (docs/design/layered-guest-image-2026-10-31.md 1.9): the
// store overlay over the app and base erofs layers is reported as layers
// app+base, not stacked. Any other multi-layer lowerdir stays stacked.
func TestParseStoreOverlayLayeredImageReportsAppAndBase(t *testing.T) {
	layers := "lowerdir=/nix/.ro-app:/nix/.ro-base"
	got, ok := parseStoreOverlay(overlayLine(41, "/nix/store", layers), "/nix/store")
	if !ok || got.Stacked || strings.Join(got.Layers, ",") != "app,base" || got.Upper != "" {
		t.Fatalf("layered store (no upper) = %+v, want layers app,base and not stacked", got)
	}
	got, _ = parseStoreOverlay(overlayLine(41, "/nix/store", layers+",upperdir=/u,workdir=/w"), "/nix/store")
	if got.Stacked || strings.Join(got.Layers, ",") != "app,base" || got.Upper != "/u" {
		t.Fatalf("layered store with the updater's upper = %+v", got)
	}
	// Wrong order, a third layer, an appended layer, or a layered store under a
	// second overlay is not attributable.
	for _, opts := range []string{
		"lowerdir=/nix/.ro-base:/nix/.ro-app",
		"lowerdir=/x:/nix/.ro-app:/nix/.ro-base",
		"lowerdir=/nix/.ro-app:/nix/.ro-base,lowerdir+=/y",
	} {
		g, _ := parseStoreOverlay(overlayLine(41, "/nix/store", opts), "/nix/store")
		if !g.Stacked {
			t.Fatalf("%s = %+v, want stacked", opts, g)
		}
	}
	two := overlayLine(41, "/nix/store", layers) + overlayLine(42, "/nix/store", "lowerdir=/nix/store,upperdir=/u,workdir=/w")
	if g, _ := parseStoreOverlay(two, "/nix/store"); !g.Stacked {
		t.Fatalf("two overlays over a layered store = %+v, want stacked", g)
	}
}

func TestPageCacheLayeredStoreWithoutUpperIsPartitioned(t *testing.T) {
	store, persistent := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(store, "aaa", "x"), 10)
	useGuestPaths(t, store, persistent, overlayLine(41, store, "lowerdir=/nix/.ro-app:/nix/.ro-base"))
	l := layeringRead(t)
	if l == nil || l.FSType != "overlay" || !l.Partitioned || strings.Join(l.Layers, ",") != "app,base" {
		t.Fatalf("store_layering = %+v, want partitioned with layers app,base", l)
	}
}
