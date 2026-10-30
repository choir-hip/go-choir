package yaegikernel

import (
	"path/filepath"
	"strconv"
	"strings"
)

// BuildToolchainPathEnv names the guest variable that lists the bin
// directories of the C toolchain (gcc, binutils, pkg-config) in the guest's
// own Nix store. The autoputer service sets it; each capsule hop carries it
// to the next, and workload commands get those directories appended to
// their PATH. A capsule sees the store read-only through its lower root
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option B).
const BuildToolchainPathEnv = "CHOIR_CAPSULE_BUILD_PATH"

// FrontendDepsEnv names the guest's Nix-built frontend dependencies (its
// node_modules and the package files it was built from). It rides the same
// carry as the toolchain, only as an absolute store path, so the frontend
// build job can link it instead of reaching the registry
// (docs/design/delegate-one-round-build-2026-10-21.md).
const FrontendDepsEnv = "CHOIR_FRONTEND_DEPS"

// frontendDeps returns the carried FrontendDepsEnv entry, or nothing when
// the value is not a clean absolute path strictly inside /nix/store.
func frontendDeps(lookup func(string) (string, bool)) []string {
	value, ok := lookup(FrontendDepsEnv)
	if !ok || strings.ContainsAny(value, "\x00\n\r \t") {
		return nil
	}
	if !strings.HasPrefix(value, "/nix/store/") || filepath.Clean(value) != value || strings.Contains(value, "..") {
		return nil
	}
	return []string{FrontendDepsEnv + "=" + value}
}

// CapsuleGoMaxProcsEnv and CapsuleGoMemLimitEnv carry a capsule's Go bounds
// from the executor to its workloads, which get them as GOMAXPROCS and
// GOMEMLIMIT. The capsule masks /sys, so Go cannot read the cgroup's cpu.max
// and would size GOMAXPROCS (and go build -p) to the guest's CPUs; the
// executor names the quota instead, and a GOMEMLIMIT below the cgroup limit
// (docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md).
// They are defaults: a cell's payload, or `env` in a job's argv, overrides
// them. Under their CHOIR_ names they never reach a workload.
const (
	CapsuleGoMaxProcsEnv = "CHOIR_CAPSULE_GOMAXPROCS"
	CapsuleGoMemLimitEnv = "CHOIR_CAPSULE_GOMEMLIMIT"
)

// maxCarriedGoMaxProcs is the runtime's own GOMAXPROCS ceiling
// (runtime._MaxGomaxprocs); a larger carried value is not a quota.
const maxCarriedGoMaxProcs = 1024

// MinCarriedGoMemLimit is the smallest GOMEMLIMIT a capsule workload gets:
// under it the GC would run without pause, so no bound is set at all.
const MinCarriedGoMemLimit = int64(64 << 20)

// capsuleGoBounds returns the carried bounds that are positive decimal
// integers (GOMAXPROCS at most maxCarriedGoMaxProcs, GOMEMLIMIT at least
// MinCarriedGoMemLimit), as name=value under the given names. Anything else
// is dropped: Go refuses to start on a malformed GOMEMLIMIT, and a workload
// without a bound runs as before.
func capsuleGoBounds(lookup func(string) (string, bool), procsName, memName string) []string {
	var out []string
	if v, ok := lookup(CapsuleGoMaxProcsEnv); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 && n <= maxCarriedGoMaxProcs && strconv.FormatInt(n, 10) == v {
			out = append(out, procsName+"="+v)
		}
	}
	if v, ok := lookup(CapsuleGoMemLimitEnv); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= MinCarriedGoMemLimit && strconv.FormatInt(n, 10) == v {
			out = append(out, memName+"="+v)
		}
	}
	return out
}

// BuildToolchainKeys are the cgo variables that point a capsule build at the
// guest's ICU headers and libraries (cmd/autoputer links
// dolthub/go-icu-regex). Values are store paths, never credentials.
var BuildToolchainKeys = []string{"CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "PKG_CONFIG_PATH"}

// CapsuleGoBoundsCarry returns the capsule's Go bounds, when well formed,
// for the broker to hand its session worker. The executor sets them on the
// broker from the capsule itself; the broker launch never reads them from
// the autoputer's own environment (BuildToolchainCarry does not carry them),
// so an ambient value on the guest service bounds nothing.
func CapsuleGoBoundsCarry(lookup func(string) (string, bool)) []string {
	return capsuleGoBounds(lookup, CapsuleGoMaxProcsEnv, CapsuleGoMemLimitEnv)
}

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
	return append(out, frontendDeps(lookup)...)
}

// WorkloadToolchainEnv returns the PATH a workload command runs with (base,
// then the toolchain directories, so every tool that resolved before still
// resolves to the same file) and the variables it gets: the capsule's Go
// bounds as GOMAXPROCS and GOMEMLIMIT when carried, then the cgo variables,
// CGO_ENABLED=1 included. With no build path in lookup the PATH is base and
// there is no toolchain; with no carry there is no bound.
func WorkloadToolchainEnv(base string, lookup func(string) (string, bool)) (string, []string) {
	bounds := capsuleGoBounds(lookup, "GOMAXPROCS", "GOMEMLIMIT")
	buildPath, ok := lookup(BuildToolchainPathEnv)
	if !ok {
		return base, bounds
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
		return base, bounds
	}
	path := base + ":" + strings.Join(dirs, ":")
	extra := append(bounds, "CGO_ENABLED=1")
	for _, key := range BuildToolchainKeys {
		if value, ok := lookup(key); ok {
			extra = append(extra, key+"="+value)
		}
	}
	return path, append(extra, frontendDeps(lookup)...)
}
