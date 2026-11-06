// Package packagepath classifies repository paths into package paths and
// platform paths (forced modularity, docs/design/forced-modularity-build-2026-11-04.md
// section 2). A package path lies under frontend/src/packages/<name>/; every
// other path is a platform path. The source stack carries only platform
// operations, and a package operation changes one package root only. The
// capsule, the host build, the source projection and corpusd all refuse with
// this one classifier, so no door spells a package path differently.
//
// It imports nothing from the repository, so every layer can use it.
package packagepath

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

// Root is the directory whose children are package roots (the same root as
// pkgrelease.PackagesRoot).
const Root = "frontend/src/packages"

// ReasonPackagePath is the reason code every door uses for a package path
// in a platform change.
const ReasonPackagePath = "package_path_refused"

// RootOf returns the package root a file path lies under
// ("frontend/src/packages/<name>"), or "" for a platform path. The path is
// cleaned first, so "./frontend/src/packages/x/a", "/frontend/src/packages/x/a"
// and "frontend//src/packages/x/a" classify the same. A file directly in Root
// is platform; only a path below a named directory counts.
func RootOf(p string) string {
	clean := path.Clean(strings.TrimLeft(strings.TrimSpace(p), "/"))
	rest, ok := strings.CutPrefix(clean, Root+"/")
	if !ok {
		return ""
	}
	name, more, found := strings.Cut(rest, "/")
	if !found || name == "" || more == "" {
		return ""
	}
	return Root + "/" + name
}

// NameOf is the package name of a path, or "".
func NameOf(p string) string {
	return strings.TrimPrefix(RootOf(p), Root+"/")
}

// Split is a change set sorted into package paths, grouped by root, and
// platform paths, each in input order. Roots lists the package roots, sorted.
type Split struct {
	Platform []string
	Package  []string
	ByRoot   map[string][]string
	Roots    []string
}

// Classify splits paths.
func Classify(paths []string) Split {
	s := Split{ByRoot: map[string][]string{}}
	for _, p := range paths {
		root := RootOf(p)
		if root == "" {
			s.Platform = append(s.Platform, p)
			continue
		}
		if _, seen := s.ByRoot[root]; !seen {
			s.Roots = append(s.Roots, root)
		}
		s.ByRoot[root] = append(s.ByRoot[root], p)
		s.Package = append(s.Package, p)
	}
	sort.Strings(s.Roots)
	return s
}

// Mixed reports a change set that touches both layers, or more than one
// package root: one operation, one layer, one package.
func (s Split) Mixed() bool {
	return (len(s.Package) > 0 && len(s.Platform) > 0) || len(s.Roots) > 1
}

// PackageOnly reports a change set entirely under one package root.
func (s Split) PackageOnly() bool {
	return len(s.Package) > 0 && len(s.Platform) == 0 && len(s.Roots) == 1
}

// Name is the one package a package-only change set changes, or "".
func (s Split) Name() string {
	if !s.PackageOnly() {
		return ""
	}
	return strings.TrimPrefix(s.Roots[0], Root+"/")
}

// MixedDetail names both lists of a mixed change set, so the engineer can
// cast one operation per layer (and one per package).
func (s Split) MixedDetail() string {
	var parts []string
	if len(s.Platform) > 0 {
		parts = append(parts, "platform paths: "+Shown(s.Platform, 8))
	}
	for _, root := range s.Roots {
		parts = append(parts, "package "+strings.TrimPrefix(root, Root+"/")+": "+Shown(s.ByRoot[root], 8))
	}
	return "one operation changes one layer: cast a separate operation for each of (" + strings.Join(parts, "; ") + ")"
}

// Shown names up to n paths, then how many more.
func Shown(paths []string, n int) string {
	shown := paths
	if len(shown) > n {
		shown = shown[:n]
	}
	out := strings.Join(shown, ", ")
	if len(paths) > len(shown) {
		out += " and " + strconv.Itoa(len(paths)-len(shown)) + " more"
	}
	return out
}
