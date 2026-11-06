package yaegikernel

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// DefaultSafeStdlibPackages is the desk standard-library set every desk's
// cells may import (plan MA6.5): internal/gomodule.Stdlib, the one list the
// package desk import set and the Go module closure check also read. It
// widened from 16 packages by expressivity under the MA6.4 sandbox
// (docs/design/ma65-yaegi-go-modules-2026-11-06.md section 2); nothing in it
// reaches the OS.
var DefaultSafeStdlibPackages = func() map[string]bool {
	m := map[string]bool{}
	for _, p := range gomodule.Stdlib() {
		m[p] = true
	}
	return m
}()

// BannedPackages defines packages that must NEVER be imported by model-authored
// code under any profile, to prevent sandbox escapes and ambient privilege
// inheritance. gomodule.BannedReason also bans whole trees (runtime/, os/,
// debug/ and the like); IsAllowed asks both.
var BannedPackages = map[string]string{
	"unsafe":    "unsafe memory access is strictly forbidden",
	"reflect":   "runtime reflection is forbidden in sandbox",
	"runtime":   "runtime introspection and manipulation is forbidden",
	"plugin":    "dynamic plugin loading is forbidden",
	"syscall":   "raw syscalls are forbidden",
	"os":        "direct OS manipulation must route through capsule broker",
	"os/exec":   "direct process execution must route through capsule broker",
	"net":       "raw network access is forbidden",
	"net/http":  "raw HTTP network access is forbidden",
	"io/ioutil": "raw filesystem I/O is forbidden",
}

// Allowlist defines the permitted package import policy for a Yaegi activation.
type Allowlist struct {
	allowed map[string]bool
}

// NewAllowlist creates an allowlist with the provided permitted package paths.
// When empty (no packages), it fails closed to the default safe stdlib set so
// an omitted or nil package list never yields an empty allowlist that rejects
// every safe package or, worse, an ambiguous one.
func NewAllowlist(packages ...string) *Allowlist {
	m := make(map[string]bool)
	for _, p := range packages {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
	}
	if len(m) == 0 {
		m = NewDefaultSafeAllowlist().allowed
	}
	return &Allowlist{allowed: m}
}

// with returns a copy of the allowlist that also admits packages (a desk's
// verified Go module imports).
func (a *Allowlist) with(packages ...string) *Allowlist {
	m := make(map[string]bool, len(a.allowed)+len(packages))
	for p := range a.allowed {
		m[p] = true
	}
	for _, p := range packages {
		m[p] = true
	}
	return &Allowlist{allowed: m}
}

// NewDefaultSafeAllowlist creates an allowlist with standard safe packages.
func NewDefaultSafeAllowlist() *Allowlist {
	m := make(map[string]bool)
	for p := range DefaultSafeStdlibPackages {
		m[p] = true
	}
	return &Allowlist{allowed: m}
}

// DefaultSafeStdlibPackagesList returns the default safe packages plus the
// prebound choir package for session workers.
func DefaultSafeStdlibPackagesList() []string {
	out := make([]string, 0, len(DefaultSafeStdlibPackages)+1)
	for p := range DefaultSafeStdlibPackages {
		out = append(out, p)
	}
	return append(out, "choir")
}

// IsAllowed checks if an import path is permitted under the policy.
func (a *Allowlist) IsAllowed(importPath string) error {
	importPath = strings.TrimSpace(strings.Trim(importPath, `"`))
	if reason, banned := BannedPackages[importPath]; banned {
		return fmt.Errorf("import %q refused: %s", importPath, reason)
	}
	if reason, banned := gomodule.BannedReason(importPath); banned {
		return fmt.Errorf("import %q refused: %s", importPath, reason)
	}
	if a == nil || a.allowed == nil || !a.allowed[importPath] {
		return fmt.Errorf("import %q refused: not in activation allowlist", importPath)
	}
	return nil
}
