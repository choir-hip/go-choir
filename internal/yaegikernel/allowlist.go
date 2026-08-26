package yaegikernel

import (
	"fmt"
	"strings"
)

// DefaultSafeStdlibPackages defines the restrictive allowlist of safe standard library
// packages permitted for model-authored Go evaluation in restricted profiles.
var DefaultSafeStdlibPackages = map[string]bool{
	"fmt":             true,
	"strings":         true,
	"bytes":           true,
	"time":            true,
	"math":            true,
	"math/big":        true,
	"math/rand/v2":    true,
	"sort":            true,
	"strconv":         true,
	"unicode":         true,
	"unicode/utf8":    true,
	"encoding/json":   true,
	"encoding/base64": true,
	"encoding/hex":    true,
	"errors":          true,
	"regexp":          true,
}

// BannedPackages defines packages that must NEVER be imported by model-authored
// code under any profile, to prevent sandbox escapes and ambient privilege inheritance.
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
func NewAllowlist(packages ...string) *Allowlist {
	m := make(map[string]bool)
	for _, p := range packages {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
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

// IsAllowed checks if an import path is permitted under the policy.
func (a *Allowlist) IsAllowed(importPath string) error {
	importPath = strings.TrimSpace(strings.Trim(importPath, `"`))
	if reason, banned := BannedPackages[importPath]; banned {
		return fmt.Errorf("import %q refused: %s", importPath, reason)
	}
	if a == nil || a.allowed == nil || !a.allowed[importPath] {
		return fmt.Errorf("import %q refused: not in activation allowlist", importPath)
	}
	return nil
}
