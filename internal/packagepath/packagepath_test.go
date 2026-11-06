package packagepath

import (
	"strings"
	"testing"
)

// Failure modes this classifier guards (forced modularity X1): a package
// path spelled with a leading slash, a "./", doubled slashes or ".." escapes
// classifies as platform; a file directly in the packages directory counts
// as a package; a sibling-prefix directory ("packagesX") counts.
func TestRootOf(t *testing.T) {
	for p, want := range map[string]string{
		"frontend/src/packages/image/App.svelte":        "frontend/src/packages/image",
		"/frontend/src/packages/image/App.svelte":       "frontend/src/packages/image",
		"./frontend/src/packages/image/App.svelte":      "frontend/src/packages/image",
		"frontend//src/packages/image/a/b.js":           "frontend/src/packages/image",
		"frontend/src/lib/x/../../packages/reader/a.js": "frontend/src/packages/reader",
		"frontend/src/packages/README.md":               "",
		"frontend/src/packages":                         "",
		"frontend/src/packagesX/a/b":                    "",
		"frontend/src/lib/Desktop.svelte":               "",
		"internal/agentcore/runtime.go":                 "",
	} {
		if got := RootOf(p); got != want {
			t.Errorf("RootOf(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestClassifyMixedNamesBothLists(t *testing.T) {
	s := Classify([]string{"frontend/src/lib/Desktop.svelte", "frontend/src/packages/image/App.svelte"})
	if !s.Mixed() || s.PackageOnly() {
		t.Fatalf("mixed set not mixed: %+v", s)
	}
	d := s.MixedDetail()
	if !strings.Contains(d, "frontend/src/lib/Desktop.svelte") || !strings.Contains(d, "package image: frontend/src/packages/image/App.svelte") {
		t.Fatalf("detail = %q", d)
	}
	two := Classify([]string{"frontend/src/packages/a/x", "frontend/src/packages/b/y"})
	if !two.Mixed() {
		t.Fatal("two package roots are one operation each")
	}
	one := Classify([]string{"frontend/src/packages/a/x", "frontend/src/packages/a/y/z"})
	if one.Mixed() || !one.PackageOnly() || one.Name() != "a" {
		t.Fatalf("one package: %+v", one)
	}
	plat := Classify([]string{"internal/x.go"})
	if plat.Mixed() || plat.PackageOnly() || plat.Name() != "" {
		t.Fatalf("platform: %+v", plat)
	}
}
