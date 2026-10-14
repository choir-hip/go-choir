package yaegikernel

import (
	"path/filepath"
	"strings"
)

// BuildToolchainPathEnv names the guest variable that lists the bin
// directories of the C toolchain (gcc, binutils, pkg-config) in the guest's
// own Nix store. The autoputer service sets it; each capsule hop carries it
// to the next, and workload commands get those directories appended to
// their PATH. A capsule sees the store read-only through its lower root
// (docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option B).
const BuildToolchainPathEnv = "CHOIR_CAPSULE_BUILD_PATH"

// BuildToolchainKeys are the cgo variables that point a capsule build at the
// guest's ICU headers and libraries (cmd/autoputer links
// dolthub/go-icu-regex). Values are store paths, never credentials.
var BuildToolchainKeys = []string{"CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "PKG_CONFIG_PATH"}

// BuildToolchainCarry returns the toolchain variables one capsule hop hands
// the next (executor to broker, broker to session worker): the build path
// and the cgo keys, exactly as lookup finds them. Nothing when the build
// path is unset.
func BuildToolchainCarry(lookup func(string) (string, bool)) []string {
	buildPath, ok := lookup(BuildToolchainPathEnv)
	if !ok || strings.TrimSpace(buildPath) == "" {
		return nil
	}
	out := []string{BuildToolchainPathEnv + "=" + buildPath}
	for _, key := range BuildToolchainKeys {
		if value, ok := lookup(key); ok {
			out = append(out, key+"="+value)
		}
	}
	return out
}

// WorkloadToolchainEnv returns the PATH a workload command runs with (base,
// then the toolchain directories, so every tool that resolved before still
// resolves to the same file) and the cgo variables it gets, CGO_ENABLED=1
// included. With no build path in lookup it returns base and nothing else.
func WorkloadToolchainEnv(base string, lookup func(string) (string, bool)) (string, []string) {
	buildPath, ok := lookup(BuildToolchainPathEnv)
	if !ok {
		return base, nil
	}
	var dirs []string
	for _, entry := range strings.Split(buildPath, ":") {
		entry = strings.TrimSpace(entry)
		if !filepath.IsAbs(entry) || strings.Contains(entry, "..") {
			continue
		}
		dirs = append(dirs, filepath.Clean(entry))
	}
	if len(dirs) == 0 {
		return base, nil
	}
	path := base + ":" + strings.Join(dirs, ":")
	extra := []string{"CGO_ENABLED=1"}
	for _, key := range BuildToolchainKeys {
		if value, ok := lookup(key); ok {
			extra = append(extra, key+"="+value)
		}
	}
	return path, extra
}
